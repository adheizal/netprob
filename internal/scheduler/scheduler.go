package scheduler

import (
	"context"
	"sync"
	"time"

	"netprob/internal/models"
	"netprob/internal/server"

	"github.com/google/uuid"

	"github.com/rs/zerolog/log"
)

// DirectionScheduler tracks scheduling state for a single direction.
type DirectionScheduler struct {
	Direction *models.Direction
	lastPing  time.Time
	lastMTR   time.Time
}

// Scheduler dispatches ping and MTR jobs to agents based on configured intervals.
type Scheduler struct {
	hub        *server.Hub
	store      StoreReader
	ticker     *time.Ticker
	stopCh     chan struct{}
	directions map[string]*DirectionScheduler
	mu         sync.RWMutex
}

// StoreReader provides what the scheduler needs from storage.
type StoreReader interface {
	ListDirectionsForScheduling() ([]*models.Direction, error)
	GetDirectionByID(id string) (*models.Direction, error)
}

// NewScheduler creates a new scheduler.
func NewScheduler(hub *server.Hub, store StoreReader) *Scheduler {
	return &Scheduler{
		hub:        hub,
		store:      store,
		ticker:     time.NewTicker(1 * time.Second),
		stopCh:     make(chan struct{}),
		directions: make(map[string]*DirectionScheduler),
	}
}

// Start begins the scheduling loop.
func (s *Scheduler) Start(ctx context.Context) {
	// Initial load of directions
	s.refreshDirections()

	// Start periodic refresh
	go s.refreshLoop(ctx)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.stopCh:
				return
			case <-s.ticker.C:
				s.checkSchedules()
			}
		}
	}()

	log.Info().Msg("scheduler started")
}

func (s *Scheduler) Stop() {
	s.ticker.Stop()
	close(s.stopCh)
}

// refreshDirections reloads the config from DB periodically
func (s *Scheduler) refreshDirections() {
	dirs, err := s.store.ListDirectionsForScheduling()
	if err != nil {
		log.Warn().Err(err).Msg("failed to load directions for scheduling")
		return
	}

	newDirs := make(map[string]*DirectionScheduler)
	for _, d := range dirs {
		ds := &DirectionScheduler{Direction: d}
		if existing, ok := s.directions[d.ID]; ok {
			ds.lastPing = existing.lastPing
			ds.lastMTR = existing.lastMTR
		}
		newDirs[d.ID] = ds
	}

	s.mu.Lock()
	s.directions = newDirs
	s.mu.Unlock()
}

// refreshLoop periodically reloads directions from the DB
func (s *Scheduler) refreshLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.refreshDirections()
		}
	}
}

// checkSchedules checks if any jobs need to be dispatched
func (s *Scheduler) checkSchedules() {
	s.mu.RLock()
	dirs := make([]*models.Direction, 0, len(s.directions))
	for _, ds := range s.directions {
		dirs = append(dirs, ds.Direction)
	}
	s.mu.RUnlock()

	now := time.Now()

	for _, d := range dirs {
		if !s.hub.IsAgentOnline(d.SourceAgentID) {
			continue
		}

		s.mu.RLock()
		ds := s.directions[d.ID]
		s.mu.RUnlock()

		if ds == nil {
			continue
		}

		// Check if ping is due
		if d.PingEnabled && now.Sub(ds.lastPing) >= time.Duration(d.PingInterval)*time.Second {
			s.dispatchJob(d, models.ProbePing)
			ds.lastPing = now
		}

		// Check if MTR is due
		if d.MTREnabled && now.Sub(ds.lastMTR) >= time.Duration(d.MTRInterval)*time.Second {
			s.dispatchJob(d, models.ProbeMTR)
			ds.lastMTR = now
		}
	}
}

func (s *Scheduler) dispatchJob(d *models.Direction, probeType string) {
	job := &models.Job{
		ID:          uuid.NewString(),
		Type:        probeType,
		Direction:   models.JobDirection{
			SourceAgentID:      d.SourceAgentID,
			DestinationAgentID: d.DestinationAgentID,
			TargetAddress:      d.TargetAddress,
			LinkID:             d.LinkID,
			DirectionID:        d.ID,
		},
		ScheduledAt: time.Now().Format(time.RFC3339),
		Config:      map[string]any{},
	}

	if probeType == models.ProbePing {
		job.Config["count"] = 5
		job.Config["interval"] = 1
	} else {
		job.Config["target"] = d.TargetAddress
	}

	err := s.hub.DispatchJob(d.SourceAgentID, job)
	if err != nil {
		log.Warn().
			Err(err).
			Str("agent_id", d.SourceAgentID).
			Str("probe_type", probeType).
			Str("job_id", job.ID).
			Msg("failed to dispatch job")
		return
	}

	log.Debug().
		Str("agent_id", d.SourceAgentID).
		Str("probe_type", probeType).
		Str("dest", d.DestinationAgentID).
		Str("job_id", job.ID).
		Msg("job dispatched")
}
