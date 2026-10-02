package web_test

import (
	"net/url"
	"strings"
	"testing"
)

func TestAppJSHandlesExpiredHistoryAndIME(t *testing.T) {
	env := newTestEnv(t)
	js := do(t, env.H, "GET", "/static/app.js", nil, anon).Body.String()
	assertContains(t, js, `"htmx:historyCacheMissLoadError"`, "location.reload()", "e.isComposing || e.keyCode === 229")
}

func TestShowButtonHasNoPressedState(t *testing.T) {
	env := newTestEnv(t)
	for _, path := range []string{"/signup", "/account"} {
		h := anon
		if path == "/account" {
			h = nil
		}
		assertNotContains(t, do(t, env.H, "GET", path, nil, h).Body.String(), "aria-pressed")
	}
	assertNotContains(t, do(t, env.H, "GET", "/static/auth.js", nil, anon).Body.String(), "aria-pressed")
}

func TestFocusGoesToPasswordAfterWrongLogin(t *testing.T) {
	env := newTestEnv(t)
	body := do(t, env.H, "POST", "/login", url.Values{"username": {"alice"}, "password": {"wrong password"}}, anon).Body.String()
	if !strings.Contains(body, `id="login-password" name="password" type="password" autocomplete="current-password" required autofocus`) ||
		strings.Contains(body, `autocomplete="username" required autofocus`) {
		t.Fatalf("focus not on the password field after a wrong login:\n%s", body)
	}
	empty := do(t, env.H, "GET", "/login", nil, anon).Body.String()
	assertContains(t, empty, `autocomplete="username" required autofocus`)

	v := signupForm(t, env, "carol", "short")
	body = do(t, env.H, "POST", "/signup", v, anon).Body.String()
	assertContains(t, body, `autocomplete="new-password" required autofocus`)
}

func TestSignupShowsPuzzleFailure(t *testing.T) {
	env := newTestEnv(t)
	assertContains(t, do(t, env.H, "GET", "/signup", nil, anon).Body.String(),
		`<p class="field-error" id="signup-check-error" role="alert"></p>`)
	assertContains(t, do(t, env.H, "GET", "/static/auth.js", nil, anon).Body.String(),
		"Your browser could not run the sign-up check. Please use an up-to-date browser over HTTPS.", "crypto.subtle")
}
