package storage

import (
	"netprob/internal/models"
)

// ListDirectionsForScheduling returns all active directions for the scheduler.
func (s *SQLiteStore) ListDirectionsForScheduling() ([]*models.Direction, error) {
	return s.listDirections(` WHERE ping_enabled = 1 OR mtr_enabled = 1`)
}

// ListDirections returns every configured direction for inventory metrics.
func (s *SQLiteStore) ListDirections() ([]*models.Direction, error) {
	return s.listDirections("")
}

func (s *SQLiteStore) listDirections(where string) ([]*models.Direction, error) {
	rows, err := s.db.Query(`
		SELECT id, link_id, source_agent_id, destination_agent_id, target_address,
			ping_interval_seconds, mtr_interval_seconds, ping_enabled, mtr_enabled,
			mtr_threshold_loss_percent, mtr_threshold_latency_ms, created_at, updated_at
		FROM directions` + where + ` ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dirs []*models.Direction
	for rows.Next() {
		d, err := scanDirection(rows)
		if err != nil {
			return nil, err
		}
		dirs = append(dirs, d)
	}
	return dirs, rows.Err()
}
