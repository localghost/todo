package web_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"
)

// signedUp creates user name with password pw and returns a session token.
func signedUp(t *testing.T, env *testEnv, name string) string {
	t.Helper()
	ctx := context.Background()
	u, err := env.Auth.SignUp(ctx, name, pw)
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	token, _, err := env.Auth.StartSession(ctx, u.ID, false)
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	return token
}

func TestAccountPage(t *testing.T) {
	env := newTestEnv(t)
	body := do(t, env.H, "GET", "/account", nil, nil).Body.String()
	assertContains(t, body, `<a href="/">← Back to my list</a>`, "Signed in as <strong>tester</strong> · member since ",
		"Change password", `name="current_password"`, `name="new_password"`, `action="/account/password"`,
		"Delete account", "This deletes your account and all your items. It cannot be undone.",
		"Type your username to confirm", `name="confirm_username"`, `action="/account/delete"`,
		"Delete my account and all items", `<form method="post" action="/logout" class="inline-form">`)
	if rec := do(t, env.H, "GET", "/account", nil, anon); rec.Code != http.StatusSeeOther {
		t.Fatalf("account without session: %d, want 303", rec.Code)
	}
}

func TestChangePasswordPage(t *testing.T) {
	env := newTestEnv(t)
	token := signedUp(t, env, "bob")
	other := signedUp2(t, env, "bob") // a second session of bob

	rec := do(t, env.H, "POST", "/account/password",
		url.Values{"current_password": {"wrong password"}, "new_password": {"new password 1"}}, cookie(token))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("wrong current: %d, want 422", rec.Code)
	}
	assertContains(t, rec.Body.String(), "The current password is wrong.")

	rec = do(t, env.H, "POST", "/account/password",
		url.Values{"current_password": {pw}, "new_password": {"short"}}, cookie(token))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("short new: %d, want 422", rec.Code)
	}
	assertContains(t, rec.Body.String(), "Use at least 10 characters.")

	rec = do(t, env.H, "POST", "/account/password",
		url.Values{"current_password": {pw}, "new_password": {"new password 1"}}, cookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("change: %d, want 200", rec.Code)
	}
	assertContains(t, rec.Body.String(), "Password changed. You were logged out on your other devices.")
	assertNotContains(t, rec.Body.String(), "new password 1")
	if rec := do(t, env.H, "GET", "/", nil, cookie(token)); rec.Code != http.StatusOK {
		t.Fatalf("current session ended: %d", rec.Code)
	}
	if rec := do(t, env.H, "GET", "/", nil, cookie(other)); rec.Code != http.StatusSeeOther {
		t.Fatalf("other session still valid: %d, want 303", rec.Code)
	}
}

// signedUp2 starts one more session for an existing user.
func signedUp2(t *testing.T, env *testEnv, name string) string {
	t.Helper()
	token, _, err := env.Auth.LogIn(context.Background(), name, pw, true)
	if err != nil {
		t.Fatalf("LogIn: %v", err)
	}
	return token
}

func TestDeleteAccountPage(t *testing.T) {
	env := newTestEnv(t)
	token := signedUp(t, env, "bob")

	rec := do(t, env.H, "POST", "/account/delete", url.Values{"confirm_username": {"alice"}}, cookie(token))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("mismatch: %d, want 422", rec.Code)
	}
	assertContains(t, rec.Body.String(), "The username does not match.")

	rec = do(t, env.H, "POST", "/account/delete", url.Values{"confirm_username": {"BOB"}}, cookie(token))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login?deleted=1" {
		t.Fatalf("delete: %d %q, want 303 /login?deleted=1", rec.Code, rec.Header().Get("Location"))
	}
	if c := sessionFrom(t, rec); c.MaxAge >= 0 {
		t.Fatalf("cookie not cleared: %+v", c)
	}
	assertContains(t, do(t, env.H, "GET", "/login?deleted=1", nil, anon).Body.String(), "Your account was deleted.")
}

func TestDeletedAccountCookieStopsWorking(t *testing.T) {
	env := newTestEnv(t)
	token := signedUp(t, env, "bob")
	keep := signedUp2(t, env, "bob") // a 30-day session in another browser
	do(t, env.H, "POST", "/account/delete", url.Values{"confirm_username": {"bob"}}, cookie(token))
	for _, tok := range []string{token, keep} {
		if rec := do(t, env.H, "GET", "/", nil, cookie(tok)); rec.Code != http.StatusSeeOther {
			t.Errorf("page with deleted user's cookie: %d, want 303", rec.Code)
		}
		if rec := do(t, env.H, "POST", "/items", url.Values{"text": {"x"}}, cookie(tok, htmxHeaders)); rec.Code != http.StatusUnauthorized {
			t.Errorf("htmx with deleted user's cookie: %d, want 401", rec.Code)
		}
	}
}
