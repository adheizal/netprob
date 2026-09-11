package storage

import (
	"path/filepath"
	"testing"

	"netprob/internal/models"
)

func TestListLinksReturnsCreatedLinks(t *testing.T) {
	store := &SQLiteStore{dsn: filepath.Join(t.TempDir(), "netprob.db")}
	if err := store.Open(); err != nil {
		t.Fatal(err)
	}
	defer store.DB().Close()
	if err := store.ApplyMigrations(store.DB()); err != nil {
		t.Fatal(err)
	}

	want := models.NewLink("Jakarta to Singapore", "test link")
	if err := store.CreateLink(want); err != nil {
		t.Fatal(err)
	}

	links, err := store.ListLinks()
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 || links[0].ID != want.ID {
		t.Fatalf("ListLinks() = %#v, want link %q", links, want.ID)
	}
}

func TestUpdateDirectionProbeSettings(t *testing.T) {
	store := &SQLiteStore{dsn: filepath.Join(t.TempDir(), "netprob.db")}
	if err := store.Open(); err != nil {
		t.Fatal(err)
	}
	defer store.DB().Close()
	if err := store.ApplyMigrations(store.DB()); err != nil {
		t.Fatal(err)
	}

	link := models.NewLink("Jakarta to Singapore", "test link")
	source := models.NewAgent("jkt-01", "dev", []string{"10.0.0.1"}, []string{"ping"})
	destination := models.NewAgent("sg-01", "dev", []string{"10.0.0.2"}, []string{"ping"})
	if err := store.CreateLink(link); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateAgent(source); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateAgent(destination); err != nil {
		t.Fatal(err)
	}

	direction := models.NewDirection(link.ID, source.ID, destination.ID, "10.0.0.2")
	if err := store.CreateDirection(direction); err != nil {
		t.Fatal(err)
	}
	direction.PingInterval = 30
	direction.MTRInterval = 300
	direction.PingEnabled = false
	direction.MTREnabled = true
	if err := store.UpdateDirection(direction); err != nil {
		t.Fatal(err)
	}

	got, err := store.GetDirectionByID(direction.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.PingInterval != 30 || got.MTRInterval != 300 || got.PingEnabled || !got.MTREnabled {
		t.Fatalf("updated direction = %#v", got)
	}
}

func TestDeleteLinkRemovesDirectionsAndProbeHistory(t *testing.T) {
	store := &SQLiteStore{dsn: filepath.Join(t.TempDir(), "netprob.db")}
	if err := store.Open(); err != nil {
		t.Fatal(err)
	}
	defer store.DB().Close()
	if err := store.ApplyMigrations(store.DB()); err != nil {
		t.Fatal(err)
	}

	link := models.NewLink("delete me", "")
	source := models.NewAgent("source", "dev", []string{"10.0.0.1"}, []string{"ping"})
	destination := models.NewAgent("destination", "dev", []string{"10.0.0.2"}, []string{"ping"})
	if err := store.CreateLink(link); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateAgent(source); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateAgent(destination); err != nil {
		t.Fatal(err)
	}
	direction := models.NewDirection(link.ID, source.ID, destination.ID, "10.0.0.2")
	if err := store.CreateDirection(direction); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().Exec(`INSERT INTO ping_results
		(direction_id, source_agent_id, destination_agent_id, packets_sent, packets_received)
		VALUES (?, ?, ?, 5, 5)`, direction.ID, source.ID, destination.ID); err != nil {
		t.Fatal(err)
	}

	if err := store.DeleteLink(link.ID); err != nil {
		t.Fatal(err)
	}
	for table, query := range map[string]string{
		"links":        `SELECT COUNT(*) FROM links WHERE id = ?`,
		"directions":   `SELECT COUNT(*) FROM directions WHERE link_id = ?`,
		"ping_results": `SELECT COUNT(*) FROM ping_results WHERE direction_id = ?`,
	} {
		var count int
		argument := link.ID
		if table == "ping_results" {
			argument = direction.ID
		}
		if err := store.DB().QueryRow(query, argument).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("%s still has %d rows after deleting link", table, count)
		}
	}
}
