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

func TestAddLabelAndRemoveLabel(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	it, err := svc.AddItem(ctx, 1, todo.Input{Text: "Invoice", DueDate: "2099-01-01", DueTime: "10:00"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if it, err = svc.AddLabel(ctx, 1, it.ID, " work "); err != nil || todo.LabelNames(it.Labels) != "work" {
		t.Fatalf("AddLabel = %q, %v", todo.LabelNames(it.Labels), err)
	}
	// The same name in another case changes nothing.
	before := it.UpdatedAt
	if it, err = svc.AddLabel(ctx, 1, it.ID, "WORK"); err != nil || len(it.Labels) != 1 || !it.UpdatedAt.Equal(before) {
		t.Fatalf("AddLabel again = %+v, %v", it.Labels, err)
	}
	it, _ = svc.AddLabel(ctx, 1, it.ID, "money")
	if it.Text != "Invoice" || it.DueAt == nil {
		t.Fatalf("AddLabel lost text or due date: %+v", it)
	}
	if it, err = svc.RemoveLabel(ctx, 1, it.ID, it.Labels[0].ID); err != nil || todo.LabelNames(it.Labels) != "work" {
		t.Fatalf("RemoveLabel = %q, %v", todo.LabelNames(it.Labels), err)
	}
	// EditText, SetDue and Postpone keep the labels.
	if it, err = svc.EditText(ctx, 1, it.ID, "Invoice 2"); err != nil || todo.LabelNames(it.Labels) != "work" {
		t.Fatalf("EditText = %q, %v", todo.LabelNames(it.Labels), err)
	}
	if it, err = svc.SetDue(ctx, 1, it.ID, "2099-01-01", "10:00", nil); err != nil || todo.LabelNames(it.Labels) != "work" {
		t.Fatalf("SetDue = %q, %v", todo.LabelNames(it.Labels), err)
	}
	if it, err = svc.Postpone(ctx, 1, it.ID, 5); err != nil || todo.LabelNames(it.Labels) != "work" {
		t.Fatalf("Postpone = %q, %v", todo.LabelNames(it.Labels), err)
	}
}

func TestAddLabelChecksTheName(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	it, _ := svc.AddItem(ctx, 1, todo.Input{Text: "A", Labels: "a, b, c, d, e"}, nil)
	for _, name := range []string{"", "x, y", strings.Repeat("x", 31), "f"} {
		if _, err := svc.AddLabel(ctx, 1, it.ID, name); !errors.Is(err, todo.ErrBadLabel) {
			t.Errorf("AddLabel(%q) err = %v, want ErrBadLabel", name, err)
		}
	}
	if _, err := svc.AddLabel(ctx, 1, 999, "x"); !errors.Is(err, todo.ErrNotFound) {
		t.Errorf("AddLabel on a missing item: %v", err)
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
		{"  ", "#d1382e"},
		{"a, b", "#d1382e"},
		{strings.Repeat("x", 31), "#d1382e"},
		{"ok", "red"},
		{"ok", "#d1382"},
		{"ok", "#d1382e0"},
		{"ok", "#g1382e"},
		{"ok", "d1382e9"},
		{"ok", ""},
	} {
		if _, err := svc.UpdateLabel(ctx, 1, id, c.name, c.color); !errors.Is(err, todo.ErrBadLabel) {
			t.Errorf("UpdateLabel(%q, %q) err = %v, want ErrBadLabel", c.name, c.color, err)
		}
	}
	l, err := svc.UpdateLabel(ctx, 1, id, "  deep   work ", "#2ED1C8")
	if err != nil || l.Name != "deep work" || l.Color != "#2ed1c8" {
		t.Fatalf("UpdateLabel = %+v, %v", l, err)
	}
}
