package todo

import "time"

// Repeating items (canvas page "20 · Repeat", option R1): an item with a due
// date can repeat. When it is marked done, it stays open and its due date
// moves to the next time.
const (
	RepeatDaily   = "day"
	RepeatWeekly  = "week"
	RepeatMonthly = "month"
	RepeatYearly  = "year"
)

// RepeatRules are the rules in the order the due date pop-up lists them.
var RepeatRules = []string{RepeatDaily, RepeatWeekly, RepeatMonthly, RepeatYearly}

// RepeatText names a rule for the user: "Every week". "" for no rule.
func RepeatText(rule string) string {
	if rule == "" {
		return ""
	}
	return "Every " + rule
}

// CheckRepeat checks a rule for an item with (or without) a due date.
func CheckRepeat(rule string, dueAt *time.Time) error {
	switch rule {
	case "", RepeatDaily, RepeatWeekly, RepeatMonthly, RepeatYearly:
	default:
		return &DueError{Msg: "Please choose how often the item repeats."}
	}
	if rule != "" && dueAt == nil {
		return &DueError{Msg: "A repeating item needs a due date."}
	}
	return nil
}

// repeatDay is the day of the month that a rule keeps: the due day in loc,
// or 0 without a rule.
func repeatDay(rule string, dueAt *time.Time, loc *time.Location) int {
	if rule == "" || dueAt == nil {
		return 0
	}
	return dueAt.In(loc).Day()
}

// NextDue returns the next due time of a repeating item that is marked done
// at now: the first time after due by the rule whose day (in loc) is after
// today, so missed times are skipped and the item is not due again today.
// The local clock time stays (also over a change to or from summer time);
// an all-day item keeps its 09:00.
// A monthly or yearly rule keeps day, the day of the month (0: the day of
// due); a month without that day uses its last day.
func NextDue(due time.Time, rule string, day int, now time.Time, loc *time.Location) time.Time {
	d := due.In(loc)
	y, m, dd := d.Date()
	h, mi, _ := d.Clock()
	if day == 0 {
		day = dd
	}
	ty, tm, td := now.In(loc).Date()
	tomorrow := time.Date(ty, tm, td+1, 0, 0, 0, 0, loc)
	for i := 1; ; i++ {
		var t time.Time
		switch rule {
		case RepeatDaily:
			t = time.Date(y, m, dd+i, h, mi, 0, 0, loc)
		case RepeatWeekly:
			t = time.Date(y, m, dd+7*i, h, mi, 0, 0, loc)
		case RepeatMonthly:
			first := time.Date(y, m+time.Month(i), 1, h, mi, 0, 0, loc)
			t = time.Date(first.Year(), first.Month(), min(day, daysIn(first)), h, mi, 0, 0, loc)
		case RepeatYearly:
			first := time.Date(y+i, m, 1, h, mi, 0, 0, loc)
			t = time.Date(first.Year(), first.Month(), min(day, daysIn(first)), h, mi, 0, 0, loc)
		default:
			return due
		}
		if !t.Before(tomorrow) {
			return t
		}
	}
}

// daysIn returns the number of days in the month of t.
func daysIn(t time.Time) int {
	return time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
