# Users and Login (Plan 2: Protection and Admin) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Sign-up is protected against spam without third parties, logins are limited against password guessing, the app can run behind an HTTPS proxy, and the admin can list users, reset passwords, and delete users from the command line.

**Architecture:** A new package `internal/guard` holds an in-memory sliding-window `Limiter` (with reserve/release, so a slot is taken before the slow hash) and signed sign-up `Tokens` (minimum fill time, maximum age, one use, SHA-256 proof-of-work). `internal/web` gets options (`WithClock`, `WithPowBits`, `WithSigningKey`, `WithTrustProxy`), a `clientIP` helper, and uses the guards in the sign-up and login handlers. `auth.js` solves the puzzle in the browser. `cmd/todo` gets a `users` subcommand that opens the database without migrating it.

**Tech Stack:** Go 1.27.1, standard library (`crypto/hmac`, `crypto/sha256`, `text/tabwriter`), WebCrypto `crypto.subtle.digest` in the browser.

**Execution (chosen by the user):** Native — superpowers:executing-plans, then one whole-branch review by a fresh reviewer. One commit per task on branch `users-guard`. No push, no PR.

**Spec:** `docs/superpowers/specs/2026-10-01-users-design.md` — sections 6 (sign-up spam protection, login protection, client IP behind a proxy) and 8 (admin command line), plus the matching parts of 9 and 10. Plan 1 (accounts) is on `main` (`586678b`).

## Global Constraints

- Work in a new worktree `~/claude-personal/todo-worktrees/users-guard`, branch `users-guard`, from `main` (`586678b`). Plain `go …` (never `GOTOOLCHAIN=local`); fish shell; check `go test`'s own exit code.
- No new Go modules.
- Sign-up checks (spec 6): signature (HMAC-SHA256, key in `settings` row `signing_key`), at least 3 s, at most 1 h, one use, honeypot field `website` empty, proof-of-work `SHA-256(form_token + ":" + nonce)` with at least `D` leading zero bits (`D` = 16 by default), at most 5 new accounts and 30 sign-up attempts per IP per hour.
- Login limits (spec 6): 5 wrong passwords per username (key = lowercase username) and 20 failed logins per IP, each within 15 minutes; a slot is reserved before the password hash and released on success.
- Messages (exact): "Sign-up failed. Please wait a moment and try again." (all spam and rate-limit failures, status 422) · "Too many attempts. Please try again in 15 minutes." (login, status 429) · busy button text "Checking that you are human…".
- `-trust-proxy`: client IP = last address of the last `X-Forwarded-For` header line; otherwise the host of `RemoteAddr`.
- CLI (spec 8): `todo users list | reset-password <name> | delete <name> [-yes]`, `-db` (default `todo.db`), flags before or after the name. Outputs: table `USERNAME  CREATED  ITEMS  SESSIONS`; `New password for <name>: <16 characters>`; prompt `Type the username to delete <name> and all items: `; `Deleted <name>.`; unknown user → `todo: no user "<name>"`, exit 1. The CLI never migrates or creates a database.
- One commit per task, with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. No push, no PR.

## Review Focus

1. A burst of parallel wrong logins for one username must not run more than 5 password hashes in 15 minutes: the slot is reserved before hashing. → Task 3 `TestParallelWrongLoginsAreCapped`.
2. "ALICE" and "alice" share one login counter, so changing the case does not give 5 more tries. → Task 3 `TestLoginLimitIgnoresUsernameCase`.
3. After a rejected sign-up (for example a short password), the page has a fresh form token, so the corrected form works. → Task 2 `TestSignupRetryAfterRuleError`.
4. Without `-trust-proxy`, a forged `X-Forwarded-For` must not change the limiter key. → Task 3 `TestForwardedForOnlyWithTrustProxy`.
5. The admin commands on a missing or old database must not create or upgrade it. → Task 4 `TestUsersCommandNeedsCurrentDatabase`.

---

## File map

| File | Responsibility |
|---|---|
| `internal/guard/limiter.go` (new) | Sliding-window limiter with `Reserve`/`Release` |
| `internal/guard/tokens.go` (new) | Signed sign-up tokens and the proof-of-work check |
| `internal/store/sqlite/settings.go` (new) | `SigningKey` (create once, then read) |
| `internal/web/server.go` | `Option`s, guard fields, `clientIP` |
| `internal/web/auth_handlers.go` | spam checks in sign-up, limits in login |
| `internal/web/templates/auth.html`, `static/auth.js`, `static/app.css` | hidden fields, honeypot, puzzle solver |
| `internal/auth/auth.go`, `service.go` | `DeleteSessions` in the store, `ResetPassword` |
| `internal/store/sqlite/sqlite.go`, `users.go` | `Options.RequireCurrent`, `ListUsers`, `DeleteSessions` |
| `cmd/todo/main.go`, `cmd/todo/users.go` (new) | `-trust-proxy`, signing key, `users` subcommand |

---

### Task 1: Package `guard` — limiter and sign-up tokens

**Files:**
- Create: `internal/guard/limiter.go`, `internal/guard/tokens.go`
- Test: `internal/guard/guard_test.go`
- Copy this plan to `docs/superpowers/plans/2026-10-02-users-guard.md` (commit with this task)

**Interfaces:**
- Produces: `guard.NewLimiter(max int, window time.Duration, now func() time.Time) *Limiter`; `(*Limiter).Reserve(key string) bool`; `(*Limiter).Release(key string)`; `guard.NewTokens(key []byte, now func() time.Time) *Tokens`; `(*Tokens).New() string`; `(*Tokens).Check(token, nonce string, bits int) error`; `guard.ProofOK(token, nonce string, bits int) bool`; errors `ErrBadToken`, `ErrTooFast`, `ErrTooOld`, `ErrUsedToken`, `ErrWeakProof`; constants `MinFillTime = 3 * time.Second`, `MaxTokenAge = time.Hour`, `DefaultPowBits = 16`.

- [ ] **Step 1: Worktree and plan copy**

```bash
git -C ~/claude-personal/todo worktree add -b users-guard ../todo-worktrees/users-guard
cd ~/claude-personal/todo-worktrees/users-guard
```

Copy this plan to `docs/superpowers/plans/2026-10-02-users-guard.md`.

- [ ] **Step 2: Write the failing tests** — `internal/guard/guard_test.go`:

```go
package guard_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"todo/internal/guard"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func TestLimiterReserveAndRelease(t *testing.T) {
	c := &clock{t: time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)}
	l := guard.NewLimiter(3, 15*time.Minute, c.now)
	for i := 0; i < 3; i++ {
		if !l.Reserve("a") {
			t.Fatalf("reserve %d refused", i+1)
		}
	}
	if l.Reserve("a") {
		t.Fatal("4th reserve allowed, want refused")
	}
	if !l.Reserve("b") {
		t.Fatal("other key refused")
	}
	l.Release("a")
	if !l.Reserve("a") {
		t.Fatal("reserve after release refused")
	}
	c.t = c.t.Add(15*time.Minute + time.Second)
	if !l.Reserve("a") {
		t.Fatal("reserve after the window refused")
	}
	l.Release("never-used") // must not panic
}

func solve(t *testing.T, token string, bits int) string {
	t.Helper()
	for i := 0; i < 1<<22; i++ {
		if n := strconv.Itoa(i); guard.ProofOK(token, n, bits) {
			return n
		}
	}
	t.Fatal("no nonce found")
	return ""
}

func TestTokens(t *testing.T) {
	c := &clock{t: time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)}
	tk := guard.NewTokens([]byte("0123456789abcdef0123456789abcdef"), c.now)
	tok := tk.New()
	nonce := solve(t, tok, 8)

	if err := tk.Check(tok, nonce, 8); !errors.Is(err, guard.ErrTooFast) {
		t.Fatalf("at once: err = %v, want ErrTooFast", err)
	}
	c.t = c.t.Add(guard.MinFillTime)
	if err := tk.Check(tok, solveWeak(t, tok, 8), 8); !errors.Is(err, guard.ErrWeakProof) {
		t.Fatalf("weak nonce: err = %v, want ErrWeakProof", err)
	}
	if err := tk.Check(tok, nonce, 8); err != nil {
		t.Fatalf("valid token: err = %v", err)
	}
	if err := tk.Check(tok, nonce, 8); !errors.Is(err, guard.ErrUsedToken) {
		t.Fatalf("second use: err = %v, want ErrUsedToken", err)
	}

	old := tk.New()
	c.t = c.t.Add(guard.MaxTokenAge + time.Second)
	if err := tk.Check(old, solve(t, old, 8), 8); !errors.Is(err, guard.ErrTooOld) {
		t.Fatalf("old token: err = %v, want ErrTooOld", err)
	}

	other := guard.NewTokens([]byte("another key, 32 bytes long......"), c.now)
	forged := other.New()
	c.t = c.t.Add(guard.MinFillTime)
	last := "A"
	if strings.HasSuffix(tok, "A") {
		last = "B"
	}
	for _, bad := range []string{"", "x", forged, tok[:len(tok)-1] + last, strings.Replace(tok, ".", "", 1)} {
		if err := tk.Check(bad, "0", 0); !errors.Is(err, guard.ErrBadToken) {
			t.Errorf("Check(%q) err = %v, want ErrBadToken", bad, err)
		}
	}
}

// solveWeak finds a nonce that does NOT meet the difficulty.
func solveWeak(t *testing.T, token string, bits int) string {
	t.Helper()
	for i := 0; ; i++ {
		if n := strconv.Itoa(i); !guard.ProofOK(token, n, bits) {
			return n
		}
	}
}

func TestProofOK(t *testing.T) {
	if !guard.ProofOK("anything", "0", 0) {
		t.Fatal("0 bits must always pass")
	}
	if guard.ProofOK("anything", strings.Repeat("9", 100), 0) {
		t.Fatal("a nonce longer than 32 characters must fail")
	}
}
```

- [ ] **Step 3: Run to see them fail**

Run: `go test ./internal/guard/`
Expected: FAIL (`no non-test Go files`).

- [ ] **Step 4: Create `internal/guard/limiter.go`**

```go
// Package guard limits requests and checks sign-up forms against spam.
package guard

import (
	"sync"
	"time"
)

// Limiter counts hits per key in a sliding time window. It lives in memory
// and resets when the app restarts.
type Limiter struct {
	max    int
	window time.Duration
	now    func() time.Time

	mu    sync.Mutex
	hits  map[string][]time.Time
	calls int
}

// NewLimiter allows at most max hits per key within window.
func NewLimiter(max int, window time.Duration, now func() time.Time) *Limiter {
	return &Limiter{max: max, window: window, now: now, hits: map[string][]time.Time{}}
}

// Reserve records one hit for key if fewer than max hits are in the window,
// and reports whether it did. A reserved hit counts until Release.
func (l *Limiter) Reserve(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.calls++; l.calls%1000 == 0 {
		l.sweep()
	}
	h := l.recent(key)
	if len(h) >= l.max {
		l.hits[key] = h
		return false
	}
	l.hits[key] = append(h, l.now())
	return true
}

// Release removes one hit of key, for a reservation that should not count.
func (l *Limiter) Release(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	h := l.hits[key]
	if len(h) == 0 {
		return
	}
	if h = h[:len(h)-1]; len(h) == 0 {
		delete(l.hits, key)
		return
	}
	l.hits[key] = h
}

// recent returns the hits of key inside the window. Callers hold l.mu.
func (l *Limiter) recent(key string) []time.Time {
	cutoff := l.now().Add(-l.window)
	h := l.hits[key]
	i := 0
	for i < len(h) && !h[i].After(cutoff) {
		i++
	}
	return h[i:]
}

// sweep drops keys without recent hits. Callers hold l.mu.
func (l *Limiter) sweep() {
	for k := range l.hits {
		if h := l.recent(k); len(h) == 0 {
			delete(l.hits, k)
		} else {
			l.hits[k] = h
		}
	}
}
```

- [ ] **Step 5: Create `internal/guard/tokens.go`**

```go
package guard

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"math/bits"
	"strings"
	"sync"
	"time"
)

const (
	// MinFillTime is the shortest time between showing and sending the form.
	MinFillTime = 3 * time.Second
	// MaxTokenAge is the longest time a form token stays valid.
	MaxTokenAge = time.Hour
	// DefaultPowBits is the proof-of-work difficulty (about one second in a browser).
	DefaultPowBits = 16
)

var (
	ErrBadToken  = errors.New("guard: bad form token")
	ErrTooFast   = errors.New("guard: form sent too fast")
	ErrTooOld    = errors.New("guard: form token too old")
	ErrUsedToken = errors.New("guard: form token already used")
	ErrWeakProof = errors.New("guard: proof of work too weak")
)

var b64 = base64.RawURLEncoding

// Tokens makes and checks signed sign-up form tokens.
type Tokens struct {
	key []byte
	now func() time.Time

	mu   sync.Mutex
	used map[string]time.Time
}

// NewTokens signs tokens with key.
func NewTokens(key []byte, now func() time.Time) *Tokens {
	return &Tokens{key: key, now: now, used: map[string]time.Time{}}
}

// New returns a token: base64url(8-byte Unix seconds + 16 random bytes) "." base64url(HMAC).
func (t *Tokens) New() string {
	payload := make([]byte, 24)
	binary.BigEndian.PutUint64(payload, uint64(t.now().Unix()))
	rand.Read(payload[8:])
	p := b64.EncodeToString(payload)
	return p + "." + b64.EncodeToString(t.sign(p))
}

func (t *Tokens) sign(p string) []byte {
	m := hmac.New(sha256.New, t.key)
	m.Write([]byte(p))
	return m.Sum(nil)
}

// Check verifies the signature, the age, the proof of work, and one use.
func (t *Tokens) Check(token, nonce string, bits int) error {
	p, sig, ok := strings.Cut(token, ".")
	if !ok {
		return ErrBadToken
	}
	got, err := b64.DecodeString(sig)
	if err != nil || !hmac.Equal(got, t.sign(p)) {
		return ErrBadToken
	}
	payload, err := b64.DecodeString(p)
	if err != nil || len(payload) != 24 {
		return ErrBadToken
	}
	age := t.now().Sub(time.Unix(int64(binary.BigEndian.Uint64(payload)), 0))
	switch {
	case age < MinFillTime:
		return ErrTooFast
	case age > MaxTokenAge:
		return ErrTooOld
	}
	if !ProofOK(token, nonce, bits) {
		return ErrWeakProof
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, seen := t.used[token]; seen {
		return ErrUsedToken
	}
	if len(t.used) > 1000 {
		for k, at := range t.used {
			if t.now().Sub(at) > MaxTokenAge {
				delete(t.used, k)
			}
		}
	}
	t.used[token] = t.now()
	return nil
}

// ProofOK reports whether SHA-256(token + ":" + nonce) starts with at least
// bits zero bits. Nonces longer than 32 characters never pass.
func ProofOK(token, nonce string, want int) bool {
	if len(nonce) > 32 {
		return false
	}
	sum := sha256.Sum256([]byte(token + ":" + nonce))
	zeros := 0
	for _, b := range sum {
		if b == 0 {
			zeros += 8
			continue
		}
		zeros += bits.LeadingZeros8(b)
		break
	}
	return zeros >= want
}
```

- [ ] **Step 6: Run the tests**

Run: `go test -count=1 ./internal/guard/` then `go test -count=1 ./... && go vet ./...`
Expected: all exit 0.

- [ ] **Step 7: Commit**

```bash
git add docs/superpowers/plans/2026-10-02-users-guard.md internal/guard
git commit -m "feat: guard package with limiter and signed sign-up tokens"
```

---

### Task 2: Sign-up spam protection

**Files:**
- Create: `internal/store/sqlite/settings.go`, `internal/store/sqlite/settings_test.go`
- Modify: `internal/web/server.go`, `internal/web/auth_handlers.go`, `internal/web/templates/auth.html`, `internal/web/static/auth.js`, `internal/web/static/app.css`, `cmd/todo/main.go`, `internal/web/web_test.go`, `internal/web/auth_test.go` (replace `TestSignup`)
- Test: `internal/web/signup_guard_test.go` (new)

**Interfaces:**
- Consumes: `guard` (Task 1).
- Produces: `(*sqlite.Store).SigningKey(ctx) ([]byte, error)`; `web.Option`; `web.WithClock(func() time.Time)`, `web.WithPowBits(int)`, `web.WithSigningKey([]byte)`, `web.WithTrustProxy(bool)`; `web.New(svc, accounts, log, opts ...Option)`; `(*server).clientIP(r) string`; test helpers `testClock`, `env.Clock`, `newTestEnv(t, opts ...web.Option)`, `signupForm(t, env, username, password) url.Values`, `fromIP` header.

- [ ] **Step 1: Write the failing store test** — `internal/store/sqlite/settings_test.go`:

```go
package sqlite_test

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"todo/internal/store/sqlite"
)

func TestSigningKeyIsCreatedOnceAndKept(t *testing.T) {
	path := filepath.Join(t.TempDir(), "k.db")
	s, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	k1, err := s.SigningKey(context.Background())
	if err != nil || len(k1) != 32 {
		t.Fatalf("SigningKey = %x, %v; want 32 bytes", k1, err)
	}
	k2, _ := s.SigningKey(context.Background())
	s.Close()
	s, _ = sqlite.Open(path)
	defer s.Close()
	k3, _ := s.SigningKey(context.Background())
	if !bytes.Equal(k1, k2) || !bytes.Equal(k1, k3) {
		t.Fatal("signing key changed")
	}
}
```

- [ ] **Step 2: Test helpers** — in `internal/web/web_test.go`:

Add a clock and IP override, and let `newTestEnv` take options. Replace the `raw, err := web.New(…)` line with:

```go
	clock := &testClock{t: time.Now()}
	opts = append([]web.Option{web.WithClock(clock.Now), web.WithPowBits(4)}, opts...)
	raw, err := web.New(svc, accounts, slog.New(slog.NewTextHandler(io.Discard, nil)), opts...)
```

change the signature to `func newTestEnv(t *testing.T, opts ...web.Option) *testEnv`, add `Clock *testClock` to `testEnv` and set it in the returned struct, and in the wrapper `http.HandlerFunc` add before `raw.ServeHTTP(w, r)`:

```go
		if ip := r.Header.Get(fromIPHeader); ip != "" {
			r.RemoteAddr = ip + ":40000"
			r.Header.Del(fromIPHeader)
		}
```

Append:

```go
type testClock struct{ t time.Time }

func (c *testClock) Now() time.Time { return c.t }

// fromIPHeader makes a test request come from another client address.
const fromIPHeader = "X-Test-Remote-IP"

func fromIP(ip string) map[string]string { return map[string]string{fromIPHeader: ip, anonHeader: "1"} }

var formTokenRe = regexp.MustCompile(`name="form_token" value="([^"]+)"`)

// signupForm loads the sign-up page (as headers h, default anon), waits past
// the minimum fill time, solves the puzzle, and returns a valid form.
func signupForm(t *testing.T, env *testEnv, username, password string, h ...map[string]string) url.Values {
	t.Helper()
	headers := anon
	if len(h) > 0 {
		headers = h[0]
	}
	page := do(t, env.H, "GET", "/signup", nil, headers).Body.String()
	m := formTokenRe.FindStringSubmatch(page)
	if m == nil {
		t.Fatalf("no form_token in sign-up page:\n%s", page)
	}
	env.Clock.t = env.Clock.t.Add(guard.MinFillTime + time.Second)
	return url.Values{"username": {username}, "password": {password}, "form_token": {m[1]},
		"pow_nonce": {nonceFor(t, m[1], 4)}, "website": {""}}
}

func nonceFor(t *testing.T, token string, bits int) string {
	t.Helper()
	for i := 0; i < 1<<20; i++ {
		if n := strconv.Itoa(i); guard.ProofOK(token, n, bits) {
			return n
		}
	}
	t.Fatal("no nonce")
	return ""
}
```

Add imports `"regexp"`, `"strconv"`, `"todo/internal/guard"`.

In `internal/web/auth_test.go`, replace the whole `TestSignup` with:

```go
func TestSignup(t *testing.T) {
	env := newTestEnv(t)
	body := do(t, env.H, "GET", "/signup", nil, anon).Body.String()
	assertContains(t, body, `<form method="post" action="/signup" class="auth-form" id="signup-form" data-pow-bits="4">`,
		"Create an account", "3–32 letters, digits, - or _", "At least 10 characters", `class="show-password"`,
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
		{"carol", "short", "Use at least 10 characters."},
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
```

- [ ] **Step 3: Write the failing tests** — `internal/web/signup_guard_test.go`:

```go
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
```

- [ ] **Step 4: Run to see them fail**

Run: `go test ./internal/store/sqlite/ ./internal/web/`
Expected: FAIL to compile (`s.SigningKey undefined`, `web.WithClock undefined`).

- [ ] **Step 5: Create `internal/store/sqlite/settings.go`**

```go
package sqlite

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// SigningKey returns the key that signs sign-up form tokens. It is created
// at the first call and then kept in the settings table.
func (s *Store) SigningKey(ctx context.Context) ([]byte, error) {
	fresh := make([]byte, 32)
	if _, err := rand.Read(fresh); err != nil {
		return nil, fmt.Errorf("signing key: %w", err)
	}
	if _, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO settings (key, value) VALUES ('signing_key', ?)`, hex.EncodeToString(fresh)); err != nil {
		return nil, fmt.Errorf("signing key: %w", err)
	}
	var v string
	if err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = 'signing_key'`).Scan(&v); err != nil {
		return nil, fmt.Errorf("signing key: %w", err)
	}
	key, err := hex.DecodeString(v)
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("signing key: stored value is broken")
	}
	return key, nil
}
```

- [ ] **Step 6: `server.go`** — options, guard fields, client IP.

Add fields to `server`:

```go
	powBits    int
	trustProxy bool
	signingKey []byte
	tokens     *guard.Tokens
	signupTry  *guard.Limiter // sign-up attempts per IP
	signupNew  *guard.Limiter // new accounts per IP
	loginUser  *guard.Limiter // wrong passwords per lowercase username
	loginIP    *guard.Limiter // failed logins per IP
```

Change `New`:

```go
// Option changes how the app is set up.
type Option func(*server)

// WithClock replaces the clock used for labels and the guards (for tests).
func WithClock(now func() time.Time) Option { return func(s *server) { s.now = now } }

// WithPowBits sets the sign-up proof-of-work difficulty.
func WithPowBits(bits int) Option { return func(s *server) { s.powBits = bits } }

// WithSigningKey sets the key for sign-up form tokens. Without it, a random
// key is used, and open sign-up forms stop working after a restart.
func WithSigningKey(key []byte) Option { return func(s *server) { s.signingKey = key } }

// WithTrustProxy makes the app take the client IP from X-Forwarded-For.
func WithTrustProxy(trust bool) Option { return func(s *server) { s.trustProxy = trust } }

// New returns the HTTP handler for the app.
func New(svc *todo.Service, accounts *auth.Service, log *slog.Logger, opts ...Option) (http.Handler, error) {
	s := &server{svc: svc, accounts: accounts, log: log, now: time.Now, powBits: guard.DefaultPowBits}
	for _, o := range opts {
		o(s)
	}
	if s.signingKey == nil {
		s.signingKey = make([]byte, 32)
		rand.Read(s.signingKey)
	}
	s.tokens = guard.NewTokens(s.signingKey, s.now)
	s.signupTry = guard.NewLimiter(30, time.Hour, s.now)
	s.signupNew = guard.NewLimiter(5, time.Hour, s.now)
	s.loginUser = guard.NewLimiter(5, 15*time.Minute, s.now)
	s.loginIP = guard.NewLimiter(20, 15*time.Minute, s.now)
```

(the rest of `New` stays). Append:

```go
// clientIP is the address used for the limits. With -trust-proxy it is the
// last address in X-Forwarded-For, which the proxy adds.
func (s *server) clientIP(r *http.Request) string {
	if s.trustProxy {
		if vals := r.Header.Values("X-Forwarded-For"); len(vals) > 0 {
			parts := strings.Split(vals[len(vals)-1], ",")
			if ip := strings.TrimSpace(parts[len(parts)-1]); net.ParseIP(ip) != nil {
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
```

Imports: add `"crypto/rand"`, `"net"`, `"todo/internal/guard"`.

- [ ] **Step 7: Sign-up handler** — in `auth_handlers.go`, extend `signupView` with `FormToken string` and `PowBits int`, add the helper and replace `signupPage` and `signup`:

```go
const signupFailed = "Sign-up failed. Please wait a moment and try again."

// newSignupView returns a sign-up view with a fresh form token.
func (s *server) newSignupView(username string) signupView {
	return signupView{Username: username, FormToken: s.tokens.New(), PowBits: s.powBits}
}

func (s *server) signupPage(w http.ResponseWriter, r *http.Request) {
	if s.alreadyLoggedIn(w, r) {
		return
	}
	s.render(w, r, http.StatusOK, part{"signup", s.newSignupView("")})
}

func (s *server) signup(w http.ResponseWriter, r *http.Request) {
	username := r.PostFormValue("username")
	ip := s.clientIP(r)
	failed := func() {
		view := s.newSignupView(username)
		view.Error = signupFailed
		s.render(w, r, http.StatusUnprocessableEntity, part{"signup", view})
	}
	if !s.signupTry.Reserve(ip) { // every attempt counts; never released
		failed()
		return
	}
	if r.PostFormValue("website") != "" {
		failed()
		return
	}
	if err := s.tokens.Check(r.PostFormValue("form_token"), r.PostFormValue("pow_nonce"), s.powBits); err != nil {
		s.log.Info("sign-up rejected", "ip", ip, "reason", err)
		failed()
		return
	}
	if !s.signupNew.Reserve(ip) {
		failed()
		return
	}
	u, err := s.accounts.SignUp(r.Context(), username, r.PostFormValue("password"))
	if err != nil {
		s.signupNew.Release(ip) // no account was created
	}
	var rule *auth.RuleError
	switch {
	case errors.Is(err, auth.ErrUsernameTaken):
		view := s.newSignupView(username)
		view.UsernameError = "This username is taken."
		s.render(w, r, http.StatusUnprocessableEntity, part{"signup", view})
		return
	case errors.As(err, &rule):
		view := s.newSignupView(username)
		if rule.Field == "username" {
			view.UsernameError = rule.Msg
		} else {
			view.PasswordError = rule.Msg
		}
		s.render(w, r, http.StatusUnprocessableEntity, part{"signup", view})
		return
	case err != nil:
		s.serverError(w, r, err)
		return
	}
	token, sess, err := s.accounts.StartSession(r.Context(), u.ID, false)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	setSessionCookie(w, token, sess)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
```

- [ ] **Step 8: Template** — in `auth.html`, `signup`: change the form tag to

```html
<form method="post" action="/signup" class="auth-form" id="signup-form" data-pow-bits="{{.PowBits}}">
<input type="hidden" name="form_token" value="{{.FormToken}}">
<input type="hidden" name="pow_nonce" value="">
<div class="hp" aria-hidden="true"><label for="website">Website</label><input id="website" name="website" tabindex="-1" autocomplete="off"></div>
```

(the rest of the form stays).

- [ ] **Step 9: CSS** — append to `app.css`:

```css
/* Honeypot: hidden from people, visible to simple bots. */
.hp { position: absolute; left: -10000px; top: auto; width: 1px; height: 1px; overflow: hidden; }
.btn-primary:disabled { opacity: 0.75; cursor: progress; }
```

- [ ] **Step 10: `auth.js`** — append:

```js
// Sign-up: before the form is sent, find a proof-of-work nonce (about one
// second). The server checks SHA-256(form_token + ":" + nonce).
const signupForm = document.getElementById("signup-form");
if (signupForm) {
  signupForm.addEventListener("submit", async (e) => {
    const nonceField = signupForm.elements.pow_nonce;
    if (nonceField.value) {
      return; // solved: let the browser send the form
    }
    e.preventDefault();
    const button = signupForm.querySelector('button[type="submit"]');
    const label = button.textContent;
    button.disabled = true;
    button.textContent = "Checking that you are human…";
    try {
      nonceField.value = await solve(signupForm.elements.form_token.value, Number(signupForm.dataset.powBits));
      signupForm.submit();
    } catch (err) {
      button.disabled = false;
      button.textContent = label;
    }
  });
}

async function solve(token, powBits) {
  const encoder = new TextEncoder();
  for (let start = 0; ; start += 256) {
    const tries = [];
    for (let i = start; i < start + 256; i++) {
      tries.push(crypto.subtle.digest("SHA-256", encoder.encode(token + ":" + i))
        .then((hash) => (zeroBits(new Uint8Array(hash)) >= powBits ? i : -1)));
    }
    const found = (await Promise.all(tries)).find((n) => n >= 0);
    if (found !== undefined) {
      return String(found);
    }
  }
}

function zeroBits(bytes) {
  let n = 0;
  for (const b of bytes) {
    if (b === 0) {
      n += 8;
      continue;
    }
    return n + Math.clz32(b) - 24;
  }
  return n;
}
```

- [ ] **Step 11: `main.go`** — load the signing key and pass it (after `accounts := …`):

```go
	key, err := store.SigningKey(context.Background())
	if err != nil {
		return err
	}
	handler, err := web.New(todo.NewService(store), accounts, logger, web.WithSigningKey(key))
```

(replacing the old `handler, err := web.New(…)` line).

- [ ] **Step 12: Run all tests, vet, JS check**

Run: `go test -count=1 ./...`, `go vet ./...`, `node --check internal/web/static/auth.js`
Expected: all exit 0.

- [ ] **Step 13: Commit**

```bash
git add cmd internal
git commit -m "feat: sign-up spam protection (signed token, honeypot, proof of work, IP limits)"
```

---

### Task 3: Login limits and `-trust-proxy`

**Files:**
- Modify: `internal/web/auth_handlers.go` (`login`), `cmd/todo/main.go`, `docs/superpowers/specs/2026-10-01-users-design.md` (section 6, client IP)
- Test: `internal/web/login_guard_test.go` (new)

**Interfaces:**
- Consumes: `loginUser`, `loginIP`, `clientIP`, `WithTrustProxy` (Task 2); test helpers (Task 2).
- Produces: the final `login` handler; flag `-trust-proxy`.

- [ ] **Step 1: Write the failing tests** — `internal/web/login_guard_test.go`:

```go
package web_test

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"testing"
	"time"

	"todo/internal/web"
)

const lockedMsg = "Too many attempts. Please try again in 15 minutes."

func login(t *testing.T, env *testEnv, user, pass string, h map[string]string) int {
	t.Helper()
	return do(t, env.H, "POST", "/login", url.Values{"username": {user}, "password": {pass}}, h).Code
}

func TestLoginLimitPerUsername(t *testing.T) {
	env := newTestEnv(t)
	env.Auth.SignUp(context.Background(), "alice", pw)
	for i := 0; i < 5; i++ {
		if code := login(t, env, "alice", "wrong password", fromIP("203.0.113."+strconv.Itoa(i+1))); code != http.StatusUnprocessableEntity {
			t.Fatalf("wrong %d: %d, want 422", i+1, code)
		}
	}
	rec := do(t, env.H, "POST", "/login", url.Values{"username": {"alice"}, "password": {pw}}, fromIP("203.0.113.99"))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("6th try with the right password: %d, want 429", rec.Code)
	}
	assertContains(t, rec.Body.String(), lockedMsg)
	if code := login(t, env, "tester-other", "wrong password", fromIP("203.0.113.99")); code != http.StatusUnprocessableEntity {
		t.Fatalf("other username: %d, want 422 (not blocked)", code)
	}
	env.Clock.t = env.Clock.t.Add(15*time.Minute + time.Second)
	if code := login(t, env, "alice", pw, fromIP("203.0.113.99")); code != http.StatusSeeOther {
		t.Fatalf("after 15 minutes: %d, want 303", code)
	}
}

func TestLoginLimitIgnoresUsernameCase(t *testing.T) {
	env := newTestEnv(t)
	env.Auth.SignUp(context.Background(), "alice", pw)
	for i, name := range []string{"alice", "ALICE", "Alice", "aLiCe", "ALIce"} {
		login(t, env, name, "wrong password", fromIP("203.0.113."+strconv.Itoa(i+1)))
	}
	if code := login(t, env, "alicE", pw, fromIP("203.0.113.50")); code != http.StatusTooManyRequests {
		t.Fatalf("after 5 wrong tries in mixed case: %d, want 429", code)
	}
}

func TestLoginLimitPerIP(t *testing.T) {
	env := newTestEnv(t)
	env.Auth.SignUp(context.Background(), "alice", pw)
	ip := fromIP("192.0.2.77")
	for i := 0; i < 20; i++ {
		login(t, env, "nobody"+strconv.Itoa(i), "wrong password", ip)
	}
	if code := login(t, env, "alice", pw, ip); code != http.StatusTooManyRequests {
		t.Fatalf("21st login from one IP: %d, want 429", code)
	}
	if code := login(t, env, "alice", pw, fromIP("192.0.2.78")); code != http.StatusSeeOther {
		t.Fatalf("other IP: %d, want 303", code)
	}
}

func TestSuccessfulLoginDoesNotCount(t *testing.T) {
	env := newTestEnv(t)
	env.Auth.SignUp(context.Background(), "alice", pw)
	ip := fromIP("192.0.2.80")
	for i := 0; i < 8; i++ {
		if code := login(t, env, "alice", pw, ip); code != http.StatusSeeOther {
			t.Fatalf("good login %d: %d, want 303", i+1, code)
		}
	}
}

func TestParallelWrongLoginsAreCapped(t *testing.T) {
	env := newTestEnv(t)
	env.Auth.SignUp(context.Background(), "alice", pw)
	var mu sync.Mutex
	counts := map[int]int{}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			code := login(t, env, "alice", "wrong password", fromIP("198.18.0."+strconv.Itoa(i+1)))
			mu.Lock()
			counts[code]++
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	if counts[http.StatusUnprocessableEntity] != 5 || counts[http.StatusTooManyRequests] != 15 {
		t.Fatalf("parallel wrong logins: %v, want 5×422 and 15×429", counts)
	}
}

func TestForwardedForOnlyWithTrustProxy(t *testing.T) {
	for _, trust := range []bool{false, true} {
		env := newTestEnv(t, web.WithTrustProxy(trust))
		env.Auth.SignUp(context.Background(), "alice", pw)
		proxy := "10.0.0.1"
		for i := 0; i < 20; i++ {
			h := fromIP(proxy)
			h["X-Forwarded-For"] = "1.2.3.4, 198.51.100." + strconv.Itoa(i%2+1) // forged first, real last
			login(t, env, "nobody"+strconv.Itoa(i), "wrong password", h)
		}
		h := fromIP(proxy)
		h["X-Forwarded-For"] = "198.51.100.200"
		code := login(t, env, "alice", pw, h)
		want := http.StatusTooManyRequests // without trust: all 21 requests share the proxy's IP
		if trust {
			want = http.StatusSeeOther // with trust: a new client IP
		}
		if code != want {
			t.Errorf("trust=%v: %d, want %d", trust, code, want)
		}
	}
}
```

- [ ] **Step 2: Run to see them fail**

Run: `go test ./internal/web/ -run 'LoginLimit|SuccessfulLogin|ParallelWrong|ForwardedFor'`
Expected: FAIL (no 429 yet).

- [ ] **Step 3: Replace the `login` handler** in `auth_handlers.go` (add `"strings"` import):

```go
const loginLocked = "Too many attempts. Please try again in 15 minutes."

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	username := r.PostFormValue("username")
	ip := s.clientIP(r)
	userKey := strings.ToLower(strings.TrimSpace(username))
	locked := func() {
		s.render(w, r, http.StatusTooManyRequests, part{"login", loginView{Username: username, Error: loginLocked}})
	}
	// Reserve both slots before the slow password hash; a wrong password keeps them.
	if !s.loginIP.Reserve(ip) {
		locked()
		return
	}
	if !s.loginUser.Reserve(userKey) {
		s.loginIP.Release(ip)
		locked()
		return
	}
	token, sess, err := s.accounts.LogIn(r.Context(), username, r.PostFormValue("password"), r.PostFormValue("keep") == "1")
	if errors.Is(err, auth.ErrBadLogin) {
		s.render(w, r, http.StatusUnprocessableEntity, part{"login", loginView{Username: username, Error: "Wrong username or password."}})
		return
	}
	s.loginIP.Release(ip)
	s.loginUser.Release(userKey)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	setSessionCookie(w, token, sess)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
```

- [ ] **Step 4: `main.go`** — the flag and the option:

```go
	trustProxy := flag.Bool("trust-proxy", false, "take the client IP from X-Forwarded-For (only behind an HTTPS proxy that sets it)")
```

and `web.New(…, web.WithSigningKey(key), web.WithTrustProxy(*trustProxy))`.

- [ ] **Step 5: Spec note** — in `docs/superpowers/specs/2026-10-01-users-design.md`, section 6 "Client IP behind a proxy", append:

```markdown
The proxy must also pass the original `Host` header (Caddy and nginx `proxy_set_header Host $host` do this). `http.CrossOriginProtection` compares `Origin` with `Host` when a browser sends no `Sec-Fetch-Site`, so a changed `Host` would reject correct form posts.
```

- [ ] **Step 6: Run all tests and vet**

Run: `go test -count=1 ./...` then `go vet ./...`
Expected: both exit 0.

- [ ] **Step 7: Commit**

```bash
git add cmd internal docs
git commit -m "feat: login limits per username and IP; -trust-proxy"
```

---

### Task 4: Admin command line

**Files:**
- Create: `cmd/todo/users.go`, `cmd/todo/users_test.go`
- Modify: `cmd/todo/main.go`, `internal/auth/auth.go`, `internal/auth/service.go`, `internal/store/sqlite/sqlite.go`, `internal/store/sqlite/users.go`
- Test: `internal/store/sqlite/users_test.go`, `internal/auth/auth_test.go`

**Interfaces:**
- Consumes: plan 1 store and service.
- Produces: `auth.Store.DeleteSessions(ctx, userID int64) error`; `(*auth.Service).ResetPassword(ctx, username string) (string, error)`; `sqlite.Options.RequireCurrent bool`; `sqlite.UserSummary{Username string; CreatedAt time.Time; Items, Sessions int}`; `(*sqlite.Store).ListUsers(ctx, now time.Time) ([]UserSummary, error)`; `runUsers(args []string, in io.Reader, out io.Writer) error` in `cmd/todo`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/store/sqlite/users_test.go`:

```go
func TestListUsersAndDeleteSessions(t *testing.T) {
	s := newStore(t) // alice
	ctx := context.Background()
	bob, _ := s.CreateUser(ctx, "Bob", "x", t0)
	s.Create(ctx, 1, todo.Change{Text: "a"}, t0)
	s.Create(ctx, 1, todo.Change{Text: "b"}, t0)
	s.CreateSession(ctx, auth.Session{TokenHash: "live", UserID: 1, CreatedAt: t0, ExpiresAt: t0.Add(time.Hour)})
	s.CreateSession(ctx, auth.Session{TokenHash: "old", UserID: 1, CreatedAt: t0, ExpiresAt: t0.Add(-time.Hour)})
	s.CreateSession(ctx, auth.Session{TokenHash: "bob", UserID: bob.ID, CreatedAt: t0, ExpiresAt: t0.Add(time.Hour)})

	list, err := s.ListUsers(ctx, t0)
	if err != nil || len(list) != 2 {
		t.Fatalf("ListUsers = %+v, %v", list, err)
	}
	if list[0].Username != "alice" || list[0].Items != 2 || list[0].Sessions != 1 || list[1].Username != "Bob" || list[1].Sessions != 1 {
		t.Fatalf("ListUsers = %+v (want alice 2 items 1 session, then Bob)", list)
	}
	if err := s.DeleteSessions(ctx, 1); err != nil {
		t.Fatalf("DeleteSessions: %v", err)
	}
	if _, _, err := s.SessionUser(ctx, "live", t0); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("alice's session still valid: %v", err)
	}
	if _, _, err := s.SessionUser(ctx, "bob", t0); err != nil {
		t.Fatalf("bob's session ended: %v", err)
	}
}

func TestRequireCurrent(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.db")
	if _, err := sqlite.OpenWith(missing, sqlite.Options{RequireCurrent: true}); err == nil {
		t.Fatal("RequireCurrent on a missing file: err = nil")
	}
	if _, err := os.Stat(missing); err == nil {
		t.Fatal("RequireCurrent created the missing file")
	}
	old := oldDatabase(t, 1)
	if _, err := sqlite.OpenWith(old, sqlite.Options{RequireCurrent: true}); err == nil {
		t.Fatal("RequireCurrent on an old file: err = nil")
	}
	if v := userVersion(t, old); v != 0 {
		t.Fatalf("RequireCurrent changed the old file to version %d", v)
	}
}
```

(add `"os"` to the imports of `users_test.go` if missing).

Append to `internal/auth/auth_test.go`:

```go
func TestResetPassword(t *testing.T) {
	a, _ := newAuth(t)
	ctx := context.Background()
	a.SignUp(ctx, "alice", pw)
	token, _, _ := a.LogIn(ctx, "alice", pw, true)
	newPw, err := a.ResetPassword(ctx, "ALICE")
	if err != nil || len([]rune(newPw)) != 16 {
		t.Fatalf("ResetPassword = %q, %v; want 16 characters", newPw, err)
	}
	if _, _, err := a.Authenticate(ctx, token); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("old session still valid: %v", err)
	}
	if _, _, err := a.LogIn(ctx, "alice", newPw, false); err != nil {
		t.Fatalf("login with the new password: %v", err)
	}
	if _, err := a.ResetPassword(ctx, "nobody"); !errors.Is(err, auth.ErrNoUser) {
		t.Fatalf("unknown user: err = %v, want ErrNoUser", err)
	}
}
```

Create `cmd/todo/users_test.go`:

```go
package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"todo/internal/auth"
	"todo/internal/store/sqlite"
	"todo/internal/todo"
)

func usersDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "todo.db")
	s, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()
	a := auth.NewService(s)
	ctx := context.Background()
	alice, _ := a.SignUp(ctx, "alice", "correct horse battery")
	a.SignUp(ctx, "bob", "correct horse battery")
	todo.NewService(s).Add(ctx, alice.ID, "x", "", "")
	a.StartSession(ctx, alice.ID, false)
	return path
}

func TestUsersList(t *testing.T) {
	path := usersDB(t)
	var out bytes.Buffer
	if err := runUsers([]string{"list", "-db", path}, nil, &out); err != nil {
		t.Fatalf("list: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 || strings.Join(strings.Fields(lines[0]), " ") != "USERNAME CREATED ITEMS SESSIONS" {
		t.Fatalf("output:\n%s", out.String())
	}
	if f := strings.Fields(lines[1]); f[0] != "alice" || f[2] != "1" || f[3] != "1" {
		t.Fatalf("alice line = %q", lines[1])
	}
	if f := strings.Fields(lines[2]); f[0] != "bob" || f[2] != "0" || f[3] != "0" {
		t.Fatalf("bob line = %q", lines[2])
	}
}

func TestUsersResetPassword(t *testing.T) {
	path := usersDB(t)
	var out bytes.Buffer
	if err := runUsers([]string{"reset-password", "-db", path, "alice"}, nil, &out); err != nil {
		t.Fatalf("reset-password: %v", err)
	}
	line := strings.TrimSpace(out.String())
	if !strings.HasPrefix(line, "New password for alice: ") || len(strings.TrimPrefix(line, "New password for alice: ")) != 16 {
		t.Fatalf("output = %q", line)
	}
	err := runUsers([]string{"reset-password", "-db", path, "nobody"}, nil, &out)
	if err == nil || err.Error() != `no user "nobody"` {
		t.Fatalf("unknown user err = %v", err)
	}
}

func TestUsersDelete(t *testing.T) {
	path := usersDB(t)
	var out bytes.Buffer
	err := runUsers([]string{"delete", "-db", path, "bob"}, strings.NewReader("alice\n"), &out)
	if err == nil || !strings.Contains(out.String(), "Type the username to delete bob and all items: ") {
		t.Fatalf("wrong confirmation: err = %v, output %q", err, out.String())
	}
	out.Reset()
	if err := runUsers([]string{"delete", "-db", path, "bob"}, strings.NewReader("bob\n"), &out); err != nil || !strings.Contains(out.String(), "Deleted bob.") {
		t.Fatalf("delete: %v, output %q", err, out.String())
	}
	out.Reset()
	if err := runUsers([]string{"delete", "-db", path, "alice", "-yes"}, nil, &out); err != nil || strings.TrimSpace(out.String()) != "Deleted alice." {
		t.Fatalf("delete -yes: %v, output %q", err, out.String())
	}
	out.Reset()
	runUsers([]string{"list", "-db", path}, nil, &out)
	if strings.Count(strings.TrimSpace(out.String()), "\n") != 0 {
		t.Fatalf("users left:\n%s", out.String())
	}
}

func TestUsersCommandNeedsCurrentDatabase(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "none.db")
	var out bytes.Buffer
	if err := runUsers([]string{"list", "-db", missing}, nil, &out); err == nil {
		t.Fatal("list on a missing database: err = nil")
	}
	if err := runUsers([]string{"frobnicate"}, nil, &out); err == nil || !strings.Contains(err.Error(), "usage") {
		t.Fatalf("unknown command err = %v, want usage", err)
	}
}
```

- [ ] **Step 2: Run to see them fail**

Run: `go test ./internal/... ./cmd/...`
Expected: FAIL to compile (`s.ListUsers undefined`, `a.ResetPassword undefined`, `runUsers undefined`).

- [ ] **Step 3: Store**

In `internal/store/sqlite/sqlite.go`, add to `Options`:

```go
	// RequireCurrent opens only an existing database at the current schema
	// version and never migrates (for the admin commands).
	RequireCurrent bool
```

At the start of `OpenWith` (after the `?#` check):

```go
	if opts.RequireCurrent {
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("database %q does not exist", path)
		}
	}
```

and right before the `if err := migrate(db, opts); err != nil {` line:

```go
	if opts.RequireCurrent {
		var version int
		if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil || version != 2 {
			db.Close()
			return nil, fmt.Errorf("database %q is not at the current schema version; start the server once to upgrade it", path)
		}
		return &Store{db: db}, nil
	}
```

Append to `internal/store/sqlite/users.go`:

```go
func (s *Store) DeleteSessions(ctx context.Context, userID int64) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("delete sessions: %w", err)
	}
	return nil
}

// UserSummary is one line of the admin user list.
type UserSummary struct {
	Username  string
	CreatedAt time.Time
	Items     int
	Sessions  int // not expired at now
}

// ListUsers returns all users sorted by username (case does not matter).
func (s *Store) ListUsers(ctx context.Context, now time.Time) ([]UserSummary, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT u.username, u.created_at,
		        (SELECT COUNT(*) FROM items i WHERE i.user_id = u.id),
		        (SELECT COUNT(*) FROM sessions se WHERE se.user_id = u.id AND se.expires_at > ?)
		 FROM users u ORDER BY u.username COLLATE NOCASE`, fixedTime(now))
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	var list []UserSummary
	for rows.Next() {
		var u UserSummary
		var created string
		if err := rows.Scan(&u.Username, &created, &u.Items, &u.Sessions); err != nil {
			return nil, fmt.Errorf("list users: %w", err)
		}
		if u.CreatedAt, err = time.Parse(time.RFC3339Nano, created); err != nil {
			return nil, fmt.Errorf("list users: %w", err)
		}
		list = append(list, u)
	}
	return list, rows.Err()
}
```

- [ ] **Step 4: Auth** — add to the `Store` interface in `internal/auth/auth.go`:

```go
	DeleteSessions(ctx context.Context, userID int64) error
```

and append to `internal/auth/service.go` (add `"math/big"` to the imports):

```go
const resetAlphabet = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// ResetPassword sets a random 16-character password and ends all sessions of the user.
func (s *Service) ResetPassword(ctx context.Context, username string) (string, error) {
	u, err := s.store.UserByName(ctx, username)
	if err != nil {
		return "", err
	}
	pw := make([]byte, 16)
	for i := range pw {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(resetAlphabet))))
		if err != nil {
			return "", fmt.Errorf("reset password: %w", err)
		}
		pw[i] = resetAlphabet[n.Int64()]
	}
	hash, err := HashPassword(string(pw))
	if err != nil {
		return "", err
	}
	if err := s.store.SetPasswordHash(ctx, u.ID, hash); err != nil {
		return "", err
	}
	if err := s.store.DeleteSessions(ctx, u.ID); err != nil {
		return "", err
	}
	return string(pw), nil
}
```

- [ ] **Step 5: CLI** — create `cmd/todo/users.go`:

```go
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"todo/internal/auth"
	"todo/internal/store/sqlite"
)

const usersUsage = "usage: todo users list | reset-password <name> | delete <name> [-yes]   (options: -db file)"

// runUsers runs the admin command "todo users …".
func runUsers(args []string, in io.Reader, out io.Writer) error {
	if len(args) == 0 {
		return errors.New(usersUsage)
	}
	cmd, args := args[0], args[1:]
	fs := flag.NewFlagSet("users "+cmd, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbPath := fs.String("db", "todo.db", "path to the SQLite database file")
	yes := fs.Bool("yes", false, "delete without asking")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%v; %s", err, usersUsage)
	}
	var name string
	if rest := fs.Args(); len(rest) > 0 {
		name = rest[0]
		if err := fs.Parse(rest[1:]); err != nil { // flags after the name
			return fmt.Errorf("%v; %s", err, usersUsage)
		}
		if fs.NArg() > 0 {
			return errors.New(usersUsage)
		}
	}
	needName := cmd == "reset-password" || cmd == "delete"
	switch {
	case cmd != "list" && !needName:
		return errors.New(usersUsage)
	case needName && name == "", cmd == "list" && name != "":
		return errors.New(usersUsage)
	}

	store, err := sqlite.OpenWith(*dbPath, sqlite.Options{RequireCurrent: true})
	if err != nil {
		return err
	}
	defer store.Close()
	ctx := context.Background()

	switch cmd {
	case "list":
		list, err := store.ListUsers(ctx, time.Now())
		if err != nil {
			return err
		}
		tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "USERNAME\tCREATED\tITEMS\tSESSIONS")
		for _, u := range list {
			fmt.Fprintf(tw, "%s\t%s\t%d\t%d\n", u.Username, u.CreatedAt.Local().Format("2006-01-02"), u.Items, u.Sessions)
		}
		return tw.Flush()
	case "reset-password":
		pw, err := auth.NewService(store).ResetPassword(ctx, name)
		if errors.Is(err, auth.ErrNoUser) {
			return fmt.Errorf("no user %q", name)
		}
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "New password for %s: %s\n", name, pw)
		return nil
	default: // delete
		u, err := store.UserByName(ctx, name)
		if errors.Is(err, auth.ErrNoUser) {
			return fmt.Errorf("no user %q", name)
		}
		if err != nil {
			return err
		}
		if !*yes {
			fmt.Fprintf(out, "Type the username to delete %s and all items: ", u.Username)
			line, _ := bufio.NewReader(in).ReadString('\n')
			if !strings.EqualFold(strings.TrimSpace(line), u.Username) {
				return errors.New("the typed name does not match; nothing was deleted")
			}
		}
		if err := store.DeleteUser(ctx, u.ID); err != nil {
			return err
		}
		fmt.Fprintf(out, "Deleted %s.\n", u.Username)
		return nil
	}
}
```

In `cmd/todo/main.go`, replace `main` with:

```go
func main() {
	var err error
	if len(os.Args) > 1 && os.Args[1] == "users" {
		err = runUsers(os.Args[2:], os.Stdin, os.Stdout)
	} else {
		err = run()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "todo:", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 6: Run all tests and vet**

Run: `go test -count=1 ./...` then `go vet ./...`
Expected: both exit 0. Also: `go build ./cmd/todo && ./todo users list -db /tmp/none.db; echo $status` → `todo: database "/tmp/none.db" does not exist`, `1`, and `/tmp/none.db` is not created.

- [ ] **Step 7: Commit**

```bash
git add cmd internal
git commit -m "feat: admin commands to list users, reset passwords, delete users"
```

---

## Final check (after Task 4)

- Whole-branch review by a fresh reviewer.
- Browser check by the user (`go build ./cmd/todo && ./todo -addr 127.0.0.1:8812 -db /tmp/todo-guard.db`):
  1. Sign up: the button shows "Checking that you are human…" for about a second (note the time; if it is much more than 2 s, lower `DefaultPowBits` to 15), then the list opens.
  2. Click "Create account" within 3 seconds of opening the page → "Sign-up failed. Please wait a moment and try again."; wait and try again → works.
  3. Log in with a wrong password 5 times → the 6th try shows "Too many attempts. Please try again in 15 minutes."
  4. `./todo users list -db /tmp/todo-guard.db`, `reset-password` (log in with the printed password; the old browser session is gone), `delete` with the confirmation.
  5. No CSP errors in the console on the sign-up page.
- Then ask: merge `users-guard` into `main` locally, or keep the branch.

## Spec coverage

| Spec section | Task |
|---|---|
| 6 Sign-up spam protection | 1, 2 |
| 6 Login protection | 1, 3 |
| 6 Client IP behind a proxy (+ Host note) | 2 (`clientIP`), 3 (flag, docs) |
| 8 Admin command line | 4 |
| 9 Edge cases: restart resets counters and used tokens | 1 (in memory by design) |
| 10 Testing: login limits, sign-up checks, command line | 1–4 |
