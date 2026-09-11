package storage

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"netprob/internal/models"
)

func TestStoreMirrorsEnrichedProbeAndInventoryMetrics(t *testing.T) {
	var mu sync.Mutex
	var bodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(body))
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	store := NewStore(filepath.Join(t.TempDir(), "netprob.db"), nil)
	if err := store.DB.Open(); err != nil {
		t.Fatal(err)
	}
	defer store.DB.DB().Close()
	if err := store.DB.ApplyMigrations(store.DB.DB()); err != nil {
		t.Fatal(err)
	}

	source := models.NewAgent("jkt-01", "1.2.3", []string{"192.0.2.10"}, []string{"ping", "mtr"})
	source.PrimaryAddress = "192.0.2.10"
	source.Location.Region = "Jakarta"
	source.Location.ASName = "Example Cloud"
	source.Online = true
	destination := models.NewAgent("sg-01", "1.2.3", []string{"203.0.113.20"}, []string{"ping", "mtr"})
	destination.PrimaryAddress = "203.0.113.20"
	destination.Location.Region = "Singapore"
	destination.Location.ISP = "Destination ISP"
	for _, agent := range []*models.Agent{source, destination} {
		if err := store.DB.CreateAgent(agent); err != nil {
			t.Fatal(err)
		}
		if err := store.DB.UpdateAgentMetadata(agent.ID, agent.Hostname, agent.Version, agent.PrimaryAddress, agent.Addresses, agent.Capabilities, agent.Location); err != nil {
			t.Fatal(err)
		}
	}

	link := models.NewLink("Jakarta - Singapore", "")
	if err := store.DB.CreateLink(link); err != nil {
		t.Fatal(err)
	}
	direction := models.NewDirection(link.ID, source.ID, destination.ID, destination.PrimaryAddress)
	if err := store.DB.CreateDirection(direction); err != nil {
		t.Fatal(err)
	}

	store.Metrics = NewVMMetricsStore(server.URL)
	now := time.Now().UTC()
	avg := 12.5
	loss := 0.5
	if err := store.SavePingResult(&models.PingResult{
		DirectionID: direction.ID, SourceAgentID: source.ID, DestinationAgentID: destination.ID,
		Timestamp: now, AvgRTT: &avg, PacketLoss: &loss, PacketsSent: 5, PacketsReceived: 5,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveMTRRun(&models.MTRRun{
		ID: "mtr-1", DirectionID: direction.ID, SourceAgentID: source.ID, DestinationAgentID: destination.ID,
		Timestamp: now, Status: "success", Hops: []models.MTRHop{{HopNumber: 1, LossPercent: 0, AvgMs: &avg}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.ExportInventory(now.Add(-time.Minute), "1.2.3"); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	body := strings.Join(bodies, "\n")
	mu.Unlock()
	for _, expected := range []string{
		`source="jkt-01"`,
		`destination="sg-01"`,
		`source_region="Jakarta"`,
		`source_provider="Example Cloud"`,
		`destination_provider="Destination ISP"`,
		"netprob_ping_rtt_avg_ms",
		"netprob_mtr_hop_count",
		"netprob_controller_agents_online",
		"netprob_direction_info",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("exported metrics missing %q:\n%s", expected, body)
		}
	}
}
