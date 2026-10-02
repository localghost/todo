package web_test

import (
	"net/http"
	"net/url"
	"testing"
)

// "Back" must load fresh data from the server, not an old local copy.
func TestPageDisablesHistoryCache(t *testing.T) {
	h, _ := newTestApp(t)
	body := do(t, h, "GET", "/", nil, nil).Body.String()
	assertContains(t, body, `"historyCacheSize":0`)
}

// Another website must not be able to change items (CSRF).
func TestMutationsRejectCrossSiteRequests(t *testing.T) {
	h, _ := newTestApp(t)
	form := url.Values{"text": {"junk"}}
	cases := []struct {
		name    string
		headers map[string]string
	}{
		{"cross-site htmx", map[string]string{"HX-Request": "true", "Sec-Fetch-Site": "cross-site"}},
		{"same-site htmx", map[string]string{"HX-Request": "true", "Sec-Fetch-Site": "same-site"}},
	}
	for _, c := range cases {
		if rec := do(t, h, "POST", "/items", form, c.headers); rec.Code != http.StatusForbidden {
			t.Errorf("%s: status = %d, want 403", c.name, rec.Code)
		}
	}
	ok := map[string]string{"HX-Request": "true", "Sec-Fetch-Site": "same-origin"}
	if rec := do(t, h, "POST", "/items", form, ok); rec.Code != http.StatusOK {
		t.Errorf("same-origin htmx: status = %d, want 200", rec.Code)
	}
	if rec := do(t, h, "GET", "/", nil, nil); rec.Code != http.StatusOK {
		t.Errorf("plain GET: status = %d, want 200", rec.Code)
	}
}

// Enter that opens edit mode must not also save it: app.js saves on keydown.
func TestEditSavesOnKeydownEnter(t *testing.T) {
	h, _ := newTestApp(t)
	js := do(t, h, "GET", "/static/app.js", nil, nil).Body.String()
	assertContains(t, js, `e.key === "Enter"`, `"save-edit"`, `e.key === "Escape"`, `"cancel-edit"`)
}

// Server and network errors show a short message to the user.
func TestPageShowsErrors(t *testing.T) {
	h, _ := newTestApp(t)
	body := do(t, h, "GET", "/", nil, nil).Body.String()
	assertContains(t, body,
		`<p id="error" class="error" role="alert"></p>`,
		`{"code":"5..","swap":true,"error":true,"target":"#error","swapOverride":"innerHTML"}`)
	js := do(t, h, "GET", "/static/app.js", nil, nil).Body.String()
	assertContains(t, js, `"htmx:sendError"`, "Cannot reach the server.", `"htmx:afterRequest"`)
}
