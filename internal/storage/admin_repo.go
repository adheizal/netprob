package storage

import (
	"database/sql"
	"strings"
	"time"

	"netprob/internal/models"
)

func (s *SQLiteStore) EnsureDefaultAdmin(email, passwordHash string) error {
	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO admin_users (id, email, password_hash, must_change_password)
		VALUES (1, ?, ?, 1)
	`, strings.ToLower(strings.TrimSpace(email)), passwordHash)
	return err
}

func (s *SQLiteStore) GetAdminByEmail(email string) (*models.AdminUser, error) {
	return scanAdmin(s.db.QueryRow(`
		SELECT id, email, password_hash, must_change_password, created_at, updated_at
		FROM admin_users WHERE email = ?
	`, strings.ToLower(strings.TrimSpace(email))))
}

func (s *SQLiteStore) GetAdminBySession(tokenHash string, now time.Time) (*models.AdminUser, error) {
	admin, err := scanAdmin(s.db.QueryRow(`
		SELECT u.id, u.email, u.password_hash, u.must_change_password, u.created_at, u.updated_at
		FROM admin_users u
		JOIN admin_sessions s ON s.admin_id = u.id
		WHERE s.token_hash = ? AND s.expires_at > ?
	`, tokenHash, now))
	if err == sql.ErrNoRows {
		_, _ = s.db.Exec(`DELETE FROM admin_sessions WHERE token_hash = ?`, tokenHash)
	}
	return admin, err
}

func (s *SQLiteStore) CreateAdminSession(tokenHash string, adminID int, expiresAt time.Time) error {
	_, err := s.db.Exec(`
		INSERT INTO admin_sessions (token_hash, admin_id, expires_at) VALUES (?, ?, ?)
	`, tokenHash, adminID, expiresAt)
	return err
}

func (s *SQLiteStore) DeleteAdminSession(tokenHash string) error {
	_, err := s.db.Exec(`DELETE FROM admin_sessions WHERE token_hash = ?`, tokenHash)
	return err
}

func (s *SQLiteStore) DeleteExpiredAdminSessions(now time.Time) (int64, error) {
	result, err := s.db.Exec(`DELETE FROM admin_sessions WHERE expires_at <= ?`, now)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *SQLiteStore) UpdateAdminCredentials(adminID int, email, passwordHash string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`
		UPDATE admin_users
		SET email = ?, password_hash = ?, must_change_password = 0, updated_at = ?
		WHERE id = ?
	`, strings.ToLower(strings.TrimSpace(email)), passwordHash, time.Now().UTC(), adminID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM admin_sessions WHERE admin_id = ?`, adminID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLiteStore) GetSecuritySettings() (*models.SecuritySettings, error) {
	settings := &models.SecuritySettings{}
	err := s.db.QueryRow(`
		SELECT auth_enabled, updated_at FROM security_settings WHERE id = 1
	`).Scan(&settings.AuthEnabled, &settings.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return settings, nil
}

func (s *SQLiteStore) UpdateSecuritySettings(enabled bool) (*models.SecuritySettings, error) {
	settings := &models.SecuritySettings{AuthEnabled: enabled, UpdatedAt: time.Now().UTC()}
	_, err := s.db.Exec(`
		UPDATE security_settings SET auth_enabled = ?, updated_at = ? WHERE id = 1
	`, settings.AuthEnabled, settings.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return settings, nil
}

func scanAdmin(row interface{ Scan(...any) error }) (*models.AdminUser, error) {
	admin := &models.AdminUser{}
	err := row.Scan(&admin.ID, &admin.Email, &admin.PasswordHash, &admin.MustChangePassword, &admin.CreatedAt, &admin.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return admin, nil
}
