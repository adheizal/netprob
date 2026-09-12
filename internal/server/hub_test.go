package server

import (
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"netprob/internal/auth"
	"netprob/internal/config"
	"netprob/internal/models"
	"netprob/internal/protocol"
	"netprob/internal/storage"

	"github.com/gorilla/websocket"
)

func TestMergeAgentMetadataUsesHelloValues(t *testing.T) {
	agent := &models.Agent{
		Hostname:     "pending",
		Version:      "pending",
		Addresses:    []string{"10.0.0.1"},
		Capabilities: []string{"manual"},
		Location: models.AgentLocation{
			Region: "old-region",
		},
	}
	hello := &models.AgentHello{
		Hostname:       " jkt-01 ",
		Version:        "1.2.3",
		Addresses:      []string{"127.0.0.1", "10.10.0.20", "10.10.0.20", "fe80::1", "2001:db8::20"},
		PrimaryAddress: "10.10.0.20",
		Capabilities:   []string{"ping", "mtr", "ping", ""},
		Location: models.AgentLocation{
			PublicIP:    "203.0.113.30",
			CountryCode: "ID",
			Region:      "Jakarta",
			City:        "Jakarta",
			Timezone:    "Asia/Jakarta",
			ASName:      "Example Cloud Pte Ltd",
			ISP:         "Example Cloud",
		},
	}

	hostname, version, primaryAddress, addresses, capabilities, location := mergeAgentMetadata(agent, hello)
	if hostname != "jkt-01" || version != "1.2.3" {
		t.Fatalf("metadata = %q, %q; want jkt-01, 1.2.3", hostname, version)
	}
	if want := []string{"10.10.0.20", "2001:db8::20"}; !reflect.DeepEqual(addresses, want) {
		t.Fatalf("addresses = %#v, want %#v", addresses, want)
	}
	if primaryAddress != "10.10.0.20" {
		t.Fatalf("primary address = %q", primaryAddress)
	}
	if want := []string{"ping", "mtr"}; !reflect.DeepEqual(capabilities, want) {
		t.Fatalf("capabilities = %#v, want %#v", capabilities, want)
	}
	if location.PublicIP != "203.0.113.30" || location.CountryCode != "ID" ||
		location.Region != "Jakarta" || location.City != "Jakarta" || location.Timezone != "Asia/Jakarta" ||
		location.ASName != "Example Cloud Pte Ltd" || location.ISP != "Example Cloud" {
		t.Fatalf("location = %#v", location)
	}
}

func TestReplacementConnectionClosesPreviousWebSocket(t *testing.T) {
	store := storage.NewStore(filepath.Join(t.TempDir(), "netprob.db"), nil)
	if err := store.DB.Open(); err != nil {
		t.Fatal(err)
	}
	defer store.DB.DB().Close()
	if err := store.DB.ApplyMigrations(store.DB.DB()); err != nil {
		t.Fatal(err)
	}
	token := "replacement-test-token"
	agent := models.NewAgent("pending", "test", []string{}, []string{})
	agent.TokenHash = auth.HashToken(token)
	if err := store.DB.CreateAgent(agent); err != nil {
		t.Fatal(err)
	}

	hub := NewHub(store)
	api := NewAPIServer(store, hub, config.DefaultConfig())
	httpServer := httptest.NewServer(api.Handler())
	defer httpServer.Close()
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/ws"

	dial := func() *websocket.Conn {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatal(err)
		}
		hello := protocol.Envelope{Type: protocol.TypeHello, Payload: models.AgentHello{
			AgentID: "same-instance", Hostname: "agent", Version: "test", Token: token,
		}}
		if err := conn.WriteJSON(hello); err != nil {
			t.Fatal(err)
		}
		if _, _, err := conn.ReadMessage(); err != nil {
			t.Fatalf("read hello_ok: %v", err)
		}
		return conn
	}

	first := dial()
	defer first.Close()
	second := dial()
	if err := first.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := first.ReadMessage(); err == nil {
		t.Fatal("previous WebSocket remained open after replacement")
	}
	if !hub.IsAgentOnline(agent.ID) {
		t.Fatal("replacement connection is not online")
	}

	_ = second.Close()
	deadline := time.Now().Add(2 * time.Second)
	for hub.IsAgentOnline(agent.ID) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if hub.IsAgentOnline(agent.ID) {
		t.Fatal("agent remained online after connection close")
	}
}

func TestMergeAgentMetadataKeepsRegisteredFallback(t *testing.T) {
	agent := &models.Agent{
		Hostname:       "manual-name",
		Version:        "manual-version",
		Addresses:      []string{"agent.example.com"},
		PrimaryAddress: "10.0.0.10",
		Capabilities:   []string{"ping"},
		Location: models.AgentLocation{
			Region: "manual-region",
		},
	}

	hostname, version, primaryAddress, addresses, capabilities, location := mergeAgentMetadata(agent, &models.AgentHello{})
	if hostname != agent.Hostname || version != agent.Version ||
		primaryAddress != agent.PrimaryAddress ||
		!reflect.DeepEqual(addresses, agent.Addresses) ||
		!reflect.DeepEqual(capabilities, agent.Capabilities) ||
		!reflect.DeepEqual(location, agent.Location) {
		t.Fatalf("empty hello did not preserve registered metadata")
	}
}
