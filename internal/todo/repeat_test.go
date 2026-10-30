package todo_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"todo/internal/todo"
)

// Canvas page "20 · Repeat", board "repeating items", option R1, and its
// table "The next date".
func TestNextDue(t *testing.T) {
	warsaw, err := time.LoadLocation("Europe/Warsaw")
	if err != nil {
		t.Fatal(err)
	}
	at := func(y int, m time.Month, d, h, min int) time.Time { return time.Date(y, m, d, h, min, 0, 0, warsaw) }
	cases := []struct {
		name string
		due  time.Time
		rule string
		day  int
		now  time.Time
		want time.Time
	}{
		{"weekly, on the day", at(2026, 11, 2, 18, 0), todo.RepeatWeekly, 0, at(2026, 11, 2, 17, 0), at(2026, 11, 9, 18, 0)},
		{"weekly, a day early", at(2026, 11, 2, 18, 0), todo.RepeatWeekly, 0, at(2026, 11, 1, 12, 0), at(2026, 11, 9, 18, 0)},
		{"daily, 3 days late, all day", at(2026, 11, 1, 9, 0), todo.RepeatDaily, 0, at(2026, 11, 4, 15, 0), at(2026, 11, 5, 9, 0)},
		{"daily, before its time today", at(2026, 11, 4, 18, 0), todo.RepeatDaily, 0, at(2026, 11, 4, 10, 0), at(2026, 11, 5, 18, 0)},
		{"daily over the end of summer time", at(2026, 10, 24, 18, 0), todo.RepeatDaily, 0, at(2026, 10, 24, 19, 0), at(2026, 10, 25, 18, 0)},
		{"monthly, 31 Jan", at(2027, 1, 31, 9, 0), todo.RepeatMonthly, 31, at(2027, 1, 31, 10, 0), at(2027, 2, 28, 9, 0)},
		{"monthly, back to the 31st", at(2027, 2, 28, 9, 0), todo.RepeatMonthly, 31, at(2027, 2, 28, 10, 0), at(2027, 3, 31, 9, 0)},
		{"monthly, day 0 uses the due day", at(2027, 1, 15, 9, 0), todo.RepeatMonthly, 0, at(2027, 1, 15, 10, 0), at(2027, 2, 15, 9, 0)},
		{"monthly, many months late", at(2026, 1, 10, 8, 30), todo.RepeatMonthly, 10, at(2026, 10, 30, 12, 0), at(2026, 11, 10, 8, 30)},
		{"yearly, 29 Feb", at(2028, 2, 29, 9, 0), todo.RepeatYearly, 29, at(2028, 2, 29, 12, 0), at(2029, 2, 28, 9, 0)},
		{"yearly, back to 29 Feb", at(2031, 2, 28, 9, 0), todo.RepeatYearly, 29, at(2031, 2, 28, 12, 0), at(2032, 2, 29, 9, 0)},
	}
	for _, c := range cases {
		got := todo.NextDue(c.due, c.rule, c.day, c.now, warsaw)
		if !got.Equal(c.want) {
			t.Errorf("%s: NextDue = %v, want %v", c.name, got.In(warsaw), c.want)
		}
	}
}

// Done on a repeating item: it stays open, its date moves to the next time,
// and it can notify again.
func TestSetDoneMovesRepeatingItem(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	it, err := svc.AddItem(ctx, 1, todo.Input{Text: "Water the plants", DueDate: "2099-10-05", DueTime: "18:00", Repeat: todo.RepeatWeekly}, time.Local)
	if err != nil {
		t.Fatal(err)
	}
	if it.Repeat != todo.RepeatWeekly || it.RepeatDay != 5 {
		t.Fatalf("added: repeat %q day %d, want week 5", it.Repeat, it.RepeatDay)
	}
	svc.ClaimDue(ctx, 1, time.Date(2099, 10, 6, 0, 0, 0, 0, time.Local))
	got, err := svc.SetDoneIn(ctx, 1, it.ID, true, time.Local)
	want := time.Date(2099, 10, 12, 18, 0, 0, 0, time.Local)
	if err != nil || got.Done || got.DueAt == nil || !got.DueAt.Equal(want) || got.NotifiedAt != nil || got.Repeat != todo.RepeatWeekly {
		t.Fatalf("SetDoneIn = %+v, %v; want open, due %v, not notified, still weekly", got, err, want)
	}
	// Toggle does the same.
	got, err = svc.ToggleIn(ctx, 1, it.ID, time.Local)
	if err != nil || got.Done || !got.DueAt.Equal(want.AddDate(0, 0, 7)) {
		t.Fatalf("ToggleIn = %+v, %v; want open, a week later", got, err)
	}
	// An item without repeat becomes done as before.
	plain, _ := svc.Add(ctx, 1, "Pay the rent", "2099-10-05", "")
	got, err = svc.SetDoneIn(ctx, 1, plain.ID, true, time.Local)
	if err != nil || !got.Done || !got.DueAt.Equal(*plain.DueAt) {
		t.Fatalf("plain SetDoneIn = %+v, %v; want done, same date", got, err)
	}
}

func TestSetDueRepeat(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	it, _ := svc.Add(ctx, 1, "Pay the rent", "", "")

	if _, err := svc.SetDueRepeat(ctx, 1, it.ID, "2099-01-31", "", "fortnight", time.Local); !errors.Is(err, todo.ErrBadDue) {
		t.Fatalf("bad rule: err = %v, want ErrBadDue", err)
	}
	var dueErr *todo.DueError
	if _, err := svc.SetDueRepeat(ctx, 1, it.ID, "", "", todo.RepeatMonthly, time.Local); !errors.As(err, &dueErr) || dueErr.Msg != "A repeating item needs a due date." {
		t.Fatalf("no date: err = %v, want the message", err)
	}
	got, err := svc.SetDueRepeat(ctx, 1, it.ID, "2099-01-31", "", todo.RepeatMonthly, time.Local)
	if err != nil || got.Repeat != todo.RepeatMonthly || got.RepeatDay != 31 {
		t.Fatalf("SetDueRepeat = %+v, %v; want monthly on the 31st", got, err)
	}

	// The text, a postpone and SetDue without a rule (the API) keep the repeat.
	got, _ = svc.EditText(ctx, 1, it.ID, "Pay the rent now")
	if got.Repeat != todo.RepeatMonthly {
		t.Fatalf("EditText lost the repeat: %+v", got)
	}
	got, _ = svc.SetDue(ctx, 1, it.ID, "2099-02-10", "09:30", time.Local)
	if got.Repeat != todo.RepeatMonthly || got.RepeatDay != 10 {
		t.Fatalf("SetDue: repeat %q day %d, want month 10", got.Repeat, got.RepeatDay)
	}
	got, _ = svc.Postpone(ctx, 1, it.ID, 5)
	if got.Repeat != todo.RepeatMonthly {
		t.Fatalf("Postpone lost the repeat: %+v", got)
	}

	// No date: no repeat.
	got, err = svc.SetDue(ctx, 1, it.ID, "", "", time.Local)
	if err != nil || got.DueAt != nil || got.Repeat != "" || got.RepeatDay != 0 {
		t.Fatalf("remove the date = %+v, %v; want no date and no repeat", got, err)
	}
	if _, err := svc.AddItem(ctx, 1, todo.Input{Text: "x", Repeat: todo.RepeatDaily}, time.Local); !errors.As(err, &dueErr) {
		t.Fatalf("AddItem repeat without date: err = %v, want DueError", err)
	}
}
