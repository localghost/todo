package todo_test

import (
	"strings"
	"testing"

	"todo/internal/todo"
)

// In due date order the overdue items come first, then the nearest due date,
// then the items without a due date. An all-day item is due at the end of
// its day, so it comes after the timed items of that day. Items with the
// same due moment, and the items without a due date, keep the hand-made order.
func TestDueOrder(t *testing.T) {
	warsaw := zone(t, "Europe/Warsaw")
	now := at(warsaw, 2, 16, 0)
	named := func(it todo.Item, text string) todo.Item {
		it.Text = text
		return it
	}
	plain := func(text string) todo.Item { return todo.Item{Text: text} }
	doneItem := func(it todo.Item) todo.Item {
		it.Done = true
		return it
	}
	// The hand-made order.
	items := []todo.Item{
		named(due(at(warsaw, 2, 14, 0), false), "dentist"), // overdue
		plain("plants"), // no date
		named(due(at(warsaw, 3, 10, 0), false), "invoice"),     // tomorrow 10:00
		named(due(at(warsaw, 2, 9, 0), true), "today all day"), // all day today
		named(due(at(warsaw, 6, 9, 0), false), "meeting"),      // next week
		named(due(at(warsaw, 2, 18, 0), false), "rent"),        // today 18:00
		plain("bike"), // no date
		doneItem(named(due(at(warsaw, 1, 9, 0), false), "milk")), // done, yesterday
		named(due(at(warsaw, 3, 10, 0), false), "invoice two"),   // same moment as invoice
		named(due(at(warsaw, 1, 9, 0), true), "library"),         // overdue all day yesterday
	}
	var got []string
	for _, it := range todo.DueOrder(items, now) {
		got = append(got, it.Text)
	}
	want := []string{"library", "dentist", "milk", "rent", "today all day", "invoice", "invoice two", "meeting", "plants", "bike"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("order\n got %v\nwant %v", got, want)
	}
}
