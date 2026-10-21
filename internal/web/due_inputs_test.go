package web_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// The add form has only the text field and Add (canvas page 10, "add form
// without the due date"). A due date is set with the calendar icon on a row.
func TestAddFormHasNoDueFields(t *testing.T) {
	h, _ := newTestApp(t)
	body := do(t, h, "GET", "/", nil, nil).Body.String()
	form := body[strings.Index(body, `<form id="add-form"`):]
	form = form[:strings.Index(form, "</form>")]
	assertNotContains(t, form, `id="new-due-date"`, `name="due_date"`, `name="due_time"`, `<label for="new-due-date">`, "due-line")
	assertContains(t, form, `<input id="new-item" name="text"`, `autofocus`)
}

// POST /items ignores due fields: a new item never has a due date.
func TestAddIgnoresDueFields(t *testing.T) {
	h, svc := newTestApp(t)
	rec := do(t, h, "POST", "/items",
		url.Values{"text": {"Call the dentist"}, "due_date": {"2020-01-01"}, "due_time": {"14:00"}}, htmxHeaders)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("HX-Retarget"); got != "" {
		t.Errorf("HX-Retarget = %q, want none (the row is added at the end)", got)
	}
	got, _ := svc.Get(context.Background(), 1, 1)
	if got.Text != "Call the dentist" || got.DueAt != nil {
		t.Fatalf("stored %+v, want the text and no due date", got)
	}
	rec = do(t, h, "POST", "/items", url.Values{"text": {""}, "due_date": {"bad"}}, htmxHeaders)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty text: status = %d, want 422", rec.Code)
	}
	assertContains(t, rec.Body.String(), "Please type some text first.")
	assertNotContains(t, rec.Body.String(), "add-due-error", "due_date")
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
