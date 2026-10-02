package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// Pages with private data must not be kept by the browser cache (Back after logout).
func TestPrivatePagesAreNotCached(t *testing.T) {
	env := newTestEnv(t)
	for _, r := range []struct {
		path    string
		headers map[string]string
	}{{"/", nil}, {"/account", nil}, {"/login", anon}, {"/signup", anon}, {"/items/1", htmxHeaders}} {
		if got := do(t, env.H, "GET", r.path, nil, r.headers).Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("%s Cache-Control = %q, want no-store", r.path, got)
		}
	}
}

// Very large request bodies are rejected before they are parsed.
func TestLargeBodiesRejected(t *testing.T) {
	env := newTestEnv(t)
	big := url.Values{"text": {strings.Repeat("a", 70<<10)}}
	if rec := do(t, env.H, "POST", "/items", big, htmxHeaders); rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("70 KiB item: %d, want 413", rec.Code)
	}
	login := url.Values{"username": {"x"}, "password": {strings.Repeat("a", 70<<10)}}
	if rec := do(t, env.H, "POST", "/login", login, anon); rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("70 KiB login: %d, want 413", rec.Code)
	}
}
