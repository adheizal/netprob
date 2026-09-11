package storage

import (
	"time"

	"netprob/internal/models"
)

// Store is the high-level storage facade combining config and metrics storage.
type Store struct {
	DB      *SQLiteStore
	Metrics MetricsStore
}

// MetricsStore is the interface for external time-series probe and inventory storage.
type MetricsStore interface {
	WritePing(result *models.PingResult, context MetricContext) error
	WriteMTR(run *models.MTRRun, context MetricContext) error
	WriteProbeStatus(probe string, success bool, timestamp time.Time, context MetricContext) error
	WriteInventory(snapshot InventorySnapshot) error
	QueryPing(sourceAgentID, destAgentID string, from, to time.Time, limit int) ([]*models.PingResult, error)
}

// NewStore creates a new Store with SQLite for config and the given metrics backend.
func NewStore(dsn string, metrics MetricsStore) *Store {
	return &Store{
		DB:      &SQLiteStore{dsn: dsn},
		Metrics: metrics,
	}
}

func (s *Store) DSN() string {
	return s.DB.dsn
}

// SavePingResult always keeps the local SQLite copy used by the API and, when
// configured, mirrors the sample to the external metrics backend.
func (s *Store) SavePingResult(result *models.PingResult) error {
	if err := s.DB.SavePingResult(result); err != nil {
		return err
	}
	if s.Metrics == nil {
		return nil
	}
	if _, isSQLite := s.Metrics.(*SQLiteMetricsStore); isSQLite {
		return nil
	}
	context := s.metricContext(result.DirectionID, result.SourceAgentID, result.DestinationAgentID)
	return s.Metrics.WritePing(result, context)
}

// SaveMTRRun keeps the full run and its hops in SQLite, then mirrors a bounded
// summary to the external metrics backend when configured.
func (s *Store) SaveMTRRun(run *models.MTRRun) error {
	if err := s.DB.SaveMTRRun(run); err != nil {
		return err
	}
	if s.Metrics == nil {
		return nil
	}
	if _, isSQLite := s.Metrics.(*SQLiteMetricsStore); isSQLite {
		return nil
	}
	context := s.metricContext(run.DirectionID, run.SourceAgentID, run.DestinationAgentID)
	return s.Metrics.WriteMTR(run, context)
}

// WriteProbeStatus records probe failures that do not produce a result model.
func (s *Store) WriteProbeStatus(probe string, success bool, timestamp time.Time, directionID, sourceAgentID, destinationAgentID string) error {
	if s.Metrics == nil {
		return nil
	}
	if _, isSQLite := s.Metrics.(*SQLiteMetricsStore); isSQLite {
		return nil
	}
	context := s.metricContext(directionID, sourceAgentID, destinationAgentID)
	return s.Metrics.WriteProbeStatus(probe, success, timestamp, context)
}

// ExportInventory mirrors a point-in-time controller snapshot when an external
// time-series backend is enabled.
func (s *Store) ExportInventory(startedAt time.Time, controllerVersion string) error {
	if s.Metrics == nil {
		return nil
	}
	if _, isSQLite := s.Metrics.(*SQLiteMetricsStore); isSQLite {
		return nil
	}
	agents, err := s.DB.ListAgents()
	if err != nil {
		return err
	}
	links, err := s.DB.ListLinks()
	if err != nil {
		return err
	}
	directions, err := s.DB.ListDirections()
	if err != nil {
		return err
	}
	return s.Metrics.WriteInventory(InventorySnapshot{
		Timestamp:         time.Now().UTC(),
		StartedAt:         startedAt,
		ControllerVersion: controllerVersion,
		Agents:            agents,
		Links:             links,
		Directions:        directions,
	})
}
