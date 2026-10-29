package todo

import (
	"sort"
	"time"
)

// DueOrder returns the items in due date order: the overdue items first (as
// OverdueFirst), then the nearest due date, then the items without a due
// date. An all-day item counts as due at the end of its day. Items with the
// same due moment, and the items without a due date, keep their order.
func DueOrder(items []Item, now time.Time) []Item {
	items = OverdueFirst(items, now)
	n := 0
	for n < len(items) && items[n].Overdue(now) {
		n++
	}
	rest := items[n:]
	sort.SliceStable(rest, func(i, j int) bool {
		a, aOK := rest[i].dueEnd(now.Location())
		b, bOK := rest[j].dueEnd(now.Location())
		if aOK != bOK {
			return aOK
		}
		return aOK && a.Before(b)
	})
	return items
}
