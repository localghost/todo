package todo_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"todo/internal/store/sqlite"
	"todo/internal/todo"
)

func newService(t *testing.T) *todo.Service {
	t.Helper()
	s, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return todo.NewService(s)
}

func TestAddNormalizesWhitespace(t *testing.T) {
	svc := newService(t)
	it, err := svc.Add(context.Background(), 1, "  Buy \n oat\tmilk  ")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if it.Text != "Buy oat milk" {
		t.Fatalf("Text = %q, want %q", it.Text, "Buy oat milk")
	}
}

func TestAddRejectsEmpty(t *testing.T) {
	svc := newService(t)
	for _, in := range []string{"", "   ", " \n\t "} {
		if _, err := svc.Add(context.Background(), 1, in); !errors.Is(err, todo.ErrEmptyText) {
			t.Errorf("Add(%q) err = %v, want ErrEmptyText", in, err)
		}
	}
	open, done, _ := svc.Counts(context.Background(), 1)
	if open+done != 0 {
		t.Fatalf("items stored after empty Add: open=%d done=%d", open, done)
	}
}

func TestAddAppendsAtEnd(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	svc.Add(ctx, 1, "a")
	svc.Add(ctx, 1, "b")
	items, err := svc.List(ctx, 1, false)
	if err != nil || len(items) != 2 || items[0].Text != "a" || items[1].Text != "b" {
		t.Fatalf("List = %+v, %v; want a, b", items, err)
	}
}

func TestUpdateTextRejectsEmptyAndKeepsOld(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	it, _ := svc.Add(ctx, 1, "Buy milk")
	if _, err := svc.UpdateText(ctx, 1, it.ID, "  "); !errors.Is(err, todo.ErrEmptyText) {
		t.Fatalf("UpdateText(empty) err = %v, want ErrEmptyText", err)
	}
	got, _ := svc.Get(ctx, 1, it.ID)
	if got.Text != "Buy milk" {
		t.Fatalf("Text = %q, want unchanged %q", got.Text, "Buy milk")
	}
	up, err := svc.UpdateText(ctx, 1, it.ID, " Buy  oat milk ")
	if err != nil || up.Text != "Buy oat milk" {
		t.Fatalf("UpdateText = %+v, %v", up, err)
	}
}

func TestToggleTwice(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	it, _ := svc.Add(ctx, 1, "a")
	first, err := svc.Toggle(ctx, 1, it.ID)
	if err != nil || !first.Done {
		t.Fatalf("first Toggle = %+v, %v; want done", first, err)
	}
	second, err := svc.Toggle(ctx, 1, it.ID)
	if err != nil || second.Done {
		t.Fatalf("second Toggle = %+v, %v; want not done", second, err)
	}
}

func TestOtherUserGetsNotFound(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	it, _ := svc.Add(ctx, 1, "a")
	if _, err := svc.Toggle(ctx, 2, it.ID); !errors.Is(err, todo.ErrNotFound) {
		t.Errorf("Toggle err = %v, want ErrNotFound", err)
	}
	if _, err := svc.UpdateText(ctx, 2, it.ID, "x"); !errors.Is(err, todo.ErrNotFound) {
		t.Errorf("UpdateText err = %v, want ErrNotFound", err)
	}
	if err := svc.Delete(ctx, 2, it.ID); !errors.Is(err, todo.ErrNotFound) {
		t.Errorf("Delete err = %v, want ErrNotFound", err)
	}
}

func TestDeleteRemovesItem(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	it, _ := svc.Add(ctx, 1, "a")
	if err := svc.Delete(ctx, 1, it.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := svc.Get(ctx, 1, it.ID); !errors.Is(err, todo.ErrNotFound) {
		t.Fatalf("Get after Delete err = %v, want ErrNotFound", err)
	}
}
