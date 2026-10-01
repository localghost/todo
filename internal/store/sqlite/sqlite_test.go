package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"todo/internal/store/sqlite"
	"todo/internal/todo"
)

var t0 = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

func newStore(t *testing.T) *sqlite.Store {
	t.Helper()
	s, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func mustCreate(t *testing.T, s *sqlite.Store, text string) todo.Item {
	t.Helper()
	it, err := s.Create(context.Background(), 1, todo.Change{Text: text}, t0)
	if err != nil {
		t.Fatalf("Create(%q): %v", text, err)
	}
	return it
}

func TestOpenTwiceKeepsData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	if _, err := s.Create(context.Background(), 1, todo.Change{Text: "Buy milk"}, t0); err != nil {
		t.Fatalf("Create: %v", err)
	}
	s.Close()

	s, err = sqlite.Open(path)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer s.Close()
	items, err := s.List(context.Background(), 1, false)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 1 || items[0].Text != "Buy milk" {
		t.Fatalf("items = %+v, want one item %q", items, "Buy milk")
	}
}

func TestOpenMissingDir(t *testing.T) {
	_, err := sqlite.Open(filepath.Join(t.TempDir(), "no-such-dir", "test.db"))
	if err == nil {
		t.Fatal("Open in a missing folder: got nil error, want error")
	}
	if msg := err.Error(); !strings.Contains(msg, "does not exist") || strings.Contains(msg, "out of memory") {
		t.Fatalf("error = %q, want a clear 'does not exist' message", msg)
	}
}

func TestOpenRejectsDSNCharacters(t *testing.T) {
	for _, name := range []string{"a?b.db", "a#b.db"} {
		_, err := sqlite.Open(filepath.Join(t.TempDir(), name))
		if err == nil || !strings.Contains(err.Error(), "must not contain") {
			t.Errorf("Open(%q) err = %v, want a 'must not contain' error", name, err)
		}
	}
}

func TestCreateSetsFields(t *testing.T) {
	s := newStore(t)
	it := mustCreate(t, s, "Buy milk")
	if it.ID == 0 || it.UserID != 1 || it.Text != "Buy milk" || it.Done || it.Position != 1 {
		t.Fatalf("item = %+v", it)
	}
	if !it.CreatedAt.Equal(t0) || !it.UpdatedAt.Equal(t0) {
		t.Fatalf("times = %v, %v; want %v", it.CreatedAt, it.UpdatedAt, t0)
	}
}

func TestCreateAppendsAtEnd(t *testing.T) {
	s := newStore(t)
	a := mustCreate(t, s, "a")
	b := mustCreate(t, s, "b")
	if b.Position != a.Position+1 {
		t.Fatalf("positions = %d, %d; want consecutive", a.Position, b.Position)
	}
}

func TestListOrderAndHideDone(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	a := mustCreate(t, s, "a")
	mustCreate(t, s, "b")
	mustCreate(t, s, "c")
	if _, err := s.SetDone(ctx, 1, a.ID, true, t0); err != nil {
		t.Fatalf("SetDone: %v", err)
	}

	all, err := s.List(ctx, 1, false)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got := texts(all); got != "a,b,c" {
		t.Fatalf("List(all) = %s, want a,b,c", got)
	}
	open, err := s.List(ctx, 1, true)
	if err != nil {
		t.Fatalf("List(hideDone): %v", err)
	}
	if got := texts(open); got != "b,c" {
		t.Fatalf("List(hideDone) = %s, want b,c", got)
	}
}

func TestListEmptyIsNotNil(t *testing.T) {
	s := newStore(t)
	items, err := s.List(context.Background(), 1, false)
	if err != nil || items == nil || len(items) != 0 {
		t.Fatalf("List = %v, %v; want empty non-nil slice", items, err)
	}
}

func TestGetUnknownAndOtherUser(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	it := mustCreate(t, s, "a")
	if _, err := s.Get(ctx, 1, 999); !errors.Is(err, todo.ErrNotFound) {
		t.Fatalf("Get(999) err = %v, want ErrNotFound", err)
	}
	if _, err := s.Get(ctx, 2, it.ID); !errors.Is(err, todo.ErrNotFound) {
		t.Fatalf("Get(other user) err = %v, want ErrNotFound", err)
	}
}

func TestUpdateTextAndSetDone(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	it := mustCreate(t, s, "a")
	t1 := t0.Add(time.Hour)

	up, err := s.UpdateItem(ctx, 1, it.ID, todo.Change{Text: "b"}, t1)
	if err != nil || up.Text != "b" || !up.UpdatedAt.Equal(t1) {
		t.Fatalf("UpdateText = %+v, %v", up, err)
	}
	done, err := s.SetDone(ctx, 1, it.ID, true, t1)
	if err != nil || !done.Done {
		t.Fatalf("SetDone = %+v, %v", done, err)
	}
	if _, err := s.UpdateItem(ctx, 2, it.ID, todo.Change{Text: "x"}, t1); !errors.Is(err, todo.ErrNotFound) {
		t.Fatalf("UpdateText(other user) err = %v, want ErrNotFound", err)
	}
	if _, err := s.SetDone(ctx, 1, 999, true, t1); !errors.Is(err, todo.ErrNotFound) {
		t.Fatalf("SetDone(999) err = %v, want ErrNotFound", err)
	}
}

func TestDelete(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	it := mustCreate(t, s, "a")
	if err := s.Delete(ctx, 2, it.ID); !errors.Is(err, todo.ErrNotFound) {
		t.Fatalf("Delete(other user) err = %v, want ErrNotFound", err)
	}
	if err := s.Delete(ctx, 1, it.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := s.Delete(ctx, 1, it.ID); !errors.Is(err, todo.ErrNotFound) {
		t.Fatalf("second Delete err = %v, want ErrNotFound", err)
	}
}

func TestCounts(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	open, done, err := s.Counts(ctx, 1)
	if err != nil || open != 0 || done != 0 {
		t.Fatalf("Counts(empty) = %d, %d, %v", open, done, err)
	}
	a := mustCreate(t, s, "a")
	mustCreate(t, s, "b")
	if _, err := s.SetDone(ctx, 1, a.ID, true, t0); err != nil {
		t.Fatalf("SetDone: %v", err)
	}
	open, done, err = s.Counts(ctx, 1)
	if err != nil || open != 1 || done != 1 {
		t.Fatalf("Counts = %d, %d, %v; want 1, 1", open, done, err)
	}
}

func texts(items []todo.Item) string {
	out := ""
	for i, it := range items {
		if i > 0 {
			out += ","
		}
		out += it.Text
	}
	return out
}

func TestDueFieldsRoundTrip(t *testing.T) {
	s := newStore(t)
	due := t0.Add(2 * time.Hour)
	it, err := s.Create(context.Background(), 1, todo.Change{Text: "a", DueAt: &due, DueAllDay: true}, t0)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if it.DueAt == nil || !it.DueAt.Equal(due) || !it.DueAllDay || it.NotifiedAt != nil {
		t.Fatalf("item = %+v", it)
	}
	plain := mustCreate(t, s, "b")
	if plain.DueAt != nil || plain.DueAllDay {
		t.Fatalf("item without due = %+v", plain)
	}
}

func TestClaimDue(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	past, future := t0.Add(-time.Hour), t0.Add(time.Hour)
	due, _ := s.Create(ctx, 1, todo.Change{Text: "due", DueAt: &past}, t0)
	done, _ := s.Create(ctx, 1, todo.Change{Text: "done", DueAt: &past}, t0)
	if _, err := s.SetDone(ctx, 1, done.ID, true, t0); err != nil {
		t.Fatalf("SetDone: %v", err)
	}
	gone, _ := s.Create(ctx, 1, todo.Change{Text: "deleted", DueAt: &past}, t0)
	if err := s.Delete(ctx, 1, gone.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	s.Create(ctx, 1, todo.Change{Text: "future", DueAt: &future}, t0)
	s.Create(ctx, 1, todo.Change{Text: "no due"}, t0)

	got, err := s.ClaimDue(ctx, 1, t0)
	if err != nil || len(got) != 1 || got[0].ID != due.ID || got[0].NotifiedAt == nil {
		t.Fatalf("ClaimDue = %+v, %v; want only %q, notified", got, err, "due")
	}
	again, err := s.ClaimDue(ctx, 1, t0)
	if err != nil || len(again) != 0 {
		t.Fatalf("second ClaimDue = %+v, %v; want none", again, err)
	}
	if other, _ := s.ClaimDue(ctx, 2, t0); len(other) != 0 {
		t.Fatalf("ClaimDue(other user) = %+v; want none", other)
	}
}

func TestClaimDueAtExactSecond(t *testing.T) {
	s := newStore(t)
	due := t0
	s.Create(context.Background(), 1, todo.Change{Text: "a", DueAt: &due}, t0)
	got, err := s.ClaimDue(context.Background(), 1, t0.Add(500*time.Millisecond))
	if err != nil || len(got) != 1 {
		t.Fatalf("ClaimDue at due time = %+v, %v; want 1 item", got, err)
	}
}

func TestUpdateItemResetsNotified(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	past := t0.Add(-time.Hour)
	it, _ := s.Create(ctx, 1, todo.Change{Text: "a", DueAt: &past}, t0)
	s.ClaimDue(ctx, 1, t0)

	kept, err := s.UpdateItem(ctx, 1, it.ID, todo.Change{Text: "b", DueAt: &past}, t0)
	if err != nil || kept.NotifiedAt == nil {
		t.Fatalf("UpdateItem without reset = %+v, %v; want NotifiedAt kept", kept, err)
	}
	reset, err := s.UpdateItem(ctx, 1, it.ID, todo.Change{Text: "b", DueAt: &past, ResetNotified: true}, t0)
	if err != nil || reset.NotifiedAt != nil {
		t.Fatalf("UpdateItem with reset = %+v, %v; want NotifiedAt nil", reset, err)
	}
}

func TestOpenMigratesOldDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL)`,
		`INSERT INTO users (id, name) VALUES (1, 'default')`,
		`CREATE TABLE items (id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id),
			text TEXT NOT NULL, done INTEGER NOT NULL DEFAULT 0, position INTEGER NOT NULL,
			created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`INSERT INTO items (user_id, text, done, position, created_at, updated_at)
			VALUES (1, 'Old item', 0, 1, '2026-09-01T10:00:00Z', '2026-09-01T10:00:00Z')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("old schema: %v", err)
		}
	}
	db.Close()

	s, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("Open old database: %v", err)
	}
	defer s.Close()
	items, err := s.List(context.Background(), 1, false)
	if err != nil || len(items) != 1 || items[0].Text != "Old item" || items[0].DueAt != nil {
		t.Fatalf("List = %+v, %v; want the old item without due date", items, err)
	}
	due := t0
	if _, err := s.UpdateItem(context.Background(), 1, items[0].ID, todo.Change{Text: "Old item", DueAt: &due}, t0); err != nil {
		t.Fatalf("UpdateItem with due on migrated database: %v", err)
	}
}
