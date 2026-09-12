package storage

import (
	"database/sql"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStore handles application/config persistence in SQLite.
type SQLiteStore struct {
	db  *sql.DB
	dsn string
}

func (s *SQLiteStore) Open() error {
	separator := "?"
	if strings.Contains(s.dsn, "?") {
		separator = "&"
	}
	dsn := s.dsn + separator + "_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return err
	}
	db.SetConnMaxLifetime(time.Hour)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)
	if err := db.Ping(); err != nil {
		db.Close()
		return err
	}
	s.db = db
	return nil
}

func (s *SQLiteStore) DB() *sql.DB {
	return s.db
}
