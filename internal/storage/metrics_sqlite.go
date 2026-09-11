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

func (s *SQLiteMetricsStore) QueryPing(sourceAgentID, destAgentID string, from, to time.Time, limit int) ([]*models.PingResult, error) {
	var directionID string
	err := s.db.db.QueryRow(
		`SELECT id FROM directions WHERE source_agent_id = ? AND destination_agent_id = ?`,
		sourceAgentID, destAgentID,
	).Scan(&directionID)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.db.Query(`
		SELECT id, direction_id, source_agent_id, destination_agent_id, timestamp,
			min_rtt_ms, avg_rtt_ms, max_rtt_ms, jitter_ms, packet_loss_percent, packets_sent, packets_received
		FROM ping_results WHERE direction_id = ? AND timestamp >= ? AND timestamp <= ?
		ORDER BY timestamp DESC LIMIT ?
	`, directionID, from, to, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]*models.PingResult, 0)
	for rows.Next() {
		r, err := scanPingResult(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}
