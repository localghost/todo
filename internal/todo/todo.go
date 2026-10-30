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
	// ErrTextTooLong means the text has more than MaxTextChars characters.
	ErrTextTooLong = errors.New("todo: text is too long")
	// ErrTooManyItems means the user already has MaxItemsPerUser items.
	ErrTooManyItems = errors.New("todo: too many items")
)

// Limits that keep one account from filling the disk (code review 1).
const (
	MaxTextChars    = 500  // characters (not bytes) in an item text, after whitespace is collapsed
	MaxItemsPerUser = 5000 // items of one user, done or not
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
	Repeat     string     // "", RepeatDaily, RepeatWeekly, RepeatMonthly or RepeatYearly
	RepeatDay  int        // the day of the month that the rule keeps (repeat.go); 0 without a rule
}

// Change holds the editable fields of an item for Create and UpdateItem.
type Change struct {
	Text          string
	DueAt         *time.Time
	DueAllDay     bool
	ResetNotified bool // UpdateItem only: set NotifiedAt back to nil
	Repeat        string
	RepeatDay     int
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
	// DeleteDone deletes the done items (with labelID != 0 only the ones with
	// that label) and returns how many.
	DeleteDone(ctx context.Context, userID, labelID int64) (int, error)
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
	// SetLabelColor returns ErrNotFound if the user has no label id.
	SetLabelColor(ctx context.Context, userID, id int64, color string) error
	DeleteLabel(ctx context.Context, userID, id int64) error
	// CreateLabel returns the user's label name, and makes it first, with
	// color ("": the next fixed color), if the user has none of that name.
	// ErrTooManyLabels.
	CreateLabel(ctx context.Context, userID int64, name, color string, now time.Time) (Label, error)
	// AddItemLabel puts the label name on the item in one transaction; it makes
	// the label if the user has none of that name. A label the item has
	// already changes nothing. ErrItemLabelsFull, ErrTooManyLabels, ErrNotFound.
	AddItemLabel(ctx context.Context, userID, itemID int64, name string, now time.Time) (Item, error)
	// RemoveItemLabel takes label labelID off the item. ErrNotFound for an
	// item of another user; a label the item does not have changes nothing.
	RemoveItemLabel(ctx context.Context, userID, itemID, labelID int64, now time.Time) (Item, error)
}
