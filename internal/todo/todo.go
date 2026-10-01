// Package todo holds the todo items and the rules for them.
package todo

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrEmptyText means the text is empty after whitespace is removed.
	ErrEmptyText = errors.New("todo: text is empty")
	// ErrNotFound means the item does not exist or belongs to another user.
	ErrNotFound = errors.New("todo: item not found")
)

// Item is one entry in a user's todo list.
type Item struct {
	ID        int64
	UserID    int64
	Text      string
	Done      bool
	Position  int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Store keeps items. Methods return ErrNotFound for unknown items and
// for items of another user.
type Store interface {
	List(ctx context.Context, userID int64, hideDone bool) ([]Item, error)
	Get(ctx context.Context, userID, id int64) (Item, error)
	Create(ctx context.Context, userID int64, text string, now time.Time) (Item, error)
	UpdateText(ctx context.Context, userID, id int64, text string, now time.Time) (Item, error)
	SetDone(ctx context.Context, userID, id int64, done bool, now time.Time) (Item, error)
	Delete(ctx context.Context, userID, id int64) error
	Counts(ctx context.Context, userID int64) (open, done int, err error)
}
