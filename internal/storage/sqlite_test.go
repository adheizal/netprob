package storage

import (
	"path/filepath"
	"testing"
)

func TestSQLiteConnectionSafetyPragmas(t *testing.T) {
	store := &SQLiteStore{dsn: filepath.Join(t.TempDir(), "netprob.db")}
	if err := store.Open(); err != nil {
		t.Fatal(err)
	}
	defer store.DB().Close()

	var journalMode string
	var busyTimeout, foreignKeys int
	if err := store.DB().QueryRow(`PRAGMA journal_mode`).Scan(&journalMode); err != nil {
		t.Fatal(err)
	}
	if err := store.DB().QueryRow(`PRAGMA busy_timeout`).Scan(&busyTimeout); err != nil {
		t.Fatal(err)
	}
	if err := store.DB().QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
		t.Fatal(err)
	}
	if journalMode != "wal" || busyTimeout != 5000 || foreignKeys != 1 {
		t.Fatalf("SQLite pragmas = journal:%s busy:%d foreign_keys:%d", journalMode, busyTimeout, foreignKeys)
	}
}
