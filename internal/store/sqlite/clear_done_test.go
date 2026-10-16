package sqlite_test

import (
	"context"
	"testing"

	"todo/internal/todo"
)

// DeleteDone removes the user's done items: all of them, or with labelID
// only the ones with that label. Open items and other users' items stay.
func TestDeleteDone(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	bob, err := s.CreateUser(ctx, "bob", "x", t0)
	if err != nil {
		t.Fatal(err)
	}
	done := func(userID int64, text string, labels ...string) todo.Item {
		t.Helper()
		it, err := s.Create(ctx, userID, todo.Change{Text: text, Labels: labels}, t0)
		if err != nil {
			t.Fatal(err)
		}
		if it, err = s.SetDone(ctx, userID, it.ID, true, t0); err != nil {
			t.Fatal(err)
		}
		return it
	}
	mustCreate(t, s, "open")
	work := done(1, "done work", "work")
	done(1, "done home", "home")
	done(bob.ID, "bob done")

	n, err := s.DeleteDone(ctx, 1, work.Labels[0].ID)
	if err != nil || n != 1 {
		t.Fatalf("DeleteDone(work) = %d, %v; want 1", n, err)
	}
	items, _ := s.List(ctx, 1, false)
	if got := texts(items); got != "open,done home" {
		t.Fatalf("after DeleteDone(work) items = %q, want open,done home", got)
	}
	labels, _ := s.Labels(ctx, 1)
	for _, l := range labels {
		if l.Name == "work" && l.Items != 0 {
			t.Errorf("label work still counts %d items", l.Items)
		}
	}

	n, err = s.DeleteDone(ctx, 1, 0)
	if err != nil || n != 1 {
		t.Fatalf("DeleteDone(all) = %d, %v; want 1", n, err)
	}
	if items, _ = s.List(ctx, 1, false); texts(items) != "open" {
		t.Fatalf("after DeleteDone(all) items = %q, want open", texts(items))
	}
	if items, _ = s.List(ctx, bob.ID, false); texts(items) != "bob done" {
		t.Fatalf("bob's items = %q, want bob done", texts(items))
	}
	if n, err = s.DeleteDone(ctx, 1, 0); err != nil || n != 0 {
		t.Fatalf("DeleteDone(nothing) = %d, %v; want 0, nil", n, err)
	}
}
