package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"todo/internal/auth"
	"todo/internal/store/sqlite"
	"todo/internal/todo"
)

// A user can have at most todo.MaxItemsPerUser items; Create checks it in
// its transaction. Other users are not affected.
func TestItemsPerUserLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	ctx := context.Background()
	for _, name := range []string{"alice", "bob"} {
		if _, err := s.CreateUser(ctx, name, "x", t0); err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
	}
	// Fill alice's list up to one below the limit in one fast transaction.
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()
	tx, _ := db.Begin()
	for i := 1; i < todo.MaxItemsPerUser; i++ {
		if _, err := tx.Exec(`INSERT INTO items (user_id, text, done, position, created_at, updated_at) VALUES (1, ?, 0, ?, ?, ?)`,
			fmt.Sprint("item ", i), i, "2026-10-01T12:00:00Z", "2026-10-01T12:00:00Z"); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if _, err := s.Create(ctx, 1, todo.Change{Text: "the last one"}, t0); err != nil {
		t.Fatalf("item %d: %v", todo.MaxItemsPerUser, err)
	}
	if _, err := s.Create(ctx, 1, todo.Change{Text: "one too many"}, t0); !errors.Is(err, todo.ErrTooManyItems) {
		t.Fatalf("item %d: err = %v, want ErrTooManyItems", todo.MaxItemsPerUser+1, err)
	}
	if open, done, _ := s.Counts(ctx, 1); open+done != todo.MaxItemsPerUser {
		t.Fatalf("alice has %d items, want %d", open+done, todo.MaxItemsPerUser)
	}
	if _, err := s.Create(ctx, 2, todo.Change{Text: "bob's item"}, t0); err != nil {
		t.Fatalf("bob: %v", err)
	}
}

// A user keeps at most auth.MaxSessionsPerUser sessions: a new session ends
// the oldest one. Other users' sessions stay.
func TestSessionsPerUserLimit(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	if _, err := s.CreateUser(ctx, "bob", "x", t0); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := s.CreateSession(ctx, auth.Session{TokenHash: "bob", UserID: 2, CreatedAt: t0, ExpiresAt: t0.Add(time.Hour)}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	for i := 0; i <= auth.MaxSessionsPerUser; i++ {
		sess := auth.Session{TokenHash: fmt.Sprint("t", i), UserID: 1, CreatedAt: t0.Add(time.Duration(i) * time.Second), ExpiresAt: t0.Add(time.Hour)}
		if err := s.CreateSession(ctx, sess); err != nil {
			t.Fatalf("CreateSession %d: %v", i, err)
		}
	}
	if _, _, err := s.SessionUser(ctx, "t0", t0); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("the oldest session still works: %v", err)
	}
	for i := 1; i <= auth.MaxSessionsPerUser; i++ {
		if _, _, err := s.SessionUser(ctx, fmt.Sprint("t", i), t0); err != nil {
			t.Fatalf("session t%d: %v", i, err)
		}
	}
	if _, _, err := s.SessionUser(ctx, "bob", t0); err != nil {
		t.Fatalf("bob's session: %v", err)
	}
}
