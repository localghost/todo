package web_test

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"testing"
)

const wantCSP = "default-src 'self'; style-src 'self' https://fonts.googleapis.com; font-src https://fonts.gstatic.com; " +
	"img-src 'self' data:; frame-ancestors 'none'; base-uri 'none'; form-action 'self'"

func TestSecurityHeaders(t *testing.T) {
	env := newTestEnv(t)
	for _, r := range []struct {
		path    string
		headers map[string]string
	}{
		{"/", nil}, {"/login", anon}, {"/static/app.js", nil}, {"/items/1", htmxHeaders},
	} {
		rec := do(t, env.H, "GET", r.path, nil, r.headers)
		if got := rec.Header().Get("Content-Security-Policy"); got != wantCSP {
			t.Errorf("%s CSP = %q", r.path, got)
		}
		if rec.Header().Get("X-Content-Type-Options") != "nosniff" || rec.Header().Get("Referrer-Policy") != "same-origin" {
			t.Errorf("%s missing nosniff or Referrer-Policy: %v", r.path, rec.Header())
		}
	}
}

var trigFilter = regexp.MustCompile(`hx-trigger="[^"]*\[`)

func TestNoEvalInPages(t *testing.T) {
	env := newTestEnv(t)
	env.Svc.Add(context.Background(), env.User.ID, "Overdue", "2020-01-01", "08:00")
	pages := map[string]string{
		"list":   do(t, env.H, "GET", "/", nil, nil).Body.String(),
		"edit":   do(t, env.H, "GET", "/items/1/edit", nil, htmxHeaders).Body.String(),
		"login":  do(t, env.H, "GET", "/login", nil, anon).Body.String(),
		"signup": do(t, env.H, "GET", "/signup", nil, anon).Body.String(),
		"acct":   do(t, env.H, "GET", "/account", nil, nil).Body.String(),
	}
	inlineScript := regexp.MustCompile(`<script>|<script [^>]*>[^<]`)
	for name, body := range pages {
		assertNotContains(t, body, "hx-on", "javascript:")
		if trigFilter.MatchString(body) {
			t.Errorf("%s has an htmx trigger filter (needs eval)", name)
		}
		if inlineScript.MatchString(body) {
			t.Errorf("%s has an inline script", name)
		}
	}
	assertContains(t, pages["list"], `"allowEval":false`, `"includeIndicatorStyles":false`)
	assertContains(t, pages["edit"], `hx-trigger="cancel-edit"`, `hx-trigger="save-edit"`)
}

func TestCrossSiteFormsRejected(t *testing.T) {
	env := newTestEnv(t)
	cross := map[string]string{"Sec-Fetch-Site": "cross-site", anonHeader: "1"}
	for _, path := range []string{"/login", "/signup"} {
		if rec := do(t, env.H, "POST", path, url.Values{"username": {"x"}, "password": {"y"}}, cross); rec.Code != http.StatusForbidden {
			t.Errorf("cross-site POST %s: %d, want 403", path, rec.Code)
		}
	}
	if rec := do(t, env.H, "POST", "/account/delete", url.Values{"confirm_username": {"tester"}},
		map[string]string{"Sec-Fetch-Site": "cross-site"}); rec.Code != http.StatusForbidden {
		t.Errorf("cross-site account delete: %d, want 403", rec.Code)
	}
}
