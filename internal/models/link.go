package models

import (
	"time"

	"github.com/google/uuid"
)

type Link struct {
	ID          string    `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description,omitempty" db:"description"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

type Direction struct {
	ID               string  `json:"id" db:"id"`
	LinkID           string  `json:"link_id" db:"link_id"`
	SourceAgentID    string  `json:"source_agent_id" db:"source_agent_id"`
	DestinationAgentID string  `json:"destination_agent_id" db:"destination_agent_id"`
	TargetAddress    string  `json:"target_address" db:"target_address"`
	PingInterval     int     `json:"ping_interval_seconds" db:"ping_interval_seconds"`
	MTRInterval      int     `json:"mtr_interval_seconds" db:"mtr_interval_seconds"`
	PingEnabled      bool    `json:"ping_enabled" db:"ping_enabled"`
	MTREnabled       bool    `json:"mtr_enabled" db:"mtr_enabled"`
	MTRThresholdLoss *float64 `json:"mtr_threshold_loss_percent,omitempty" db:"mtr_threshold_loss_percent"`
	MTRThresholdRtt  *float64 `json:"mtr_threshold_latency_ms,omitempty" db:"mtr_threshold_latency_ms"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}

func NewLink(name, description string) *Link {
	return &Link{
		ID:          uuid.NewString(),
		Name:        name,
		Description: description,
	}
}

func NewDirection(linkID, sourceID, destID, targetAddress string) *Direction {
	return &Direction{
		ID:                 uuid.NewString(),
		LinkID:             linkID,
		SourceAgentID:      sourceID,
		DestinationAgentID: destID,
		TargetAddress:      targetAddress,
		PingInterval:       5,
		MTRInterval:        120,
		PingEnabled:        true,
		MTREnabled:         true,
	}
}
