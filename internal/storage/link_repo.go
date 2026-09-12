package storage

import (
	"database/sql"
	"time"

	"netprob/internal/models"
)

func (s *SQLiteStore) CreateLink(l *models.Link) error {
	_, err := s.db.Exec(`
		INSERT INTO links (id, name, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`, l.ID, l.Name, l.Description, time.Now(), time.Now())
	return err
}

func (s *SQLiteStore) GetLinkByID(id string) (*models.Link, error) {
	row := s.db.QueryRow(`SELECT id, name, description, created_at, updated_at FROM links WHERE id = ?`, id)
	return scanLink(row)
}

func (s *SQLiteStore) ListLinks() ([]*models.Link, error) {
	rows, err := s.db.Query(`SELECT id, name, description, created_at, updated_at FROM links ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	links := make([]*models.Link, 0)
	for rows.Next() {
		l, err := scanLink(rows)
		if err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

func (s *SQLiteStore) UpdateLink(id, name, description string) error {
	_, err := s.db.Exec(`UPDATE links SET name = ?, description = ?, updated_at = ? WHERE id = ?`, name, description, time.Now(), id)
	return err
}

func (s *SQLiteStore) DeleteLink(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	statements := []string{
		`DELETE FROM mtr_hops WHERE mtr_run_id IN (
			SELECT id FROM mtr_runs WHERE direction_id IN (SELECT id FROM directions WHERE link_id = ?)
		)`,
		`DELETE FROM mtr_runs WHERE direction_id IN (SELECT id FROM directions WHERE link_id = ?)`,
		`DELETE FROM ping_results WHERE direction_id IN (SELECT id FROM directions WHERE link_id = ?)`,
		`DELETE FROM directions WHERE link_id = ?`,
		`DELETE FROM links WHERE id = ?`,
	}
	for _, statement := range statements {
		if _, err := tx.Exec(statement, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) CreateDirection(d *models.Direction) error {
	_, err := s.db.Exec(`
		INSERT INTO directions (id, link_id, source_agent_id, destination_agent_id, target_address,
			ping_interval_seconds, mtr_interval_seconds, ping_enabled, mtr_enabled,
			mtr_threshold_loss_percent, mtr_threshold_latency_ms, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, d.ID, d.LinkID, d.SourceAgentID, d.DestinationAgentID, d.TargetAddress,
		d.PingInterval, d.MTRInterval, d.PingEnabled, d.MTREnabled,
		d.MTRThresholdLoss, d.MTRThresholdRtt, time.Now(), time.Now())
	return err
}

func (s *SQLiteStore) GetDirectionByID(id string) (*models.Direction, error) {
	row := s.db.QueryRow(`
		SELECT id, link_id, source_agent_id, destination_agent_id, target_address,
			ping_interval_seconds, mtr_interval_seconds, ping_enabled, mtr_enabled,
			mtr_threshold_loss_percent, mtr_threshold_latency_ms, created_at, updated_at
		FROM directions WHERE id = ?
	`, id)
	return scanDirection(row)
}

func (s *SQLiteStore) ListDirectionsByLink(linkID string) ([]*models.Direction, error) {
	rows, err := s.db.Query(`
		SELECT id, link_id, source_agent_id, destination_agent_id, target_address,
			ping_interval_seconds, mtr_interval_seconds, ping_enabled, mtr_enabled,
			mtr_threshold_loss_percent, mtr_threshold_latency_ms, created_at, updated_at
		FROM directions WHERE link_id = ? ORDER BY id
	`, linkID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dirs := make([]*models.Direction, 0)
	for rows.Next() {
		d, err := scanDirection(rows)
		if err != nil {
			return nil, err
		}
		dirs = append(dirs, d)
	}
	return dirs, rows.Err()
}

func (s *SQLiteStore) GetDirectionByEndpoints(linkID, sourceID, destID string) (*models.Direction, error) {
	row := s.db.QueryRow(`
		SELECT id, link_id, source_agent_id, destination_agent_id, target_address,
			ping_interval_seconds, mtr_interval_seconds, ping_enabled, mtr_enabled,
			mtr_threshold_loss_percent, mtr_threshold_latency_ms, created_at, updated_at
		FROM directions WHERE link_id = ? AND source_agent_id = ? AND destination_agent_id = ?
	`, linkID, sourceID, destID)
	return scanDirection(row)
}

func (s *SQLiteStore) UpdateDirection(d *models.Direction) error {
	_, err := s.db.Exec(`
		UPDATE directions
		SET target_address = ?, ping_interval_seconds = ?, mtr_interval_seconds = ?,
			ping_enabled = ?, mtr_enabled = ?, mtr_threshold_loss_percent = ?,
			mtr_threshold_latency_ms = ?, updated_at = ?
		WHERE id = ? AND link_id = ?
	`, d.TargetAddress, d.PingInterval, d.MTRInterval, d.PingEnabled, d.MTREnabled,
		d.MTRThresholdLoss, d.MTRThresholdRtt, time.Now(), d.ID, d.LinkID)
	return err
}

func scanLink(row interface {
	Scan(dest ...any) error
}) (*models.Link, error) {
	var l models.Link
	var desc sql.NullString
	var createdAt, updatedAt time.Time
	err := row.Scan(&l.ID, &l.Name, &desc, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	if desc.Valid {
		l.Description = desc.String
	}
	l.CreatedAt = createdAt
	l.UpdatedAt = updatedAt
	return &l, nil
}

func scanDirection(row interface {
	Scan(dest ...any) error
}) (*models.Direction, error) {
	var d models.Direction
	var lossPct, rttThreshold sql.NullFloat64
	var createdAt, updatedAt time.Time
	err := row.Scan(
		&d.ID, &d.LinkID, &d.SourceAgentID, &d.DestinationAgentID, &d.TargetAddress,
		&d.PingInterval, &d.MTRInterval, &d.PingEnabled, &d.MTREnabled,
		&lossPct, &rttThreshold, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	if lossPct.Valid {
		d.MTRThresholdLoss = &lossPct.Float64
	}
	if rttThreshold.Valid {
		d.MTRThresholdRtt = &rttThreshold.Float64
	}
	d.CreatedAt = createdAt
	d.UpdatedAt = updatedAt
	return &d, nil
}
