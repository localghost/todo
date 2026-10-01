package web_test

import (
	"net/http"
	"net/url"
	"testing"
)

// Setting a due date in edit mode must update the permission bar.
func TestEditUpdatesNotifyBar(t *testing.T) {
	h, svc := newTestApp(t)
	mustAdd(t, svc, "Call the dentist")
	rec := do(t, h, "PUT", "/items/1",
		url.Values{"text": {"Call the dentist"}, "due_date": {"2026-10-03"}, "due_time": {""}}, htmxHeaders)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	assertContains(t, rec.Body.String(),
		`<div id="notify-bar" class="notify-bar" data-has-due="true" hx-swap-oob="true"></div>`)
}

// Focus leaving the row is decided after one tick, and clicks inside the
// row (labels, gaps, "Clear") must not move focus out of it.
func TestAppJSDefersRowSave(t *testing.T) {
	h, _ := newTestApp(t)
	body := do(t, h, "GET", "/static/app.js", nil, nil).Body.String()
	assertContains(t, body, "setTimeout(", "row.contains(document.activeElement)",
		`"mousedown"`, `e.target.closest("input, label")`)
}

// A half-typed date must not save as "no due date".
func TestEditRowValidatesDueInputs(t *testing.T) {
	h, svc := newTestApp(t)
	mustAdd(t, svc, "Call the dentist")
	body := do(t, h, "GET", "/items/1/edit", nil, htmxHeaders).Body.String()
	assertContains(t, body, `hx-validate="true"`)
	js := do(t, h, "GET", "/static/app.js", nil, nil).Body.String()
	assertContains(t, js, `"htmx:validation:halted"`, "Please choose a valid date and time.")
}

// A failing Notification constructor must not show a false network error.
func TestAppJSSeparatesNotificationErrors(t *testing.T) {
	h, _ := newTestApp(t)
	js := do(t, h, "GET", "/static/app.js", nil, nil).Body.String()
	assertContains(t, js, `console.warn("Cannot show a notification"`, `console.warn("Bad claim response"`)
}

// After a notification, the row reloads, so its label turns red at once.
func TestAppJSRefreshesNotifiedRows(t *testing.T) {
	h, _ := newTestApp(t)
	js := do(t, h, "GET", "/static/app.js", nil, nil).Body.String()
	assertContains(t, js, `htmx.ajax("GET", "/items/" + item.id`, `!row.classList.contains("editing")`)
}

// A permission change in the browser settings updates the bar at once.
func TestAppJSWatchesPermission(t *testing.T) {
	h, _ := newTestApp(t)
	js := do(t, h, "GET", "/static/app.js", nil, nil).Body.String()
	assertContains(t, js, `navigator.permissions.query({ name: "notifications" })`, "status.onchange")
}
