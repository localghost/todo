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

func (s *Service) Add(ctx context.Context, userID int64, text, dueDate, dueTime string) (Item, error) {
	text, err := cleanText(text)
	if err != nil {
		return Item{}, err
	}
	dueAt, allDay, err := parseDue(dueDate, dueTime, s.loc)
	if err != nil {
		return Item{}, err
	}
	return s.store.Create(ctx, userID, Change{Text: text, DueAt: dueAt, DueAllDay: allDay}, s.now())
}

// Edit changes the text and the due date. A changed due date can notify again.
func (s *Service) Edit(ctx context.Context, userID, id int64, text, dueDate, dueTime string) (Item, error) {
	text, err := cleanText(text)
	if err != nil {
		return Item{}, err
	}
	dueAt, allDay, err := parseDue(dueDate, dueTime, s.loc)
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
		Change{Text: text, DueAt: dueAt, DueAllDay: allDay, ResetNotified: !dueSame}, s.now())
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
		Change{Text: it.Text, DueAt: &due, DueAllDay: false, ResetNotified: true}, now)
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

func (s *Service) Counts(ctx context.Context, userID int64) (open, done int, err error) {
	return s.store.Counts(ctx, userID)
}

// cleanText collapses each run of whitespace to one space and trims the ends.
func cleanText(text string) (string, error) {
	text = strings.Join(strings.Fields(text), " ")
	if text == "" {
		return "", ErrEmptyText
	}
	return text, nil
}
