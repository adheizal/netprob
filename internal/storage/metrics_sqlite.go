package storage

import (
	"time"

	"netprob/internal/models"
)

// SQLiteMetricsStore stores ping results in SQLite (fallback when VictoriaMetrics is not configured)
type SQLiteMetricsStore struct {
	db *SQLiteStore
}

func NewSQLiteMetricsStore(db *SQLiteStore) *SQLiteMetricsStore {
	return &SQLiteMetricsStore{db: db}
}

func (s *SQLiteMetricsStore) WritePing(result *models.PingResult, _ MetricContext) error {
	return s.db.SavePingResult(result)
}

func (s *SQLiteMetricsStore) WriteMTR(_ *models.MTRRun, _ MetricContext) error {
	return nil
}

func (s *SQLiteMetricsStore) WriteProbeStatus(_ string, _ bool, _ time.Time, _ MetricContext) error {
	return nil
}

func (s *SQLiteMetricsStore) WriteInventory(_ InventorySnapshot) error {
	return nil
}
