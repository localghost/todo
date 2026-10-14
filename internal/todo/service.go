package todo

import (
	"context"
	"slices"
	"strings"
	"time"
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
	labels, err := ParseLabels(in.Labels)
	if err != nil {
		return Item{}, err
	}
	return s.store.Create(ctx, userID, Change{Text: text, DueAt: dueAt, DueAllDay: allDay, Labels: labels}, s.now())
}

// Edit changes the text and the due date. A changed due date can notify again.
func (s *Service) Edit(ctx context.Context, userID, id int64, text, dueDate, dueTime string) (Item, error) {
	return s.EditIn(ctx, userID, id, text, dueDate, dueTime, nil)
}

// EditIn is Edit with the due date and time read in loc (nil: the service's zone).
func (s *Service) EditIn(ctx context.Context, userID, id int64, text, dueDate, dueTime string, loc *time.Location) (Item, error) {
	text, err := cleanText(text)
	if err != nil {
		return Item{}, err
	}
	dueAt, allDay, err := parseDue(dueDate, dueTime, s.zone(loc))
	if err != nil {
		return Item{}, err
	}
	it, err := s.store.Get(ctx, userID, id)
	if err != nil {
		return Item{}, err
	}
	dueSame := sameDue(it.DueAt, it.DueAllDay, dueAt, allDay)
	if it.Text == text && dueSame {
		return it, nil
	}
	return s.store.UpdateItem(ctx, userID, id,
		Change{Text: text, DueAt: dueAt, DueAllDay: allDay, ResetNotified: !dueSame, KeepLabels: true}, s.now())
}

// EditItem changes the text, the due date and the labels, reading the due
// date in loc (nil: the service's zone). A changed due date can notify again.
func (s *Service) EditItem(ctx context.Context, userID, id int64, in Input, loc *time.Location) (Item, error) {
	text, err := cleanText(in.Text)
	if err != nil {
		return Item{}, err
	}
	dueAt, allDay, err := parseDue(in.DueDate, in.DueTime, s.zone(loc))
	if err != nil {
		return Item{}, err
	}
	labels, err := ParseLabels(in.Labels)
	if err != nil {
		return Item{}, err
	}
	it, err := s.store.Get(ctx, userID, id)
	if err != nil {
		return Item{}, err
	}
	dueSame := sameDue(it.DueAt, it.DueAllDay, dueAt, allDay)
	if it.Text == text && dueSame && sameLabels(it.Labels, labels) {
		return it, nil
	}
	return s.store.UpdateItem(ctx, userID, id,
		Change{Text: text, DueAt: dueAt, DueAllDay: allDay, ResetNotified: !dueSame, Labels: labels}, s.now())
}

// sameLabels reports whether the item's labels have exactly the names, in
// any order, ignoring case (the store reuses a label of another case).
func sameLabels(have []Label, names []string) bool {
	if len(have) != len(names) {
		return false
	}
	for _, n := range names {
		if !slices.ContainsFunc(have, func(l Label) bool { return strings.EqualFold(l.Name, n) }) {
			return false
		}
	}
	return true
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
		Change{Text: it.Text, DueAt: &due, DueAllDay: false, ResetNotified: true, KeepLabels: true}, now)
}

// ClaimDue returns the items that should notify now. Each item comes only once.
func (s *Service) ClaimDue(ctx context.Context, userID int64, now time.Time) ([]Item, error) {
	return s.store.ClaimDue(ctx, userID, now)
}

func (s *Service) Toggle(ctx context.Context, userID, id int64) (Item, error) {
	it, err := s.store.Get(ctx, userID, id)
	if err != nil {
		return Item{}, err
	}
	return s.store.SetDone(ctx, userID, id, !it.Done, s.now())
}

func (s *Service) Delete(ctx context.Context, userID, id int64) error {
	return s.store.Delete(ctx, userID, id)
}

// Move puts item id after item afterID (0: first). now carries the browser's
// zone, because the overdue rule uses it for all-day items.
func (s *Service) Move(ctx context.Context, userID, id, afterID int64, now time.Time) error {
	it, err := s.store.Get(ctx, userID, id)
	if err != nil {
		return err
	}
	if it.Overdue(now) {
		return ErrCannotMove
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
func cleanText(text string) (string, error) {
	text = strings.Join(strings.Fields(text), " ")
	if text == "" {
		return "", ErrEmptyText
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
