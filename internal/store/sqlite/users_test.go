package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"todo/internal/auth"
	"todo/internal/store/sqlite"
	"todo/internal/todo"
)

func TestCreateUserAndFind(t *testing.T) {
	s := newStore(t) // already has "alice" (ID 1)
	ctx := context.Background()
	u, err := s.CreateUser(ctx, "Bob_2", "hash", t0)
	if err != nil || u.ID == 0 || u.Username != "Bob_2" || u.PasswordHash != "hash" || !u.CreatedAt.Equal(t0) {
		t.Fatalf("CreateUser = %+v, %v", u, err)
	}
	byName, err := s.UserByName(ctx, "bob_2")
	if err != nil || byName.ID != u.ID || byName.Username != "Bob_2" {
		t.Fatalf("UserByName(lower case) = %+v, %v", byName, err)
	}
	if _, err := s.CreateUser(ctx, "ALICE", "hash", t0); !errors.Is(err, auth.ErrUsernameTaken) {
		t.Fatalf("CreateUser(ALICE) err = %v, want ErrUsernameTaken", err)
	}
	if _, err := s.UserByName(ctx, "nobody"); !errors.Is(err, auth.ErrNoUser) {
		t.Fatalf("UserByName(nobody) err = %v, want ErrNoUser", err)
	}
	if _, err := s.UserByID(ctx, 999); !errors.Is(err, auth.ErrNoUser) {
		t.Fatalf("UserByID(999) err = %v, want ErrNoUser", err)
	}
}

func TestSetPasswordHash(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	if err := s.SetPasswordHash(ctx, 1, "new"); err != nil {
		t.Fatalf("SetPasswordHash: %v", err)
	}
	u, _ := s.UserByID(ctx, 1)
	if u.PasswordHash != "new" {
		t.Fatalf("hash = %q, want new", u.PasswordHash)
	}
	if err := s.SetPasswordHash(ctx, 999, "x"); !errors.Is(err, auth.ErrNoUser) {
		t.Fatalf("SetPasswordHash(999) err = %v, want ErrNoUser", err)
	}
}

func TestSessions(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	live := auth.Session{TokenHash: "a", UserID: 1, CreatedAt: t0, ExpiresAt: t0.Add(time.Hour)}
	other := auth.Session{TokenHash: "b", UserID: 1, CreatedAt: t0, ExpiresAt: t0.Add(time.Hour), Persistent: true}
	old := auth.Session{TokenHash: "c", UserID: 1, CreatedAt: t0, ExpiresAt: t0.Add(-time.Minute)}
	for _, sess := range []auth.Session{live, other, old} {
		if err := s.CreateSession(ctx, sess); err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
	}
	got, u, err := s.SessionUser(ctx, "b", t0)
	if err != nil || u.Username != "alice" || !got.Persistent || !got.ExpiresAt.Equal(other.ExpiresAt) {
		t.Fatalf("SessionUser(b) = %+v, %+v, %v", got, u, err)
	}
	if _, _, err := s.SessionUser(ctx, "c", t0); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("expired session err = %v, want ErrNoSession", err)
	}
	if _, _, err := s.SessionUser(ctx, "zzz", t0); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("unknown session err = %v, want ErrNoSession", err)
	}
	if n, err := s.DeleteExpiredSessions(ctx, t0); err != nil || n != 1 {
		t.Fatalf("DeleteExpiredSessions = %d, %v; want 1", n, err)
	}
	if err := s.DeleteOtherSessions(ctx, 1, "a"); err != nil {
		t.Fatalf("DeleteOtherSessions: %v", err)
	}
	if _, _, err := s.SessionUser(ctx, "b", t0); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("other session still valid: %v", err)
	}
	if err := s.DeleteSession(ctx, "a"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if _, _, err := s.SessionUser(ctx, "a", t0); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("deleted session still valid: %v", err)
	}
}

func TestDeleteUserCascades(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	s.CreateSession(ctx, auth.Session{TokenHash: "a", UserID: 1, CreatedAt: t0, ExpiresAt: t0.Add(time.Hour)})
	if _, err := s.Create(ctx, 1, todo.Change{Text: "x"}, t0); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := s.DeleteUser(ctx, 1); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	if open, done, _ := s.Counts(ctx, 1); open+done != 0 {
		t.Fatalf("items left after DeleteUser: %d", open+done)
	}
	if _, _, err := s.SessionUser(ctx, "a", t0); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("session left after DeleteUser: %v", err)
	}
	if err := s.DeleteUser(ctx, 1); !errors.Is(err, auth.ErrNoUser) {
		t.Fatalf("second DeleteUser err = %v, want ErrNoUser", err)
	}
}

// A deleted user's ID is never given to a new user, so leftover rows
// (for example after a delete without foreign keys) cannot move to someone else.
func TestUserIDsAreNotReused(t *testing.T) {
	s := newStore(t) // alice = 1
	ctx := context.Background()
	bob, _ := s.CreateUser(ctx, "bob", "x", t0)
	if err := s.DeleteUser(ctx, bob.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	carol, err := s.CreateUser(ctx, "carol", "x", t0)
	if err != nil || carol.ID == bob.ID {
		t.Fatalf("carol = %+v, %v; must not reuse bob's ID %d", carol, err, bob.ID)
	}
}

// The cascade really deletes session rows (SessionUser's JOIN would hide orphans).
func TestDeleteUserRemovesSessionRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.db")
	s, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()
	ctx := context.Background()
	u, _ := s.CreateUser(ctx, "alice", "x", t0)
	s.CreateSession(ctx, auth.Session{TokenHash: "a", UserID: u.ID, CreatedAt: t0, ExpiresAt: t0.Add(time.Hour)})
	if err := s.DeleteUser(ctx, u.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("session rows after DeleteUser = %d, %v; want 0", n, err)
	}
}
