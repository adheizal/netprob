package storage

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDeleteExpiredAdminSessions(t *testing.T) {
	store := &SQLiteStore{dsn: filepath.Join(t.TempDir(), "netprob.db")}
	if err := store.Open(); err != nil {
		t.Fatal(err)
	}
	defer store.DB().Close()
	if err := store.ApplyMigrations(store.DB()); err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureDefaultAdmin("admin@example.com", "unused-hash"); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := store.CreateAdminSession("expired", 1, now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateAdminSession("active", 1, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	deleted, err := store.DeleteExpiredAdminSessions(now)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("deleted sessions = %d, want 1", deleted)
	}
	var count int
	if err := store.DB().QueryRow(`SELECT COUNT(*) FROM admin_sessions`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("remaining sessions = %d, want 1", count)
	}
}
