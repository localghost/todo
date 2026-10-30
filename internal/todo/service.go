package todo

import (
	"context"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// Service holds the rules for todo items. All handlers go through it.
type Service struct {
	store Store
	now   func() time.Time
	loc   *time.Location // time zone for due date input
}

// NewService returns a Service that keeps items in store.
func NewService(store Store) *Service {
	return &Service{store: store, now: time.Now, loc: time.Local}
}

func (s *Service) List(ctx context.Context, userID int64, hideDone bool) ([]Item, error) {
	return s.store.List(ctx, userID, hideDone)
}

func (s *Service) Get(ctx context.Context, userID, id int64) (Item, error) {
	return s.store.Get(ctx, userID, id)
}

// ListLabel returns the user's items with label labelID (0: all items).
func (s *Service) ListLabel(ctx context.Context, userID int64, hideDone bool, labelID int64) ([]Item, error) {
	return s.store.ListLabel(ctx, userID, hideDone, labelID)
}

func (s *Service) Add(ctx context.Context, userID int64, text, dueDate, dueTime string) (Item, error) {
	return s.AddIn(ctx, userID, text, dueDate, dueTime, nil)
}

// AddIn is Add with the due date and time read in loc (nil: the service's zone).
func (s *Service) AddIn(ctx context.Context, userID int64, text, dueDate, dueTime string, loc *time.Location) (Item, error) {
	text, err := cleanText(text)
	if err != nil {
		return Item{}, err
	}
	dueAt, allDay, err := parseDue(dueDate, dueTime, s.zone(loc))
	if err != nil {
		return Item{}, err
	}
	return s.store.Create(ctx, userID, Change{Text: text, DueAt: dueAt, DueAllDay: allDay}, s.now())
}

// AddItem adds an item with its labels, reading the due date in loc (nil:
// the service's zone). Missing labels are created.
func (s *Service) AddItem(ctx context.Context, userID int64, in Input, loc *time.Location) (Item, error) {
	text, err := cleanText(in.Text)
	if err != nil {
		return Item{}, err
	}
	dueAt, allDay, err := parseDue(in.DueDate, in.DueTime, s.zone(loc))
	if err != nil {
		return Item{}, err
	}
	if err := CheckRepeat(in.Repeat, dueAt); err != nil {
		return Item{}, err
	}
	labels, err := ParseLabels(in.Labels)
	if err != nil {
		return Item{}, err
	}
	return s.store.Create(ctx, userID, Change{Text: text, DueAt: dueAt, DueAllDay: allDay, Labels: labels,
		Repeat: in.Repeat, RepeatDay: repeatDay(in.Repeat, dueAt, s.zone(loc))}, s.now())
}

// EditText changes only the text of an item. The due date, its notified
// state, and the labels stay.
func (s *Service) EditText(ctx context.Context, userID, id int64, text string) (Item, error) {
	text, err := cleanText(text)
	if err != nil {
		return Item{}, err
	}
	it, err := s.store.Get(ctx, userID, id)
	if err != nil {
		return Item{}, err
	}
	if it.Text == text {
		return it, nil
	}
	return s.store.UpdateItem(ctx, userID, id,
		Change{Text: text, DueAt: it.DueAt, DueAllDay: it.DueAllDay, KeepLabels: true,
			Repeat: it.Repeat, RepeatDay: it.RepeatDay}, s.now())
}

// SetDue changes only the due date of an item, read in loc (nil: the
// service's zone). An empty date and time removes the due date, and with it
// the repeat; a new date keeps the item's repeat rule.
func (s *Service) SetDue(ctx context.Context, userID, id int64, date, clock string, loc *time.Location) (Item, error) {
	it, err := s.store.Get(ctx, userID, id)
	if err != nil {
		return Item{}, err
	}
	repeat := it.Repeat
	if strings.TrimSpace(date) == "" && strings.TrimSpace(clock) == "" {
		repeat = "" // the date goes, and the repeat with it
	}
	return s.setDue(ctx, it, date, clock, repeat, loc)
}

// SetDueRepeat is SetDue with a repeat rule ("": none), as the due date
// pop-up sends them. A rule needs a date.
func (s *Service) SetDueRepeat(ctx context.Context, userID, id int64, date, clock, repeat string, loc *time.Location) (Item, error) {
	it, err := s.store.Get(ctx, userID, id)
	if err != nil {
		return Item{}, err
	}
	return s.setDue(ctx, it, date, clock, repeat, loc)
}

// setDue sets the due date and the rule of it. A new due date resets the
// notified state, so the item can notify again.
func (s *Service) setDue(ctx context.Context, it Item, date, clock, repeat string, loc *time.Location) (Item, error) {
	loc = s.zone(loc)
	dueAt, allDay, err := parseDue(date, clock, loc)
	if err != nil {
		return Item{}, err
	}
	if err := CheckRepeat(repeat, dueAt); err != nil {
		return Item{}, err
	}
	day := repeatDay(repeat, dueAt, loc)
	if sameDue(it.DueAt, it.DueAllDay, dueAt, allDay) && repeat == it.Repeat && day == it.RepeatDay {
		return it, nil
	}
	return s.store.UpdateItem(ctx, it.UserID, it.ID,
		Change{Text: it.Text, DueAt: dueAt, DueAllDay: allDay, ResetNotified: !sameDue(it.DueAt, it.DueAllDay, dueAt, allDay),
			KeepLabels: true, Repeat: repeat, RepeatDay: day}, s.now())
}

// Postpone moves the due time by minutes, counted from the later of the
// due time and now. The item becomes a timed item and can notify again.
func (s *Service) Postpone(ctx context.Context, userID, id int64, minutes int) (Item, error) {
	if !slices.Contains(PostponeMinutes, minutes) {
		return Item{}, ErrBadPostpone
	}
	it, err := s.store.Get(ctx, userID, id)
	if err != nil {
		return Item{}, err
	}
	if it.Done || it.DueAt == nil {
		return Item{}, ErrCannotPostpone
	}
	now := s.now()
	base := *it.DueAt
	if now.After(base) {
		base = now
	}
	due := base.Add(time.Duration(minutes) * time.Minute)
	return s.store.UpdateItem(ctx, userID, id,
		Change{Text: it.Text, DueAt: &due, DueAllDay: false, ResetNotified: true, KeepLabels: true,
			Repeat: it.Repeat, RepeatDay: it.RepeatDay}, now)
}

// ClaimDue returns the items that should notify now. Each item comes only once.
func (s *Service) ClaimDue(ctx context.Context, userID int64, now time.Time) ([]Item, error) {
	return s.store.ClaimDue(ctx, userID, now)
}

func (s *Service) Toggle(ctx context.Context, userID, id int64) (Item, error) {
	return s.ToggleIn(ctx, userID, id, nil)
}

// ToggleIn is Toggle, with the next date of a repeating item counted in loc
// (nil: the service's zone).
func (s *Service) ToggleIn(ctx context.Context, userID, id int64, loc *time.Location) (Item, error) {
	it, err := s.store.Get(ctx, userID, id)
	if err != nil {
		return Item{}, err
	}
	return s.setDone(ctx, it, !it.Done, loc)
}

// SetDone marks the item done or not done. Unlike Toggle, sending it twice
// gives the same result, so a stale tab cannot undo a change.
func (s *Service) SetDone(ctx context.Context, userID, id int64, done bool) (Item, error) {
	return s.SetDoneIn(ctx, userID, id, done, nil)
}

// SetDoneIn is SetDone, with the next date of a repeating item counted in
// loc (nil: the service's zone).
func (s *Service) SetDoneIn(ctx context.Context, userID, id int64, done bool, loc *time.Location) (Item, error) {
	it, err := s.store.Get(ctx, userID, id)
	if err != nil {
		return Item{}, err
	}
	return s.setDone(ctx, it, done, loc)
}

// setDone marks the item done or not done. A repeating item that becomes
// done stays open, and its due date moves to the next time (repeat.go).
func (s *Service) setDone(ctx context.Context, it Item, done bool, loc *time.Location) (Item, error) {
	now := s.now()
	if done && !it.Done && it.Repeat != "" && it.DueAt != nil {
		next := NextDue(*it.DueAt, it.Repeat, it.RepeatDay, now, s.zone(loc))
		return s.store.UpdateItem(ctx, it.UserID, it.ID,
			Change{Text: it.Text, DueAt: &next, DueAllDay: it.DueAllDay, ResetNotified: true, KeepLabels: true,
				Repeat: it.Repeat, RepeatDay: it.RepeatDay}, now)
	}
	return s.store.SetDone(ctx, it.UserID, it.ID, done, now)
}

func (s *Service) Delete(ctx context.Context, userID, id int64) error {
	return s.store.Delete(ctx, userID, id)
}

// ClearDone deletes the user's done items, or with labelID != 0 only the
// done items with that label, and returns how many.
func (s *Service) ClearDone(ctx context.Context, userID, labelID int64) (int, error) {
	return s.store.DeleteDone(ctx, userID, labelID)
}

// Move puts item id after item afterID (0: first). An overdue item moves like
// any other item.
func (s *Service) Move(ctx context.Context, userID, id, afterID int64) error {
	if _, err := s.store.Get(ctx, userID, id); err != nil {
		return err
	}
	return s.store.Move(ctx, userID, id, afterID)
}

func (s *Service) Counts(ctx context.Context, userID int64) (open, done int, err error) {
	return s.store.Counts(ctx, userID)
}

// CountsLabel counts the open and done items with label labelID (0: all items).
func (s *Service) CountsLabel(ctx context.Context, userID, labelID int64) (open, done int, err error) {
	return s.store.CountsLabel(ctx, userID, labelID)
}

// cleanText collapses each run of whitespace to one space and trims the ends.
// The result must have 1 to MaxTextChars characters.
func cleanText(text string) (string, error) {
	text = strings.Join(strings.Fields(text), " ")
	if text == "" {
		return "", ErrEmptyText
	}
	if utf8.RuneCountInString(text) > MaxTextChars {
		return "", ErrTextTooLong
	}
	return text, nil
}

// zone returns loc, or the service's zone if loc is nil.
func (s *Service) zone(loc *time.Location) *time.Location {
	if loc == nil {
		return s.loc
	}
	return loc
}
