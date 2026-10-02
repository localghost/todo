package web_test

import (
	"testing"

	"todo/internal/auth"
)

func TestMinPasswordLengthShown(t *testing.T) {
	env := newTestEnv(t)
	assertContains(t, do(t, env.H, "GET", "/account", nil, nil).Body.String(), "At least 8 characters")

	env = newTestEnvAuth(t, []auth.Option{auth.WithMinPasswordChars(12)})
	assertContains(t, do(t, env.H, "GET", "/signup", nil, anon).Body.String(), "At least 12 characters")
	assertContains(t, do(t, env.H, "GET", "/account", nil, nil).Body.String(), "At least 12 characters")
	rec := do(t, env.H, "POST", "/signup", signupForm(t, env, "carol", "12345678901"), anon)
	assertContains(t, rec.Body.String(), "Use at least 12 characters.", "At least 12 characters")
}
