# Deferred Minors (Users and Login) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Fix the 15 deferred minor points from the two users-and-login reviews (security hardening, usability, limits and logging, admin command polish).

**Architecture:** Small changes in `internal/auth`, `internal/store/sqlite`, `internal/guard`, `internal/web`, and `cmd/todo`; no new packages. One commit per group.

**Tech Stack:** Go 1.27.1, htmx 2.0.11, existing code on `main` (`3ad0a3f`).

**Execution:** Native (same as plans 1 and 2), then one whole-branch review by a fresh reviewer. Worktree `~/claude-personal/todo-worktrees/minors2`, branch `minors2`, from `main`. One commit per task (Co-Authored-By line). No push.

**Spec:** `docs/superpowers/specs/2026-10-01-users-design.md` (behavior unchanged except the new message below).

## Global Constraints

- Plain `go …`; check `go test`'s own exit code; `node --check` for changed JS.
- New visible text (approved by the user, no canvas): "Your browser could not run the sign-up check. Please use an up-to-date browser over HTTPS." — red error line under the sign-up button.
- No behavior change beyond the 15 points.

## Review Focus

1. A stored hash with huge parameters must be rejected without allocating memory. → Task 1 `TestCheckPasswordRejectsExtremeParameters`.
2. Logging in again in the same browser must end the old session. → Task 1 `TestLoginEndsPreviousSession`.
3. The admin commands must not change an old database's journal mode. → Task 1 `TestRequireCurrentDoesNotTouchOldFile`.
4. Releasing a limiter slot must free the caller's own hit. → Task 3 `TestReleaseFreesOwnHit`.
5. With `-trust-proxy`, an `X-Forwarded-For` entry with a port must still give the client's key. → Task 3 `TestForwardedForWithPort`.

---

### Task 1: Security hardening

**Files:** `internal/auth/password.go`, `internal/auth/service.go`, `internal/auth/auth.go`, `internal/store/sqlite/users.go`, `internal/store/sqlite/sqlite.go`, `internal/web/auth_handlers.go`; tests in `internal/auth/auth_test.go`, `internal/store/sqlite/users_test.go`, `internal/web/auth_test.go`. Copy this plan to `docs/superpowers/plans/2026-10-02-users-minors.md`.

**Interfaces:** Produces `auth.Store.SetPasswordAndEndSessions(ctx, userID int64, hash, keepTokenHash string) error` (replaces `SetPasswordHash`, `DeleteOtherSessions`, `DeleteSessions` in the interface; the sqlite methods `SetPasswordHash`, `DeleteOtherSessions`, `DeleteSessions` stay for their tests and the admin code).

- [ ] **Step 1: Worktree and plan copy**

```bash
git -C ~/claude-personal/todo worktree add -b minors2 ../todo-worktrees/minors2
cd ~/claude-personal/todo-worktrees/minors2
```

Copy this plan to `docs/superpowers/plans/2026-10-02-users-minors.md`.

- [ ] **Step 2: Failing tests**

Append to `internal/auth/auth_test.go`:

```go
func TestCheckPasswordRejectsExtremeParameters(t *testing.T) {
	salt := "AAAAAAAAAAAAAAAAAAAAAA"                          // 16 bytes
	key := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"     // 32 bytes
	for _, params := range []string{"m=4194304,t=3,p=4", "m=65536,t=11,p=4", "m=65536,t=3,p=17", "m=65536,t=3,p=0", "m=7,t=3,p=4"} {
		h := "$argon2id$v=19$" + params + "$" + salt + "$" + key
		start := time.Now()
		if auth.CheckPassword(h, pw) {
			t.Errorf("%s accepted", params)
		}
		if d := time.Since(start); d > 50*time.Millisecond {
			t.Errorf("%s took %v; must be rejected before hashing", params, d)
		}
	}
	for _, sk := range [][2]string{{"AAAA", key}, {salt, "AAAA"}} {
		if auth.CheckPassword("$argon2id$v=19$m=65536,t=3,p=4$"+sk[0]+"$"+sk[1], pw) {
			t.Errorf("short salt or key accepted: %v", sk)
		}
	}
}
```

Append to `internal/store/sqlite/users_test.go`:

```go
func TestSetPasswordAndEndSessions(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	for _, h := range []string{"keep", "other1", "other2"} {
		s.CreateSession(ctx, auth.Session{TokenHash: h, UserID: 1, CreatedAt: t0, ExpiresAt: t0.Add(time.Hour)})
	}
	if err := s.SetPasswordAndEndSessions(ctx, 1, "new-hash", "keep"); err != nil {
		t.Fatalf("SetPasswordAndEndSessions: %v", err)
	}
	if u, _ := s.UserByID(ctx, 1); u.PasswordHash != "new-hash" {
		t.Fatalf("hash = %q", u.PasswordHash)
	}
	if _, _, err := s.SessionUser(ctx, "keep", t0); err != nil {
		t.Fatalf("kept session ended: %v", err)
	}
	if _, _, err := s.SessionUser(ctx, "other1", t0); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("other session still valid: %v", err)
	}
	if err := s.SetPasswordAndEndSessions(ctx, 1, "h2", ""); err != nil {
		t.Fatalf("end all: %v", err)
	}
	if _, _, err := s.SessionUser(ctx, "keep", t0); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("keep=\"\" must end all sessions: %v", err)
	}
	if err := s.SetPasswordAndEndSessions(ctx, 999, "x", ""); !errors.Is(err, auth.ErrNoUser) {
		t.Fatalf("unknown user: %v, want ErrNoUser", err)
	}
}

func TestRequireCurrentDoesNotTouchOldFile(t *testing.T) {
	old := oldDatabase(t, 1)
	if _, err := sqlite.OpenWith(old, sqlite.Options{RequireCurrent: true}); err == nil {
		t.Fatal("old file accepted")
	}
	if _, err := os.Stat(old + "-wal"); err == nil {
		t.Fatal("RequireCurrent created a -wal file (journal mode changed)")
	}
	db, _ := sql.Open("sqlite", "file:"+old)
	defer db.Close()
	var mode string
	db.QueryRow(`PRAGMA journal_mode`).Scan(&mode)
	if mode != "delete" {
		t.Fatalf("journal_mode = %q, want delete (unchanged)", mode)
	}
}
```

Append to `internal/web/auth_test.go`:

```go
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
```

Run: `go test ./internal/...` → FAIL (undefined `SetPasswordAndEndSessions`; old tests fail).

- [ ] **Step 3: `password.go`** — in `CheckPassword`, after the `Sscanf` of m/t/p and after decoding salt and key, add bounds:

```go
	// Bounds stop a broken or hostile stored hash from using huge memory or time.
	if memory < 8*1024 || memory > 1024*1024 || time == 0 || time > 10 || threads == 0 || threads > 16 {
		return false
	}
```

(placed right after the `Sscanf` line, replacing the old `threads == 0 || time == 0` check), and after decoding:

```go
	if len(salt) < 8 || len(salt) > 64 || len(key) < 16 || len(key) > 64 {
		return false
	}
```

- [ ] **Step 4: Store + service** — append to `internal/store/sqlite/users.go`:

```go
// SetPasswordAndEndSessions sets a new password hash and deletes the user's
// sessions except keepTokenHash ("" deletes all), in one transaction.
func (s *Store) SetPasswordAndEndSessions(ctx context.Context, userID int64, passwordHash, keepTokenHash string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("set password: %w", err)
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, passwordHash, userID)
	if err != nil {
		return fmt.Errorf("set password: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return auth.ErrNoUser
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ? AND token_hash != ?`, userID, keepTokenHash); err != nil {
		return fmt.Errorf("set password: %w", err)
	}
	return tx.Commit()
}
```

In `internal/auth/auth.go`, replace the interface lines `SetPasswordHash…`, `DeleteOtherSessions…`, and `DeleteSessions…` with:

```go
	SetPasswordAndEndSessions(ctx context.Context, userID int64, passwordHash, keepTokenHash string) error // ErrNoUser; keep "" ends all
```

In `internal/auth/service.go`: in `ChangePassword` replace the `SetPasswordHash` call and the final `DeleteOtherSessions` return with `return s.store.SetPasswordAndEndSessions(ctx, userID, hash, HashToken(token))`; in `ResetPassword` replace the `SetPasswordHash` and `DeleteSessions` calls with:

```go
	if err := s.store.SetPasswordAndEndSessions(ctx, u.ID, hash, ""); err != nil {
		return "", err
	}
```

- [ ] **Step 5: RequireCurrent read-only** — in `sqlite.go` `OpenWith`, extend the first `if opts.RequireCurrent {` block (after the `os.Stat`) with a read-only version check, before the normal open:

```go
		ro, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
		if err != nil {
			return nil, fmt.Errorf("open database %q: %w", path, err)
		}
		var version int
		err = ro.QueryRow(`PRAGMA user_version`).Scan(&version)
		ro.Close()
		if err != nil || version != 2 {
			return nil, fmt.Errorf("database %q is not at the current schema version; start the server once to upgrade it", path)
		}
```

and reduce the later `if opts.RequireCurrent {` block (after Ping) to `if opts.RequireCurrent { return &Store{db: db}, nil }`.

- [ ] **Step 6: End the previous session** — in `auth_handlers.go`, in `login` right before `setSessionCookie(w, token, sess)` and in `signup` right before its `setSessionCookie`, add:

```go
	if err := s.accounts.LogOut(r.Context(), sessionToken(r)); err != nil {
		s.log.Error("end previous session", "err", err)
	}
```

- [ ] **Step 7: Run all tests and vet** — `go test -count=1 ./...`, `go vet ./...` → exit 0.

- [ ] **Step 8: Commit** — `git add docs internal && git commit -m "fix: hardening — hash parameter bounds, atomic password change, read-only version check, end old session on login"`

---

### Task 2: Usability

**Files:** `internal/web/static/app.js`, `internal/web/static/auth.js`, `internal/web/templates/auth.html`, `internal/web/static/app.css`; test `internal/web/usability_test.go` (new).

- [ ] **Step 1: Failing tests** — `internal/web/usability_test.go`:

```go
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
```

Run → FAIL.

- [ ] **Step 2: `app.js`** — append:

```js
// 9. If the session ended and the user presses Back, htmx cannot load the
// old page (401). Reload, so the server sends the log-in page.
document.addEventListener("htmx:historyCacheMissLoadError", () => {
  location.reload();
});
```

In the edit-row keydown handler (part 7), add as the first line inside the listener:

```js
  if (e.isComposing || e.keyCode === 229) {
    return; // the key ends an IME composition; it is not a real Enter
  }
```

- [ ] **Step 3: Show button** — in `auth.html` remove ` aria-pressed="false"` from both `show-password` buttons; in `auth.js` delete the line `button.setAttribute("aria-pressed", show ? "true" : "false");` (the changing text "Show"/"Hide" is announced).

- [ ] **Step 4: Focus** — in `auth.html`:
  - login username input: replace ` required autofocus>` with ` required{{if not .Username}} autofocus{{end}}>`; login password input: replace `autocomplete="current-password" required>` with `autocomplete="current-password" required{{if .Username}} autofocus{{end}}>`.
  - sign-up username input: replace `required autofocus{{if .UsernameError}}` with `required{{if not .PasswordError}} autofocus{{end}}{{if .UsernameError}}`; sign-up password input: replace `autocomplete="new-password" required{{if .PasswordError}}` with `autocomplete="new-password" required{{if .PasswordError}} autofocus{{end}}{{if .PasswordError}}`.

- [ ] **Step 5: Puzzle failure message** — in `auth.html`, after `<button type="submit" class="btn-primary">Create account</button>` in the sign-up form, add `<p class="field-error" id="signup-check-error" role="alert"></p>`; in `app.css` add `#signup-check-error:empty { display: none; }`. In `auth.js`, in the sign-up submit handler: before `e.preventDefault();` nothing changes; replace the `try … catch` block with:

```js
    const errorLine = document.getElementById("signup-check-error");
    errorLine.textContent = "";
    try {
      if (!window.crypto || !crypto.subtle) {
        throw new Error("no crypto.subtle");
      }
      nonceField.value = await solve(signupForm.elements.form_token.value, Number(signupForm.dataset.powBits));
      signupForm.submit();
    } catch (err) {
      button.disabled = false;
      button.textContent = label;
      errorLine.textContent = "Your browser could not run the sign-up check. Please use an up-to-date browser over HTTPS.";
    }
```

- [ ] **Step 6: Run** — `go test -count=1 ./...`, `go vet ./...`, `node --check internal/web/static/app.js && node --check internal/web/static/auth.js` → exit 0.

- [ ] **Step 7: Commit** — `git add internal && git commit -m "fix: usability — Back after expiry, IME Enter, Show button, focus after errors, puzzle failure message"`

---

### Task 3: Limits and logging

**Files:** `internal/guard/limiter.go`, `internal/guard/tokens.go`, `internal/web/server.go`, `internal/web/auth_handlers.go`; tests `internal/guard/guard_test.go`, `internal/web/login_guard_test.go`, `internal/web/logging_test.go` (new).

**Interfaces:** `(*Limiter).Reserve(key) (Ticket, bool)`; `(*Limiter).Release(Ticket)`; `guard.Ticket` (opaque).

- [ ] **Step 1: Failing tests**

Replace the body of `TestLimiterReserveAndRelease` in `guard_test.go` to use tickets, and append `TestReleaseFreesOwnHit`:

```go
func TestLimiterReserveAndRelease(t *testing.T) {
	c := &clock{t: time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)}
	l := guard.NewLimiter(3, 15*time.Minute, c.now)
	var first guard.Ticket
	for i := 0; i < 3; i++ {
		tk, ok := l.Reserve("a")
		if !ok {
			t.Fatalf("reserve %d refused", i+1)
		}
		if i == 0 {
			first = tk
		}
	}
	if _, ok := l.Reserve("a"); ok {
		t.Fatal("4th reserve allowed, want refused")
	}
	if _, ok := l.Reserve("b"); !ok {
		t.Fatal("other key refused")
	}
	l.Release(first)
	if _, ok := l.Reserve("a"); !ok {
		t.Fatal("reserve after release refused")
	}
	c.t = c.t.Add(15*time.Minute + time.Second)
	if _, ok := l.Reserve("a"); !ok {
		t.Fatal("reserve after the window refused")
	}
	l.Release(guard.Ticket{}) // must not panic
}

func TestReleaseFreesOwnHit(t *testing.T) {
	c := &clock{t: time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)}
	l := guard.NewLimiter(2, 15*time.Minute, c.now)
	a1, _ := l.Reserve("k") // at 12:00
	c.t = c.t.Add(10 * time.Minute)
	l.Reserve("k") // at 12:10, kept
	l.Release(a1)  // frees the 12:00 hit, not the 12:10 one
	c.t = c.t.Add(5*time.Minute + time.Second) // 12:15:01
	if _, ok := l.Reserve("k"); !ok {
		t.Fatal("first reserve refused")
	}
	if _, ok := l.Reserve("k"); ok {
		t.Fatal("second reserve allowed: Release removed the wrong hit")
	}
}
```

Append to `internal/web/login_guard_test.go`:

```go
func TestForwardedForWithPort(t *testing.T) {
	env := newTestEnv(t, web.WithTrustProxy(true))
	env.Auth.SignUp(context.Background(), "alice", pw)
	for i := 0; i < 20; i++ {
		h := fromIP("10.0.0.1")
		h["X-Forwarded-For"] = "198.51.100.7:" + strconv.Itoa(40000+i)
		login(t, env, "nobody"+strconv.Itoa(i), "wrong password", h)
	}
	h := fromIP("10.0.0.1")
	h["X-Forwarded-For"] = "198.51.100.7:5"
	if code := login(t, env, "alice", pw, h); code != http.StatusTooManyRequests {
		t.Fatalf("same client with ports: %d, want 429", code)
	}
	h = fromIP("10.0.0.1")
	h["X-Forwarded-For"] = "[2001:db8::5]:443"
	if code := login(t, env, "alice", pw, h); code != http.StatusSeeOther {
		t.Fatalf("other client: %d, want 303", code)
	}
}
```

Create `internal/web/logging_test.go`:

```go
package web_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"todo/internal/auth"
	"todo/internal/store/sqlite"
	"todo/internal/todo"
	"todo/internal/web"
)

type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) { s.mu.Lock(); defer s.mu.Unlock(); return s.b.Write(p) }
func (s *syncBuffer) String() string              { s.mu.Lock(); defer s.mu.Unlock(); return s.b.String() }

func TestSecurityEventsAreLogged(t *testing.T) {
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "log.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var logs syncBuffer
	clock := &testClock{t: time.Now()}
	accounts := auth.NewService(store)
	accounts.SignUp(context.Background(), "alice", pw)
	h, _ := web.New(todo.NewService(store), accounts, slog.New(slog.NewTextHandler(&logs, nil)), web.WithClock(clock.Now), web.WithPowBits(4))

	post := func(path string, v url.Values, xff string) {
		req := httptest.NewRequest("POST", path, strings.NewReader(v.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if xff != "" {
			req.Header.Set("X-Forwarded-For", xff)
		}
		h.ServeHTTP(httptest.NewRecorder(), req)
	}
	post("/signup", url.Values{"username": {"bot1"}, "password": {pw}, "website": {"spam"}}, "")
	for i := 0; i < 6; i++ {
		post("/login", url.Values{"username": {"alice"}, "password": {"wrong password"}}, "198.51.100.1")
	}
	out := logs.String()
	for _, want := range []string{"reason=honeypot", "login blocked", "X-Forwarded-For"} {
		if !strings.Contains(out, want) {
			t.Errorf("log does not contain %q:\n%s", want, out)
		}
	}
	if strings.Count(out, "X-Forwarded-For") != 1 {
		t.Errorf("the X-Forwarded-For warning must appear once:\n%s", out)
	}
}
```

Run → FAIL to compile (`Reserve` returns one value; `guard.Ticket` undefined).

- [ ] **Step 2: Limiter with tickets** — in `limiter.go`, change the stored hits to entries with an ID:

```go
type hit struct {
	at time.Time
	id uint64
}

// Ticket identifies one reserved hit, for Release.
type Ticket struct {
	key string
	id  uint64
}
```

`Limiter` fields: `hits map[string][]hit`, add `nextID uint64`. `NewLimiter` makes `map[string][]hit{}`. Replace `Reserve`, `Release`, `recent`, `sweep`:

```go
// Reserve records one hit for key if fewer than max hits are in the window.
// The ticket releases exactly this hit.
func (l *Limiter) Reserve(key string) (Ticket, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.calls++; l.calls%1000 == 0 {
		l.sweep()
	}
	h := l.recent(key)
	if len(h) >= l.max {
		l.hits[key] = h
		return Ticket{}, false
	}
	l.nextID++
	l.hits[key] = append(h, hit{at: l.now(), id: l.nextID})
	return Ticket{key: key, id: l.nextID}, true
}

// Release removes the hit of t, for a reservation that should not count.
func (l *Limiter) Release(t Ticket) {
	l.mu.Lock()
	defer l.mu.Unlock()
	h := l.hits[t.key]
	for i, e := range h {
		if e.id == t.id {
			h = append(h[:i:i], h[i+1:]...)
			break
		}
	}
	if len(h) == 0 {
		delete(l.hits, t.key)
		return
	}
	l.hits[t.key] = h
}

func (l *Limiter) recent(key string) []hit {
	cutoff := l.now().Add(-l.window)
	h := l.hits[key]
	i := 0
	for i < len(h) && !h[i].at.After(cutoff) {
		i++
	}
	return h[i:]
}
```

(`sweep` stays, it uses `recent`.)

- [ ] **Step 3: Token cleanup at most once per minute** — in `tokens.go` add field `lastSweep time.Time` to `Tokens` and replace the `if len(t.used) > 1000 {` condition with `if len(t.used) > 1000 && t.now().Sub(t.lastSweep) > time.Minute {` and set `t.lastSweep = t.now()` inside the block. (Performance only; existing token tests keep the behavior.)

- [ ] **Step 4: clientIP and warning** — in `server.go`, add fields `xffWarn sync.Once` to `server`; in `clientIP`, replace the trusted-proxy part with:

```go
	if vals := r.Header.Values("X-Forwarded-For"); len(vals) > 0 {
		if !s.trustProxy {
			s.xffWarn.Do(func() {
				s.log.Warn("requests have X-Forwarded-For but -trust-proxy is off; all clients behind the proxy share one limit")
			})
		} else {
			parts := strings.Split(vals[len(vals)-1], ",")
			last := strings.TrimSpace(parts[len(parts)-1])
			if ap, err := netip.ParseAddrPort(last); err == nil {
				last = ap.Addr().String()
			}
			last = strings.TrimSuffix(strings.TrimPrefix(last, "["), "]")
			if key, ok := ipKey(last); ok {
				return key
			}
			s.log.Warn("cannot read the client address from X-Forwarded-For", "value", last)
		}
	}
```

(add `"sync"` import).

- [ ] **Step 5: Use tickets and log events** — in `auth_handlers.go`:
  - login: `ipTicket, ok := s.loginIP.Reserve(ip)`; if `!ok` → `s.log.Info("login blocked", "ip", ip, "reason", "ip limit")`, locked. Username: `var userTicket guard.Ticket`; `if countUser { if userTicket, ok = s.loginUser.Reserve(userKey); !ok { s.loginIP.Release(ipTicket); s.log.Info("login blocked", "ip", ip, "reason", "username limit"); locked(); return } }`. On success/error: `s.loginIP.Release(ipTicket)`; `if countUser { s.loginUser.Release(userTicket) }`.
  - signup: `if _, ok := s.signupTry.Reserve(ip); !ok { s.log.Info("sign-up rejected", "ip", ip, "reason", "attempt limit"); failed(); return }`; honeypot: add `s.log.Info("sign-up rejected", "ip", ip, "reason", "honeypot")`; accounts: `newTicket, ok := s.signupNew.Reserve(ip); if !ok { s.log.Info("sign-up rejected", "ip", ip, "reason", "account limit"); failed(); return }`; on `SignUp` error `s.signupNew.Release(newTicket)`.
  - add import `"todo/internal/guard"`.

- [ ] **Step 6: Run** — `go test -race -count=1 ./...`, `go vet ./...` → exit 0.

- [ ] **Step 7: Commit** — `git add internal && git commit -m "fix: limiter tickets, token cleanup pace, X-Forwarded-For with ports and warnings, security event logs"`

---

### Task 4: Admin command polish

**Files:** `cmd/todo/users.go`; test `cmd/todo/users_test.go`.

- [ ] **Step 1: Failing tests** — append to `cmd/todo/users_test.go`:

```go
func TestUsersPolish(t *testing.T) {
	path := usersDB(t)
	var out bytes.Buffer
	if err := runUsers([]string{"reset-password", "-db", path, "ALICE"}, nil, &out); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if !strings.HasPrefix(out.String(), "New password for alice: ") {
		t.Fatalf("output %q, want the stored username", out.String())
	}
	err := runUsers([]string{"frobnicate"}, nil, &out)
	if err == nil || !strings.Contains(err.Error(), "--") {
		t.Fatalf("usage %v must mention --", err)
	}
}
```

Run → FAIL.

- [ ] **Step 2: Code** — `usersUsage` becomes `"usage: todo users list | reset-password <name> | delete <name> [-yes]   (options: -db file; put -- before a name that starts with -)"`. In `case "reset-password":` look up the user first:

```go
	case "reset-password":
		u, err := store.UserByName(ctx, name)
		if errors.Is(err, auth.ErrNoUser) {
			return fmt.Errorf("no user %q", name)
		}
		if err != nil {
			return err
		}
		pw, err := auth.NewService(store).ResetPassword(ctx, u.Username)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "New password for %s: %s\n", u.Username, pw)
		return nil
```

- [ ] **Step 3: Run** — `go test -count=1 ./...`, `go vet ./...` → exit 0.

- [ ] **Step 4: Commit** — `git add cmd && git commit -m "fix: admin command polish — stored username in reset output, -- in usage"`

---

## Final check

- Whole-branch review by a fresh reviewer.
- Browser check (user): log in twice in one browser (old session gone); wrong password → cursor in password field; Back after logout → log-in page; sign-up still works (puzzle); the account page "Show" still works.
- Then ask: merge `minors2` into `main`, or keep the branch.
