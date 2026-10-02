package web_test

import (
	"net/http"
	"net/url"
	"testing"
)

// Rows send the hide state with every request, also click-to-edit and Esc.
func TestListIncludesHideStateForAllRowRequests(t *testing.T) {
	h, _ := newTestApp(t)
	body := do(t, h, "GET", "/", nil, nil).Body.String()
	assertContains(t, body, `<ul id="items" hx-include="#hide-done">`)
}

// Enter that opens edit mode must not also save it: save on keydown.
func TestEditSavesOnKeydownEnter(t *testing.T) {
	h, s := newTestApp(t)
	mustAdd(t, s, "Buy milk")
	body := do(t, h, "GET", "/items/1/edit", nil, htmxHeaders).Body.String()
	assertContains(t, body, `hx-trigger="keydown[key=='Enter'], save-edit"`)
	assertNotContains(t, body, `keyup[key=='Enter']`)
}

// "Back" must load fresh data from the server, not an old local copy.
func TestPageDisablesHistoryCache(t *testing.T) {
	h, _ := newTestApp(t)
	body := do(t, h, "GET", "/", nil, nil).Body.String()
	assertContains(t, body, `"historyCacheSize":0`)
}

// Server and network errors show a short message to the user.
func TestPageShowsErrors(t *testing.T) {
	h, _ := newTestApp(t)
	body := do(t, h, "GET", "/", nil, nil).Body.String()
	assertContains(t, body,
		`<p id="error" class="error" role="alert"></p>`,
		`{"code":"5..","swap":true,"error":true,"target":"#error","swapOverride":"innerHTML"}`,
		`hx-on::send-error=`, "Cannot reach the server.",
		`hx-on::after-request=`)
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
