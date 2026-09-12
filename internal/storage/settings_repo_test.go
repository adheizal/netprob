package storage

import (
	"path/filepath"
	"testing"
	"time"

	"netprob/internal/models"
)

func TestRetentionSettingsAndCleanup(t *testing.T) {
	store := &SQLiteStore{dsn: filepath.Join(t.TempDir(), "netprob.db")}
	if err := store.Open(); err != nil {
		t.Fatal(err)
	}
	defer store.DB().Close()
	if err := store.ApplyMigrations(store.DB()); err != nil {
		t.Fatal(err)
	}
	seedProbeDirection(t, store, "direction", "source", "destination")

	settings, err := store.GetRetentionSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.PingRetentionDays != 0 || settings.MTRRetentionDays != 0 {
		t.Fatalf("default retention settings = %#v, want both zero", settings)
	}

	settings.PingRetentionDays = 7
	settings.MTRRetentionDays = 30
	if err := store.UpdateRetentionSettings(settings); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetRetentionSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.PingRetentionDays != 7 || got.MTRRetentionDays != 30 {
		t.Fatalf("updated retention settings = %#v", got)
	}

	now := time.Now().UTC()
	for _, timestamp := range []time.Time{now.AddDate(0, 0, -31), now.AddDate(0, 0, -1)} {
		if _, err := store.DB().Exec(`
			INSERT INTO ping_results (direction_id, source_agent_id, destination_agent_id, timestamp)
			VALUES ('direction', 'source', 'destination', ?)
		`, timestamp); err != nil {
			t.Fatal(err)
		}
	}
	for _, run := range []struct {
		id        string
		timestamp time.Time
	}{{"old", now.AddDate(0, 0, -31)}, {"new", now.AddDate(0, 0, -1)}} {
		if _, err := store.DB().Exec(`
			INSERT INTO mtr_runs (id, direction_id, source_agent_id, destination_agent_id, timestamp)
			VALUES (?, 'direction', 'source', 'destination', ?)
		`, run.id, run.timestamp); err != nil {
			t.Fatal(err)
		}
		if _, err := store.DB().Exec(`INSERT INTO mtr_hops (mtr_run_id, hop_number) VALUES (?, 1)`, run.id); err != nil {
			t.Fatal(err)
		}
	}

	cleanup, err := store.CleanupExpiredProbeHistory(settings, now)
	if err != nil {
		t.Fatal(err)
	}
	if cleanup.PingResultsDeleted != 1 || cleanup.MTRRunsDeleted != 1 {
		t.Fatalf("cleanup result = %#v, want one deletion of each type", cleanup)
	}
	for table, want := range map[string]int{"ping_results": 1, "mtr_runs": 1, "mtr_hops": 1} {
		var count int
		if err := store.DB().QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != want {
			t.Fatalf("%s row count = %d, want %d", table, count, want)
		}
	}

	keepForever := &models.RetentionSettings{}
	cleanup, err = store.CleanupExpiredProbeHistory(keepForever, now.AddDate(10, 0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if cleanup.PingResultsDeleted != 0 || cleanup.MTRRunsDeleted != 0 {
		t.Fatalf("keep-forever cleanup unexpectedly deleted data: %#v", cleanup)
	}
}
