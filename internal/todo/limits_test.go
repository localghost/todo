package todo_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"todo/internal/todo"
)

// An item text has at most 500 characters (letters, not bytes), counted
// after the whitespace is collapsed. It applies to add and to edit.
func TestTextLimit(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	max := strings.Repeat("ż", todo.MaxTextChars)
	if _, err := svc.Add(ctx, 1, "  "+max+"  ", "", ""); err != nil {
		t.Fatalf("500 letters: %v", err)
	}
	if _, err := svc.Add(ctx, 1, max+"a", "", ""); !errors.Is(err, todo.ErrTextTooLong) {
		t.Fatalf("501 letters: err = %v, want ErrTextTooLong", err)
	}
	if _, err := svc.AddItem(ctx, 1, todo.Input{Text: max + "a"}, nil); !errors.Is(err, todo.ErrTextTooLong) {
		t.Fatalf("AddItem with 501 letters: err = %v, want ErrTextTooLong", err)
	}
	it, _ := svc.Add(ctx, 1, "short", "", "")
	if _, err := svc.EditText(ctx, 1, it.ID, max+"a"); !errors.Is(err, todo.ErrTextTooLong) {
		t.Fatalf("edit with 501 letters: err = %v, want ErrTextTooLong", err)
	}
	if got, _ := svc.Get(ctx, 1, it.ID); got.Text != "short" {
		t.Fatalf("text after a refused edit = %q, want %q", got.Text, "short")
	}
	if open, _, _ := svc.Counts(ctx, 1); open != 2 {
		t.Fatalf("open items = %d, want 2", open)
	}
}
