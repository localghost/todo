package todo

import (
	"testing"
	"time"
)

// The Today view (issue #7, canvas page "22 · Today"): open items due today
// or earlier, and done items due today, in now's zone.
func TestInToday(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Warsaw")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 1, 10, 0, 0, 0, loc)
	at := func(y int, m time.Month, d, h, min int) *time.Time {
		v := time.Date(y, m, d, h, min, 0, 0, loc)
		return &v
	}
	cases := []struct {
		name string
		it   Item
		want bool
	}{
		{"no due date", Item{}, false},
		{"open, all day yesterday", Item{DueAt: at(2026, 10, 31, 0, 0), DueAllDay: true}, true},
		{"open, last week at 09:00", Item{DueAt: at(2026, 10, 25, 9, 0)}, true},
		{"open, today at 09:00 (overdue)", Item{DueAt: at(2026, 11, 1, 9, 0)}, true},
		{"open, today at 23:30", Item{DueAt: at(2026, 11, 1, 23, 30)}, true},
		{"open, all day today", Item{DueAt: at(2026, 11, 1, 0, 0), DueAllDay: true}, true},
		{"open, tomorrow at 00:00", Item{DueAt: at(2026, 11, 2, 0, 0)}, false},
		{"open, all day tomorrow", Item{DueAt: at(2026, 11, 2, 0, 0), DueAllDay: true}, false},
		{"done, today at 08:00", Item{Done: true, DueAt: at(2026, 11, 1, 8, 0)}, true},
		{"done, all day today", Item{Done: true, DueAt: at(2026, 11, 1, 0, 0), DueAllDay: true}, true},
		{"done, yesterday", Item{Done: true, DueAt: at(2026, 10, 31, 12, 0)}, false},
		{"done, tomorrow", Item{Done: true, DueAt: at(2026, 11, 2, 12, 0)}, false},
		{"done, no due date", Item{Done: true}, false},
		// 23:30 UTC on 31 Oct is 00:30 on 1 Nov in Warsaw: today there.
		{"open, today in now's zone only", Item{DueAt: func() *time.Time { v := time.Date(2026, 10, 31, 23, 30, 0, 0, time.UTC); return &v }()}, true},
	}
	for _, c := range cases {
		if got := c.it.InToday(now); got != c.want {
			t.Errorf("%s: InToday = %v, want %v", c.name, got, c.want)
		}
	}
}

// TodayView keeps the order of the items; NextDay is the next midnight.
func TestTodayViewAndNextDay(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Warsaw")
	now := time.Date(2026, 11, 1, 10, 0, 0, 0, loc)
	today := time.Date(2026, 11, 1, 12, 0, 0, 0, loc)
	later := time.Date(2026, 11, 5, 12, 0, 0, 0, loc)
	items := []Item{{ID: 1, DueAt: &today}, {ID: 2}, {ID: 3, DueAt: &later}, {ID: 4, DueAt: &today}}
	got := TodayView(items, now)
	if len(got) != 2 || got[0].ID != 1 || got[1].ID != 4 {
		t.Fatalf("TodayView = %+v, want items 1 and 4", got)
	}
	if want := time.Date(2026, 11, 2, 0, 0, 0, 0, loc); !NextDay(now).Equal(want) {
		t.Fatalf("NextDay = %v, want %v", NextDay(now), want)
	}
}
