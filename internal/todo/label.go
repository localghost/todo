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

// LabelColors are the colors new labels get, in turn. A label can have any
// color of the form #rrggbb (spec 2026-10-20-label-color-wheel-design.md):
// these are the hues of the eight fixed colors the app had before.
var LabelColors = []string{"#868279", "#d1382e", "#e5711a", "#e9bd16", "#38c75d", "#2ed1c8", "#2555da", "#6b44bb"}

// CleanColor returns color in lower case if it has the form #rrggbb.
func CleanColor(color string) (string, bool) {
	if len(color) != 7 || color[0] != '#' {
		return "", false
	}
	color = strings.ToLower(color)
	for _, c := range color[1:] {
		if !('0' <= c && c <= '9' || 'a' <= c && c <= 'f') {
			return "", false
		}
	}
	return color, true
}

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
	Color  string // #rrggbb
	Items  int    // number of items with the label (only from Store.Labels)
}

// Input is what the user typed in the add form.
type Input struct {
	Text    string
	DueDate string // YYYY-MM-DD or empty
	DueTime string // HH:MM or empty
	Labels  string // label names separated by commas (the add form sends the filter's label)
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

// cleanLabelName checks a single label name, for a rename or the label list.
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

// SetLabelColor gives a label a new color, #rrggbb in any case.
func (s *Service) SetLabelColor(ctx context.Context, userID, id int64, color string) error {
	color, ok := CleanColor(color)
	if !ok {
		return &LabelError{Msg: "Pick a color."}
	}
	return s.store.SetLabelColor(ctx, userID, id, color)
}

// DeleteLabel removes a label from all its items and deletes it. The items stay.
func (s *Service) DeleteLabel(ctx context.Context, userID, id int64) error {
	return s.store.DeleteLabel(ctx, userID, id)
}

// AddLabel puts one label on an item. A name the user has no label for
// makes a new label, with color if it is #rrggbb, else with the next fixed
// color. A label the item already has (ignoring case) changes nothing.
func (s *Service) AddLabel(ctx context.Context, userID, itemID int64, name, color string) (Item, error) {
	name, err := cleanLabelName(name)
	if err != nil {
		return Item{}, err
	}
	it, err := s.store.Get(ctx, userID, itemID)
	if err != nil {
		return Item{}, err
	}
	names := make([]string, 0, len(it.Labels)+1)
	for _, l := range it.Labels {
		if strings.EqualFold(l.Name, name) {
			return it, nil
		}
		names = append(names, l.Name)
	}
	if len(names) >= MaxLabelsPerItem {
		return Item{}, &LabelError{Msg: "An item can have at most 5 labels."}
	}
	labels, err := s.store.Labels(ctx, userID)
	if err != nil {
		return Item{}, err
	}
	isNew := !slices.ContainsFunc(labels, func(l Label) bool { return strings.EqualFold(l.Name, name) })
	it, err = s.setLabels(ctx, userID, it, append(names, name))
	color, ok := CleanColor(color)
	if err != nil || !isNew || !ok {
		return it, err
	}
	for _, l := range it.Labels {
		if strings.EqualFold(l.Name, name) {
			if err := s.store.SetLabelColor(ctx, userID, l.ID, color); err != nil {
				return Item{}, err
			}
			return s.store.Get(ctx, userID, it.ID)
		}
	}
	return it, nil
}

// RemoveLabel takes label labelID off an item. The label itself stays.
func (s *Service) RemoveLabel(ctx context.Context, userID, itemID, labelID int64) (Item, error) {
	it, err := s.store.Get(ctx, userID, itemID)
	if err != nil {
		return Item{}, err
	}
	if !it.HasLabel(labelID) {
		return it, nil
	}
	names := []string{}
	for _, l := range it.Labels {
		if l.ID != labelID {
			names = append(names, l.Name)
		}
	}
	return s.setLabels(ctx, userID, it, names)
}

// setLabels replaces the item's labels, keeping its text and due date.
func (s *Service) setLabels(ctx context.Context, userID int64, it Item, names []string) (Item, error) {
	return s.store.UpdateItem(ctx, userID, it.ID,
		Change{Text: it.Text, DueAt: it.DueAt, DueAllDay: it.DueAllDay, Labels: names}, s.now())
}
