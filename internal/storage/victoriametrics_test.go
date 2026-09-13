package storage

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"netprob/internal/models"
)

func TestVMMetricsStoreWritesPrometheusImportFormat(t *testing.T) {
	var requestPath, requestBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		requestBody = string(body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	avg := 12.5
	result := &models.PingResult{
		DirectionID:        "direction-1",
		SourceAgentID:      "agent-a",
		DestinationAgentID: "agent-b",
		Timestamp:          time.UnixMilli(1_700_000_000_123),
		AvgRTT:             &avg,
		PacketsSent:        5,
		PacketsReceived:    5,
	}

	context := MetricContext{
		LinkID:              "link-1",
		LinkName:            "Jakarta - Singapore",
		DirectionID:         "direction-1",
		TargetAddress:       "203.0.113.20",
		SourceAgentID:       "agent-a",
		SourceHostname:      "jkt-01",
		SourceAddress:       "192.0.2.10",
		SourceRegion:        "Jakarta",
		SourceProvider:      "Example Cloud",
		DestinationAgentID:  "agent-b",
		DestinationHostname: "sg-01",
		DestinationAddress:  "203.0.113.20",
		DestinationRegion:   "Singapore",
		DestinationProvider: "Other Cloud",
	}

	if err := NewVMMetricsStore(server.URL).WritePing(result, context); err != nil {
		t.Fatal(err)
	}
	if requestPath != "/api/v1/import/prometheus" {
		t.Fatalf("request path = %q", requestPath)
	}
	if !strings.Contains(requestBody, `netprob_ping_rtt_avg_ms{`) ||
		!strings.Contains(requestBody, `source="jkt-01"`) ||
		!strings.Contains(requestBody, `destination="sg-01"`) ||
		!strings.Contains(requestBody, `source_agent_id="agent-a"`) ||
		!strings.Contains(requestBody, `source_provider="Example Cloud"`) ||
		!strings.Contains(requestBody, `} 12.5 1700000000123`) {
		t.Fatalf("unexpected request body: %s", requestBody)
	}
	if !strings.Contains(requestBody, `netprob_probe_success{`) || !strings.Contains(requestBody, `probe="ping"`) {
		t.Fatalf("ping status metric missing: %s", requestBody)
	}
}

func TestVMMetricsStoreBuildsMTRSummaryAndRoute(t *testing.T) {
	avg := 9.75
	run := &models.MTRRun{
		Timestamp: time.UnixMilli(1_700_000_000_123),
		Status:    "success",
		Hops: []models.MTRHop{
			{HopNumber: 1, Host: "gateway", IP: "192.0.2.1", LossPercent: 0},
			{HopNumber: 2, Host: "destination.example", IP: "203.0.113.20", LossPercent: 2.5, AvgMs: &avg},
		},
	}
	lines := NewVMMetricsStore("http://unused").mtrToPrometheusLines(run, MetricContext{
		SourceHostname:      "jkt-01",
		DestinationHostname: "sg-01",
	})
	body := strings.Join(lines, "\n")
	for _, expected := range []string{
		`netprob_probe_success{`,
		`probe="mtr"`,
		`netprob_mtr_hop_count{`,
		`} 2 1700000000123`,
		`netprob_mtr_max_hop_loss_percent{`,
		`} 2.5 1700000000123`,
		`netprob_mtr_destination_rtt_avg_ms{`,
		`} 9.75 1700000000123`,
		`netprob_mtr_hop_observed_timestamp_seconds{`,
		`hop_number="1"`,
		`hop_host="gateway"`,
		`hop_ip="192.0.2.1"`,
		`} 1700000000.123 1700000000123`,
		`hop_number="2"`,
		`hop_host="destination.example"`,
		`hop_ip="203.0.113.20"`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("MTR summary missing %q: %s", expected, body)
		}
	}
}

func TestVMMetricsStoreBuildsInventoryMetrics(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	lastSeen := now.Add(-10 * time.Second)
	lines := NewVMMetricsStore("http://unused").inventoryToPrometheusLines(InventorySnapshot{
		Timestamp:         now,
		StartedAt:         now.Add(-time.Hour),
		ControllerVersion: "1.2.3",
		Agents: []*models.Agent{{
			ID: "agent-a", Hostname: "jkt-01", PrimaryAddress: "192.0.2.10", Version: "1.2.3", Online: true, LastSeen: &lastSeen,
			Location: models.AgentLocation{Region: "Jakarta", ASName: "Example Cloud"},
		}},
		Links: []*models.Link{{ID: "link-1", Name: "JKT - SG"}},
		Directions: []*models.Direction{{
			ID: "direction-1", LinkID: "link-1", SourceAgentID: "agent-a", DestinationAgentID: "agent-b",
			PingEnabled: true, MTREnabled: true, PingInterval: 5, MTRInterval: 120,
		}},
	})
	body := strings.Join(lines, "\n")
	for _, metric := range []string{
		"netprob_controller_info", "netprob_controller_uptime_seconds", "netprob_controller_agents_online",
		"netprob_agent_online", "netprob_agent_last_seen_seconds", "netprob_link_info", "netprob_direction_info",
	} {
		if !strings.Contains(body, metric) {
			t.Fatalf("inventory metric %q missing: %s", metric, body)
		}
	}
}
