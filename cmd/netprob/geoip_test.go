package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLookupAgentLocation(t *testing.T) {
	fields := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fields <- r.URL.Query().Get("fields")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","query":"203.0.113.10","countryCode":"ID","regionName":"Jakarta","city":"Jakarta","timezone":"Asia/Jakarta","asname":"Example Cloud Pte Ltd","isp":"Example Cloud"}`))
	}))
	defer server.Close()

	location, err := lookupAgentLocation(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if got := <-fields; got != geoIPFields {
		t.Fatalf("fields = %q, want %q", got, geoIPFields)
	}
	if location.PublicIP != "203.0.113.10" || location.CountryCode != "ID" ||
		location.Region != "Jakarta" || location.City != "Jakarta" || location.Timezone != "Asia/Jakarta" ||
		location.ASName != "Example Cloud Pte Ltd" || location.ISP != "Example Cloud" {
		t.Fatalf("location = %#v", location)
	}
}

func TestResolveAgentLocationRegionOverrideWithoutProvider(t *testing.T) {
	location, err := resolveAgentLocation(context.Background(), "", " manual-region ")
	if err != nil {
		t.Fatal(err)
	}
	if location.Region != "manual-region" {
		t.Fatalf("region = %q, want manual-region", location.Region)
	}
}
