package todo_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"todo/internal/todo"
)

func TestPostponeOverdueCountsFromNow(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	it, _ := svc.Add(ctx, 1, "a", "2020-01-01", "08:00")
	svc.ClaimDue(ctx, 1, time.Now())

	before := time.Now()
	got, err := svc.Postpone(ctx, 1, it.ID, 10)
	after := time.Now()
	if err != nil {
		t.Fatalf("Postpone: %v", err)
	}
	if got.DueAt == nil || got.DueAt.Before(before.Add(10*time.Minute).Truncate(time.Second)) || got.DueAt.After(after.Add(10*time.Minute)) {
		t.Fatalf("DueAt = %v, want now + 10 min", got.DueAt)
	}
	if got.DueAllDay || got.NotifiedAt != nil {
		t.Fatalf("item = %+v, want timed and not notified", got)
	}
}

func TestPostponeNotYetDueCountsFromDueTime(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	it, _ := svc.Add(ctx, 1, "a", "2099-10-03", "14:00")
	got, err := svc.Postpone(ctx, 1, it.ID, 30)
	want := time.Date(2099, 10, 3, 14, 30, 0, 0, time.Local)
	if err != nil || got.DueAt == nil || !got.DueAt.Equal(want) {
		t.Fatalf("Postpone = %+v, %v; want due %v", got, err, want)
	}
}

func TestPostponeAllDayBecomesTimed(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	it, _ := svc.Add(ctx, 1, "a", "2099-10-03", "")
	got, err := svc.Postpone(ctx, 1, it.ID, 5)
	want := time.Date(2099, 10, 3, 9, 5, 0, 0, time.Local)
	if err != nil || got.DueAllDay || got.DueAt == nil || !got.DueAt.Equal(want) {
		t.Fatalf("Postpone = %+v, %v; want timed due %v", got, err, want)
	}
}

func TestPostponeErrors(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	noDue, _ := svc.Add(ctx, 1, "no due", "", "")
	done, _ := svc.Add(ctx, 1, "done", "2020-01-01", "08:00")
	svc.Toggle(ctx, 1, done.ID)
	due, _ := svc.Add(ctx, 1, "due", "2020-01-01", "08:00")

	if _, err := svc.Postpone(ctx, 1, noDue.ID, 5); !errors.Is(err, todo.ErrCannotPostpone) {
		t.Errorf("no due: err = %v, want ErrCannotPostpone", err)
	}
	if _, err := svc.Postpone(ctx, 1, done.ID, 5); !errors.Is(err, todo.ErrCannotPostpone) {
		t.Errorf("done: err = %v, want ErrCannotPostpone", err)
	}
	if _, err := svc.Postpone(ctx, 1, due.ID, 7); !errors.Is(err, todo.ErrBadPostpone) {
		t.Errorf("7 minutes: err = %v, want ErrBadPostpone", err)
	}
	if _, err := svc.Postpone(ctx, 2, due.ID, 5); !errors.Is(err, todo.ErrNotFound) {
		t.Errorf("other user: err = %v, want ErrNotFound", err)
	}
}
