package web_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// The add form has the text field and Add. The due date and labels of the
// new item are under it, in #add-extras (canvas page 10, "calendar and +
// under the add field", option B): hidden fields of the form and two buttons.
func TestAddFormHasDueAndLabelButtons(t *testing.T) {
	h, _ := newTestApp(t)
	body := do(t, h, "GET", "/", nil, nil).Body.String()
	form := body[strings.Index(body, `<form id="add-form"`):]
	form = form[:strings.Index(form, "</form>")]
	assertNotContains(t, form, `name="due_date"`, "due-line")
	assertContains(t, form, `<input id="new-item" name="text"`, `autofocus`)
	assertContains(t, body, `<div id="add-extras" class="add-extras"`,
		`<input type="hidden" form="add-form" name="due_date" value="">`,
		`<input type="hidden" form="add-form" name="labels" value="">`,
		`id="due-add-0" class="icon-btn draft-due" aria-label="Set the due date of the new item"`, `data-picker="/new/due?due_date=&amp;due_time="`,
		`id="label-add-0" class="icon-btn draft-label" aria-label="Add a label to the new item"`, `data-picker="/new/labels?labels="`)
	// Icons only, as on the rows (canvas page 10, "icons only under the add field", option B).
	assertNotContains(t, body, ">Due date</button>", ">Label</button>")
}

// POST /items reads the due date of #add-extras. A new item that is not
// overdue goes at the end; an overdue one comes with the whole list.
func TestAddWithDueDate(t *testing.T) {
	h, svc := newTestApp(t)
	rec := do(t, h, "POST", "/items",
		url.Values{"text": {"Call the dentist"}, "due_date": {"2099-01-01"}, "due_time": {"1400"}}, htmxHeaders)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("HX-Retarget"); got != "" {
		t.Errorf("HX-Retarget = %q, want none (the row is added at the end)", got)
	}
	// The buttons start again: no due date.
	assertContains(t, rec.Body.String(), `<div id="add-extras" class="add-extras" role="group" aria-label="Due date and labels of the new item" hx-swap-oob="true">`, `class="icon-btn draft-due"`)
	got, _ := svc.Get(context.Background(), 1, 1)
	if got.Text != "Call the dentist" || got.DueAt == nil || got.DueAllDay {
		t.Fatalf("stored %+v, want the text and a due time", got)
	}

	// An overdue new item also goes at the end: only its row comes back.
	rec = do(t, h, "POST", "/items", url.Values{"text": {"Pay rent"}, "due_date": {"2020-01-01"}}, htmxHeaders)
	if rec.Code != http.StatusOK || rec.Header().Get("HX-Retarget") != "" {
		t.Fatalf("overdue: status = %d, HX-Retarget = %q, want 200 and none", rec.Code, rec.Header().Get("HX-Retarget"))
	}
	assertContains(t, rec.Body.String(), `<li id="item-2" class="item">`, "Pay rent", "Overdue since", `id="add-extras"`)
	assertNotContains(t, rec.Body.String(), `<section id="list-section"`)

	rec = do(t, h, "POST", "/items", url.Values{"text": {""}, "due_date": {"2099-01-01"}}, htmxHeaders)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty text: status = %d, want 422", rec.Code)
	}
	assertContains(t, rec.Body.String(), "Please type some text first.")

	rec = do(t, h, "POST", "/items", url.Values{"text": {"Bad"}, "due_date": {"bad"}}, htmxHeaders)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad date: status = %d, want 422", rec.Code)
	}
	assertContains(t, rec.Body.String(), "Please choose a valid date and time.", `value="Bad"`)
}

// Edit mode changes only the text; the due date has its own pop-up.
func TestEditRowHasOnlyText(t *testing.T) {
	h, svc := newTestApp(t)
	svc.Add(context.Background(), 1, "Timed", "2026-10-03", "14:00")
	edit := do(t, h, "GET", "/items/1/edit", nil, htmxHeaders).Body.String()
	assertContains(t, edit, `name="text" value="Timed"`, `<p class="hint">Enter to save · Esc to cancel</p>`)
	assertNotContains(t, edit, `name="due_date"`, `name="due_time"`, `name="due_tz"`, "clear-due", `hx-validate`, "all day")
}

// A text save keeps the due date, also when old due fields are sent.
func TestEditKeepsDue(t *testing.T) {
	h, svc := newTestApp(t)
	it, _ := svc.Add(context.Background(), 1, "Call", "2099-10-03", "14:00")
	rec := do(t, h, "PUT", "/items/1", url.Values{"text": {"Call mum"}, "due_date": {""}, "due_time": {""}}, htmxHeaders)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	got, _ := svc.Get(context.Background(), 1, 1)
	if got.Text != "Call mum" || got.DueAt == nil || !got.DueAt.Equal(*it.DueAt) {
		t.Fatalf("after a text edit: %+v, want text changed and due kept", got)
	}
}

func TestAppJSHandlesDueFields(t *testing.T) {
	h, _ := newTestApp(t)
	body := do(t, h, "GET", "/static/app.js", nil, nil).Body.String()
	assertContains(t, body, `"save-edit"`, `const ADD_FIELDS = ["text"];`)
	assertNotContains(t, body, ".clear-due", "htmx:validation:halted")
}

// The due time is a 24-hour text field in the due pop-up.
func TestDueTimeIs24HourText(t *testing.T) {
	h, svc := newTestApp(t)
	svc.Add(context.Background(), 1, "Call the dentist", "2026-10-03", "14:00")
	edit := do(t, h, "GET", "/items/1/due", nil, htmxHeaders).Body.String()
	if strings.Contains(edit, `type="time"`) {
		t.Errorf("the due pop-up still has a native time field")
	}
	assertContains(t, edit, `type="text" name="due_time"`, `placeholder="HH:MM"`, `inputmode="numeric"`,
		`pattern="([01]?[0-9]|2[0-3]):?([0-5][0-9])?"`)
	assertContains(t, edit, `name="due_time" value="14:00"`)
}

// The due time field adds the ":" while typing and completes a short time
// before a send (canvas page 12, "type digits only").
func TestAppJSFormatsDueTime(t *testing.T) {
	h, _ := newTestApp(t)
	js := do(t, h, "GET", "/static/app.js", nil, nil).Body.String()
	assertContains(t, js,
		"function formatTyping(", "function completeTime(",
		`e.target.matches(".due-time")`, `e.inputType.startsWith("insert")`,
		`addEventListener("change"`, `addEventListener("submit"`)
}
