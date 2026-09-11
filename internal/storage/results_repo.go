package storage

import (
	"database/sql"

	"netprob/internal/models"
)

func (s *SQLiteStore) SavePingResult(r *models.PingResult) error {
	_, err := s.db.Exec(`
		INSERT INTO ping_results (direction_id, source_agent_id, destination_agent_id, timestamp,
			min_rtt_ms, avg_rtt_ms, max_rtt_ms, jitter_ms, packet_loss_percent, packets_sent, packets_received)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, r.DirectionID, r.SourceAgentID, r.DestinationAgentID, r.Timestamp,
		r.MinRTT, r.AvgRTT, r.MaxRTT, r.Jitter, r.PacketLoss, r.PacketsSent, r.PacketsReceived)
	return err
}

func (s *SQLiteStore) QueryPingResults(directionID string, limit int) ([]*models.PingResult, error) {
	rows, err := s.db.Query(`
		SELECT id, direction_id, source_agent_id, destination_agent_id, timestamp,
			min_rtt_ms, avg_rtt_ms, max_rtt_ms, jitter_ms, packet_loss_percent, packets_sent, packets_received
		FROM ping_results WHERE direction_id = ? ORDER BY timestamp DESC LIMIT ?
	`, directionID, limit)
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

func (s *SQLiteStore) GetLatestPingResult(directionID string) (*models.PingResult, error) {
	row := s.db.QueryRow(`
		SELECT id, direction_id, source_agent_id, destination_agent_id, timestamp,
			min_rtt_ms, avg_rtt_ms, max_rtt_ms, jitter_ms, packet_loss_percent, packets_sent, packets_received
		FROM ping_results WHERE direction_id = ? ORDER BY timestamp DESC LIMIT 1
	`, directionID)
	return scanPingResult(row)
}

func (s *SQLiteStore) SaveMTRRun(run *models.MTRRun) error {
	if run.Status == "" {
		if run.Error != "" {
			run.Status = "error"
		} else {
			run.Status = "success"
		}
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO mtr_runs (id, direction_id, source_agent_id, destination_agent_id, timestamp, status, error)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, run.ID, run.DirectionID, run.SourceAgentID, run.DestinationAgentID, run.Timestamp, run.Status, run.Error)
	if err != nil {
		return err
	}

	for i := range run.Hops {
		h := &run.Hops[i]
		_, err = tx.Exec(`
			INSERT INTO mtr_hops (mtr_run_id, hop_number, host, ip, loss_percent, sent, last_ms, avg_ms, best_ms, worst_ms)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, run.ID, h.HopNumber, h.Host, h.IP, h.LossPercent, h.Sent, h.LastMs, h.AvgMs, h.BestMs, h.WorstMs)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *SQLiteStore) ListMTRRuns(directionID string, limit int) ([]*models.MTRRun, error) {
	rows, err := s.db.Query(`
		SELECT id, direction_id, source_agent_id, destination_agent_id, timestamp, status, error
		FROM mtr_runs WHERE direction_id = ? ORDER BY timestamp DESC LIMIT ?
	`, directionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	runs := make([]*models.MTRRun, 0)
	for rows.Next() {
		var run models.MTRRun
		err := rows.Scan(&run.ID, &run.DirectionID, &run.SourceAgentID, &run.DestinationAgentID, &run.Timestamp, &run.Status, &run.Error)
		if err != nil {
			return nil, err
		}
		// Load hops
		hopRows, err := s.db.Query(`
			SELECT hop_number, host, ip, loss_percent, sent, last_ms, avg_ms, best_ms, worst_ms
			FROM mtr_hops WHERE mtr_run_id = ? ORDER BY hop_number
		`, run.ID)
		if err != nil {
			return nil, err
		}
		for hopRows.Next() {
			var h models.MTRHop
			var host, ip sql.NullString
			var lastMs, avgMs, bestMs, worstMs sql.NullFloat64
			hopRows.Scan(&h.HopNumber, &host, &ip, &h.LossPercent, &h.Sent, &lastMs, &avgMs, &bestMs, &worstMs)
			if host.Valid {
				h.Host = host.String
			}
			if ip.Valid {
				h.IP = ip.String
			}
			if lastMs.Valid {
				h.LastMs = &lastMs.Float64
			}
			if avgMs.Valid {
				h.AvgMs = &avgMs.Float64
			}
			if bestMs.Valid {
				h.BestMs = &bestMs.Float64
			}
			if worstMs.Valid {
				h.WorstMs = &worstMs.Float64
			}
			run.Hops = append(run.Hops, h)
		}
		hopRows.Close()
		runs = append(runs, &run)
	}
	return runs, rows.Err()
}

func (s *SQLiteStore) GetMTRRunByID(id string) (*models.MTRRun, error) {
	row := s.db.QueryRow(`
		SELECT id, direction_id, source_agent_id, destination_agent_id, timestamp, status, error
		FROM mtr_runs WHERE id = ?
	`, id)
	var run models.MTRRun
	err := row.Scan(&run.ID, &run.DirectionID, &run.SourceAgentID, &run.DestinationAgentID, &run.Timestamp, &run.Status, &run.Error)
	if err != nil {
		return nil, err
	}

	hopRows, err := s.db.Query(`
		SELECT hop_number, host, ip, loss_percent, sent, last_ms, avg_ms, best_ms, worst_ms
		FROM mtr_hops WHERE mtr_run_id = ? ORDER BY hop_number
	`, run.ID)
	if err != nil {
		return nil, err
	}
	defer hopRows.Close()
	for hopRows.Next() {
		var h models.MTRHop
		var host, ip sql.NullString
		var lastMs, avgMs, bestMs, worstMs sql.NullFloat64
		hopRows.Scan(&h.HopNumber, &host, &ip, &h.LossPercent, &h.Sent, &lastMs, &avgMs, &bestMs, &worstMs)
		if host.Valid {
			h.Host = host.String
		}
		if ip.Valid {
			h.IP = ip.String
		}
		if lastMs.Valid {
			h.LastMs = &lastMs.Float64
		}
		if avgMs.Valid {
			h.AvgMs = &avgMs.Float64
		}
		if bestMs.Valid {
			h.BestMs = &bestMs.Float64
		}
		if worstMs.Valid {
			h.WorstMs = &worstMs.Float64
		}
		run.Hops = append(run.Hops, h)
	}
	return &run, nil
}

func scanPingResult(row interface {
	Scan(dest ...any) error
}) (*models.PingResult, error) {
	var r models.PingResult
	err := row.Scan(
		&r.ID, &r.DirectionID, &r.SourceAgentID, &r.DestinationAgentID, &r.Timestamp,
		&r.MinRTT, &r.AvgRTT, &r.MaxRTT, &r.Jitter, &r.PacketLoss, &r.PacketsSent, &r.PacketsReceived,
	)
	if err != nil {
		return nil, err
	}
	return &r, nil
}
