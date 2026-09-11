package models

import "time"

type PingResult struct {
	ID                 int64     `json:"id" db:"id"`
	DirectionID        string    `json:"direction_id" db:"direction_id"`
	SourceAgentID      string    `json:"source_agent_id" db:"source_agent_id"`
	DestinationAgentID string    `json:"destination_agent_id" db:"destination_agent_id"`
	Timestamp          time.Time `json:"timestamp" db:"timestamp"`
	MinRTT             *float64  `json:"min_rtt_ms,omitempty" db:"min_rtt_ms"`
	AvgRTT             *float64  `json:"avg_rtt_ms,omitempty" db:"avg_rtt_ms"`
	MaxRTT             *float64  `json:"max_rtt_ms,omitempty" db:"max_rtt_ms"`
	Jitter             *float64  `json:"jitter_ms,omitempty" db:"jitter_ms"`
	PacketLoss         *float64  `json:"packet_loss_percent,omitempty" db:"packet_loss_percent"`
	PacketsSent        int       `json:"packets_sent" db:"packets_sent"`
	PacketsReceived    int       `json:"packets_received" db:"packets_received"`
}

type MTRHop struct {
	HopNumber   int      `json:"hop"`
	Host        string   `json:"host,omitempty"`
	IP          string   `json:"ip,omitempty"`
	LossPercent float64  `json:"loss_percent"`
	Sent        int      `json:"sent"`
	LastMs      *float64 `json:"last_ms,omitempty"`
	AvgMs       *float64 `json:"avg_ms,omitempty"`
	BestMs      *float64 `json:"best_ms,omitempty"`
	WorstMs     *float64 `json:"worst_ms,omitempty"`
}

type MTRRun struct {
	ID                 string    `json:"id" db:"id"`
	DirectionID        string    `json:"direction_id" db:"direction_id"`
	SourceAgentID      string    `json:"source_agent_id" db:"source_agent_id"`
	DestinationAgentID string    `json:"destination_agent_id" db:"destination_agent_id"`
	Timestamp          time.Time `json:"timestamp" db:"timestamp"`
	Status             string    `json:"status" db:"status"`
	Error              string    `json:"error,omitempty" db:"error"`
	Hops               []MTRHop  `json:"hops"`
}
