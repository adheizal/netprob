package storage

import (
	"path/filepath"
	"testing"
	"time"

	"netprob/internal/models"
)

func TestListLinksPageClassifiesFiltersAndPaginates(t *testing.T) {
	store := &SQLiteStore{dsn: filepath.Join(t.TempDir(), "netprob.db")}
	if err := store.Open(); err != nil {
		t.Fatal(err)
	}
	defer store.DB().Close()
	if err := store.ApplyMigrations(store.DB()); err != nil {
		t.Fatal(err)
	}

	type testLink struct {
		name       string
		sourceName string
		online     bool
		loss       *float64
	}
	zero, partial, total := 0.0, 12.5, 100.0
	tests := []testLink{
		{name: "healthy-link", sourceName: "healthy-source", online: true, loss: &zero},
		{name: "degraded-link", sourceName: "degraded-source", online: true, loss: &partial},
		{name: "down-link", sourceName: "down-source", online: true, loss: &total},
		{name: "inactive-link", sourceName: "inactive-source", online: false},
	}
	agentByName := make(map[string]*models.Agent)
	for _, test := range tests {
		link := models.NewLink(test.name, "dashboard test")
		source := models.NewAgent(test.sourceName, "test", []string{"10.0.0.1"}, []string{"ping"})
		destination := models.NewAgent(test.name+"-destination", "test", []string{"10.0.0.2"}, []string{"ping"})
		for _, agent := range []*models.Agent{source, destination} {
			if err := store.CreateAgent(agent); err != nil {
				t.Fatal(err)
			}
		}
		if test.online {
			if err := store.UpdateAgentStatus(source.ID, true, time.Now()); err != nil {
				t.Fatal(err)
			}
		}
		if err := store.CreateLink(link); err != nil {
			t.Fatal(err)
		}
		direction := models.NewDirection(link.ID, source.ID, destination.ID, "10.0.0.2")
		if err := store.CreateDirection(direction); err != nil {
			t.Fatal(err)
		}
		if test.loss != nil {
			average := 15.0
			if err := store.SavePingResult(&models.PingResult{
				DirectionID: direction.ID, SourceAgentID: source.ID, DestinationAgentID: destination.ID,
				Timestamp: time.Now(), AvgRTT: &average, PacketLoss: test.loss, PacketsSent: 5,
			}); err != nil {
				t.Fatal(err)
			}
		}
		agentByName[test.sourceName] = source
	}

	items, totalCount, err := store.ListLinksPage(LinkListOptions{Limit: 2, Sort: "status"})
	if err != nil {
		t.Fatal(err)
	}
	if totalCount != 4 || len(items) != 2 || items[0].Status != "down" || items[1].Status != "degraded" {
		t.Fatalf("problem-first page = %#v, total = %d", items, totalCount)
	}

	items, totalCount, err = store.ListLinksPage(LinkListOptions{Limit: 20, Search: "healthy-source", Status: "healthy"})
	if err != nil {
		t.Fatal(err)
	}
	if totalCount != 1 || len(items) != 1 || items[0].Link.Name != "healthy-link" {
		t.Fatalf("searched healthy links = %#v, total = %d", items, totalCount)
	}

	items, totalCount, err = store.ListLinksPage(LinkListOptions{Limit: 20, AgentID: agentByName["down-source"].ID})
	if err != nil {
		t.Fatal(err)
	}
	if totalCount != 1 || len(items) != 1 || items[0].Status != "down" {
		t.Fatalf("agent-filtered links = %#v, total = %d", items, totalCount)
	}

	counts, err := store.CountLinkStatuses()
	if err != nil {
		t.Fatal(err)
	}
	if counts.Total != 4 || counts.Healthy != 1 || counts.Degraded != 1 || counts.Down != 1 || counts.Inactive != 1 {
		t.Fatalf("status counts = %#v", counts)
	}
}

func TestListAgentsPageFilteredSearchesOnlineAgents(t *testing.T) {
	store := &SQLiteStore{dsn: filepath.Join(t.TempDir(), "netprob.db")}
	if err := store.Open(); err != nil {
		t.Fatal(err)
	}
	defer store.DB().Close()
	if err := store.ApplyMigrations(store.DB()); err != nil {
		t.Fatal(err)
	}
	online := models.NewAgent("core-api-jakarta", "test", []string{"10.0.0.1"}, []string{"ping"})
	offline := models.NewAgent("core-api-singapore", "test", []string{"10.0.0.2"}, []string{"ping"})
	for _, agent := range []*models.Agent{online, offline} {
		if err := store.CreateAgent(agent); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.UpdateAgentStatus(online.ID, true, time.Now()); err != nil {
		t.Fatal(err)
	}

	agents, err := store.ListAgentsPageFiltered(20, 0, "jakarta", "online")
	if err != nil {
		t.Fatal(err)
	}
	count, err := store.CountAgentsFiltered("jakarta", "online")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || len(agents) != 1 || agents[0].ID != online.ID {
		t.Fatalf("filtered agents = %#v, count = %d", agents, count)
	}
}
