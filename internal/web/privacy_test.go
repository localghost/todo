package web_test

import (
	"net/http"
	"strings"
	"testing"
	"testing/fstest"

	"todo/docs"
	"todo/internal/web"
)

// The privacy policy (canvas page 2, "privacy policy at /privacy", option A)
// is public: it opens with no session, with the look of the user guide.
func TestPrivacyIsPublic(t *testing.T) {
	env := newTestEnv(t, web.WithGuide(docs.Guide))
	rec := do(t, env.H, "GET", "/privacy", nil, anon)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	assertContains(t, body, "<title>Privacy policy – Todo</title>", `<h1 id="privacy-policy">Privacy policy</h1>`,
		`<a href="/">Open the app →</a>`, `<p id="guide-toc-title">Contents</p>`,
		`<li><a href="#who-is-responsible">Who is responsible</a></li>`, `<h2 id="your-rights">Your rights</h2>`,
		"<table>", "zkostrzewa.dev@gmail.com")
	assertNotContains(t, body, "<script", `<a href="/privacy">`)
	if strings.Index(body, `id="guide-toc-title"`) > strings.Index(body, `<h2 id="who-is-responsible">`) {
		t.Errorf("the contents list must come before the first section")
	}
}

// The policy is the same for everyone, so the browser may keep it for an hour.
func TestPrivacyCacheAndCSP(t *testing.T) {
	env := newTestEnv(t, web.WithGuide(docs.Guide))
	rec := do(t, env.H, "GET", "/privacy", nil, anon)
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=3600" {
		t.Errorf("Cache-Control = %q", got)
	}
	if got := rec.Header().Get("Content-Security-Policy"); got != wantCSP {
		t.Errorf("CSP = %q", got)
	}
}

// With no docs, or docs without privacy-policy.md, /privacy answers 404 and
// the guide still works.
func TestPrivacyMissing(t *testing.T) {
	if rec := do(t, newTestEnv(t).H, "GET", "/privacy", nil, anon); rec.Code != http.StatusNotFound {
		t.Errorf("no docs: status %d, want 404", rec.Code)
	}
	env := newTestEnv(t, web.WithGuide(fstest.MapFS{"user-guide.md": {Data: []byte("# User guide\n\n## A\n")}}))
	if rec := do(t, env.H, "GET", "/privacy", nil, anon); rec.Code != http.StatusNotFound {
		t.Errorf("guide only: status %d, want 404", rec.Code)
	}
	if rec := do(t, env.H, "GET", "/guide", nil, anon); rec.Code != http.StatusOK {
		t.Errorf("guide only: /guide status %d, want 200", rec.Code)
	}
	assertNotContains(t, do(t, env.H, "GET", "/guide", nil, anon).Body.String(), `<a href="/privacy">`)
}

// Option A: the login, sign-up and Account pages and the user guide link the
// policy. The main list does not.
func TestPrivacyLinks(t *testing.T) {
	env := newTestEnv(t, web.WithGuide(docs.Guide))
	for _, path := range []string{"/login", "/signup"} {
		assertContains(t, do(t, env.H, "GET", path, nil, anon).Body.String(),
			`<p class="auth-foot">New here? <a href="/guide">Read the user guide</a> · <a href="/privacy">Privacy policy</a></p>`)
	}
	assertContains(t, do(t, env.H, "GET", "/account", nil, nil).Body.String(),
		`<a href="/">← Back to my list</a> · <a href="/guide">User guide</a> · <a href="/privacy">Privacy policy</a>`)
	assertContains(t, do(t, env.H, "GET", "/guide", nil, anon).Body.String(),
		`<span><a href="/privacy">Privacy policy</a> · <a href="/">Open the app →</a></span>`)
	assertNotContains(t, do(t, env.H, "GET", "/", nil, nil).Body.String(), `href="/privacy"`)
}
