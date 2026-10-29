package web_test

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// The due date pop-up of the add form shows the picked date and answers
// with #add-extras (canvas page 10, "calendar and + under the add field").
func TestDraftDueDate(t *testing.T) {
	env := newTestEnv(t)
	body := do(t, env.H, "GET", "/new/due?due_date=2099-03-04&due_time=09:30", nil, htmxHeaders).Body.String()
	assertContains(t, body, `id="due-picker-0"`, `aria-label="Due date of the new item"`, `hx-post="/new/due"`,
		`hx-target="#add-extras"`, `value="2099-03-04"`, `value="09:30"`, "Remove date")

	rec := do(t, env.H, "POST", "/new/due", url.Values{"due_date": {"2099-03-04"}, "due_time": {"930"}, "labels": {""}}, htmxHeaders)
	if rec.Code != http.StatusOK {
		t.Fatalf("set: %d", rec.Code)
	}
	assertContains(t, rec.Body.String(), `<div id="add-extras"`, `name="due_date" value="2099-03-04"`, `name="due_time" value="09:30"`,
		`<span class="due-add-wrap due-set"><button type="button" id="due-add-0" class="pill draft-due" aria-label="Due date: 4 Mar 2099, 09:30"`,
		`</button><button type="button" class="pill-x" aria-label="Remove the due date"`)

	// All day: no time.
	rec = do(t, env.H, "POST", "/new/due", url.Values{"due_date": {"2099-03-04"}}, htmxHeaders)
	assertContains(t, rec.Body.String(), `name="due_time" value=""`, ">4 Mar 2099</button>")

	// A bad value answers the pop-up with a message.
	rec = do(t, env.H, "POST", "/new/due", url.Values{"due_time": {"14:00"}}, htmxHeaders)
	if rec.Code != http.StatusUnprocessableEntity || rec.Header().Get("HX-Retarget") != "#due-picker-0" {
		t.Fatalf("bad: %d %q", rec.Code, rec.Header().Get("HX-Retarget"))
	}
	assertContains(t, rec.Body.String(), "Pick a day for the due date.", `value="14:00"`)

	// "clear" (the × and "Remove date") takes the date away.
	rec = do(t, env.H, "POST", "/new/due", url.Values{"due_date": {"2099-03-04"}, "clear": {"1"}}, htmxHeaders)
	assertContains(t, rec.Body.String(), ">Due date</button>", `name="due_date" value=""`)
	assertNotContains(t, rec.Body.String(), `class="pill-x"`, "due-set")
}

// The label list of the add form picks labels without an item.
func TestDraftLabels(t *testing.T) {
	env := newTestEnv(t)
	a := addLabeled(t, env.Svc, env.User.ID, "Invoice", "work, home")
	work, home := labelID(t, a, "work"), labelID(t, a, "home")

	body := do(t, env.H, "GET", fmt.Sprintf("/new/labels?labels=%d", work), nil, htmxHeaders).Body.String()
	assertContains(t, body, `id="picker-0"`, `aria-label="Labels of the new item"`, `data-draft="true"`, `data-name="home"`)
	// The list of the add form has no delete.
	assertNotContains(t, body, `class="picker-x"`)
	if !strings.Contains(body, `aria-pressed="true">work</button>`) || !strings.Contains(body, `aria-pressed="false">home</button>`) {
		t.Fatalf("picked state wrong:\n%s", body)
	}

	// Pick an existing label, then make a new one with its color.
	rec := do(t, env.H, "POST", "/new/labels", url.Values{"labels": {fmt.Sprint(work)}, "name": {"home"}}, htmxHeaders)
	assertContains(t, rec.Body.String(), fmt.Sprintf(`name="labels" value="%d,%d"`, work, home), `<span class="chip-name">home</span>`)
	rec = do(t, env.H, "POST", "/new/labels", url.Values{"labels": {fmt.Sprint(work)}, "name": {"garden"}, "color": {"#112233"}}, htmxHeaders)
	assertContains(t, rec.Body.String(), `<span class="chip-name">garden</span>`, `data-color="#112233"`)

	// Take one off; the due date stays.
	rec = do(t, env.H, "POST", "/new/labels", url.Values{"labels": {fmt.Sprintf("%d,%d", work, home)}, "remove": {fmt.Sprint(work)}, "due_date": {"2099-03-04"}}, htmxHeaders)
	assertContains(t, rec.Body.String(), fmt.Sprintf(`name="labels" value="%d"`, home), `name="due_date" value="2099-03-04"`)
	assertNotContains(t, rec.Body.String(), "chip-name\">work<")

	// A bad name answers the list with a message.
	rec = do(t, env.H, "POST", "/new/labels", url.Values{"name": {strings.Repeat("x", 31)}}, htmxHeaders)
	if rec.Code != http.StatusUnprocessableEntity || rec.Header().Get("HX-Retarget") != "#picker-0" {
		t.Fatalf("bad name: %d %q", rec.Code, rec.Header().Get("HX-Retarget"))
	}
	assertContains(t, rec.Body.String(), "A label can have at most 30 characters.")
}

// A new item can have at most 5 labels, as an item on the list.
func TestDraftLabelsFull(t *testing.T) {
	env := newTestEnv(t)
	a := addLabeled(t, env.Svc, env.User.ID, "Invoice", "a, b, c, d, e")
	ids := []string{}
	for _, n := range []string{"a", "b", "c", "d", "e"} {
		ids = append(ids, fmt.Sprint(labelID(t, a, n)))
	}
	rec := do(t, env.H, "POST", "/new/labels", url.Values{"labels": {strings.Join(ids, ",")}, "name": {"f"}}, htmxHeaders)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("sixth label: %d", rec.Code)
	}
	assertContains(t, rec.Body.String(), "An item can have at most 5 labels.")

	// POST /items gets the labels by id.
	rec = do(t, env.H, "POST", "/items", url.Values{"text": {"Five"}, "labels": {strings.Join(ids, ",")}}, htmxHeaders)
	assertContains(t, rec.Body.String(), ">a</a>", ">e</a>")
}
