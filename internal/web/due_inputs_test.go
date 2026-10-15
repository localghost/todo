package web_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestAddFormHasDueLine(t *testing.T) {
	h, _ := newTestApp(t)
	body := do(t, h, "GET", "/", nil, nil).Body.String()
	assertContains(t, body,
		`<input type="date" id="new-due-date" name="due_date" value=""`,
		`<input type="text" name="due_time" value="" class="due-time" placeholder="HH:MM"`)
}

func TestAddWithDue(t *testing.T) {
	h, _ := newTestApp(t)
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	rec := do(t, h, "POST", "/items",
		url.Values{"text": {"Call the dentist"}, "due_date": {tomorrow}, "due_time": {"14:00"}}, htmxHeaders)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	assertContains(t, rec.Body.String(), "Due tomorrow, 14:00")
}

func TestAddBadDueKeepsValues(t *testing.T) {
	h, svc := newTestApp(t)
	rec := do(t, h, "POST", "/items",
		url.Values{"text": {"Call the dentist"}, "due_date": {""}, "due_time": {"14:00"}}, htmxHeaders)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	if got := rec.Header().Get("HX-Retarget"); got != "#add-form" {
		t.Errorf("HX-Retarget = %q, want #add-form", got)
	}
	assertContains(t, rec.Body.String(),
		`<p id="add-due-error" class="error">Pick a day for the due date.</p>`,
		`value="Call the dentist"`, `name="due_time" value="14:00"`)
	assertNotContains(t, rec.Body.String(), "Please type some text first.")
	if open, done, _ := svc.Counts(context.Background(), 1); open+done != 0 {
		t.Fatalf("an item was stored")
	}
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
	assertContains(t, body, `"save-edit"`, `"due_date"`, `"due_time"`)
	assertNotContains(t, body, ".clear-due", "htmx:validation:halted")
}

// After a due-date error the cursor goes to the date field, not the text field.
func TestDueErrorFocusesDateField(t *testing.T) {
	h, _ := newTestApp(t)
	add := do(t, h, "POST", "/items",
		url.Values{"text": {"Call the dentist"}, "due_date": {""}, "due_time": {"14:00"}}, htmxHeaders).Body.String()
	assertContains(t, add, `aria-describedby="add-due-error" autofocus>`)
	assertNotContains(t, add, `value="Call the dentist" autofocus`)
}

// The due time is a 24-hour text field, in the add form and in the due pop-up.
func TestDueTimeIs24HourText(t *testing.T) {
	h, svc := newTestApp(t)
	svc.Add(context.Background(), 1, "Call the dentist", "2026-10-03", "14:00")
	page := do(t, h, "GET", "/", nil, nil).Body.String()
	edit := do(t, h, "GET", "/items/1/due", nil, htmxHeaders).Body.String()
	for name, body := range map[string]string{"add form": page, "due pop-up": edit} {
		if strings.Contains(body, `type="time"`) {
			t.Errorf("%s still has a native time field", name)
		}
		assertContains(t, body, `type="text" name="due_time"`, `placeholder="HH:MM"`, `inputmode="numeric"`,
			`pattern="([01][0-9]|2[0-3]):[0-5][0-9]"`)
	}
	assertContains(t, edit, `name="due_time" value="14:00"`)
}
