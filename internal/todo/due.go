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
	t, err := time.ParseInLocation("2006-01-02 15:04", date+" "+clock, loc)
	if err != nil {
		return nil, false, bad
	}
	return &t, false, nil
}

func sameDue(a *time.Time, aAllDay bool, b *time.Time, bAllDay bool) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b) && aAllDay == bAllDay
}
