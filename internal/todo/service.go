package todo

import (
	"context"
	"strings"
	"time"
)

// Service holds the rules for todo items. All handlers go through it.
type Service struct {
	store Store
	now   func() time.Time
}

// NewService returns a Service that keeps items in store.
func NewService(store Store) *Service {
	return &Service{store: store, now: time.Now}
}

func (s *Service) List(ctx context.Context, userID int64, hideDone bool) ([]Item, error) {
	return s.store.List(ctx, userID, hideDone)
}

func (s *Service) Get(ctx context.Context, userID, id int64) (Item, error) {
	return s.store.Get(ctx, userID, id)
}

func (s *Service) Add(ctx context.Context, userID int64, text string) (Item, error) {
	text, err := cleanText(text)
	if err != nil {
		return Item{}, err
	}
	return s.store.Create(ctx, userID, text, s.now())
}

func (s *Service) UpdateText(ctx context.Context, userID, id int64, text string) (Item, error) {
	text, err := cleanText(text)
	if err != nil {
		return Item{}, err
	}
	return s.store.UpdateText(ctx, userID, id, text, s.now())
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
