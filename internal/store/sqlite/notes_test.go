package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"todo/internal/store/sqlite"
	"todo/internal/todo"
)

// A version 7 file gets the column notes; its items keep their data and
// have no notes.
func TestMigrateV7ToV8AddsNotes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v7.db")
	s, err := sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	u, err := s.CreateUser(ctx, "alice", "x", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(ctx, u.ID, todo.Change{Text: "Visit grandma", Repeat: ""}, time.Now()); err != nil {
		t.Fatal(err)
	}
	s.Close()
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{`ALTER TABLE items DROP COLUMN notes`, `PRAGMA user_version = 7`} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	db.Close()

	s, err = sqlite.Open(path)
	if err != nil {
		t.Fatalf("Open v7: %v", err)
	}
	defer s.Close()
	if v := userVersion(t, path); v != 8 {
		t.Fatalf("user_version = %d, want 8", v)
	}
	items, err := s.List(ctx, u.ID, false)
	if err != nil || len(items) != 1 || items[0].Text != "Visit grandma" || items[0].Notes != "" {
		t.Fatalf("items after migration = %+v, %v", items, err)
	}
}

// SetNotes writes only the notes and the change time. UpdateItem, which the
// other changes use, keeps the notes.
func TestSetNotesStore(t *testing.T) {
	s, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	u, _ := s.CreateUser(ctx, "alice", "x", time.Now())
	other, _ := s.CreateUser(ctx, "bob", "x", time.Now())
	it, err := s.Create(ctx, u.ID, todo.Change{Text: "Visit grandma", Labels: []string{"family"}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	later := it.UpdatedAt.Add(time.Minute)
	got, err := s.SetNotes(ctx, u.ID, it.ID, "line 1\nline 2", later)
	if err != nil {
		t.Fatal(err)
	}
	if got.Notes != "line 1\nline 2" || got.Text != "Visit grandma" || len(got.Labels) != 1 || !got.UpdatedAt.Equal(later) {
		t.Fatalf("SetNotes = %+v", got)
	}
	got, err = s.UpdateItem(ctx, u.ID, it.ID, todo.Change{Text: "Visit grandma on Sunday", KeepLabels: true}, time.Now())
	if err != nil || got.Notes != "line 1\nline 2" {
		t.Fatalf("UpdateItem: notes = %q, %v; want them kept", got.Notes, err)
	}
	if _, err := s.SetNotes(ctx, other.ID, it.ID, "x", time.Now()); !errors.Is(err, todo.ErrNotFound) {
		t.Fatalf("SetNotes of another user's item: err = %v, want ErrNotFound", err)
	}
}
