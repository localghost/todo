package todo_test

import (
	"context"
	"errors"
	"fmt"
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
	if it, err = svc.AddLabel(ctx, 1, it.ID, " work ", ""); err != nil || todo.LabelNames(it.Labels) != "work" {
		t.Fatalf("AddLabel = %q, %v", todo.LabelNames(it.Labels), err)
	}
	// The same name in another case changes nothing.
	before := it.UpdatedAt
	if it, err = svc.AddLabel(ctx, 1, it.ID, "WORK", ""); err != nil || len(it.Labels) != 1 || !it.UpdatedAt.Equal(before) {
		t.Fatalf("AddLabel again = %+v, %v", it.Labels, err)
	}
	it, _ = svc.AddLabel(ctx, 1, it.ID, "money", "")
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
		if _, err := svc.AddLabel(ctx, 1, it.ID, name, ""); !errors.Is(err, todo.ErrBadLabel) {
			t.Errorf("AddLabel(%q) err = %v, want ErrBadLabel", name, err)
		}
	}
	if _, err := svc.AddLabel(ctx, 1, 999, "x", ""); !errors.Is(err, todo.ErrNotFound) {
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

func TestSetLabelColor(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	it, _ := svc.AddItem(ctx, 1, todo.Input{Text: "A", Labels: "work"}, nil)
	id := it.Labels[0].ID
	for _, color := range []string{"red", "#d1382", "#d1382e0", "#g1382e", "d1382e9", ""} {
		if err := svc.SetLabelColor(ctx, 1, id, color); !errors.Is(err, todo.ErrBadLabel) {
			t.Errorf("SetLabelColor(%q) err = %v, want ErrBadLabel", color, err)
		}
	}
	if err := svc.SetLabelColor(ctx, 1, id, "#2ED1C8"); err != nil {
		t.Fatal(err)
	}
	if l, _ := svc.Label(ctx, 1, id); l.Name != "work" || l.Color != "#2ed1c8" {
		t.Fatalf("label = %+v", l)
	}
	if err := svc.SetLabelColor(ctx, 1, 999, "#2ed1c8"); !errors.Is(err, todo.ErrNotFound) {
		t.Fatalf("unknown label: %v, want ErrNotFound", err)
	}
}

func TestAddLabelWithColor(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	it, _ := svc.AddItem(ctx, 1, todo.Input{Text: "A", Labels: "work"}, nil)
	old := it.Labels[0].Color
	b, _ := svc.AddItem(ctx, 1, todo.Input{Text: "B"}, nil)

	// A new label gets the chosen color.
	b, err := svc.AddLabel(ctx, 1, b.ID, "gym", "#12AB34")
	if err != nil || len(b.Labels) != 1 || b.Labels[0].Color != "#12ab34" {
		t.Fatalf("new label = %+v, %v", b.Labels, err)
	}
	// A label that exists keeps its color.
	b, err = svc.AddLabel(ctx, 1, b.ID, "WORK", "#000000")
	if err != nil {
		t.Fatal(err)
	}
	if l, _ := svc.Label(ctx, 1, it.Labels[0].ID); l.Color != old {
		t.Fatalf("work color = %q, want %q", l.Color, old)
	}
	// No color or a bad one: the next fixed color.
	b, _ = svc.AddLabel(ctx, 1, b.ID, "home", "red")
	for _, l := range b.Labels {
		if l.Name == "home" && l.Color != todo.LabelColors[2] {
			t.Fatalf("home color = %q, want %q", l.Color, todo.LabelColors[2])
		}
	}
}

func TestCreateLabel(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	work, err := svc.CreateLabel(ctx, 1, "  work ", "")
	if err != nil || work.ID == 0 || work.Name != "work" || work.Color != todo.LabelColors[0] {
		t.Fatalf("CreateLabel = %+v, %v", work, err)
	}
	// A new label with no item is in the list, with 0 items.
	labels, err := svc.Labels(ctx, 1)
	if err != nil || len(labels) != 1 || labels[0].ID != work.ID || labels[0].Items != 0 {
		t.Fatalf("Labels = %+v, %v", labels, err)
	}
	// The same name in another case gives the label that exists, with its color.
	again, err := svc.CreateLabel(ctx, 1, "WORK", "#000000")
	if err != nil || again != work {
		t.Fatalf("again = %+v, %v; want %+v", again, err, work)
	}
	// A new label gets the chosen color; a bad color gives the next fixed one.
	if gym, err := svc.CreateLabel(ctx, 1, "gym", "#12AB34"); err != nil || gym.Color != "#12ab34" {
		t.Fatalf("gym = %+v, %v", gym, err)
	}
	if home, err := svc.CreateLabel(ctx, 1, "home", "red"); err != nil || home.Color != todo.LabelColors[2] {
		t.Fatalf("home = %+v, %v", home, err)
	}
	if _, err := svc.CreateLabel(ctx, 1, "  ", ""); !errors.Is(err, todo.ErrBadLabel) {
		t.Fatalf("empty name: err = %v", err)
	}
	// An item can get the new label by its name.
	it, _ := svc.AddItem(ctx, 1, todo.Input{Text: "Report"}, nil)
	if it, err = svc.AddLabel(ctx, 1, it.ID, "Work", ""); err != nil || len(it.Labels) != 1 || it.Labels[0].ID != work.ID {
		t.Fatalf("AddLabel = %+v, %v", it.Labels, err)
	}
}

func TestCreateLabelLimit(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	for i := 0; i < todo.MaxLabelsPerUser; i++ {
		if _, err := svc.CreateLabel(ctx, 1, fmt.Sprintf("l%d", i), ""); err != nil {
			t.Fatalf("label %d: %v", i, err)
		}
	}
	if _, err := svc.CreateLabel(ctx, 1, "one more", ""); !errors.Is(err, todo.ErrTooManyLabels) {
		t.Fatalf("label 51: err = %v, want ErrTooManyLabels", err)
	}
	// A name that exists is still fine.
	if _, err := svc.CreateLabel(ctx, 1, "L0", ""); err != nil {
		t.Fatalf("existing name: %v", err)
	}
}
