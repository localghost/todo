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
	// ErrCannotMove means the item is overdue; overdue items stay at the top.
	ErrCannotMove = errors.New("todo: an overdue item cannot move")
)

// Item is one entry in a user's todo list.
type Item struct {
	ID         int64
	UserID     int64
	Text       string
	Done       bool
	Position   int64
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DueAt      *time.Time // nil: no due date
	DueAllDay  bool       // true: no time was given; DueAt is 09:00 local
	NotifiedAt *time.Time // nil: not notified yet
	Labels     []Label    // sorted by name; empty from ClaimDue
}

// Change holds the editable fields of an item for Create and UpdateItem.
type Change struct {
	Text          string
	DueAt         *time.Time
	DueAllDay     bool
	ResetNotified bool // UpdateItem only: set NotifiedAt back to nil
	// Labels are label names; missing labels are created. In UpdateItem they
	// replace the item's labels, unless KeepLabels is set.
	Labels     []string
	KeepLabels bool
}

// Store keeps items. Methods return ErrNotFound for unknown items and
// for items of another user.
type Store interface {
	List(ctx context.Context, userID int64, hideDone bool) ([]Item, error)
	// ListLabel is List with only the items that have label labelID (0: all items).
	ListLabel(ctx context.Context, userID int64, hideDone bool, labelID int64) ([]Item, error)
	Get(ctx context.Context, userID, id int64) (Item, error)
	Create(ctx context.Context, userID int64, c Change, now time.Time) (Item, error)
	UpdateItem(ctx context.Context, userID, id int64, c Change, now time.Time) (Item, error)
	SetDone(ctx context.Context, userID, id int64, done bool, now time.Time) (Item, error)
	Delete(ctx context.Context, userID, id int64) error
	// Move puts item id right after item afterID in the user's list, or
	// first if afterID is 0.
	Move(ctx context.Context, userID, id, afterID int64) error
	Counts(ctx context.Context, userID int64) (open, done int, err error)
	// CountsLabel is Counts for the items with label labelID (0: all items).
	CountsLabel(ctx context.Context, userID, labelID int64) (open, done int, err error)
	// ClaimDue marks open, due, not yet notified items as notified at now
	// and returns them. Each item is returned only once.
	ClaimDue(ctx context.Context, userID int64, now time.Time) ([]Item, error)
	// Labels returns the user's labels by name, with their item counts.
	Labels(ctx context.Context, userID int64) ([]Label, error)
	Label(ctx context.Context, userID, id int64) (Label, error)
	// UpdateLabel returns ErrLabelExists if another label of the user has name.
	UpdateLabel(ctx context.Context, userID, id int64, name, color string) (Label, error)
	DeleteLabel(ctx context.Context, userID, id int64) error
}
