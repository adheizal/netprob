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
