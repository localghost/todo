package sqlite_test

import (
	"context"
	"errors"
	"testing"

	"todo/internal/store/sqlite"
	"todo/internal/todo"
)

// order returns the texts of user 1's items in list order.
func order(t *testing.T, s *sqlite.Store, hideDone bool) string {
	t.Helper()
	items, err := s.List(context.Background(), 1, hideDone)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	return texts(items)
}

func TestMovePlacesItem(t *testing.T) {
	ctx := context.Background()
	for _, c := range []struct {
		name, move, after, want string
	}{
		{"first", "c", "", "c,a,b"},
		{"middle", "a", "b", "b,a,c"},
		{"end", "a", "c", "b,c,a"},
		{"after itself", "b", "b", "a,b,c"},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := newStore(t)
			ids := map[string]int64{"": 0}
			for _, text := range []string{"a", "b", "c"} {
				ids[text] = mustCreate(t, s, text).ID
			}
			if err := s.Move(ctx, 1, ids[c.move], ids[c.after]); err != nil {
				t.Fatalf("Move: %v", err)
			}
			if got := order(t, s, false); got != c.want {
				t.Fatalf("order = %s, want %s", got, c.want)
			}
		})
	}
}

func TestMoveUnknownItems(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	mustCreate(t, s, "a")
	b := mustCreate(t, s, "b")
	if _, err := s.CreateUser(ctx, "bob", "x", t0); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	bobs, err := s.Create(ctx, 2, todo.Change{Text: "bob's"}, t0)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	for _, c := range []struct {
		name      string
		id, after int64
	}{
		{"unknown id", 999, 0},
		{"unknown after", b.ID, 999},
		{"other user's id", bobs.ID, 0},
		{"other user's after", b.ID, bobs.ID},
	} {
		if err := s.Move(ctx, 1, c.id, c.after); !errors.Is(err, todo.ErrNotFound) {
			t.Errorf("%s: err = %v, want ErrNotFound", c.name, err)
		}
	}
	if got := order(t, s, false); got != "a,b" {
		t.Fatalf("order = %s, want a,b unchanged", got)
	}
}

func TestMoveKeepsHiddenDoneItemsInPlace(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	a := mustCreate(t, s, "a")
	done := mustCreate(t, s, "done")
	mustCreate(t, s, "b")
	c := mustCreate(t, s, "c")
	if _, err := s.SetDone(ctx, 1, done.ID, true, t0); err != nil {
		t.Fatalf("SetDone: %v", err)
	}
	// The page hides "done" and shows a, b, c. The user drags c after a.
	if err := s.Move(ctx, 1, c.ID, a.ID); err != nil {
		t.Fatalf("Move: %v", err)
	}
	if got := order(t, s, true); got != "a,c,b" {
		t.Fatalf("visible order = %s, want a,c,b", got)
	}
	if got := order(t, s, false); got != "a,c,done,b" {
		t.Fatalf("full order = %s, want a,c,done,b", got)
	}
}

func TestCreateAfterMoveGoesToEnd(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	a := mustCreate(t, s, "a")
	mustCreate(t, s, "b")
	if err := s.Move(ctx, 1, a.ID, 0); err != nil {
		t.Fatalf("Move: %v", err)
	}
	mustCreate(t, s, "new")
	if got := order(t, s, false); got != "a,b,new" {
		t.Fatalf("order = %s, want a,b,new", got)
	}
}
