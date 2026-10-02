package todo_test

import (
	"testing"
	"time"

	"todo/internal/todo"
)

func zone(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func at(loc *time.Location, day, hour, minute int) time.Time {
	return time.Date(2026, 10, day, hour, minute, 0, 0, loc)
}

func due(t time.Time, allDay bool) todo.Item {
	return todo.Item{DueAt: &t, DueAllDay: allDay}
}

func TestOverdueRule(t *testing.T) {
	warsaw, ny := zone(t, "Europe/Warsaw"), zone(t, "America/New_York")
	timed := due(at(warsaw, 3, 14, 0), false)
	for _, c := range []struct {
		name string
		now  time.Time
		want bool
	}{
		{"timed, before", at(warsaw, 3, 13, 59), false},
		{"timed, at the due moment", at(warsaw, 3, 14, 0), false},
		{"timed, after", at(warsaw, 3, 14, 1), true},
	} {
		if got := timed.Overdue(c.now); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
	allDay := due(at(warsaw, 3, 9, 0), true)
	if allDay.Overdue(at(warsaw, 3, 23, 59)) {
		t.Error("all-day item overdue on its own day")
	}
	midnight := at(warsaw, 4, 0, 0)
	if !allDay.Overdue(midnight) {
		t.Error("all-day item not overdue at midnight after its day (Warsaw)")
	}
	if allDay.Overdue(midnight.In(ny)) {
		t.Error("all-day item overdue in New York at 18:00 on its own day")
	}
	done := timed
	done.Done = true
	if done.Overdue(at(warsaw, 4, 0, 0)) {
		t.Error("a done item is never overdue")
	}
	if (todo.Item{}).Overdue(at(warsaw, 4, 0, 0)) {
		t.Error("an item without a due date is never overdue")
	}
}

func TestOverdueFirst(t *testing.T) {
	warsaw := zone(t, "Europe/Warsaw")
	now := at(warsaw, 2, 16, 0)
	library := todo.Item{ID: 1, Text: "library"}
	dentist := due(at(warsaw, 2, 14, 0), false)
	dentist.ID, dentist.Text = 2, "dentist"
	rent := due(at(warsaw, 1, 9, 0), true)
	rent.ID, rent.Text = 3, "rent"
	milk := due(at(warsaw, 1, 10, 0), false)
	milk.ID, milk.Text, milk.Done = 4, "milk", true
	plants := due(at(warsaw, 3, 14, 0), false)
	plants.ID, plants.Text = 5, "plants"

	got := todo.OverdueFirst([]todo.Item{library, dentist, rent, milk, plants}, now)
	var order []string
	for _, it := range got {
		order = append(order, it.Text)
	}
	want := []string{"rent", "dentist", "library", "milk", "plants"}
	if len(order) != len(want) {
		t.Fatalf("order %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order %v, want %v", order, want)
		}
	}
}

func TestNextOverdue(t *testing.T) {
	warsaw := zone(t, "Europe/Warsaw")
	now := at(warsaw, 2, 16, 0)
	items := []todo.Item{
		due(at(warsaw, 3, 14, 0), false), // timed: tomorrow 14:00
		due(at(warsaw, 2, 9, 0), true),   // all-day today: overdue from tomorrow 00:00
		due(at(warsaw, 2, 15, 0), false), // already overdue
	}
	next, ok := todo.NextOverdue(items, now)
	if want := at(warsaw, 3, 0, 0); !ok || !next.Equal(want) {
		t.Fatalf("NextOverdue = %v, %v; want %v", next, ok, want)
	}
	if _, ok := todo.NextOverdue([]todo.Item{{Text: "no due"}}, now); ok {
		t.Fatal("NextOverdue without due items must be false")
	}
}
