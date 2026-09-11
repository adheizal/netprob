package server

import (
	"time"

	"netprob/internal/models"
)

type registerAgentRequest struct {
	Hostname     string   `json:"hostname"`
	Version      string   `json:"version"`
	Addresses    []string `json:"addresses"`
	Capabilities []string `json:"capabilities"`
}

type registerAgentResponse struct {
	ID    string `json:"id"`
	Token string `json:"token"`
}

type createLinkRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type createDirectionRequest struct {
	SourceAgentID      string   `json:"source_agent_id"`
	DestinationAgentID string   `json:"destination_agent_id"`
	TargetAddress      string   `json:"target_address"`
	PingInterval       *int     `json:"ping_interval_seconds"`
	MTRInterval        *int     `json:"mtr_interval_seconds"`
	PingEnabled        *bool    `json:"ping_enabled"`
	MTREnabled         *bool    `json:"mtr_enabled"`
	MTRThresholdLoss   *float64 `json:"mtr_threshold_loss_percent"`
	MTRThresholdRtt    *float64 `json:"mtr_threshold_latency_ms"`
}

type updateDirectionRequest struct {
	TargetAddress *string `json:"target_address"`
	PingInterval  *int    `json:"ping_interval_seconds"`
	MTRInterval   *int    `json:"mtr_interval_seconds"`
	PingEnabled   *bool   `json:"ping_enabled"`
	MTREnabled    *bool   `json:"mtr_enabled"`
}

type updateRetentionSettingsRequest struct {
	PingRetentionDays *int `json:"ping_retention_days"`
	MTRRetentionDays  *int `json:"mtr_retention_days"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type updateAdminAccountRequest struct {
	Email           string `json:"email"`
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type updateSecuritySettingsRequest struct {
	AuthEnabled *bool `json:"auth_enabled"`
}

type authSessionResponse struct {
	AuthEnabled        bool   `json:"auth_enabled"`
	Authenticated      bool   `json:"authenticated"`
	Email              string `json:"email,omitempty"`
	MustChangePassword bool   `json:"must_change_password"`
}

type linkWithAgents struct {
	*models.Link
	Directions []directionSummary `json:"directions"`
}

type directionSummary struct {
	ID                 string             `json:"id"`
	SourceAgentID      string             `json:"source_agent_id"`
	DestinationAgentID string             `json:"destination_agent_id"`
	SourceAgent        *agentSummary      `json:"source_agent,omitempty"`
	DestAgent          *agentSummary      `json:"dest_agent,omitempty"`
	TargetAddress      string             `json:"target_address"`
	PingInterval       int                `json:"ping_interval_seconds"`
	MTRInterval        int                `json:"mtr_interval_seconds"`
	PingEnabled        bool               `json:"ping_enabled"`
	MTREnabled         bool               `json:"mtr_enabled"`
	Online             bool               `json:"online"`
	LatestPing         *models.PingResult `json:"latest_ping,omitempty"`
}

type agentSummary struct {
	ID             string     `json:"id"`
	Hostname       string     `json:"hostname"`
	Version        string     `json:"version"`
	Addresses      []string   `json:"addresses"`
	PrimaryAddress string     `json:"primary_address"`
	Online         bool       `json:"online"`
	LastSeen       *time.Time `json:"last_seen"`
}
