package web_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"todo/internal/todo"
)

// The Labels page (canvas page 16, board "Labels page again"): the header
// links to it (option E1), and it lists each label with its color circle,
// name, number of items and ×.
func TestLabelsPage(t *testing.T) {
	env := newTestEnv(t)
	a := addLabeled(t, env.Svc, env.User.ID, "Invoice", "work, home")
	addLabeled(t, env.Svc, env.User.ID, "Report", "work")
	work, home := labelID(t, a, "work"), labelID(t, a, "home")

	assertContains(t, do(t, env.H, "GET", "/", nil, nil).Body.String(), `·</span> <a href="/labels">Labels</a> · <a href="/guide">Help</a>`)

	rec := do(t, env.H, "GET", "/labels", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /labels: %d", rec.Code)
	}
	body := rec.Body.String()
	assertContains(t, body, "<title>Labels – TooDoo</title>", `<a href="/">← Back to my list</a>`, "<h1>Labels</h1>",
		`<script src="/static/labels.js" defer></script>`, `<script src="/static/wheel.js" defer></script>`,
		`hx-post="/labels"`, `placeholder="New label"`, `>Create</button>`,
		fmt.Sprintf(`<a class="lp-name" href="/?label=%d" title="Show only “work”">work</a><span class="lp-count">2 items</span>`, work),
		fmt.Sprintf(`<a class="lp-name" href="/?label=%d" title="Show only “home”">home</a><span class="lp-count">1 item</span>`, home),
		`aria-label="Color of “work”"`, `class="lp-x" aria-label="Delete label “work”"`, `class="wheel" role="slider"`,
		`<span class="lp-sw-wrap"><button type="button" class="sw"`, `class="picker lp-color" role="dialog"`)
	assertNotContains(t, body, "You have no labels yet", "Search or create", "lp-none")

	// Anonymous: the login page.
	if rec := do(t, env.H, "GET", "/labels", nil, anon); rec.Code != http.StatusSeeOther {
		t.Fatalf("anonymous: %d, want 303", rec.Code)
	}
	if rec := do(t, env.H, "GET", "/static/labels.js", nil, nil); rec.Code != http.StatusOK {
		t.Fatalf("labels.js: %d", rec.Code)
	}
}

func TestLabelsPageEmpty(t *testing.T) {
	env := newTestEnv(t)
	body := do(t, env.H, "GET", "/labels", nil, nil).Body.String()
	assertContains(t, body, "You have no labels yet.")
	assertNotContains(t, body, `class="lp-list"`)
}

// Create makes a label with the chosen color; the answer is the section.
func TestLabelsPageCreate(t *testing.T) {
	env := newTestEnv(t)
	rec := do(t, env.H, "POST", "/labels", url.Values{"name": {"  garden  "}, "color": {"#112233"}}, htmxHeaders)
	if rec.Code != http.StatusOK {
		t.Fatalf("create: %d", rec.Code)
	}
	assertContains(t, rec.Body.String(), `<section id="labels-section"`, `data-name="garden"`, `data-color="#112233"`, "no items")
	assertNotContains(t, rec.Body.String(), "<html")
	labels, _ := env.Svc.Labels(context.Background(), env.User.ID)
	if len(labels) != 1 || labels[0].Name != "garden" || labels[0].Color != "#112233" {
		t.Fatalf("labels = %+v", labels)
	}
	// A bad name keeps what was typed and says why.
	rec = do(t, env.H, "POST", "/labels", url.Values{"name": {strings.Repeat("x", 31)}}, htmxHeaders)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad name: %d, want 422", rec.Code)
	}
	assertContains(t, rec.Body.String(), "A label can have at most 30 characters.", `aria-invalid="true"`, `value="`+strings.Repeat("x", 31)+`"`)
	// A name that a label has already (ignoring case and spaces) says so.
	rec = do(t, env.H, "POST", "/labels", url.Values{"name": {" GARDEN "}}, htmxHeaders)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("existing name: %d, want 422", rec.Code)
	}
	assertContains(t, rec.Body.String(), "You have a label “garden” already.", `value=" GARDEN "`)
	if labels, _ := env.Svc.Labels(context.Background(), env.User.ID); len(labels) != 1 {
		t.Fatalf("labels = %+v, want only garden", labels)
	}
	// Without JavaScript the form answers the whole page.
	rec = do(t, env.H, "POST", "/labels", url.Values{"name": {"home"}}, nil)
	assertContains(t, rec.Body.String(), "<h1>Labels</h1>", `data-name="home"`)
}

// Delete answers the section without the label; the items stay.
func TestLabelsPageDelete(t *testing.T) {
	env := newTestEnv(t)
	a := addLabeled(t, env.Svc, env.User.ID, "Invoice", "work, home")
	work := labelID(t, a, "work")
	rec := do(t, env.H, "DELETE", fmt.Sprintf("/labels/%d", work), nil, htmxHeaders)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: %d", rec.Code)
	}
	assertContains(t, rec.Body.String(), `<section id="labels-section"`, `data-name="home"`)
	assertNotContains(t, rec.Body.String(), `data-name="work"`)
	if _, err := env.Svc.Label(context.Background(), env.User.ID, work); !errors.Is(err, todo.ErrNotFound) {
		t.Fatalf("label still there: %v", err)
	}
	if got, _ := env.Svc.Get(context.Background(), env.User.ID, a.ID); got.Text != "Invoice" {
		t.Fatalf("item gone: %+v", got)
	}
	// A label that is already gone answers 404 with the section as it is.
	rec = do(t, env.H, "DELETE", fmt.Sprintf("/labels/%d", work), nil, htmxHeaders)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("gone label: %d, want 404", rec.Code)
	}
	assertContains(t, rec.Body.String(), `data-name="home"`)
}

func TestLabelsPageScript(t *testing.T) {
	env := newTestEnv(t)
	js := do(t, env.H, "GET", "/static/labels.js", nil, nil).Body.String()
	assertContains(t, js, "makeWheel(", `"/labels/" + edit.label + "/color"`, "No items use it.", `"DELETE", "/labels/" + id`,
		"sw.parentElement.append(view)", `addEventListener("mousedown"`)
	assertNotContains(t, js, "filterLines", "label-search")
}
