package todo_test

import (
	"context"
	"errors"
	"testing"
	"time"

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
	if err := svc.Move(ctx, 1, b.ID, 0, time.Now()); err != nil {
		t.Fatalf("Move: %v", err)
	}
	if got := listOrder(t, svc); len(got) != 2 || got[0] != "b" || got[1] != "a" {
		t.Fatalf("order = %v, want [b a]", got)
	}
}

func TestMoveOverdueItemFails(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	svc.Add(ctx, 1, "a", "", "")
	late, _ := svc.Add(ctx, 1, "late", "2020-01-01", "08:00")
	if err := svc.Move(ctx, 1, late.ID, 0, time.Now()); !errors.Is(err, todo.ErrCannotMove) {
		t.Fatalf("Move(overdue) err = %v, want ErrCannotMove", err)
	}
	if got := listOrder(t, svc); len(got) != 2 || got[0] != "a" || got[1] != "late" {
		t.Fatalf("order = %v, want [a late] unchanged", got)
	}
}

func TestMoveAllDayUsesZoneOfNow(t *testing.T) {
	warsaw, err := time.LoadLocation("Europe/Warsaw")
	if err != nil {
		t.Fatal(err)
	}
	svc := newService(t)
	ctx := context.Background()
	svc.Add(ctx, 1, "a", "", "")
	rent, err := svc.AddIn(ctx, 1, "rent", "2026-10-01", "", warsaw)
	if err != nil {
		t.Fatal(err)
	}
	// 23:30 on the due day in Warsaw: not overdue yet, so it can move.
	if err := svc.Move(ctx, 1, rent.ID, 0, time.Date(2026, 10, 1, 23, 30, 0, 0, warsaw)); err != nil {
		t.Fatalf("Move on the due day: %v", err)
	}
	// 00:30 on the next day in Warsaw: overdue, so it cannot move.
	if err := svc.Move(ctx, 1, rent.ID, 0, time.Date(2026, 10, 2, 0, 30, 0, 0, warsaw)); !errors.Is(err, todo.ErrCannotMove) {
		t.Fatalf("Move on the next day err = %v, want ErrCannotMove", err)
	}
}

func TestMoveUnknownItem(t *testing.T) {
	svc := newService(t)
	if err := svc.Move(context.Background(), 1, 999, 0, time.Now()); !errors.Is(err, todo.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
