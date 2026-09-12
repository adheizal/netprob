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
	seedProbeDirection(t, store, "direction-1", "source-1", "destination-1")

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
	runs, err := store.ListMTRRuns(want.DirectionID, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || runs[0].ID != want.ID || len(runs[0].Hops) != 0 {
		t.Fatalf("listed MTR runs = %#v", runs)
	}
}

func TestListLatestPingResultsReturnsOneResultPerDirection(t *testing.T) {
	store := &SQLiteStore{dsn: filepath.Join(t.TempDir(), "netprob.db")}
	if err := store.Open(); err != nil {
		t.Fatal(err)
	}
	defer store.DB().Close()
	if err := store.ApplyMigrations(store.DB()); err != nil {
		t.Fatal(err)
	}
	seedProbeDirection(t, store, "direction-latest", "source-latest", "destination-latest")
	for i, timestamp := range []time.Time{time.Now().Add(-time.Minute), time.Now()} {
		average := float64(i + 1)
		if err := store.SavePingResult(&models.PingResult{
			DirectionID: "direction-latest", SourceAgentID: "source-latest", DestinationAgentID: "destination-latest",
			Timestamp: timestamp, AvgRTT: &average, PacketsSent: 5, PacketsReceived: 5,
		}); err != nil {
			t.Fatal(err)
		}
	}
	results, err := store.ListLatestPingResults()
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results["direction-latest"].AvgRTT == nil || *results["direction-latest"].AvgRTT != 2 {
		t.Fatalf("latest results = %#v", results)
	}
}
