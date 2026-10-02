package web_test

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"todo/internal/auth"
	"todo/internal/guard"
)

const spamMsg = "Sign-up failed. Please wait a moment and try again."

func assertSpam(t *testing.T, env *testEnv, name string, form url.Values, h map[string]string) {
	t.Helper()
	rec := do(t, env.H, "POST", "/signup", form, h)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("%s: status %d, want 422", name, rec.Code)
	}
	assertContains(t, rec.Body.String(), spamMsg)
	if _, err := env.Store.UserByName(context.Background(), form.Get("username")); !errors.Is(err, auth.ErrNoUser) && form.Get("username") != "" {
		t.Errorf("%s: user %q was created", name, form.Get("username"))
	}
}

func TestSignupSpamChecks(t *testing.T) {
	env := newTestEnv(t)
	mutations := map[string]func(url.Values){
		"bad signature": func(v url.Values) { v.Set("form_token", v.Get("form_token")+"x") },
		"missing token": func(v url.Values) { v.Del("form_token") },
		"honeypot":      func(v url.Values) { v.Set("website", "http://spam.example") },
		"weak proof": func(v url.Values) {
			for i := 0; ; i++ {
				if n := strconv.Itoa(i); !guard.ProofOK(v.Get("form_token"), n, 4) {
					v.Set("pow_nonce", n)
					return
				}
			}
		},
	}
	i := 0
	for name, mutate := range mutations {
		i++
		v := signupForm(t, env, "spam"+strconv.Itoa(i), pw)
		mutate(v)
		assertSpam(t, env, name, v, anon)
	}

	// Too fast: no waiting after loading the page.
	page := do(t, env.H, "GET", "/signup", nil, anon).Body.String()
	tok := formTokenRe.FindStringSubmatch(page)[1]
	assertSpam(t, env, "too fast", url.Values{"username": {"fast1"}, "password": {pw}, "form_token": {tok},
		"pow_nonce": {nonceFor(t, tok, 4)}}, anon)

	// Too old.
	v := signupForm(t, env, "old1", pw)
	env.Clock.t = env.Clock.t.Add(guard.MaxTokenAge + time.Minute)
	assertSpam(t, env, "too old", v, anon)

	// Reused token.
	v = signupForm(t, env, "reuse1", pw)
	if rec := do(t, env.H, "POST", "/signup", v, anon); rec.Code != http.StatusSeeOther {
		t.Fatalf("first use: %d, want 303", rec.Code)
	}
	v.Set("username", "reuse2")
	assertSpam(t, env, "reused token", v, anon)
}

func TestSignupRetryAfterRuleError(t *testing.T) {
	env := newTestEnv(t)
	rec := do(t, env.H, "POST", "/signup", signupForm(t, env, "dave", "short"), anon)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("short password: %d, want 422", rec.Code)
	}
	m := formTokenRe.FindStringSubmatch(rec.Body.String())
	if m == nil {
		t.Fatal("no fresh form_token after a rule error")
	}
	env.Clock.t = env.Clock.t.Add(guard.MinFillTime + time.Second)
	fixed := url.Values{"username": {"dave"}, "password": {pw}, "form_token": {m[1]}, "pow_nonce": {nonceFor(t, m[1], 4)}}
	if rec := do(t, env.H, "POST", "/signup", fixed, anon); rec.Code != http.StatusSeeOther {
		t.Fatalf("corrected form: %d, want 303", rec.Code)
	}
}

func TestSignupAccountsPerIPLimit(t *testing.T) {
	env := newTestEnv(t)
	ip := fromIP("198.51.100.1")
	for i := 1; i <= 5; i++ {
		if rec := do(t, env.H, "POST", "/signup", signupForm(t, env, "user"+strconv.Itoa(i), pw, ip), ip); rec.Code != http.StatusSeeOther {
			t.Fatalf("account %d: %d, want 303", i, rec.Code)
		}
	}
	assertSpam(t, env, "6th account", signupForm(t, env, "user6", pw, ip), ip)
	other := fromIP("198.51.100.2")
	if rec := do(t, env.H, "POST", "/signup", signupForm(t, env, "user7", pw, other), other); rec.Code != http.StatusSeeOther {
		t.Fatalf("other IP: %d, want 303", rec.Code)
	}
	env.Clock.t = env.Clock.t.Add(time.Hour)
	if rec := do(t, env.H, "POST", "/signup", signupForm(t, env, "user8", pw, ip), ip); rec.Code != http.StatusSeeOther {
		t.Fatalf("after an hour: %d, want 303", rec.Code)
	}
}

func TestSignupAttemptsPerIPLimit(t *testing.T) {
	env := newTestEnv(t)
	ip := fromIP("198.51.100.9")
	for i := 0; i < 30; i++ {
		do(t, env.H, "POST", "/signup", url.Values{"username": {"x"}, "password": {"y"}}, ip)
	}
	assertSpam(t, env, "31st attempt", signupForm(t, env, "valid1", pw, ip), ip)
}

func TestAuthJSSolvesPuzzle(t *testing.T) {
	env := newTestEnv(t)
	js := do(t, env.H, "GET", "/static/auth.js", nil, anon).Body.String()
	assertContains(t, js, `crypto.subtle.digest("SHA-256"`, "Checking that you are human…", "pow_nonce", "powBits")
}
