package storage

import (
	"path/filepath"
	"testing"

	"netprob/internal/models"
)

func TestAgentMetadataIncludesLocation(t *testing.T) {
	store := &SQLiteStore{dsn: filepath.Join(t.TempDir(), "netprob.db")}
	if err := store.Open(); err != nil {
		t.Fatal(err)
	}
	defer store.DB().Close()
	if err := store.ApplyMigrations(store.DB()); err != nil {
		t.Fatal(err)
	}

	agent := models.NewAgent("pending", "pending", []string{}, []string{})
	if err := store.CreateAgent(agent); err != nil {
		t.Fatal(err)
	}
	location := models.AgentLocation{
		PublicIP:    "203.0.113.40",
		CountryCode: "ID",
		Region:      "Jakarta",
		City:        "Jakarta",
		Timezone:    "Asia/Jakarta",
		ASName:      "Example Cloud Pte Ltd",
		ISP:         "Example Cloud",
	}
	if err := store.UpdateAgentMetadata(agent.ID, "jkt-01", "1.0.0", "10.0.0.10", []string{"10.0.0.10"}, []string{"ping", "mtr"}, location); err != nil {
		t.Fatal(err)
	}

	got, err := store.GetAgentByID(agent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Hostname != "jkt-01" || got.PrimaryAddress != "10.0.0.10" || got.Location != location {
		t.Fatalf("agent = %#v, want hostname and location %#v", got, location)
	}
}

func TestAgentLocationMigrationFromVersionOne(t *testing.T) {
	store := &SQLiteStore{dsn: filepath.Join(t.TempDir(), "netprob-v1.db")}
	if err := store.Open(); err != nil {
		t.Fatal(err)
	}
	defer store.DB().Close()
	if _, err := store.DB().Exec(`CREATE TABLE schema_migrations (version INTEGER, applied_at DATETIME DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().Exec(initialSchema); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().Exec(`INSERT INTO schema_migrations (version) VALUES (1)`); err != nil {
		t.Fatal(err)
	}
	if err := store.ApplyMigrations(store.DB()); err != nil {
		t.Fatal(err)
	}

	agent := models.NewAgent("pending", "pending", []string{}, []string{})
	if err := store.CreateAgent(agent); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetAgentByID(agent.ID); err != nil {
		t.Fatalf("read agent after v1 migration: %v", err)
	}
}

func TestReusableEnrollmentTokenCreatesDistinctAgentInstances(t *testing.T) {
	store := &SQLiteStore{dsn: filepath.Join(t.TempDir(), "netprob.db")}
	if err := store.Open(); err != nil {
		t.Fatal(err)
	}
	defer store.DB().Close()
	if err := store.ApplyMigrations(store.DB()); err != nil {
		t.Fatal(err)
	}

	registered := models.NewAgent("pending", "pending", []string{}, []string{})
	registered.TokenHash = "shared-token-hash"
	if err := store.CreateAgent(registered); err != nil {
		t.Fatal(err)
	}

	first, err := store.ResolveAgentByTokenAndInstance(registered.TokenHash, "jkt-01")
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != registered.ID {
		t.Fatalf("first instance id = %q, want registration id %q", first.ID, registered.ID)
	}

	second, err := store.ResolveAgentByTokenAndInstance(registered.TokenHash, "sg-01")
	if err != nil {
		t.Fatal(err)
	}
	if second.ID == first.ID {
		t.Fatal("different instances resolved to the same agent")
	}

	reconnected, err := store.ResolveAgentByTokenAndInstance(registered.TokenHash, "jkt-01")
	if err != nil {
		t.Fatal(err)
	}
	if reconnected.ID != first.ID {
		t.Fatalf("reconnect id = %q, want %q", reconnected.ID, first.ID)
	}

	agents, err := store.ListAgents()
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 2 {
		t.Fatalf("agent count = %d, want 2", len(agents))
	}
}

func TestListAgentsPage(t *testing.T) {
	store := &SQLiteStore{dsn: filepath.Join(t.TempDir(), "netprob.db")}
	if err := store.Open(); err != nil {
		t.Fatal(err)
	}
	defer store.DB().Close()
	if err := store.ApplyMigrations(store.DB()); err != nil {
		t.Fatal(err)
	}

	for _, hostname := range []string{"agent-a", "agent-b", "agent-c"} {
		agent := models.NewAgent(hostname, "test", []string{}, []string{})
		if err := store.CreateAgent(agent); err != nil {
			t.Fatal(err)
		}
	}

	total, err := store.CountAgents()
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 {
		t.Fatalf("agent count = %d, want 3", total)
	}

	first, err := store.ListAgentsPage(2, 0)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.ListAgentsPage(2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 || len(second) != 1 {
		t.Fatalf("page lengths = %d and %d, want 2 and 1", len(first), len(second))
	}
	if first[0].ID == second[0].ID || first[1].ID == second[0].ID {
		t.Fatal("agent appeared on more than one page")
	}
}

func TestDeleteAgentRemovesDirectionsProbeHistoryAndEmptyLink(t *testing.T) {
	store := &SQLiteStore{dsn: filepath.Join(t.TempDir(), "netprob.db")}
	if err := store.Open(); err != nil {
		t.Fatal(err)
	}
	defer store.DB().Close()
	if err := store.ApplyMigrations(store.DB()); err != nil {
		t.Fatal(err)
	}
	direction := seedProbeDirection(t, store, "direction-delete", "source-delete", "destination-keep")
	if _, err := store.DB().Exec(`INSERT INTO ping_results
		(direction_id, source_agent_id, destination_agent_id, packets_sent, packets_received)
		VALUES (?, ?, ?, 5, 0)`, direction.ID, direction.SourceAgentID, direction.DestinationAgentID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().Exec(`INSERT INTO mtr_runs
		(id, direction_id, source_agent_id, destination_agent_id, status, error)
		VALUES ('mtr-delete', ?, ?, ?, 'success', '')`, direction.ID, direction.SourceAgentID, direction.DestinationAgentID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().Exec(`INSERT INTO mtr_hops (mtr_run_id, hop_number) VALUES ('mtr-delete', 1)`); err != nil {
		t.Fatal(err)
	}

	if err := store.DeleteAgent(direction.SourceAgentID); err != nil {
		t.Fatal(err)
	}
	for table, want := range map[string]int{
		"agents": 1, "links": 0, "directions": 0, "ping_results": 0, "mtr_runs": 0, "mtr_hops": 0,
	} {
		var count int
		if err := store.DB().QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != want {
			t.Fatalf("%s count = %d, want %d", table, count, want)
		}
	}
}
