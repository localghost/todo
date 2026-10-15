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

func TestEditTextKeepsDueAndNotified(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	it, _ := svc.Add(ctx, 1, "a", "2020-01-01", "08:00")
	if got, _ := svc.ClaimDue(ctx, 1, time.Now()); len(got) != 1 {
		t.Fatalf("first claim = %d items, want 1", len(got))
	}
	got, err := svc.EditText(ctx, 1, it.ID, "b")
	if err != nil || got.Text != "b" || got.NotifiedAt == nil || got.DueAt == nil || !got.DueAt.Equal(*it.DueAt) {
		t.Fatalf("EditText = %+v, %v; want text b, the same due date, NotifiedAt kept", got, err)
	}
	if got, _ := svc.ClaimDue(ctx, 1, time.Now()); len(got) != 0 {
		t.Fatalf("claim after text edit = %d items, want 0", len(got))
	}
}

func TestSetDueResetsNotified(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	it, _ := svc.Add(ctx, 1, "a", "2020-01-01", "08:00")
	svc.ClaimDue(ctx, 1, time.Now())
	moved, err := svc.SetDue(ctx, 1, it.ID, "2020-01-02", "08:00", nil)
	if err != nil || moved.NotifiedAt != nil || moved.Text != "a" {
		t.Fatalf("SetDue = %+v, %v; want NotifiedAt reset, text kept", moved, err)
	}
	if got, _ := svc.ClaimDue(ctx, 1, time.Now()); len(got) != 1 {
		t.Fatalf("claim after due change = %d items, want 1", len(got))
	}
}

func TestSetDueEmptyRemovesDue(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	it, _ := svc.Add(ctx, 1, "a", "2026-10-03", "14:00")
	got, err := svc.SetDue(ctx, 1, it.ID, "", "", nil)
	if err != nil || got.DueAt != nil || got.DueAllDay {
		t.Fatalf("SetDue empty = %+v, %v; want no due", got, err)
	}
}

func TestSetDueUnchangedDoesNotWrite(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	it, _ := svc.Add(ctx, 1, "a", "2026-10-03", "")
	got, err := svc.SetDue(ctx, 1, it.ID, " 2026-10-03 ", "", nil)
	if err != nil || !got.UpdatedAt.Equal(it.UpdatedAt) {
		t.Fatalf("SetDue unchanged = %+v, %v; want no write", got, err)
	}
}

func TestSetDueBadKeepsItem(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	it, _ := svc.Add(ctx, 1, "a", "2026-10-03", "14:00")
	if _, err := svc.SetDue(ctx, 1, it.ID, "", "10:00", nil); !errors.Is(err, todo.ErrBadDue) {
		t.Fatalf("err = %v, want ErrBadDue", err)
	}
	got, _ := svc.Get(ctx, 1, it.ID)
	if got.DueAt == nil || !got.DueAt.Equal(*it.DueAt) {
		t.Fatalf("item changed after bad SetDue: %+v", got)
	}
}

func TestAddRejectsOutOfRangeTime(t *testing.T) {
	svc := newService(t)
	for _, clock := range []string{"25:00", "14:60"} {
		_, err := svc.Add(context.Background(), 1, "a", "2026-10-03", clock)
		var dueErr *todo.DueError
		if !errors.As(err, &dueErr) || dueErr.Msg != "Please choose a valid date and time." {
			t.Errorf("Add with time %q: err = %v, want the invalid date/time message", clock, err)
		}
	}
}

func TestAddInAndSetDueReadTheGivenZone(t *testing.T) {
	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	svc := newService(t)
	ctx := context.Background()
	it, err := svc.AddIn(ctx, 1, "a", "2026-10-03", "14:00", tokyo)
	want := time.Date(2026, 10, 3, 5, 0, 0, 0, time.UTC)
	if err != nil || it.DueAt == nil || !it.DueAt.Equal(want) {
		t.Fatalf("AddIn = %+v, %v; want due %v", it, err, want)
	}
	it, err = svc.SetDue(ctx, 1, it.ID, "2026-10-03", "", tokyo) // all-day: 09:00 Tokyo
	want = time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	if err != nil || it.DueAt == nil || !it.DueAt.Equal(want) || !it.DueAllDay {
		t.Fatalf("SetDue = %+v, %v; want all-day due %v", it, err, want)
	}
	it, err = svc.AddIn(ctx, 1, "b", "2026-10-03", "14:00", nil) // nil: the service default
	want = time.Date(2026, 10, 3, 14, 0, 0, 0, time.Local)
	if err != nil || it.DueAt == nil || !it.DueAt.Equal(want) {
		t.Fatalf("AddIn(nil) = %+v, %v; want due %v", it, err, want)
	}
}
