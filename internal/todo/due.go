package todo

import (
	"errors"
	"strings"
	"time"
)

// ErrBadDue means the due date or time input is not valid.
var ErrBadDue = errors.New("todo: bad due date")

// DueError is an ErrBadDue with a message for the user.
type DueError struct{ Msg string }

func (e *DueError) Error() string        { return "todo: bad due date: " + e.Msg }
func (e *DueError) Is(target error) bool { return target == ErrBadDue }

// allDayHour is the local hour when an item without a due time is due.
const allDayHour = 9

// ParseDue checks a due date and time as the add form sends them, and gives
// the due moment in loc (nil when both are empty) and whether it is all day.
func ParseDue(date, clock string, loc *time.Location) (*time.Time, bool, error) {
	return parseDue(date, clock, loc)
}

// parseDue reads the browser's date (YYYY-MM-DD) and optional time (HH:MM)
// in loc. Both empty means no due date.
func parseDue(date, clock string, loc *time.Location) (*time.Time, bool, error) {
	date, clock = strings.TrimSpace(date), strings.TrimSpace(clock)
	if date == "" {
		if clock == "" {
			return nil, false, nil
		}
		return nil, false, &DueError{Msg: "Pick a day for the due date."}
	}
	bad := &DueError{Msg: "Please choose a valid date and time."}
	if clock == "" {
		d, err := time.ParseInLocation("2006-01-02", date, loc)
		if err != nil {
			return nil, false, bad
		}
		t := time.Date(d.Year(), d.Month(), d.Day(), allDayHour, 0, 0, 0, loc)
		return &t, true, nil
	}
	clock, ok := normalizeClock(clock)
	if !ok {
		return nil, false, bad
	}
	t, err := time.ParseInLocation("2006-01-02 15:04", date+" "+clock, loc)
	if err != nil {
		return nil, false, bad
	}
	return &t, false, nil
}

// normalizeClock reads a 24-hour time as typed and returns it as HH:MM. A
// phone's number keyboard has no ":", so digits only work too: "14" and
// "14:" are 14:00, "930" is 09:30, "1430" is 14:30. app.js completes the
// field the same way (completeTime).
func normalizeClock(s string) (string, bool) {
	h, m, colon := strings.Cut(s, ":")
	if !colon {
		switch len(s) {
		case 1, 2:
			h, m = s, ""
		case 3, 4:
			h, m = s[:len(s)-2], s[len(s)-2:]
		default:
			return "", false
		}
	}
	if m == "" {
		m = "00"
	}
	if len(h) < 1 || len(h) > 2 || len(m) != 2 || !digits(h) || !digits(m) {
		return "", false
	}
	if len(h) == 1 {
		h = "0" + h
	}
	if h > "23" || m > "59" {
		return "", false
	}
	return h + ":" + m, true
}

func digits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func sameDue(a *time.Time, aAllDay bool, b *time.Time, bAllDay bool) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b) && aAllDay == bAllDay
}

// PostponeMinutes are the amounts an item can be postponed by.
var PostponeMinutes = []int{5, 10, 15, 30}

var (
	// ErrBadPostpone means the amount is not one of PostponeMinutes.
	ErrBadPostpone = errors.New("todo: postpone amount not allowed")
	// ErrCannotPostpone means the item is done or has no due date.
	ErrCannotPostpone = errors.New("todo: item cannot be postponed")
)
