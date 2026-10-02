package web_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

const pw = "correct horse battery"

func TestProtectedRoutesNeedSession(t *testing.T) {
	env := newTestEnv(t)
	rec := do(t, env.H, "GET", "/", nil, anon)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
		t.Fatalf("GET / without session: %d %q, want 303 /login", rec.Code, rec.Header().Get("Location"))
	}
	for _, r := range []struct{ method, path string }{
		{"GET", "/items/1/edit"}, {"POST", "/items"}, {"POST", "/notifications/claim"},
	} {
		rec := do(t, env.H, r.method, r.path, url.Values{}, map[string]string{anonHeader: "1", "HX-Request": "true"})
		if rec.Code != http.StatusUnauthorized || rec.Header().Get("HX-Redirect") != "/login" {
			t.Errorf("%s %s without session: %d, HX-Redirect %q; want 401 /login", r.method, r.path, rec.Code, rec.Header().Get("HX-Redirect"))
		}
	}
	if rec := do(t, env.H, "GET", "/static/app.css", nil, anon); rec.Code != http.StatusOK {
		t.Errorf("static file without session: %d, want 200", rec.Code)
	}
}

func TestLoginPage(t *testing.T) {
	env := newTestEnv(t)
	body := do(t, env.H, "GET", "/login", nil, anon).Body.String()
	assertContains(t, body, `<form method="post" action="/login"`, `name="username"`, `name="password"`,
		`type="password"`, `name="keep"`, "Keep me logged in", "Log in", `href="/signup"`, "No account yet?")
	rec := do(t, env.H, "GET", "/login", nil, nil) // with the tester's cookie
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("GET /login while logged in: %d %q, want 303 /", rec.Code, rec.Header().Get("Location"))
	}
}

func TestLogin(t *testing.T) {
	env := newTestEnv(t)
	if _, err := env.Auth.SignUp(context.Background(), "alice", pw); err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	rec := do(t, env.H, "POST", "/login", url.Values{"username": {"alice"}, "password": {pw}}, anon)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("login: %d %q, want 303 /", rec.Code, rec.Header().Get("Location"))
	}
	c := sessionFrom(t, rec)
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.MaxAge != 0 {
		t.Fatalf("cookie = %+v, want HttpOnly Secure Lax Path=/ without Max-Age", c)
	}
	page := do(t, env.H, "GET", "/", nil, cookie(c.Value)).Body.String()
	assertContains(t, page, "Signed in as <strong>alice</strong>")

	keep := do(t, env.H, "POST", "/login", url.Values{"username": {"alice"}, "password": {pw}, "keep": {"1"}}, anon)
	if c := sessionFrom(t, keep); c.MaxAge != 30*24*3600 {
		t.Fatalf("Keep me logged in: Max-Age = %d, want %d", c.MaxAge, 30*24*3600)
	}

	for _, form := range []url.Values{
		{"username": {"alice"}, "password": {"wrong password"}},
		{"username": {"nobody"}, "password": {pw}},
	} {
		rec := do(t, env.H, "POST", "/login", form, anon)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("bad login %v: status %d, want 422", form, rec.Code)
		}
		assertContains(t, rec.Body.String(), "Wrong username or password.", `value="`+form.Get("username")+`"`)
	}
}

func TestSignup(t *testing.T) {
	env := newTestEnv(t)
	body := do(t, env.H, "GET", "/signup", nil, anon).Body.String()
	assertContains(t, body, `<form method="post" action="/signup" class="auth-form" id="signup-form" data-pow-bits="4">`,
		"Create an account", "3–32 letters, digits, - or _", "At least 8 characters", `class="show-password"`,
		"There is no email. If you forget your password, ask the admin.", "Create account", `href="/login"`,
		`<script src="/static/auth.js" defer></script>`, `name="form_token"`, `name="pow_nonce"`,
		`<div class="hp" aria-hidden="true">`, `name="website" tabindex="-1" autocomplete="off"`)

	rec := do(t, env.H, "POST", "/signup", signupForm(t, env, "bob", pw), anon)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("sign-up: %d %q, want 303 /", rec.Code, rec.Header().Get("Location"))
	}
	if c := sessionFrom(t, rec); c.MaxAge != 0 {
		t.Fatalf("sign-up session must not be persistent: Max-Age %d", c.MaxAge)
	}

	cases := []struct {
		user, pass, msg string
	}{
		{"BOB", pw, "This username is taken."},
		{"b", pw, "Use 3–32 letters, digits, - or _."},
		{"carol", "short", "Use at least 8 characters."},
	}
	for _, c := range cases {
		rec := do(t, env.H, "POST", "/signup", signupForm(t, env, c.user, c.pass), anon)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: status %d, want 422", c.user, rec.Code)
		}
		assertContains(t, rec.Body.String(), c.msg, `value="`+c.user+`"`)
		assertNotContains(t, rec.Body.String(), `value="`+pw+`"`)
	}
}

func TestLogout(t *testing.T) {
	env := newTestEnv(t)
	rec := do(t, env.H, "POST", "/logout", url.Values{}, nil)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
		t.Fatalf("logout: %d %q, want 303 /login", rec.Code, rec.Header().Get("Location"))
	}
	if c := sessionFrom(t, rec); c.MaxAge >= 0 || c.Value != "" {
		t.Fatalf("logout cookie = %+v, want cleared", c)
	}
	if rec := do(t, env.H, "GET", "/", nil, cookie(env.Token)); rec.Code != http.StatusSeeOther {
		t.Fatalf("old token after logout: %d, want 303", rec.Code)
	}
}

func TestHeaderShowsUserAndLogout(t *testing.T) {
	env := newTestEnv(t)
	body := do(t, env.H, "GET", "/", nil, nil).Body.String()
	assertContains(t, body, "Signed in as <strong>tester</strong>", `<a href="/account">Account</a>`,
		`<form method="post" action="/logout" class="inline-form">`, `<button type="submit" class="link-button">Log out</button>`)
}

func TestItemsAreIsolatedPerUser(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	a, _ := env.Svc.Add(ctx, env.User.ID, "Secret of tester", "2020-01-01", "08:00")
	bob, err := env.Auth.SignUp(ctx, "bob", pw)
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	bobToken, _, _ := env.Auth.StartSession(ctx, bob.ID, false)
	id := strconv.FormatInt(a.ID, 10)
	asBob := cookie(bobToken, htmxHeaders)

	if body := do(t, env.H, "GET", "/", nil, cookie(bobToken)).Body.String(); strings.Contains(body, "Secret of tester") {
		t.Fatal("bob sees tester's item in the list")
	}
	for _, r := range []struct {
		method, path string
		form         url.Values
	}{
		{"GET", "/items/" + id, nil},
		{"GET", "/items/" + id + "/edit", nil},
		{"PUT", "/items/" + id, url.Values{"text": {"hacked"}}},
		{"POST", "/items/" + id + "/toggle", url.Values{}},
		{"POST", "/items/" + id + "/postpone?minutes=5", url.Values{}},
		{"DELETE", "/items/" + id, nil},
	} {
		if rec := do(t, env.H, r.method, r.path, r.form, asBob); rec.Code != http.StatusNotFound {
			t.Errorf("bob %s %s: %d, want 404", r.method, r.path, rec.Code)
		}
	}
	var claimed []json.RawMessage
	json.Unmarshal(do(t, env.H, "POST", "/notifications/claim", nil, asBob).Body.Bytes(), &claimed)
	if len(claimed) != 0 {
		t.Fatalf("bob claimed tester's due item: %d", len(claimed))
	}
	got, err := env.Svc.Get(ctx, env.User.ID, a.ID)
	if err != nil || got.Text != "Secret of tester" || got.Done {
		t.Fatalf("tester's item changed: %+v, %v", got, err)
	}
	if added := do(t, env.H, "POST", "/items", url.Values{"text": {"Bob's item"}}, asBob); added.Code != http.StatusOK {
		t.Fatalf("bob add: %d", added.Code)
	}
	if items, _ := env.Svc.List(ctx, bob.ID, false); len(items) != 1 || items[0].Text != "Bob's item" {
		t.Fatalf("bob's list = %+v", items)
	}
}

func TestAppJSStopsClaimingWhenLoggedOut(t *testing.T) {
	env := newTestEnv(t)
	js := do(t, env.H, "GET", "/static/app.js", nil, anon).Body.String()
	assertContains(t, js, "res.status === 401", "claimStopped = true")
}

func TestLoginEndsPreviousSession(t *testing.T) {
	env := newTestEnv(t)
	env.Auth.SignUp(context.Background(), "alice", pw)
	old, _, _ := env.Auth.LogIn(context.Background(), "alice", pw, true)
	rec := do(t, env.H, "POST", "/login", url.Values{"username": {"alice"}, "password": {pw}}, cookie(old))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("login: %d", rec.Code)
	}
	if rec := do(t, env.H, "GET", "/", nil, cookie(old)); rec.Code != http.StatusSeeOther {
		t.Fatalf("old session still valid after a new login: %d", rec.Code)
	}
}

func TestSignupEndsPreviousSession(t *testing.T) {
	env := newTestEnv(t)
	env.Auth.SignUp(context.Background(), "alice", pw)
	old, _, _ := env.Auth.LogIn(context.Background(), "alice", pw, true)
	v := signupForm(t, env, "bob", pw) // the form comes from an anonymous page load
	if rec := do(t, env.H, "POST", "/signup", v, cookie(old)); rec.Code != http.StatusSeeOther {
		t.Fatalf("sign-up: %d", rec.Code)
	}
	if rec := do(t, env.H, "GET", "/", nil, cookie(old)); rec.Code != http.StatusSeeOther {
		t.Fatalf("old session still valid after a sign-up in the same browser: %d", rec.Code)
	}
}

func TestLoginWithInvalidOldCookie(t *testing.T) {
	env := newTestEnv(t)
	env.Auth.SignUp(context.Background(), "alice", pw)
	rec := do(t, env.H, "POST", "/login", url.Values{"username": {"alice"}, "password": {pw}}, cookie("garbage"))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("login: %d %q, want 303 /", rec.Code, rec.Header().Get("Location"))
	}
	if c := sessionFrom(t, rec); c.Value == "" || c.Value == "garbage" {
		t.Fatalf("no new session cookie: %+v", c)
	}
}
