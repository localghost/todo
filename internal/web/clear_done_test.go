package web_test

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

// "Clear done (N)" shows next to "Hide done (N)" only when there is a done
// item (canvas page 3, "Clear done – ask first").
func TestClearDoneButtonShowsOnlyWithDoneItems(t *testing.T) {
	env := newTestEnv(t)
	a := mustAdd(t, env.Svc, "Buy milk")
	body := do(t, env.H, "GET", "/", nil, nil).Body.String()
	assertNotContains(t, body, "Clear done")
	assertContains(t, body, `<div id="clear-confirm"></div>`)
	env.Svc.Toggle(context.Background(), 1, a.ID)
	body = do(t, env.H, "GET", "/", nil, nil).Body.String()
	assertContains(t, body,
		`<button class="filter clear-done" type="button" aria-controls="clear-confirm" aria-expanded="false" hx-get="/items/done/clear" hx-target="#clear-confirm">Clear done (1)</button>`)
}

// The button asks first; the question names the count, and the label when
// the list is filtered.
func TestClearDoneAsksFirst(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	for _, text := range []string{"a", "b"} {
		it := mustAdd(t, env.Svc, text)
		env.Svc.Toggle(ctx, 1, it.ID)
	}
	rec := do(t, env.H, "GET", "/items/done/clear", nil, htmxHeaders)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	assertContains(t, rec.Body.String(),
		`<div class="confirm clear-confirm" role="alert"><p>Delete 2 done items? You cannot undo this.</p>`,
		`hx-post="/items/done/delete" hx-target="#list-section" hx-swap="outerHTML" autofocus>Delete 2 items</button>`,
		`<button type="button" class="btn-small clear-cancel">Cancel</button>`)

	w := addLabeled(t, env.Svc, 1, "Send the invoice", "work")
	env.Svc.Toggle(ctx, 1, w.ID)
	body := do(t, env.H, "GET", "/items/done/clear", nil, filtered(labelID(t, w, "work"))).Body.String()
	assertContains(t, body, "Delete 1 done item with the label “work”? You cannot undo this.", ">Delete 1 item</button>")
}

func TestClearDoneAskWithNothingDoneIsEmpty(t *testing.T) {
	env := newTestEnv(t)
	mustAdd(t, env.Svc, "open")
	rec := do(t, env.H, "GET", "/items/done/clear", nil, htmxHeaders)
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "" {
		t.Fatalf("got %d %q, want 200 and an empty body", rec.Code, rec.Body.String())
	}
}

// Delete removes the done items and answers with the whole list, so the
// rows, the counter, and the button change together.
func TestClearDoneDeletes(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	mustAdd(t, env.Svc, "Buy milk")
	d := mustAdd(t, env.Svc, "Water the plants")
	env.Svc.Toggle(ctx, 1, d.ID)
	rec := do(t, env.H, "POST", "/items/done/delete", nil, htmxHeaders)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	assertContains(t, body, `id="list-section"`, "1 open · 0 done", "Buy milk")
	assertNotContains(t, body, "Water the plants", "Clear done")
}

// With a label filter only that label's done items go.
func TestClearDoneWithLabelFilter(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	w := addLabeled(t, env.Svc, 1, "Send the invoice", "work")
	h := addLabeled(t, env.Svc, 1, "Water the plants", "home")
	env.Svc.Toggle(ctx, 1, w.ID)
	env.Svc.Toggle(ctx, 1, h.ID)
	body := do(t, env.H, "POST", "/items/done/delete", nil, filtered(labelID(t, w, "work"))).Body.String()
	assertContains(t, body, "0 open · 0 done")
	if open, done, _ := env.Svc.Counts(ctx, 1); open != 0 || done != 1 {
		t.Fatalf("Counts = %d, %d; want the home item to stay", open, done)
	}
}

func TestClearDoneWithoutHTMXRedirects(t *testing.T) {
	env := newTestEnv(t)
	rec := do(t, env.H, "POST", "/items/done/delete", nil, nil)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("got %d %q, want 303 /", rec.Code, rec.Header().Get("Location"))
	}
}

func TestClearDoneNeedsLogin(t *testing.T) {
	env := newTestEnv(t)
	d := mustAdd(t, env.Svc, "done")
	env.Svc.Toggle(context.Background(), 1, d.ID)
	for _, m := range []string{"GET /items/done/clear", "POST /items/done/delete"} {
		method, path, _ := strings.Cut(m, " ")
		if rec := do(t, env.H, method, path, nil, anon); rec.Code == http.StatusOK {
			t.Errorf("%s without a session: 200", m)
		}
	}
	if _, done, _ := env.Svc.Counts(context.Background(), 1); done != 1 {
		t.Fatal("a request without a session deleted items")
	}
}

// A change to any row re-renders the toolbar; the open question goes with
// it, so its count cannot be old.
func TestRowChangeClosesTheQuestion(t *testing.T) {
	env := newTestEnv(t)
	a := mustAdd(t, env.Svc, "a")
	rec := do(t, env.H, "POST", "/items/"+strconv.FormatInt(a.ID, 10)+"/toggle", nil, htmxHeaders)
	assertContains(t, rec.Body.String(), `<div id="clear-confirm" hx-swap-oob="true"></div>`)
}

// Cancel and Esc close the question and give the focus back to the button.
func TestAppJSClosesTheQuestion(t *testing.T) {
	h, _ := newTestApp(t)
	js := do(t, h, "GET", "/static/app.js", nil, nil).Body.String()
	assertContains(t, js, `function closeClearConfirm(`, `e.target.closest(".clear-cancel")`, `e.target.closest(".clear-done")`)
}
