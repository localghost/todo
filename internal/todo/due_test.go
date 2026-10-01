package todo_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"todo/internal/todo"
)

func TestAddWithDueDateAndTime(t *testing.T) {
	svc := newService(t)
	it, err := svc.Add(context.Background(), 1, "a", "2026-10-03", "14:00")
	want := time.Date(2026, 10, 3, 14, 0, 0, 0, time.Local)
	if err != nil || it.DueAt == nil || !it.DueAt.Equal(want) || it.DueAllDay {
		t.Fatalf("Add = %+v, %v; want due %v, not all-day", it, err, want)
	}
}

func TestAddWithDueDateOnlyIsAllDayAt0900(t *testing.T) {
	svc := newService(t)
	it, err := svc.Add(context.Background(), 1, "a", "2026-10-03", "")
	want := time.Date(2026, 10, 3, 9, 0, 0, 0, time.Local)
	if err != nil || it.DueAt == nil || !it.DueAt.Equal(want) || !it.DueAllDay {
		t.Fatalf("Add = %+v, %v; want due %v, all-day", it, err, want)
	}
}

func TestAddBadDue(t *testing.T) {
	svc := newService(t)
	cases := []struct{ date, clock, msg string }{
		{"", "14:00", "Pick a day for the due date."},
		{"2026-13-01", "", "Please choose a valid date and time."},
		{"2026-10-03", "25:00", "Please choose a valid date and time."},
		{"tomorrow", "", "Please choose a valid date and time."},
	}
	for _, c := range cases {
		_, err := svc.Add(context.Background(), 1, "a", c.date, c.clock)
		var dueErr *todo.DueError
		if !errors.Is(err, todo.ErrBadDue) || !errors.As(err, &dueErr) || dueErr.Msg != c.msg {
			t.Errorf("Add(%q, %q) err = %v, want DueError %q", c.date, c.clock, err, c.msg)
		}
	}
	if open, done, _ := svc.Counts(context.Background(), 1); open+done != 0 {
		t.Fatalf("items stored after bad due: %d", open+done)
	}
}

func TestAddEmptyTextIsCheckedFirst(t *testing.T) {
	svc := newService(t)
	if _, err := svc.Add(context.Background(), 1, "  ", "bad", ""); !errors.Is(err, todo.ErrEmptyText) {
		t.Fatalf("err = %v, want ErrEmptyText", err)
	}
}

func TestEditKeepsNotifiedUnlessDueChanges(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	it, _ := svc.Add(ctx, 1, "a", "2020-01-01", "08:00")
	if got, _ := svc.ClaimDue(ctx, 1, time.Now()); len(got) != 1 {
		t.Fatalf("first claim = %d items, want 1", len(got))
	}

	textOnly, err := svc.Edit(ctx, 1, it.ID, "b", "2020-01-01", "08:00")
	if err != nil || textOnly.Text != "b" || textOnly.NotifiedAt == nil {
		t.Fatalf("Edit text only = %+v, %v; want NotifiedAt kept", textOnly, err)
	}
	if got, _ := svc.ClaimDue(ctx, 1, time.Now()); len(got) != 0 {
		t.Fatalf("claim after text edit = %d items, want 0", len(got))
	}

	moved, err := svc.Edit(ctx, 1, it.ID, "b", "2020-01-02", "08:00")
	if err != nil || moved.NotifiedAt != nil {
		t.Fatalf("Edit due = %+v, %v; want NotifiedAt reset", moved, err)
	}
	if got, _ := svc.ClaimDue(ctx, 1, time.Now()); len(got) != 1 {
		t.Fatalf("claim after due change = %d items, want 1", len(got))
	}
}

func TestEditClearsDue(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	it, _ := svc.Add(ctx, 1, "a", "2026-10-03", "14:00")
	got, err := svc.Edit(ctx, 1, it.ID, "a", "", "")
	if err != nil || got.DueAt != nil || got.DueAllDay {
		t.Fatalf("Edit clear = %+v, %v; want no due", got, err)
	}
}

func TestEditUnchangedWithDueDoesNotWrite(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	it, _ := svc.Add(ctx, 1, "a", "2026-10-03", "")
	got, err := svc.Edit(ctx, 1, it.ID, " a ", "2026-10-03", "")
	if err != nil || !got.UpdatedAt.Equal(it.UpdatedAt) {
		t.Fatalf("Edit unchanged = %+v, %v; want no write", got, err)
	}
}

func TestEditBadDueKeepsItem(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	it, _ := svc.Add(ctx, 1, "a", "2026-10-03", "14:00")
	if _, err := svc.Edit(ctx, 1, it.ID, "b", "", "10:00"); !errors.Is(err, todo.ErrBadDue) {
		t.Fatalf("err = %v, want ErrBadDue", err)
	}
	got, _ := svc.Get(ctx, 1, it.ID)
	if got.Text != "a" || got.DueAt == nil || !got.DueAt.Equal(*it.DueAt) {
		t.Fatalf("item changed after bad edit: %+v", got)
	}
}
