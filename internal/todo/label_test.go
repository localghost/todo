package todo_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"todo/internal/todo"
)

func TestParseLabels(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{" , ,", ""},
		{"work", "work"},
		{" work ,  home  office ,", "work|home office"},
		{"Work, work, WORK, home", "Work|home"},
	}
	for _, c := range cases {
		got, err := todo.ParseLabels(c.in)
		if err != nil || strings.Join(got, "|") != c.want {
			t.Errorf("ParseLabels(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
	}
}

func TestParseLabelsLimits(t *testing.T) {
	for _, in := range []string{
		"a, b, c, d, e, f",
		strings.Repeat("x", 31),
		strings.Repeat("a,", 3000),
	} {
		_, err := todo.ParseLabels(in)
		var le *todo.LabelError
		if !errors.As(err, &le) || !errors.Is(err, todo.ErrBadLabel) {
			t.Errorf("ParseLabels(%.20q…) err = %v, want a LabelError", in, err)
		}
	}
	if _, err := todo.ParseLabels(strings.Repeat("ł", 30)); err != nil {
		t.Errorf("30 non-ASCII characters: %v", err)
	}
}

func TestAddItemAndEditItemLabels(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	it, err := svc.AddItem(ctx, 1, todo.Input{Text: "Invoice", Labels: "work, money"}, nil)
	if err != nil || todo.LabelNames(it.Labels) != "money, work" {
		t.Fatalf("AddItem = %q, %v", todo.LabelNames(it.Labels), err)
	}
	it, err = svc.EditItem(ctx, 1, it.ID, todo.Input{Text: "Invoice", Labels: "work"}, nil)
	if err != nil || todo.LabelNames(it.Labels) != "work" {
		t.Fatalf("EditItem = %q, %v", todo.LabelNames(it.Labels), err)
	}
	// Edit (without labels) and Postpone keep the labels.
	if it, err = svc.Edit(ctx, 1, it.ID, "Invoice 2", "2099-01-01", "10:00"); err != nil || todo.LabelNames(it.Labels) != "work" {
		t.Fatalf("Edit = %q, %v", todo.LabelNames(it.Labels), err)
	}
	if it, err = svc.Postpone(ctx, 1, it.ID, 5); err != nil || todo.LabelNames(it.Labels) != "work" {
		t.Fatalf("Postpone = %q, %v", todo.LabelNames(it.Labels), err)
	}
	if it, err = svc.EditItem(ctx, 1, it.ID, todo.Input{Text: "Invoice 2", Labels: ""}, nil); err != nil || len(it.Labels) != 0 {
		t.Fatalf("EditItem to no labels = %+v, %v", it.Labels, err)
	}
}

func TestEditItemSameLabelsOtherCaseIsNoChange(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	it, _ := svc.AddItem(ctx, 1, todo.Input{Text: "A", Labels: "work"}, nil)
	got, err := svc.EditItem(ctx, 1, it.ID, todo.Input{Text: "A", Labels: "WORK"}, nil)
	if err != nil || !got.UpdatedAt.Equal(it.UpdatedAt) {
		t.Fatalf("EditItem with the same labels changed the item: %+v, %v", got, err)
	}
}

func TestAddItemBadLabelsSavesNothing(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	if _, err := svc.AddItem(ctx, 1, todo.Input{Text: "A", Labels: "a,b,c,d,e,f"}, nil); !errors.Is(err, todo.ErrBadLabel) {
		t.Fatalf("err = %v, want ErrBadLabel", err)
	}
	if open, _, _ := svc.Counts(ctx, 1); open != 0 {
		t.Fatalf("open = %d, want 0", open)
	}
}

func TestUpdateLabelChecksNameAndColor(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	it, _ := svc.AddItem(ctx, 1, todo.Input{Text: "A", Labels: "work"}, nil)
	id := it.Labels[0].ID
	for _, c := range []struct{ name, color string }{
		{"  ", "red"},
		{"a, b", "red"},
		{strings.Repeat("x", 31), "red"},
		{"ok", "pink"},
	} {
		if _, err := svc.UpdateLabel(ctx, 1, id, c.name, c.color); !errors.Is(err, todo.ErrBadLabel) {
			t.Errorf("UpdateLabel(%q, %q) err = %v, want ErrBadLabel", c.name, c.color, err)
		}
	}
	l, err := svc.UpdateLabel(ctx, 1, id, "  deep   work ", "teal")
	if err != nil || l.Name != "deep work" || l.Color != "teal" {
		t.Fatalf("UpdateLabel = %+v, %v", l, err)
	}
}
