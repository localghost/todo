package sqlite_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"todo/internal/store/sqlite"
	"todo/internal/todo"
)

// A version 6 file gets the columns repeat and repeat_day; its items keep
// their data and have no repeat.
func TestMigrateV6ToV7AddsRepeat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v6.db")
	s, err := sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	u, err := s.CreateUser(ctx, "alice", "x", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	due := time.Date(2099, 10, 5, 9, 0, 0, 0, time.UTC)
	if _, err := s.Create(ctx, u.ID, todo.Change{Text: "Pay the rent", DueAt: &due, DueAllDay: true}, time.Now()); err != nil {
		t.Fatal(err)
	}
	s.Close()
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{`ALTER TABLE items DROP COLUMN notes`, `ALTER TABLE items DROP COLUMN repeat`, `ALTER TABLE items DROP COLUMN repeat_day`, `PRAGMA user_version = 6`} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	db.Close()

	s, err = sqlite.Open(path)
	if err != nil {
		t.Fatalf("Open v6: %v", err)
	}
	defer s.Close()
	if v := userVersion(t, path); v != 8 {
		t.Fatalf("user_version = %d, want 8", v)
	}
	items, err := s.List(ctx, u.ID, false)
	if err != nil || len(items) != 1 || items[0].Text != "Pay the rent" || items[0].Repeat != "" || items[0].RepeatDay != 0 {
		t.Fatalf("items after migration = %+v, %v", items, err)
	}
	it, err := s.UpdateItem(ctx, u.ID, items[0].ID, todo.Change{Text: "Pay the rent", DueAt: &due, DueAllDay: true,
		KeepLabels: true, Repeat: todo.RepeatMonthly, RepeatDay: 5}, time.Now())
	if err != nil || it.Repeat != todo.RepeatMonthly || it.RepeatDay != 5 {
		t.Fatalf("UpdateItem = %+v, %v", it, err)
	}
}
