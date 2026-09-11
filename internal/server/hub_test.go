package server

import (
	"reflect"
	"testing"

	"netprob/internal/models"
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
