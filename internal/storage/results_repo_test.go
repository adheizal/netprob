package storage

import (
	"path/filepath"
	"testing"
	"time"

	"netprob/internal/models"
)

func TestMTRRunRoundTripIncludesStatusErrorAndTimestamp(t *testing.T) {
	store := &SQLiteStore{dsn: filepath.Join(t.TempDir(), "netprob.db")}
	if err := store.Open(); err != nil {
		t.Fatal(err)
	}
	defer store.DB().Close()
	if err := store.ApplyMigrations(store.DB()); err != nil {
		t.Fatal(err)
	}

	wantTime := time.Date(2026, 9, 11, 5, 40, 0, 0, time.UTC)
	want := &models.MTRRun{
		ID:                 "mtr-failure",
		DirectionID:        "direction-1",
		SourceAgentID:      "source-1",
		DestinationAgentID: "destination-1",
		Timestamp:          wantTime,
		Status:             "error",
		Error:              "mtr output was not JSON",
		Hops:               []models.MTRHop{},
	}
	if err := store.SaveMTRRun(want); err != nil {
		t.Fatal(err)
	}

	got, err := store.GetMTRRunByID(want.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != want.Status || got.Error != want.Error || !got.Timestamp.Equal(wantTime) {
		t.Fatalf("MTR run = %#v, want status/error/timestamp from %#v", got, want)
	}
}
