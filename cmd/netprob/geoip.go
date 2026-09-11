package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"netprob/internal/models"
)

const geoIPFields = "status,message,query,countryCode,regionName,city,timezone,asname,isp"

type geoIPResponse struct {
	Status      string `json:"status"`
	Message     string `json:"message"`
	Query       string `json:"query"`
	CountryCode string `json:"countryCode"`
	RegionName  string `json:"regionName"`
	City        string `json:"city"`
	Timezone    string `json:"timezone"`
	ASName      string `json:"asname"`
	ISP         string `json:"isp"`
}

func resolveAgentLocation(ctx context.Context, providerURL, regionOverride string) (models.AgentLocation, error) {
	location := models.AgentLocation{}
	var lookupErr error
	if strings.TrimSpace(providerURL) != "" {
		resolved, err := lookupAgentLocation(ctx, providerURL)
		if err == nil {
			location = resolved
		} else {
			lookupErr = err
		}
	}
	if region := strings.TrimSpace(regionOverride); region != "" {
		location.Region = region
	}
	return location, lookupErr
}

func lookupAgentLocation(ctx context.Context, providerURL string) (models.AgentLocation, error) {
	endpoint, err := url.Parse(providerURL)
	if err != nil {
		return models.AgentLocation{}, fmt.Errorf("parse GeoIP URL: %w", err)
	}
	if endpoint.Scheme != "http" && endpoint.Scheme != "https" {
		return models.AgentLocation{}, fmt.Errorf("unsupported GeoIP URL scheme %q", endpoint.Scheme)
	}
	query := endpoint.Query()
	query.Set("fields", geoIPFields)
	endpoint.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return models.AgentLocation{}, fmt.Errorf("create GeoIP request: %w", err)
	}
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return models.AgentLocation{}, fmt.Errorf("request GeoIP: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return models.AgentLocation{}, fmt.Errorf("GeoIP returned HTTP %d", response.StatusCode)
	}

	var payload geoIPResponse
	decoder := json.NewDecoder(io.LimitReader(response.Body, 64<<10))
	if err := decoder.Decode(&payload); err != nil {
		return models.AgentLocation{}, fmt.Errorf("decode GeoIP response: %w", err)
	}
	if payload.Status != "success" {
		return models.AgentLocation{}, fmt.Errorf("GeoIP lookup failed: %s", payload.Message)
	}
	publicIP := net.ParseIP(strings.TrimSpace(payload.Query))
	if publicIP == nil || !publicIP.IsGlobalUnicast() {
		return models.AgentLocation{}, fmt.Errorf("GeoIP returned invalid public IP")
	}

	return models.AgentLocation{
		PublicIP:    publicIP.String(),
		CountryCode: strings.TrimSpace(payload.CountryCode),
		Region:      strings.TrimSpace(payload.RegionName),
		City:        strings.TrimSpace(payload.City),
		Timezone:    strings.TrimSpace(payload.Timezone),
		ASName:      strings.TrimSpace(payload.ASName),
		ISP:         strings.TrimSpace(payload.ISP),
	}, nil
}
