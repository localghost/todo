package todo

import "time"

// The Today view (issue #7, canvas page "22 · Today", option V1): the items
// that are overdue or due today.

// InToday reports whether the item is in the Today view at now: an open item
// due today or earlier, or a done item due today. The day is the day in
// now's zone.
func (it Item) InToday(now time.Time) bool {
	if it.DueAt == nil {
		return false
	}
	due := dayOf(*it.DueAt, now.Location())
	today := dayOf(now, now.Location())
	if it.Done {
		return due.Equal(today)
	}
	return !due.After(today)
}

// TodayView returns the items of the Today view, in their order.
func TodayView(items []Item, now time.Time) []Item {
	var out []Item
	for _, it := range items {
		if it.InToday(now) {
			out = append(out, it)
		}
	}
	return out
}

// NextDay returns the start of the day after now, in now's zone: then the
// Today view changes.
func NextDay(now time.Time) time.Time {
	y, m, d := now.Date()
	next := time.Date(y, m, d+1, 0, 0, 0, 0, now.Location())
	// Where midnight does not exist (a DST jump at 00:00), step on until the
	// next day starts, as dueEnd does.
	for next.Day() == d {
		next = next.Add(time.Hour)
	}
	return next
}

// dayOf is the date of t in loc, at 00:00 UTC, so that days compare.
func dayOf(t time.Time, loc *time.Location) time.Time {
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
