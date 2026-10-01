package web

import (
	"time"

	"todo/internal/todo"
)

// dueView is the due label of one item.
type dueView struct {
	Text    string
	Overdue bool
}

// dueLabel returns the due label for the list. Open items past their due
// time say "Overdue since …"; all-day items only after their day ends.
func dueLabel(it todo.Item, now time.Time) dueView {
	if it.DueAt == nil {
		return dueView{}
	}
	d := it.DueAt.In(now.Location())
	if !it.Done && isOverdue(d, it.DueAllDay, now) {
		return dueView{Text: "Overdue since " + dueWhen(d, it.DueAllDay, now), Overdue: true}
	}
	return dueView{Text: dueText(it, now)}
}

// dueText is the plain "Due …" label, also used in notifications.
func dueText(it todo.Item, now time.Time) string {
	if it.DueAt == nil {
		return ""
	}
	return "Due " + dueWhen(it.DueAt.In(now.Location()), it.DueAllDay, now)
}

func dueWhen(d time.Time, allDay bool, now time.Time) string {
	if allDay {
		return dayName(d, now)
	}
	return dayName(d, now) + ", " + d.Format("15:04")
}

func isOverdue(d time.Time, allDay bool, now time.Time) bool {
	if allDay {
		y, m, day := d.Date()
		return !now.Before(time.Date(y, m, day+1, 0, 0, 0, 0, d.Location()))
	}
	return now.After(d)
}

// dayName names a day relative to now: today, tomorrow, yesterday, or a date.
func dayName(d, now time.Time) string {
	sameDay := func(a, b time.Time) bool {
		ay, am, ad := a.Date()
		by, bm, bd := b.Date()
		return ay == by && am == bm && ad == bd
	}
	switch {
	case sameDay(d, now):
		return "today"
	case sameDay(d, now.AddDate(0, 0, 1)):
		return "tomorrow"
	case sameDay(d, now.AddDate(0, 0, -1)):
		return "yesterday"
	case d.Year() == now.Year():
		return d.Format("2 Jan")
	default:
		return d.Format("2 Jan 2006")
	}
}

// dueInputs returns the due date and time as the browser inputs expect them.
func dueInputs(it todo.Item, loc *time.Location) (date, clock string) {
	if it.DueAt == nil {
		return "", ""
	}
	d := it.DueAt.In(loc)
	if it.DueAllDay {
		return d.Format("2006-01-02"), ""
	}
	return d.Format("2006-01-02"), d.Format("15:04")
}

// lateAfter: a claim this long after the due time says "overdue", not "due now".
const lateAfter = 5 * time.Minute

// notifyText returns the notification title and due text for a claimed item.
func notifyText(it todo.Item, now time.Time) (title, due string) {
	label := dueLabel(it, now)
	if label.Overdue && now.Sub(*it.DueAt) > lateAfter {
		return "Todo: overdue", label.Text
	}
	return "Todo: due now", dueText(it, now)
}
