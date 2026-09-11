package storage

import (
	"database/sql"
	_ "embed"
	"fmt"
)

//go:embed migrations/001_initial_schema.sql
var initialSchema string

//go:embed migrations/002_agent_location.sql
var agentLocationSchema string

//go:embed migrations/003_agent_primary_address.sql
var agentPrimaryAddressSchema string

//go:embed migrations/004_agent_instance_id.sql
var agentInstanceIDSchema string

//go:embed migrations/005_mtr_status.sql
var mtrStatusSchema string

//go:embed migrations/006_retention_settings.sql
var retentionSettingsSchema string

//go:embed migrations/007_admin_auth.sql
var adminAuthSchema string

//go:embed migrations/008_agent_provider.sql
var agentProviderSchema string

const schemaVersion = 8

func (s *SQLiteStore) ApplyMigrations(db *sql.DB) error {
	var version int
	row := db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`)
	if err := row.Scan(&version); err != nil {
		// table doesn't exist yet, create it
		if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER, applied_at DATETIME DEFAULT CURRENT_TIMESTAMP)`); err != nil {
			return fmt.Errorf("create schema_migrations: %w", err)
		}
	}

	for v := version + 1; v <= schemaVersion; v++ {
		migrationSQL, err := loadMigration(v)
		if err != nil {
			return err
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(migrationSQL); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d: %w", v, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, v); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %d: %w", v, err)
		}
		tx.Commit()
	}
	return nil
}

func loadMigration(version int) (string, error) {
	switch version {
	case 1:
		return initialSchema, nil
	case 2:
		return agentLocationSchema, nil
	case 3:
		return agentPrimaryAddressSchema, nil
	case 4:
		return agentInstanceIDSchema, nil
	case 5:
		return mtrStatusSchema, nil
	case 6:
		return retentionSettingsSchema, nil
	case 7:
		return adminAuthSchema, nil
	case 8:
		return agentProviderSchema, nil
	default:
		return "", fmt.Errorf("unknown migration version %d", version)
	}
}
