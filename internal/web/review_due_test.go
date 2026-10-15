package web_test

import (
	"testing"
)

// Focus leaving the row is decided after one tick, and clicks inside the
// row (gaps, the hint) must not move focus out of it.
func TestAppJSDefersRowSave(t *testing.T) {
	h, _ := newTestApp(t)
	body := do(t, h, "GET", "/static/app.js", nil, nil).Body.String()
	assertContains(t, body, "setTimeout(", "row.contains(document.activeElement)",
		`"mousedown"`, `e.target.closest("input, label")`)
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
