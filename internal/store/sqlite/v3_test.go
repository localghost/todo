package sqlite_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"todo/internal/store/sqlite"
)

// v2Database writes a database at schema version 2 (frozen copy of that
// schema) with one user, one session and one item.
func v2Database(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "v2.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, stmt := range []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT NOT NULL UNIQUE COLLATE NOCASE,
			password_hash TEXT NOT NULL, created_at TEXT NOT NULL)`,
		`CREATE TABLE sessions (token_hash TEXT PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			created_at TEXT NOT NULL, expires_at TEXT NOT NULL, persistent INTEGER NOT NULL)`,
		`CREATE INDEX sessions_user ON sessions (user_id)`,
		`CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT NOT NULL)`,
		`CREATE TABLE items (id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			text TEXT NOT NULL, done INTEGER NOT NULL DEFAULT 0, position INTEGER NOT NULL, created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL, due_at TEXT, due_all_day INTEGER NOT NULL DEFAULT 0, notified_at TEXT)`,
		`CREATE INDEX items_user_position ON items (user_id, position)`,
		`INSERT INTO users (id, username, password_hash, created_at) VALUES (1, 'alice', 'x', '2026-10-01T10:00:00Z')`,
		`INSERT INTO sessions VALUES ('tok', 1, '2026-10-01T10:00:00Z', '2099-01-01T00:00:00Z', 0)`,
		`INSERT INTO items (user_id, text, position, created_at, updated_at) VALUES (1, 'Keep me', 1, '2026-10-01T10:00:00Z', '2026-10-01T10:00:00Z')`,
		`PRAGMA user_version = 2`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	return path
}

func TestMigrateV2ToV3KeepsData(t *testing.T) {
	path := v2Database(t)
	if _, err := sqlite.OpenWith(path, sqlite.Options{RequireCurrent: true}); err == nil || !strings.Contains(err.Error(), "start the server once") {
		t.Fatalf("admin open of a v2 file: %v, want the upgrade message", err)
	}
	s, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()
	if v := userVersion(t, path); v != 3 {
		t.Fatalf("user_version = %d, want 3", v)
	}
	ctx := context.Background()
	_, u, err := s.SessionUser(ctx, "tok", time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC))
	if err != nil || u.Username != "alice" || u.HideDone {
		t.Fatalf("SessionUser after migration = %+v, %v; want alice, HideDone false", u, err)
	}
	items, err := s.List(ctx, 1, false)
	if err != nil || len(items) != 1 || items[0].Text != "Keep me" {
		t.Fatalf("items after migration = %+v, %v", items, err)
	}
	if _, err := sqlite.OpenWith(path, sqlite.Options{RequireCurrent: true}); err != nil {
		t.Fatalf("admin open of a v3 file: %v", err)
	}
}
