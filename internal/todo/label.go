package todo

import (
	"context"
	"errors"
	"slices"
	"strings"
	"unicode/utf8"
)

// Limits for labels (spec 2026-10-14-labels-design.md).
const (
	MaxLabelLen       = 30 // characters in a label name
	MaxLabelsPerItem  = 5
	MaxLabelsPerUser  = 50
	maxLabelFieldRead = 4096 // longer label fields are refused without parsing
)

// LabelColors are the colors a label can have, in the order new labels get them.
var LabelColors = []string{"gray", "red", "orange", "yellow", "green", "teal", "blue", "purple"}

var (
	// ErrBadLabel means a label name or color is not valid.
	ErrBadLabel = errors.New("todo: bad label")
	// ErrLabelExists means the user already has a label with that name.
	ErrLabelExists = errors.New("todo: label exists")
	// ErrTooManyLabels means the user would have more than MaxLabelsPerUser labels.
	ErrTooManyLabels = errors.New("todo: too many labels")
)

// LabelError is an ErrBadLabel with a message for the user.
type LabelError struct{ Msg string }

func (e *LabelError) Error() string        { return "todo: bad label: " + e.Msg }
func (e *LabelError) Is(target error) bool { return target == ErrBadLabel }

// Label is a name a user puts on items, such as "work".
type Label struct {
	ID     int64
	UserID int64
	Name   string
	Color  string // one of LabelColors
	Items  int    // number of items with the label (only from Store.Labels)
}

// Input is what the user typed in the add form or in an edit row.
type Input struct {
	Text    string
	DueDate string // YYYY-MM-DD or empty
	DueTime string // HH:MM or empty
	Labels  string // label names separated by commas
}

// ParseLabels splits a comma list into clean label names: whitespace
// collapsed, empty names dropped, duplicates (ignoring case) dropped. The
// first spelling of a duplicate is kept.
func ParseLabels(field string) ([]string, error) {
	if len(field) > maxLabelFieldRead {
		return nil, &LabelError{Msg: "An item can have at most 5 labels."}
	}
	names := []string{}
	for _, part := range strings.Split(field, ",") {
		name := strings.Join(strings.Fields(part), " ")
		if name == "" {
			continue
		}
		if utf8.RuneCountInString(name) > MaxLabelLen {
			return nil, &LabelError{Msg: "A label can have at most 30 characters."}
		}
		if slices.ContainsFunc(names, func(n string) bool { return strings.EqualFold(n, name) }) {
			continue
		}
		names = append(names, name)
	}
	if len(names) > MaxLabelsPerItem {
		return nil, &LabelError{Msg: "An item can have at most 5 labels."}
	}
	return names, nil
}

// cleanLabelName checks a single label name, for a rename.
func cleanLabelName(name string) (string, error) {
	name = strings.Join(strings.Fields(name), " ")
	switch {
	case name == "":
		return "", &LabelError{Msg: "The name cannot be empty."}
	case strings.Contains(name, ","):
		return "", &LabelError{Msg: "A label name cannot contain a comma."}
	case utf8.RuneCountInString(name) > MaxLabelLen:
		return "", &LabelError{Msg: "A label can have at most 30 characters."}
	}
	return name, nil
}

// LabelNames returns the names of labels, joined for the Labels field.
func LabelNames(labels []Label) string {
	names := make([]string, len(labels))
	for i, l := range labels {
		names[i] = l.Name
	}
	return strings.Join(names, ", ")
}

// HasLabel reports whether the item has the label id.
func (it Item) HasLabel(id int64) bool {
	return slices.ContainsFunc(it.Labels, func(l Label) bool { return l.ID == id })
}

// Labels returns the user's labels by name, each with its number of items.
func (s *Service) Labels(ctx context.Context, userID int64) ([]Label, error) {
	return s.store.Labels(ctx, userID)
}

// Label returns one label of the user.
func (s *Service) Label(ctx context.Context, userID, id int64) (Label, error) {
	return s.store.Label(ctx, userID, id)
}

// UpdateLabel renames and recolors a label. A name the user already has
// for another label gives ErrLabelExists.
func (s *Service) UpdateLabel(ctx context.Context, userID, id int64, name, color string) (Label, error) {
	name, err := cleanLabelName(name)
	if err != nil {
		return Label{}, err
	}
	if !slices.Contains(LabelColors, color) {
		return Label{}, &LabelError{Msg: "Pick one of the colors."}
	}
	return s.store.UpdateLabel(ctx, userID, id, name, color)
}

// DeleteLabel removes a label from all its items and deletes it. The items stay.
func (s *Service) DeleteLabel(ctx context.Context, userID, id int64) error {
	return s.store.DeleteLabel(ctx, userID, id)
}
