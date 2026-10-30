package todo_test

import (
	"context"
	"errors"
	"testing"

	"todo/internal/todo"
)

// listOrder returns the texts of user 1's items in stored order.
func listOrder(t *testing.T, svc *todo.Service) []string {
	t.Helper()
	items, err := svc.List(context.Background(), 1, false)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	out := []string{}
	for _, it := range items {
		out = append(out, it.Text)
	}
	return out
}

func TestMoveChangesOrder(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	svc.Add(ctx, 1, "a", "", "")
	b, _ := svc.Add(ctx, 1, "b", "", "")
	if err := svc.Move(ctx, 1, b.ID, 0); err != nil {
		t.Fatalf("Move: %v", err)
	}
	if got := listOrder(t, svc); len(got) != 2 || got[0] != "b" || got[1] != "a" {
		t.Fatalf("order = %v, want [b a]", got)
	}
}

// An overdue item moves like any other item: only "Sort by due date" puts the
// overdue items first.
func TestMoveOverdueItem(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	svc.Add(ctx, 1, "a", "", "")
	late, _ := svc.Add(ctx, 1, "late", "2020-01-01", "08:00")
	if err := svc.Move(ctx, 1, late.ID, 0); err != nil {
		t.Fatalf("Move(overdue): %v", err)
	}
	if got := listOrder(t, svc); len(got) != 2 || got[0] != "late" || got[1] != "a" {
		t.Fatalf("order = %v, want [late a]", got)
	}
}

func TestMoveUnknownItem(t *testing.T) {
	svc := newService(t)
	if err := svc.Move(context.Background(), 1, 999, 0); !errors.Is(err, todo.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
