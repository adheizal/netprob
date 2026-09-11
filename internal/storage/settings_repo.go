package storage

import (
	"fmt"
	"time"

	"netprob/internal/models"
)

func (s *SQLiteStore) GetRetentionSettings() (*models.RetentionSettings, error) {
	settings := &models.RetentionSettings{}
	err := s.db.QueryRow(`
		SELECT ping_retention_days, mtr_retention_days, updated_at
		FROM retention_settings WHERE id = 1
	`).Scan(&settings.PingRetentionDays, &settings.MTRRetentionDays, &settings.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return settings, nil
}

func (s *SQLiteStore) UpdateRetentionSettings(settings *models.RetentionSettings) error {
	if settings.PingRetentionDays < 0 || settings.MTRRetentionDays < 0 {
		return fmt.Errorf("retention days must be zero or greater")
	}
	settings.UpdatedAt = time.Now().UTC()
	_, err := s.db.Exec(`
		UPDATE retention_settings
		SET ping_retention_days = ?, mtr_retention_days = ?, updated_at = ?
		WHERE id = 1
	`, settings.PingRetentionDays, settings.MTRRetentionDays, settings.UpdatedAt)
	return err
}

// CleanupExpiredProbeHistory removes local SQLite history older than the
// configured limits. Zero retention keeps that result type forever.
func (s *SQLiteStore) CleanupExpiredProbeHistory(settings *models.RetentionSettings, now time.Time) (*models.RetentionCleanupResult, error) {
	result := &models.RetentionCleanupResult{}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if settings.PingRetentionDays > 0 {
		cutoff := now.AddDate(0, 0, -settings.PingRetentionDays)
		deleteResult, err := tx.Exec(`DELETE FROM ping_results WHERE timestamp < ?`, cutoff)
		if err != nil {
			return nil, err
		}
		result.PingResultsDeleted, err = deleteResult.RowsAffected()
		if err != nil {
			return nil, err
		}
	}

	if settings.MTRRetentionDays > 0 {
		cutoff := now.AddDate(0, 0, -settings.MTRRetentionDays)
		if _, err := tx.Exec(`
			DELETE FROM mtr_hops
			WHERE mtr_run_id IN (SELECT id FROM mtr_runs WHERE timestamp < ?)
		`, cutoff); err != nil {
			return nil, err
		}
		deleteResult, err := tx.Exec(`DELETE FROM mtr_runs WHERE timestamp < ?`, cutoff)
		if err != nil {
			return nil, err
		}
		result.MTRRunsDeleted, err = deleteResult.RowsAffected()
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}
