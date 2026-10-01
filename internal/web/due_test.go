package web

import (
	"testing"
	"time"

	"todo/internal/todo"
)

func TestDueLabel(t *testing.T) {
	cest := time.FixedZone("CEST", 2*3600)
	now := time.Date(2026, 10, 1, 14, 40, 0, 0, cest)
	at := func(y int, m time.Month, d, h, min int) *time.Time {
		t := time.Date(y, m, d, h, min, 0, 0, cest)
		return &t
	}
	cases := []struct {
		name    string
		item    todo.Item
		want    string
		overdue bool
	}{
		{"no due", todo.Item{}, "", false},
		{"later today", todo.Item{DueAt: at(2026, 10, 1, 18, 0)}, "Due today, 18:00", false},
		{"passed today", todo.Item{DueAt: at(2026, 10, 1, 14, 0)}, "Overdue since today, 14:00", true},
		{"passed but done", todo.Item{Done: true, DueAt: at(2026, 10, 1, 14, 0)}, "Due today, 14:00", false},
		{"tomorrow", todo.Item{DueAt: at(2026, 10, 2, 14, 0)}, "Due tomorrow, 14:00", false},
		{"all-day today", todo.Item{DueAt: at(2026, 10, 1, 9, 0), DueAllDay: true}, "Due today", false},
		{"all-day yesterday", todo.Item{DueAt: at(2026, 9, 30, 9, 0), DueAllDay: true}, "Overdue since yesterday", true},
		{"all-day 28 Sep", todo.Item{DueAt: at(2026, 9, 28, 9, 0), DueAllDay: true}, "Overdue since 28 Sep", true},
		{"later this year", todo.Item{DueAt: at(2026, 10, 3, 14, 0)}, "Due 3 Oct, 14:00", false},
		{"next year, all-day", todo.Item{DueAt: at(2027, 10, 3, 9, 0), DueAllDay: true}, "Due 3 Oct 2027", false},
		{"stored in UTC", todo.Item{DueAt: func() *time.Time { t := time.Date(2026, 10, 1, 16, 0, 0, 0, time.UTC); return &t }()}, "Due today, 18:00", false},
	}
	for _, c := range cases {
		got := dueLabel(c.item, now)
		if got.Text != c.want || got.Overdue != c.overdue {
			t.Errorf("%s: dueLabel = %+v, want {%q %v}", c.name, got, c.want, c.overdue)
		}
	}
	if got := dueText(todo.Item{DueAt: at(2026, 10, 1, 14, 0)}, now); got != "Due today, 14:00" {
		t.Errorf("dueText for a passed item = %q, want %q", got, "Due today, 14:00")
	}
}

func TestDueInputs(t *testing.T) {
	cest := time.FixedZone("CEST", 2*3600)
	timed := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	if d, c := dueInputs(todo.Item{DueAt: &timed}, cest); d != "2026-10-03" || c != "14:00" {
		t.Errorf("timed = %q %q", d, c)
	}
	allDay := time.Date(2026, 10, 3, 9, 0, 0, 0, cest)
	if d, c := dueInputs(todo.Item{DueAt: &allDay, DueAllDay: true}, cest); d != "2026-10-03" || c != "" {
		t.Errorf("all-day = %q %q", d, c)
	}
	if d, c := dueInputs(todo.Item{}, cest); d != "" || c != "" {
		t.Errorf("no due = %q %q", d, c)
	}
}
