package todo_test

import (
	"context"
	"sync"
	"testing"

	"todo/internal/todo"
)

// Two label changes on one item at the same time both stick: each changes
// only its own label row (code review 3). Before, each wrote the whole label
// list that it had read, so one change undid the other.
func TestParallelLabelChangesBothStick(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	for round := 0; round < 100; round++ {
		it, err := svc.AddItem(ctx, 1, todo.Input{Text: "Item", Labels: "a, b, c"}, nil)
		if err != nil {
			t.Fatalf("AddItem: %v", err)
		}
		var ids []int64
		for _, l := range it.Labels {
			if l.Name != "c" {
				ids = append(ids, l.ID)
			}
		}
		var wg sync.WaitGroup
		for _, id := range ids {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, err := svc.RemoveLabel(ctx, 1, it.ID, id); err != nil {
					t.Errorf("RemoveLabel: %v", err)
				}
			}()
		}
		for _, name := range []string{"d", "e"} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, err := svc.AddLabel(ctx, 1, it.ID, name, ""); err != nil {
					t.Errorf("AddLabel: %v", err)
				}
			}()
		}
		wg.Wait()
		got, _ := svc.Get(ctx, 1, it.ID)
		if names := todo.LabelNames(got.Labels); names != "c, d, e" {
			t.Fatalf("round %d: labels = %q, want %q", round, names, "c, d, e")
		}
	}
}

// A label change keeps a text edit that was saved after the label change
// read the item: it never writes the text.
func TestLabelChangeKeepsTheText(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	it, _ := svc.AddItem(ctx, 1, todo.Input{Text: "Old", Labels: "a"}, nil)
	var wg sync.WaitGroup
	for round := 0; round < 50; round++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			svc.AddLabel(ctx, 1, it.ID, "b", "")
			svc.RemoveLabel(ctx, 1, it.ID, it.Labels[0].ID)
		}()
		go func() { defer wg.Done(); svc.EditText(ctx, 1, it.ID, "New") }()
		wg.Wait()
		if got, _ := svc.Get(ctx, 1, it.ID); got.Text != "New" {
			t.Fatalf("round %d: text = %q, want New", round, got.Text)
		}
		svc.EditText(ctx, 1, it.ID, "Old")
		svc.AddLabel(ctx, 1, it.ID, "a", "")
	}
}
