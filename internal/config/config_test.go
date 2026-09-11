package config

import "testing"

func TestLoadConfigReturnsMissingFileError(t *testing.T) {
	if _, err := LoadConfig("/definitely/missing/netprob-config.yaml"); err == nil {
		t.Fatal("expected an error for a missing explicit config file")
	}
}

func TestAgentGeoIPEnvironmentOverrides(t *testing.T) {
	t.Setenv("NETPROB_AGENT_REGION", "jakarta-manual")
	t.Setenv("NETPROB_AGENT_PRIMARY_ADDRESS", "100.81.110.123")
	t.Setenv("NETPROB_GEOIP_URL", "")

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Agent.Region != "jakarta-manual" {
		t.Fatalf("region = %q, want jakarta-manual", cfg.Agent.Region)
	}
	if cfg.Agent.GeoIPURL != "" {
		t.Fatalf("GeoIP URL = %q, want disabled", cfg.Agent.GeoIPURL)
	}
	if cfg.Agent.PrimaryAddress != "100.81.110.123" {
		t.Fatalf("primary address = %q", cfg.Agent.PrimaryAddress)
	}
}
