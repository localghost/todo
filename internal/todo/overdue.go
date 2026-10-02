package todo

import (
	"sort"
	"time"
)

// overdueFrom returns the moment the item becomes overdue, in now's zone:
// the due moment for a timed item, the start of the next day for an all-day item.
func (it Item) overdueFrom(now time.Time) (time.Time, bool) {
	if it.Done || it.DueAt == nil {
		return time.Time{}, false
	}
	d := it.DueAt.In(now.Location())
	if it.DueAllDay {
		y, m, day := d.Date()
		next := time.Date(y, m, day+1, 0, 0, 0, 0, d.Location())
		// Where midnight does not exist (a DST jump at 00:00), time.Date can
		// give 23:00 on the due day; step on until the next day starts.
		for next.Day() == day {
			next = next.Add(time.Hour)
		}
		return next, true
	}
	return d, true
}

// Overdue reports whether the open item is past its due time at now. An all-day
// item is overdue only after its day ends, in now's zone.
func (it Item) Overdue(now time.Time) bool {
	from, ok := it.overdueFrom(now)
	if !ok {
		return false
	}
	if it.DueAllDay {
		return !now.Before(from)
	}
	return now.After(from)
}

// OverdueFirst returns the items with the overdue ones first, most overdue
// first; the other items keep their order.
func OverdueFirst(items []Item, now time.Time) []Item {
	var over, rest []Item
	for _, it := range items {
		if it.Overdue(now) {
			over = append(over, it)
		} else {
			rest = append(rest, it)
		}
	}
	sort.SliceStable(over, func(i, j int) bool {
		if !over[i].DueAt.Equal(*over[j].DueAt) {
			return over[i].DueAt.Before(*over[j].DueAt)
		}
		return over[i].ID < over[j].ID
	})
	return append(over, rest...)
}

// NextOverdue returns the earliest moment after now when an open item becomes
// overdue.
func NextOverdue(items []Item, now time.Time) (time.Time, bool) {
	var next time.Time
	found := false
	for _, it := range items {
		if it.Overdue(now) {
			continue
		}
		from, ok := it.overdueFrom(now)
		if !ok || (found && !from.Before(next)) {
			continue
		}
		next, found = from, true
	}
	return next, found
}
