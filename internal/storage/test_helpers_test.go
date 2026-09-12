package storage

import (
	"testing"

	"netprob/internal/models"
)

func seedProbeDirection(t *testing.T, store *SQLiteStore, directionID, sourceID, destinationID string) *models.Direction {
	t.Helper()
	source := models.NewAgent("source", "test", []string{"192.0.2.1"}, []string{"ping", "mtr"})
	source.ID = sourceID
	destination := models.NewAgent("destination", "test", []string{"192.0.2.2"}, []string{"ping", "mtr"})
	destination.ID = destinationID
	if err := store.CreateAgent(source); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateAgent(destination); err != nil {
		t.Fatal(err)
	}
	link := models.NewLink("test link", "")
	if err := store.CreateLink(link); err != nil {
		t.Fatal(err)
	}
	direction := models.NewDirection(link.ID, sourceID, destinationID, destination.Addresses[0])
	direction.ID = directionID
	if err := store.CreateDirection(direction); err != nil {
		t.Fatal(err)
	}
	return direction
}
