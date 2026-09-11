package storage

import (
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStore handles application/config persistence in SQLite.
type SQLiteStore struct {
	db  *sql.DB
	dsn string
}

func (s *SQLiteStore) Open() error {
	db, err := sql.Open("sqlite3", s.dsn)
	if err != nil {
		return err
	}
	db.SetConnMaxLifetime(time.Hour)
	s.db = db
	return nil
}

func (s *SQLiteStore) DB() *sql.DB {
	return s.db
}