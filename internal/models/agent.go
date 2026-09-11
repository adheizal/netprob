package models

import (
	"time"

	"github.com/google/uuid"
)

type Agent struct {
	ID             string        `json:"id" db:"id"`
	Hostname       string        `json:"hostname" db:"hostname"`
	Version        string        `json:"version" db:"version"`
	Addresses      []string      `json:"addresses" db:"addresses"`
	PrimaryAddress string        `json:"primary_address" db:"primary_address"`
	Capabilities   []string      `json:"capabilities" db:"capabilities"`
	Location       AgentLocation `json:"location"`
	TokenHash      string        `json:"-" db:"token_hash"`
	InstanceID     string        `json:"-" db:"instance_id"`
	Online         bool          `json:"online" db:"online"`
	LastSeen       *time.Time    `json:"last_seen" db:"last_seen"`
	CreatedAt      time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at" db:"updated_at"`
}

type AgentLocation struct {
	PublicIP    string `json:"public_ip"`
	CountryCode string `json:"country_code"`
	Region      string `json:"region"`
	City        string `json:"city"`
	Timezone    string `json:"timezone"`
	ASName      string `json:"as_name"`
	ISP         string `json:"isp"`
}

func NewAgent(hostname, version string, addresses, capabilities []string) *Agent {
	return &Agent{
		ID:           uuid.NewString(),
		Hostname:     hostname,
		Version:      version,
		Addresses:    addresses,
		Capabilities: capabilities,
		Online:       false,
		LastSeen:     nil,
	}
}
