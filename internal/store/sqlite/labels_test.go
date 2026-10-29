package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"todo/internal/store/sqlite"
	"todo/internal/todo"
)

func labelNames(it todo.Item) string { return todo.LabelNames(it.Labels) }

func mustLabeled(t *testing.T, s *sqlite.Store, text string, labels ...string) todo.Item {
	t.Helper()
	it, err := s.Create(context.Background(), 1, todo.Change{Text: text, Labels: labels}, t0)
	if err != nil {
		t.Fatalf("Create(%q): %v", text, err)
	}
	return it
}

func TestCreateWithLabelsReusesLabelsIgnoringCase(t *testing.T) {
	s := newStore(t)
	a := mustLabeled(t, s, "Invoice", "work", "money")
	b := mustLabeled(t, s, "Report", "Work")
	if got := labelNames(a); got != "money, work" {
		t.Fatalf("labels of a = %q, want sorted %q", got, "money, work")
	}
	if got := labelNames(b); got != "work" {
		t.Fatalf("labels of b = %q, want the first spelling %q", got, "work")
	}
	labels, err := s.Labels(context.Background(), 1)
	if err != nil || len(labels) != 2 {
		t.Fatalf("Labels = %+v, %v; want 2", labels, err)
	}
	if labels[1].Name != "work" || labels[1].Items != 2 {
		t.Fatalf("work = %+v, want 2 items", labels[1])
	}
	// New labels get the colors in turn.
	if labels[0].Color != todo.LabelColors[1] || labels[1].Color != todo.LabelColors[0] {
		t.Fatalf("colors = %s, %s", labels[0].Color, labels[1].Color)
	}
}

func TestUpdateItemReplacesOrKeepsLabels(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	it := mustLabeled(t, s, "Invoice", "work", "money")
	it, err := s.UpdateItem(ctx, 1, it.ID, todo.Change{Text: "Invoice", Labels: []string{"home"}}, t0)
	if err != nil || labelNames(it) != "home" {
		t.Fatalf("after replace: %q, %v; want home", labelNames(it), err)
	}
	it, err = s.UpdateItem(ctx, 1, it.ID, todo.Change{Text: "Invoice 2", KeepLabels: true}, t0)
	if err != nil || labelNames(it) != "home" || it.Text != "Invoice 2" {
		t.Fatalf("after keep: %+v, %v", it, err)
	}
	// The old labels stay, without items, until the user deletes them.
	labels, _ := s.Labels(ctx, 1)
	if len(labels) != 3 {
		t.Fatalf("labels = %+v, want 3", labels)
	}
}

func TestListLabelAndCountsLabel(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	a := mustLabeled(t, s, "A", "work")
	mustLabeled(t, s, "B", "home")
	c := mustLabeled(t, s, "C", "work", "home")
	if _, err := s.SetDone(ctx, 1, c.ID, true, t0); err != nil {
		t.Fatal(err)
	}
	work := a.Labels[0].ID
	items, err := s.ListLabel(ctx, 1, false, work)
	if err != nil || len(items) != 2 || items[0].Text != "A" || items[1].Text != "C" {
		t.Fatalf("ListLabel(work) = %+v, %v", items, err)
	}
	if labelNames(items[1]) != "home, work" {
		t.Fatalf("filtered item keeps all its labels: %q", labelNames(items[1]))
	}
	items, _ = s.ListLabel(ctx, 1, true, work)
	if len(items) != 1 || items[0].Text != "A" {
		t.Fatalf("ListLabel(work, hide done) = %+v", items)
	}
	open, done, err := s.CountsLabel(ctx, 1, work)
	if err != nil || open != 1 || done != 1 {
		t.Fatalf("CountsLabel(work) = %d, %d, %v; want 1, 1", open, done, err)
	}
	open, done, _ = s.CountsLabel(ctx, 1, 0)
	if open != 2 || done != 1 {
		t.Fatalf("CountsLabel(0) = %d, %d; want 2, 1", open, done)
	}
}

func TestLabelsOfAnotherUserAreHidden(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	bob, err := s.CreateUser(ctx, "bob", "x", t0)
	if err != nil {
		t.Fatal(err)
	}
	a := mustLabeled(t, s, "Alice's", "work")
	b, err := s.Create(ctx, bob.ID, todo.Change{Text: "Bob's", Labels: []string{"work"}}, t0)
	if err != nil {
		t.Fatal(err)
	}
	if a.Labels[0].ID == b.Labels[0].ID {
		t.Fatal("two users share one label")
	}
	bobLabel := b.Labels[0].ID
	if _, err := s.Label(ctx, 1, bobLabel); !errors.Is(err, todo.ErrNotFound) {
		t.Fatalf("Label(bob's) for alice: %v, want ErrNotFound", err)
	}
	if err := s.SetLabelColor(ctx, 1, bobLabel, "#d1382e"); !errors.Is(err, todo.ErrNotFound) {
		t.Fatalf("SetLabelColor(bob's) for alice: %v, want ErrNotFound", err)
	}
	if err := s.DeleteLabel(ctx, 1, bobLabel); !errors.Is(err, todo.ErrNotFound) {
		t.Fatalf("DeleteLabel(bob's) for alice: %v, want ErrNotFound", err)
	}
	if items, _ := s.ListLabel(ctx, 1, false, bobLabel); len(items) != 0 {
		t.Fatalf("alice sees items with bob's label: %+v", items)
	}
}

func TestSetLabelColorKeepsName(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	it := mustLabeled(t, s, "A", "work")
	if err := s.SetLabelColor(ctx, 1, it.Labels[0].ID, "#6b44bb"); err != nil {
		t.Fatal(err)
	}
	l, err := s.Label(ctx, 1, it.Labels[0].ID)
	if err != nil || l.Name != "work" || l.Color != "#6b44bb" || l.Items != 1 {
		t.Fatalf("label = %+v, %v", l, err)
	}
}

func TestDeleteLabelKeepsItemsAndCascades(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	it := mustLabeled(t, s, "A", "work", "home")
	work := it.Labels[1]
	if err := s.DeleteLabel(ctx, 1, work.ID); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, 1, it.ID)
	if err != nil || labelNames(got) != "home" {
		t.Fatalf("item after label delete = %q, %v; want home", labelNames(got), err)
	}
	// Deleting the item removes its links; the label stays.
	if err := s.Delete(ctx, 1, it.ID); err != nil {
		t.Fatal(err)
	}
	labels, _ := s.Labels(ctx, 1)
	if len(labels) != 1 || labels[0].Items != 0 {
		t.Fatalf("labels after item delete = %+v", labels)
	}
}

func TestTooManyLabelsPerUser(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	for i := range todo.MaxLabelsPerUser / 5 {
		var names []string
		for j := range 5 {
			names = append(names, fmt.Sprintf("l%d-%d", i, j))
		}
		mustLabeled(t, s, "x", names...)
	}
	_, err := s.Create(ctx, 1, todo.Change{Text: "one more", Labels: []string{"l0-0", "new"}}, t0)
	if !errors.Is(err, todo.ErrTooManyLabels) {
		t.Fatalf("51st label: %v, want ErrTooManyLabels", err)
	}
	// Nothing was saved: the item and the label are rolled back.
	open, _, _ := s.Counts(ctx, 1)
	if open != todo.MaxLabelsPerUser/5 {
		t.Fatalf("open = %d after refused create", open)
	}
	// Existing labels still work.
	if _, err := s.Create(ctx, 1, todo.Change{Text: "reuse", Labels: []string{"L0-0"}}, t0); err != nil {
		t.Fatalf("reuse at the limit: %v", err)
	}
}

func TestMigrateV3ToV4KeepsItems(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v3.db")
	s, err := sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateUser(context.Background(), "alice", "x", t0); err != nil {
		t.Fatal(err)
	}
	mustCreate(t, s, "Keep me")
	s.Close()
	// Turn the file back into version 3.
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{`DROP TABLE item_labels`, `DROP TABLE labels`, `ALTER TABLE users DROP COLUMN sort_due`, `PRAGMA user_version = 3`} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	db.Close()

	s, err = sqlite.Open(path)
	if err != nil {
		t.Fatalf("Open v3: %v", err)
	}
	defer s.Close()
	if v := userVersion(t, path); v != 6 {
		t.Fatalf("user_version = %d, want 6", v)
	}
	items, err := s.List(context.Background(), 1, false)
	if err != nil || len(items) != 1 || items[0].Text != "Keep me" || len(items[0].Labels) != 0 {
		t.Fatalf("items after migration = %+v, %v", items, err)
	}
	if _, err := s.Create(context.Background(), 1, todo.Change{Text: "new", Labels: []string{"work"}}, t0); err != nil {
		t.Fatalf("labels after migration: %v", err)
	}
}

func TestMigrateV4ToV5TurnsColorNamesIntoHex(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v4.db")
	s, err := sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateUser(context.Background(), "alice", "x", t0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(context.Background(), 1, todo.Change{Text: "A", Labels: []string{"work", "home", "own"}}, t0); err != nil {
		t.Fatal(err)
	}
	s.Close()
	// Turn the file back into version 4, with color names.
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`UPDATE labels SET color = 'blue' WHERE name = 'work'`,
		`UPDATE labels SET color = 'yellow' WHERE name = 'home'`,
		`UPDATE labels SET color = '#123456' WHERE name = 'own'`,
		`ALTER TABLE users DROP COLUMN sort_due`,
		`PRAGMA user_version = 4`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	db.Close()

	s, err = sqlite.Open(path)
	if err != nil {
		t.Fatalf("Open v4: %v", err)
	}
	defer s.Close()
	if v := userVersion(t, path); v != 6 {
		t.Fatalf("user_version = %d, want 6", v)
	}
	labels, err := s.Labels(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, l := range labels {
		got[l.Name] = l.Color
	}
	if got["work"] != "#2555da" || got["home"] != "#e9bd16" || got["own"] != "#123456" {
		t.Fatalf("colors after migration = %v", got)
	}
}
