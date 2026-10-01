package web_test

import (
	"net/http"
	"testing"
)

// htmx puts focus back on an element with the same id after a swap,
// so the checkbox keeps focus after a check.
func TestCheckButtonHasStableID(t *testing.T) {
	h, s := newTestApp(t)
	mustAdd(t, s, "Buy milk")
	body := do(t, h, "GET", "/", nil, nil).Body.String()
	assertContains(t, body, `<button id="check-1" class="check"`)
}

// A click outside with empty text cancels the edit instead of saving.
func TestEditRowCancelsOnEmptyBlur(t *testing.T) {
	h, s := newTestApp(t)
	mustAdd(t, s, "Buy milk")
	body := do(t, h, "GET", "/items/1/edit", nil, htmxHeaders).Body.String()
	assertContains(t, body, `hx-trigger="keyup[key=='Escape'], cancel-edit"`)
}

// Focus moves and the add-form behavior live in app.js.
func TestPageLoadsAppJS(t *testing.T) {
	h, _ := newTestApp(t)
	body := do(t, h, "GET", "/", nil, nil).Body.String()
	assertContains(t, body, `<script src="/static/app.js" defer></script>`)
	rec := do(t, h, "GET", "/static/app.js", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /static/app.js status = %d, want 200", rec.Code)
	}
	assertContains(t, rec.Body.String(), "cancel-edit", "htmx:beforeSwap", "add-form")
}

// When an item opens for editing, the cursor goes to the end of the text.
func TestAppJSPutsCursorAtEndOfEditField(t *testing.T) {
	h, _ := newTestApp(t)
	body := do(t, h, "GET", "/static/app.js", nil, nil).Body.String()
	assertContains(t, body, `"htmx:load"`, `.edit input`, "setSelectionRange(end, end)")
}

// Typing while nothing has focus starts a new item in the add field.
func TestAppJSTypeToAdd(t *testing.T) {
	h, _ := newTestApp(t)
	js := do(t, h, "GET", "/static/app.js", nil, nil).Body.String()
	assertContains(t, js, "document.activeElement === document.body", "e.key.length === 1",
		`e.key === " "`, `document.getElementById("new-item")`)
}
