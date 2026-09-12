package scheduler

import (
	"path/filepath"
	"sync"
	"testing"

	"netprob/internal/models"
	"netprob/internal/server"
	"netprob/internal/storage"
)

type concurrentRefreshStore struct {
	direction *models.Direction
}

func (s *concurrentRefreshStore) ListDirectionsForScheduling() ([]*models.Direction, error) {
	return []*models.Direction{s.direction}, nil
}

func (s *concurrentRefreshStore) GetDirectionByID(string) (*models.Direction, error) {
	return s.direction, nil
}

func TestRefreshAndScheduleAreConcurrentSafe(t *testing.T) {
	store := storage.NewStore(filepath.Join(t.TempDir(), "netprob.db"), nil)
	if err := store.DB.Open(); err != nil {
		t.Fatal(err)
	}
	defer store.DB.DB().Close()
	if err := store.DB.ApplyMigrations(store.DB.DB()); err != nil {
		t.Fatal(err)
	}
	agent := models.NewAgent("source", "test", []string{"192.0.2.1"}, []string{"ping"})
	if err := store.DB.CreateAgent(agent); err != nil {
		t.Fatal(err)
	}
	hub := server.NewHub(store)
	hub.SetAgentConnection(agent, &server.AgentConn{Send: make(chan []byte, 1000), Stop: make(chan struct{}), Done: make(chan struct{})})

	direction := models.NewDirection("link", agent.ID, "destination", "192.0.2.2")
	direction.MTREnabled = false
	scheduler := NewScheduler(hub, &concurrentRefreshStore{direction: direction})
	defer scheduler.ticker.Stop()
	scheduler.refreshDirections()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			scheduler.refreshDirections()
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			scheduler.checkSchedules()
		}
	}()
	wg.Wait()
}
