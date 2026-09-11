package models

import "time"

// RetentionSettings controls how long local probe history is retained.
// A value of zero keeps that result type forever.
type RetentionSettings struct {
	PingRetentionDays int       `json:"ping_retention_days"`
	MTRRetentionDays  int       `json:"mtr_retention_days"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type RetentionCleanupResult struct {
	PingResultsDeleted int64 `json:"ping_results_deleted"`
	MTRRunsDeleted     int64 `json:"mtr_runs_deleted"`
}
