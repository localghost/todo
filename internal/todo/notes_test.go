package todo_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"todo/internal/todo"
)

// Notes on an item (canvas page "21 · Notes", option N2).

// SetNotes keeps the line breaks (as \n), trims the ends, and changes
// nothing else of the item.
func TestSetNotes(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	it, err := svc.AddItem(ctx, 1, todo.Input{Text: "Visit grandma", DueDate: "2099-11-01", Repeat: todo.RepeatWeekly, Labels: "family"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.SetNotes(ctx, 1, it.ID, "  ul. Lipowa 12/4\r\n\r\n  tel. 600 100 200 \n")
	if err != nil {
		t.Fatal(err)
	}
	if want := "ul. Lipowa 12/4\n\n  tel. 600 100 200"; got.Notes != want {
		t.Fatalf("notes = %q, want %q", got.Notes, want)
	}
	if got.Text != it.Text || !got.DueAt.Equal(*it.DueAt) || got.Repeat != it.Repeat || len(got.Labels) != 1 {
		t.Fatalf("other fields changed: %+v, before %+v", got, it)
	}
	if again, _ := svc.Get(ctx, 1, it.ID); again.Notes != got.Notes {
		t.Fatalf("stored notes = %q, want %q", again.Notes, got.Notes)
	}
	empty, err := svc.SetNotes(ctx, 1, it.ID, " \n ")
	if err != nil || empty.Notes != "" {
		t.Fatalf("blank notes: %q, %v; want no notes", empty.Notes, err)
	}
}

// The notes have at most 2000 characters (letters, not bytes), counted
// after the ends are trimmed. A refused change keeps the old notes.
func TestNotesLimit(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	it, _ := svc.Add(ctx, 1, "Visit grandma", "", "")
	max := strings.Repeat("ż", todo.MaxNotesChars)
	if _, err := svc.SetNotes(ctx, 1, it.ID, " "+max+"\n"); err != nil {
		t.Fatalf("2000 letters: %v", err)
	}
	if _, err := svc.SetNotes(ctx, 1, it.ID, max+"a"); !errors.Is(err, todo.ErrNotesTooLong) {
		t.Fatalf("2001 letters: err = %v, want ErrNotesTooLong", err)
	}
	if got, _ := svc.Get(ctx, 1, it.ID); got.Notes != max {
		t.Fatalf("notes after a refused change have %d letters, want the old %d", len([]rune(got.Notes)), todo.MaxNotesChars)
	}
	if _, err := svc.SetNotes(ctx, 2, it.ID, "x"); !errors.Is(err, todo.ErrNotFound) {
		t.Fatalf("notes of another user's item: err = %v, want ErrNotFound", err)
	}
}

// The other changes of an item keep its notes: a new text, a due date, a
// postpone, and a repeating item that is checked off.
func TestOtherChangesKeepNotes(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	it, _ := svc.AddItem(ctx, 1, todo.Input{Text: "Water the plants", DueDate: "2000-01-03", DueTime: "18:00", Repeat: todo.RepeatWeekly}, nil)
	if _, err := svc.SetNotes(ctx, 1, it.ID, "The big one too"); err != nil {
		t.Fatal(err)
	}
	steps := map[string]func() (todo.Item, error){
		"EditText": func() (todo.Item, error) { return svc.EditText(ctx, 1, it.ID, "Water all plants") },
		"SetDue":   func() (todo.Item, error) { return svc.SetDue(ctx, 1, it.ID, "2000-01-04", "18:00", nil) },
		"Postpone": func() (todo.Item, error) { return svc.Postpone(ctx, 1, it.ID, 30) },
		"SetDone":  func() (todo.Item, error) { return svc.SetDone(ctx, 1, it.ID, true) },
	}
	for _, name := range []string{"EditText", "SetDue", "Postpone", "SetDone"} {
		got, err := steps[name]()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got.Notes != "The big one too" {
			t.Fatalf("notes after %s = %q, want them kept", name, got.Notes)
		}
	}
}
