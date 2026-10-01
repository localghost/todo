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

func TestEditRowHasDueLine(t *testing.T) {
	h, svc := newTestApp(t)
	ctx := context.Background()
	svc.Add(ctx, 1, "Timed", "2026-10-03", "14:00")
	svc.Add(ctx, 1, "All day", "2026-10-04", "")

	timed := do(t, h, "GET", "/items/1/edit", nil, htmxHeaders).Body.String()
	assertContains(t, timed,
		`<div class="edit" hx-put="/items/1" hx-trigger="keydown[key=='Enter'], save-edit" hx-include="closest li, #hide-done"`,
		`name="due_date" value="2026-10-03"`, `name="due_time" value="14:00"`,
		`<button type="button" class="clear-due">Clear</button>`)
	allDay := do(t, h, "GET", "/items/2/edit", nil, htmxHeaders).Body.String()
	assertContains(t, allDay, `name="due_date" value="2026-10-04"`, `name="due_time" value=""`)
}

func TestEditSavesDue(t *testing.T) {
	h, svc := newTestApp(t)
	mustAdd(t, svc, "Call the dentist")
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	rec := do(t, h, "PUT", "/items/1",
		url.Values{"text": {"Call the dentist"}, "due_date": {tomorrow}, "due_time": {""}}, htmxHeaders)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	assertContains(t, rec.Body.String(), `<span class="due">Due tomorrow</span>`)
}

func TestEditBadDue(t *testing.T) {
	h, svc := newTestApp(t)
	mustAdd(t, svc, "Call the dentist")
	rec := do(t, h, "PUT", "/items/1",
		url.Values{"text": {"Call the dentist!"}, "due_date": {"2026-13-40"}, "due_time": {""}}, htmxHeaders)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	assertContains(t, rec.Body.String(), `class="item editing"`,
		"Please choose a valid date and time.", `value="Call the dentist!"`)
}

func TestAppJSHandlesDueFields(t *testing.T) {
	h, _ := newTestApp(t)
	body := do(t, h, "GET", "/static/app.js", nil, nil).Body.String()
	assertContains(t, body, `"save-edit"`, `.clear-due`, `"due_date"`, `"due_time"`)
}

// After a due-date error the cursor goes to the date field, not the text field.
func TestDueErrorFocusesDateField(t *testing.T) {
	h, svc := newTestApp(t)
	add := do(t, h, "POST", "/items",
		url.Values{"text": {"Call the dentist"}, "due_date": {""}, "due_time": {"14:00"}}, htmxHeaders).Body.String()
	assertContains(t, add, `aria-describedby="add-due-error" autofocus>`)
	assertNotContains(t, add, `value="Call the dentist" autofocus`)

	mustAdd(t, svc, "Call the dentist")
	edit := do(t, h, "PUT", "/items/1",
		url.Values{"text": {"Call the dentist"}, "due_date": {""}, "due_time": {"14:00"}}, htmxHeaders).Body.String()
	assertContains(t, edit, `aria-describedby="due-error-1" autofocus>`)
	assertNotContains(t, edit, `autocomplete="off" autofocus`)
}

// The due time is a 24-hour text field, in the add form and in the edit row.
func TestDueTimeIs24HourText(t *testing.T) {
	h, svc := newTestApp(t)
	svc.Add(context.Background(), 1, "Call the dentist", "2026-10-03", "14:00")
	page := do(t, h, "GET", "/", nil, nil).Body.String()
	edit := do(t, h, "GET", "/items/1/edit", nil, htmxHeaders).Body.String()
	for name, body := range map[string]string{"add form": page, "edit row": edit} {
		if strings.Contains(body, `type="time"`) {
			t.Errorf("%s still has a native time field", name)
		}
		assertContains(t, body, `type="text" name="due_time"`, `placeholder="HH:MM"`, `inputmode="numeric"`,
			`pattern="([01][0-9]|2[0-3]):[0-5][0-9]"`)
	}
	assertContains(t, edit, `name="due_time" value="14:00"`)
}
