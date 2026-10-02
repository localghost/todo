# Users and Login (Plan 1: Accounts) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** People can sign up, log in, log out, change their password, and delete their account; each user sees only their own items; the pages are protected against cross-site requests and run under a strict CSP.

**Architecture:** A new package `internal/auth` holds account rules, argon2id hashing, and sessions behind an `auth.Store` interface that `internal/store/sqlite` implements. The database moves to schema version 2 (`PRAGMA user_version`), which drops the old single-user tables (only with `-delete-old-items` if old items exist). `internal/web` gets a session middleware, plain-form pages for login, sign-up, and account, Go's `http.CrossOriginProtection`, security headers, and htmx without `eval`.

**Tech Stack:** Go 1.27.1, `modernc.org/sqlite` v1.60.1, `golang.org/x/crypto/argon2` (new, latest), htmx 2.0.11, `html/template`.

**Spec:** `docs/superpowers/specs/2026-10-01-users-design.md` (commit `8bb634b`). This plan covers spec sections 3, 4, 5, 6 (cross-site protection, headers, CSP and the htmx changes), 7, 9, and the matching parts of 10. **Not in this plan (plan 2):** sign-up spam checks, login limits, `-trust-proxy`, admin command line. UI reference: canvas page "13 · Accounts", https://claude.ai/artifact/7mnRhC4Jd8jbnbAhjN7Woq

## Global Constraints

- Work in `~/claude-personal/todo-worktrees/users` (branch `users`). Run Go as plain `go …` (never `GOTOOLCHAIN=local`). The shell is fish. Check `go test`'s own exit code; never decide on a commit from a pipe like `go test | grep`.
- New Go dependency allowed: `golang.org/x/crypto` only (latest version).
- Username: `^[A-Za-z0-9_-]{3,32}$`, unique ignoring case, stored as typed. Password: 10–200 Unicode characters.
- argon2id: memory 65536 KiB, time 3, threads 4, salt 16 bytes, key 32 bytes; text form `$argon2id$v=19$m=65536,t=3,p=4$<salt>$<key>` (raw standard base64).
- Session token: 32 random bytes, base64url without padding; DB stores hex SHA-256. Cookie `todo_session`: `HttpOnly`, `Secure`, `SameSite=Lax`, `Path=/`. Not persistent: no `Max-Age`, server expiry 12 h. Persistent ("Keep me logged in"): `Max-Age` and server expiry 30 days.
- Messages (exact): "Use 3–32 letters, digits, - or _." · "This username is taken." · "Use at least 10 characters." · "Use at most 200 characters." · "Wrong username or password." · "The current password is wrong." · "Password changed. You were logged out on your other devices." · "The username does not match." · "Your account was deleted." · old-data start message (Task 1).
- Page texts (canvas page 13): "Log in", "Username", "Password", "Keep me logged in", "No account yet? Sign up", "Create an account", "3–32 letters, digits, - or _", "At least 10 characters", "Show", "There is no email. If you forget your password, ask the admin.", "Create account", "Already have an account? Log in", "Signed in as <name> · Account · Log out", "← Back to my list", "member since <2 Jan 2006>", "Change password", "Current password", "New password", "Delete account", "This deletes your account and all your items. It cannot be undone.", "Type your username to confirm", "Delete my account and all items".
- One commit per task, ending with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. No push, no PR.

## Review Focus

1. A user who was deleted (by themselves or later by the admin) must lose access at once, also with an open tab and a 30-day cookie. → Task 4 `TestDeletedAccountCookieStopsWorking`.
2. Username case: "ALICE" logs in to "alice"; signing up "Alice" when "alice" exists says "This username is taken." → Task 2 `TestUsernameIgnoresCase`.
3. A session past its expiry (12 h, 30 days) must not work, even if the browser still sends the cookie. → Task 2 `TestSessionExpiry`.
4. Passwords with non-ASCII letters count characters, not bytes; a 201-character password is rejected before hashing (no slow hash for huge input). → Task 2 `TestPasswordRulesCountCharacters`.
5. Every existing item route must use the logged-in user: user B must not see or change user A's items. → Task 3 `TestItemsAreIsolatedPerUser`.

---

## File map

| File | Responsibility |
|---|---|
| `internal/auth/auth.go` (new) | `User`, `Session`, errors, `RuleError`, `Store` interface |
| `internal/auth/rules.go` (new) | `ValidateUsername`, `ValidatePassword` |
| `internal/auth/password.go` (new) | `HashPassword`, `CheckPassword`, dummy hash |
| `internal/auth/service.go` (new) | `Service`: sign-up, login, sessions, password change, account deletion, cleanup |
| `internal/store/sqlite/sqlite.go` | schema version 2, `Options`, `OldItemsError`, `OpenWith` |
| `internal/store/sqlite/users.go` (new) | `auth.Store` implementation |
| `internal/web/session.go` (new) | cookie helpers, `protect` middleware, user in context |
| `internal/web/auth_handlers.go` (new) | login, sign-up, logout, account handlers |
| `internal/web/server.go` | `New` with the auth service, routes, `CrossOriginProtection`, headers |
| `internal/web/templates/auth.html` (new) | login, sign-up, account pages |
| `internal/web/static/auth.js` (new) | "Show" password toggle |
| `cmd/todo/main.go` | `-delete-old-items`, auth service, hourly cleanup |

---

### Task 1: Schema version 2 and the account store

**Files:**
- Create: `internal/auth/auth.go`, `internal/store/sqlite/users.go`
- Modify: `internal/store/sqlite/sqlite.go`, `cmd/todo/main.go`, test helpers in `internal/store/sqlite/sqlite_test.go`, `internal/todo/service_test.go`, `internal/web/web_test.go`
- Test: `internal/store/sqlite/users_test.go` (new), `internal/store/sqlite/sqlite_test.go`
- Copy: this plan stays at `docs/superpowers/plans/2026-10-02-users-accounts.md` (commit it with this task)

**Interfaces:**
- Produces: `auth.User{ID int64; Username, PasswordHash string; CreatedAt time.Time}`; `auth.Session{TokenHash string; UserID int64; CreatedAt, ExpiresAt time.Time; Persistent bool}`; errors `auth.ErrUsernameTaken`, `auth.ErrNoUser`, `auth.ErrNoSession`, `auth.ErrBadLogin`, `auth.ErrWrongPassword`, `auth.ErrConfirmMismatch`; `*auth.RuleError{Field, Msg string}`; `auth.Store` (below); `sqlite.Options{DeleteOldItems bool}`, `sqlite.OpenWith(path string, opts Options) (*Store, error)`, `*sqlite.OldItemsError{Count int}`; `*sqlite.Store` implements `auth.Store`.

- [ ] **Step 1: Add the dependency**

Run: `go get golang.org/x/crypto@latest`
Expected: `go.mod` lists `golang.org/x/crypto` (used from Task 2; `go mod tidy` would remove it now, so do not run tidy in this task).

- [ ] **Step 2: Create `internal/auth/auth.go`**

```go
// Package auth holds user accounts, password rules, and login sessions.
package auth

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrUsernameTaken means another user has this username (case does not matter).
	ErrUsernameTaken = errors.New("auth: username taken")
	// ErrNoUser means there is no user with this name or ID.
	ErrNoUser = errors.New("auth: no such user")
	// ErrNoSession means the token is unknown or its session has expired.
	ErrNoSession = errors.New("auth: no valid session")
	// ErrBadLogin means the username or the password is wrong.
	ErrBadLogin = errors.New("auth: wrong username or password")
	// ErrWrongPassword means the current password given for a change is wrong.
	ErrWrongPassword = errors.New("auth: current password is wrong")
	// ErrConfirmMismatch means the typed username does not match the account.
	ErrConfirmMismatch = errors.New("auth: confirmation does not match")
)

// RuleError is a broken username or password rule, with a message for the user.
type RuleError struct {
	Field string // "username" or "password"
	Msg   string
}

func (e *RuleError) Error() string { return "auth: " + e.Field + ": " + e.Msg }

// User is one account.
type User struct {
	ID           int64
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}

// Session is one login. Only the hash of the token is stored.
type Session struct {
	TokenHash  string
	UserID     int64
	CreatedAt  time.Time
	ExpiresAt  time.Time
	Persistent bool
}

// Store keeps users and sessions.
type Store interface {
	CreateUser(ctx context.Context, username, passwordHash string, now time.Time) (User, error) // ErrUsernameTaken
	UserByName(ctx context.Context, username string) (User, error)                             // ErrNoUser; case does not matter
	UserByID(ctx context.Context, id int64) (User, error)                                      // ErrNoUser
	SetPasswordHash(ctx context.Context, id int64, passwordHash string) error                  // ErrNoUser
	DeleteUser(ctx context.Context, id int64) error                                            // ErrNoUser; deletes items and sessions too
	CreateSession(ctx context.Context, s Session) error
	SessionUser(ctx context.Context, tokenHash string, now time.Time) (Session, User, error) // ErrNoSession if unknown or expired
	DeleteSession(ctx context.Context, tokenHash string) error
	DeleteOtherSessions(ctx context.Context, userID int64, keepTokenHash string) error
	DeleteExpiredSessions(ctx context.Context, now time.Time) (int64, error)
}
```

- [ ] **Step 3: Update existing test helpers** (items now need an existing user)

In `internal/store/sqlite/sqlite_test.go`, at the end of `newStore` before `return s`, add:

```go
	if _, err := s.CreateUser(context.Background(), "alice", "x", t0); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
```

In `TestOpenTwiceKeepsData`, before the first `s.Create(…)`, add:

```go
	if _, err := s.CreateUser(context.Background(), "alice", "x", t0); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
```

In `internal/todo/service_test.go`, `newService`, before `t.Cleanup`, add (and import `"time"` if missing):

```go
	if _, err := s.CreateUser(context.Background(), "tester", "x", time.Now()); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
```

In `internal/web/web_test.go`, `newTestApp`, after `t.Cleanup(func() { store.Close() })`, add (import `"time"`):

```go
	if _, err := store.CreateUser(context.Background(), "tester", "x", time.Now()); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
```

Replace the whole function `TestOpenMigratesOldDatabase` (from its `func` line to the end of `sqlite_test.go`) with:

```go
func oldDatabase(t *testing.T, items int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()
	stmts := []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL)`,
		`INSERT INTO users (id, name) VALUES (1, 'default')`,
		`CREATE TABLE items (id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id),
			text TEXT NOT NULL, done INTEGER NOT NULL DEFAULT 0, position INTEGER NOT NULL,
			created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
			due_at TEXT, due_all_day INTEGER NOT NULL DEFAULT 0, notified_at TEXT)`,
	}
	for i := 0; i < items; i++ {
		stmts = append(stmts, `INSERT INTO items (user_id, text, done, position, created_at, updated_at)
			VALUES (1, 'Old item', 0, 1, '2026-09-01T10:00:00Z', '2026-09-01T10:00:00Z')`)
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("old schema: %v", err)
		}
	}
	return path
}

func userVersion(t *testing.T, path string) int {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()
	var v int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		t.Fatalf("user_version: %v", err)
	}
	return v
}

func TestOpenRefusesOldItemsWithoutFlag(t *testing.T) {
	path := oldDatabase(t, 2)
	_, err := sqlite.Open(path)
	var old *sqlite.OldItemsError
	if !errors.As(err, &old) || old.Count != 2 {
		t.Fatalf("Open err = %v, want OldItemsError with 2 items", err)
	}
	want := "this database has 2 items from before user accounts.\nStart again with -delete-old-items to delete them and continue."
	if err.Error() != want {
		t.Fatalf("message = %q, want %q", err.Error(), want)
	}
	if v := userVersion(t, path); v != 0 {
		t.Fatalf("user_version = %d after refusal, want 0 (unchanged)", v)
	}
}

func TestOpenWithDeleteOldItems(t *testing.T) {
	path := oldDatabase(t, 2)
	s, err := sqlite.OpenWith(path, sqlite.Options{DeleteOldItems: true})
	if err != nil {
		t.Fatalf("OpenWith: %v", err)
	}
	defer s.Close()
	if v := userVersion(t, path); v != 2 {
		t.Fatalf("user_version = %d, want 2", v)
	}
	if _, err := s.UserByName(context.Background(), "default"); !errors.Is(err, auth.ErrNoUser) {
		t.Fatalf("old user still there: err = %v", err)
	}
}

func TestOpenOldDatabaseWithoutItemsNeedsNoFlag(t *testing.T) {
	path := oldDatabase(t, 0)
	s, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	s.Close()
	if v := userVersion(t, path); v != 2 {
		t.Fatalf("user_version = %d, want 2", v)
	}
}

func TestNewDatabaseIsVersion2AndReopens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.db")
	s, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	u, err := s.CreateUser(context.Background(), "alice", "x", t0)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	s.Close()
	s, err = sqlite.OpenWith(path, sqlite.Options{DeleteOldItems: true})
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer s.Close()
	if _, err := s.UserByID(context.Background(), u.ID); err != nil {
		t.Fatalf("user lost on reopen: %v", err)
	}
	if v := userVersion(t, path); v != 2 {
		t.Fatalf("user_version = %d, want 2", v)
	}
}
```

Add `"todo/internal/auth"` to the imports of `sqlite_test.go`.

- [ ] **Step 4: Write the failing store tests** — `internal/store/sqlite/users_test.go`:

```go
package sqlite_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"todo/internal/auth"
	"todo/internal/todo"
)

func TestCreateUserAndFind(t *testing.T) {
	s := newStore(t) // already has "alice" (ID 1)
	ctx := context.Background()
	u, err := s.CreateUser(ctx, "Bob_2", "hash", t0)
	if err != nil || u.ID == 0 || u.Username != "Bob_2" || u.PasswordHash != "hash" || !u.CreatedAt.Equal(t0) {
		t.Fatalf("CreateUser = %+v, %v", u, err)
	}
	byName, err := s.UserByName(ctx, "bob_2")
	if err != nil || byName.ID != u.ID || byName.Username != "Bob_2" {
		t.Fatalf("UserByName(lower case) = %+v, %v", byName, err)
	}
	if _, err := s.CreateUser(ctx, "ALICE", "hash", t0); !errors.Is(err, auth.ErrUsernameTaken) {
		t.Fatalf("CreateUser(ALICE) err = %v, want ErrUsernameTaken", err)
	}
	if _, err := s.UserByName(ctx, "nobody"); !errors.Is(err, auth.ErrNoUser) {
		t.Fatalf("UserByName(nobody) err = %v, want ErrNoUser", err)
	}
	if _, err := s.UserByID(ctx, 999); !errors.Is(err, auth.ErrNoUser) {
		t.Fatalf("UserByID(999) err = %v, want ErrNoUser", err)
	}
}

func TestSetPasswordHash(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	if err := s.SetPasswordHash(ctx, 1, "new"); err != nil {
		t.Fatalf("SetPasswordHash: %v", err)
	}
	u, _ := s.UserByID(ctx, 1)
	if u.PasswordHash != "new" {
		t.Fatalf("hash = %q, want new", u.PasswordHash)
	}
	if err := s.SetPasswordHash(ctx, 999, "x"); !errors.Is(err, auth.ErrNoUser) {
		t.Fatalf("SetPasswordHash(999) err = %v, want ErrNoUser", err)
	}
}

func TestSessions(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	live := auth.Session{TokenHash: "a", UserID: 1, CreatedAt: t0, ExpiresAt: t0.Add(time.Hour)}
	other := auth.Session{TokenHash: "b", UserID: 1, CreatedAt: t0, ExpiresAt: t0.Add(time.Hour), Persistent: true}
	old := auth.Session{TokenHash: "c", UserID: 1, CreatedAt: t0, ExpiresAt: t0.Add(-time.Minute)}
	for _, sess := range []auth.Session{live, other, old} {
		if err := s.CreateSession(ctx, sess); err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
	}
	got, u, err := s.SessionUser(ctx, "b", t0)
	if err != nil || u.Username != "alice" || !got.Persistent || !got.ExpiresAt.Equal(other.ExpiresAt) {
		t.Fatalf("SessionUser(b) = %+v, %+v, %v", got, u, err)
	}
	if _, _, err := s.SessionUser(ctx, "c", t0); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("expired session err = %v, want ErrNoSession", err)
	}
	if _, _, err := s.SessionUser(ctx, "zzz", t0); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("unknown session err = %v, want ErrNoSession", err)
	}
	if n, err := s.DeleteExpiredSessions(ctx, t0); err != nil || n != 1 {
		t.Fatalf("DeleteExpiredSessions = %d, %v; want 1", n, err)
	}
	if err := s.DeleteOtherSessions(ctx, 1, "a"); err != nil {
		t.Fatalf("DeleteOtherSessions: %v", err)
	}
	if _, _, err := s.SessionUser(ctx, "b", t0); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("other session still valid: %v", err)
	}
	if err := s.DeleteSession(ctx, "a"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if _, _, err := s.SessionUser(ctx, "a", t0); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("deleted session still valid: %v", err)
	}
}

func TestDeleteUserCascades(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	s.CreateSession(ctx, auth.Session{TokenHash: "a", UserID: 1, CreatedAt: t0, ExpiresAt: t0.Add(time.Hour)})
	if _, err := s.Create(ctx, 1, todo.Change{Text: "x"}, t0); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := s.DeleteUser(ctx, 1); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	if open, done, _ := s.Counts(ctx, 1); open+done != 0 {
		t.Fatalf("items left after DeleteUser: %d", open+done)
	}
	if _, _, err := s.SessionUser(ctx, "a", t0); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("session left after DeleteUser: %v", err)
	}
	if err := s.DeleteUser(ctx, 1); !errors.Is(err, auth.ErrNoUser) {
		t.Fatalf("second DeleteUser err = %v, want ErrNoUser", err)
	}
}
```

- [ ] **Step 5: Run to see them fail**

Run: `go test ./internal/store/sqlite/`
Expected: FAIL to compile (`s.CreateUser undefined`, `sqlite.OpenWith undefined`).

- [ ] **Step 6: Schema version 2 in `internal/store/sqlite/sqlite.go`**

Delete `var schema`, `var newColumns`, and `func migrate`. Add (keep `const itemCols` as it is):

```go
// schemaV2 is the schema since user accounts (PRAGMA user_version = 2).
var schemaV2 = []string{
	`CREATE TABLE users (
		id            INTEGER PRIMARY KEY,
		username      TEXT NOT NULL UNIQUE COLLATE NOCASE,
		password_hash TEXT NOT NULL,
		created_at    TEXT NOT NULL
	)`,
	`CREATE TABLE sessions (
		token_hash TEXT PRIMARY KEY,
		user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		created_at TEXT NOT NULL,
		expires_at TEXT NOT NULL,
		persistent INTEGER NOT NULL
	)`,
	`CREATE INDEX sessions_user ON sessions (user_id)`,
	`CREATE TABLE settings (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	)`,
	`CREATE TABLE items (
		id          INTEGER PRIMARY KEY,
		user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		text        TEXT    NOT NULL,
		done        INTEGER NOT NULL DEFAULT 0,
		position    INTEGER NOT NULL,
		created_at  TEXT    NOT NULL,
		updated_at  TEXT    NOT NULL,
		due_at      TEXT,
		due_all_day INTEGER NOT NULL DEFAULT 0,
		notified_at TEXT
	)`,
	`CREATE INDEX items_user_position ON items (user_id, position)`,
}

// Options change how OpenWith sets up the database.
type Options struct {
	// DeleteOldItems allows the step to schema version 2 to delete items
	// from before user accounts.
	DeleteOldItems bool
}

// OldItemsError means the database still has items from before user accounts.
type OldItemsError struct{ Count int }

func (e *OldItemsError) Error() string {
	return fmt.Sprintf("this database has %d items from before user accounts.\n"+
		"Start again with -delete-old-items to delete them and continue.", e.Count)
}

// migrate brings the database to schema version 2.
func migrate(db *sql.DB, opts Options) error {
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return err
	}
	if version >= 2 {
		return nil
	}
	var hasItems int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'items'`).Scan(&hasItems); err != nil {
		return err
	}
	if hasItems > 0 {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM items`).Scan(&n); err != nil {
			return err
		}
		if n > 0 && !opts.DeleteOldItems {
			return &OldItemsError{Count: n}
		}
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	steps := append([]string{
		`DROP TABLE IF EXISTS items`,
		`DROP TABLE IF EXISTS sessions`,
		`DROP TABLE IF EXISTS settings`,
		`DROP TABLE IF EXISTS users`,
	}, schemaV2...)
	steps = append(steps, `PRAGMA user_version = 2`)
	for _, stmt := range steps {
		if _, err := tx.Exec(stmt); err != nil {
			return err
		}
	}
	return tx.Commit()
}
```

Change `Open` into `OpenWith` and add a new `Open`:

```go
// Open opens (or creates) the database file at path with default options.
func Open(path string) (*Store, error) {
	return OpenWith(path, Options{})
}

// OpenWith opens (or creates) the database file at path and brings the
// schema to the newest version.
func OpenWith(path string, opts Options) (*Store, error) {
```

In the body, replace the `for _, stmt := range schema {…}` loop and the old `if err := migrate(db); …` block with:

```go
	if err := migrate(db, opts); err != nil {
		db.Close()
		var old *OldItemsError
		if errors.As(err, &old) {
			return nil, err
		}
		return nil, fmt.Errorf("migrate schema in %q: %w", path, err)
	}
```

Replace `formatDue` with a version built on a shared helper:

```go
// fixedTime has a fixed length, so SQLite can compare such times as text.
func fixedTime(t time.Time) string {
	return t.UTC().Format(dueLayout)
}

func formatDue(t *time.Time) any {
	if t == nil {
		return nil
	}
	return fixedTime(*t)
}
```

- [ ] **Step 7: Create `internal/store/sqlite/users.go`**

```go
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"todo/internal/auth"
)

var _ auth.Store = (*Store)(nil)

// sqliteConstraintUnique is SQLITE_CONSTRAINT_UNIQUE.
const sqliteConstraintUnique = 2067

func isUniqueViolation(err error) bool {
	var coded interface{ Code() int }
	return errors.As(err, &coded) && coded.Code() == sqliteConstraintUnique
}

const userCols = `id, username, password_hash, created_at`

func scanUser(r scanner) (auth.User, error) {
	var u auth.User
	var created string
	if err := r.Scan(&u.ID, &u.Username, &u.PasswordHash, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return auth.User{}, auth.ErrNoUser
		}
		return auth.User{}, fmt.Errorf("read user: %w", err)
	}
	var err error
	if u.CreatedAt, err = time.Parse(time.RFC3339Nano, created); err != nil {
		return auth.User{}, fmt.Errorf("read user %d created_at: %w", u.ID, err)
	}
	return u, nil
}

func (s *Store) CreateUser(ctx context.Context, username, passwordHash string, now time.Time) (auth.User, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO users (username, password_hash, created_at) VALUES (?, ?, ?)`,
		username, passwordHash, formatTime(now))
	if isUniqueViolation(err) {
		return auth.User{}, auth.ErrUsernameTaken
	}
	if err != nil {
		return auth.User{}, fmt.Errorf("create user: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return auth.User{}, fmt.Errorf("create user: %w", err)
	}
	return s.UserByID(ctx, id)
}

func (s *Store) UserByName(ctx context.Context, username string) (auth.User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE username = ?`, username))
}

func (s *Store) UserByID(ctx context.Context, id int64) (auth.User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE id = ?`, id))
}

// changeUser runs a statement that must change exactly one user row.
func (s *Store) changeUser(ctx context.Context, q string, args ...any) error {
	res, err := s.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("change user: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("change user: %w", err)
	}
	if n == 0 {
		return auth.ErrNoUser
	}
	return nil
}

func (s *Store) SetPasswordHash(ctx context.Context, id int64, passwordHash string) error {
	return s.changeUser(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, passwordHash, id)
}

func (s *Store) DeleteUser(ctx context.Context, id int64) error {
	return s.changeUser(ctx, `DELETE FROM users WHERE id = ?`, id)
}

func (s *Store) CreateSession(ctx context.Context, sess auth.Session) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (token_hash, user_id, created_at, expires_at, persistent) VALUES (?, ?, ?, ?, ?)`,
		sess.TokenHash, sess.UserID, formatTime(sess.CreatedAt), fixedTime(sess.ExpiresAt), sess.Persistent)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (s *Store) SessionUser(ctx context.Context, tokenHash string, now time.Time) (auth.Session, auth.User, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT s.token_hash, s.user_id, s.created_at, s.expires_at, s.persistent,
		        u.id, u.username, u.password_hash, u.created_at
		 FROM sessions s JOIN users u ON u.id = s.user_id
		 WHERE s.token_hash = ? AND s.expires_at > ?`, tokenHash, fixedTime(now))
	var sess auth.Session
	var u auth.User
	var sCreated, sExpires, uCreated string
	err := row.Scan(&sess.TokenHash, &sess.UserID, &sCreated, &sExpires, &sess.Persistent,
		&u.ID, &u.Username, &u.PasswordHash, &uCreated)
	if errors.Is(err, sql.ErrNoRows) {
		return auth.Session{}, auth.User{}, auth.ErrNoSession
	}
	if err != nil {
		return auth.Session{}, auth.User{}, fmt.Errorf("read session: %w", err)
	}
	for _, f := range []struct {
		dst *time.Time
		src string
	}{{&sess.CreatedAt, sCreated}, {&sess.ExpiresAt, sExpires}, {&u.CreatedAt, uCreated}} {
		if *f.dst, err = time.Parse(time.RFC3339Nano, f.src); err != nil {
			return auth.Session{}, auth.User{}, fmt.Errorf("read session time: %w", err)
		}
	}
	return sess, u, nil
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (s *Store) DeleteOtherSessions(ctx context.Context, userID int64, keepTokenHash string) error {
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM sessions WHERE user_id = ? AND token_hash != ?`, userID, keepTokenHash); err != nil {
		return fmt.Errorf("delete other sessions: %w", err)
	}
	return nil
}

func (s *Store) DeleteExpiredSessions(ctx context.Context, now time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, fixedTime(now))
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}
	return res.RowsAffected()
}
```

- [ ] **Step 8: `cmd/todo/main.go`** — add the flag and use `OpenWith`:

```go
	deleteOldItems := flag.Bool("delete-old-items", false, "allow deleting items from before user accounts when the database is upgraded")
```

(after the `dbPath` flag) and replace `store, err := sqlite.Open(*dbPath)` with:

```go
	store, err := sqlite.OpenWith(*dbPath, sqlite.Options{DeleteOldItems: *deleteOldItems})
```

- [ ] **Step 9: Run all tests and vet**

Run: `go test -count=1 ./...` then `go vet ./...`
Expected: both exit 0.

- [ ] **Step 10: Commit**

```bash
git add go.mod go.sum docs/superpowers/plans/2026-10-02-users-accounts.md internal/auth internal/store cmd internal/todo/service_test.go internal/web/web_test.go
git commit -m "feat: schema version 2 with users and sessions"
```

---

### Task 2: Account rules, password hashing, and the auth service

**Files:**
- Create: `internal/auth/rules.go`, `internal/auth/password.go`, `internal/auth/service.go`
- Test: `internal/auth/auth_test.go`

**Interfaces:**
- Consumes: `auth.Store` and types from Task 1; `sqlite.Open` (tests).
- Produces: `auth.ValidateUsername(string) error`; `auth.ValidatePassword(string) error`; `auth.HashPassword(string) (string, error)`; `auth.CheckPassword(encoded, password string) bool`; `auth.HashToken(token string) string`; `auth.NewService(store Store, opts ...Option) *Service`; `auth.WithClock(func() time.Time) Option`; methods `SignUp(ctx, username, password string) (User, error)`, `LogIn(ctx, username, password string, persistent bool) (token string, s Session, err error)`, `StartSession(ctx, userID int64, persistent bool) (string, Session, error)`, `Authenticate(ctx, token string) (User, Session, error)`, `LogOut(ctx, token string) error`, `ChangePassword(ctx, userID int64, token, current, next string) error`, `DeleteAccount(ctx, userID int64, confirm string) error`, `CleanUp(ctx) (int64, error)`; constants `ShortSession = 12 * time.Hour`, `LongSession = 30 * 24 * time.Hour`.

- [ ] **Step 1: Write the failing tests** — `internal/auth/auth_test.go`:

```go
package auth_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"todo/internal/auth"
	"todo/internal/store/sqlite"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newAuth(t *testing.T) (*auth.Service, *clock) {
	t.Helper()
	s, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	c := &clock{t: time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)}
	return auth.NewService(s, auth.WithClock(c.now)), c
}

const pw = "correct horse battery"

func ruleMsg(t *testing.T, err error) string {
	t.Helper()
	var re *auth.RuleError
	if !errors.As(err, &re) {
		t.Fatalf("err = %v, want RuleError", err)
	}
	return re.Msg
}

func TestUsernameRules(t *testing.T) {
	for _, ok := range []string{"abc", "Zbigniew", "a_b-c", strings.Repeat("x", 32)} {
		if err := auth.ValidateUsername(ok); err != nil {
			t.Errorf("ValidateUsername(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"", "ab", strings.Repeat("x", 33), "with space", "zażółć", "a.b", "<script>"} {
		if msg := ruleMsg(t, auth.ValidateUsername(bad)); msg != "Use 3–32 letters, digits, - or _." {
			t.Errorf("ValidateUsername(%q) msg = %q", bad, msg)
		}
	}
}

func TestPasswordRulesCountCharacters(t *testing.T) {
	if err := auth.ValidatePassword("zażółćgęśl"); err != nil { // 10 characters, 16 bytes
		t.Errorf("10 non-ASCII characters: %v, want nil", err)
	}
	if msg := ruleMsg(t, auth.ValidatePassword("zażółćgęś")); msg != "Use at least 10 characters." { // 9 characters, 15 bytes
		t.Errorf("9 characters msg = %q", msg)
	}
	if msg := ruleMsg(t, auth.ValidatePassword(strings.Repeat("a", 201))); msg != "Use at most 200 characters." {
		t.Errorf("201 characters msg = %q", msg)
	}
	if err := auth.ValidatePassword(strings.Repeat("ą", 200)); err != nil {
		t.Errorf("200 characters: %v, want nil", err)
	}
}

func TestHashPassword(t *testing.T) {
	h, err := auth.HashPassword(pw)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$m=65536,t=3,p=4$") || strings.Count(h, "$") != 5 {
		t.Fatalf("hash = %q, want argon2id text form", h)
	}
	if !auth.CheckPassword(h, pw) || auth.CheckPassword(h, pw+"x") {
		t.Fatal("CheckPassword gives a wrong answer")
	}
	other, _ := auth.HashPassword(pw)
	if other == h {
		t.Fatal("two hashes of the same password are equal (salt missing)")
	}
	for _, bad := range []string{"", "x", "$argon2i$v=19$m=65536,t=3,p=4$AAAA$AAAA", "$argon2id$v=19$m=x$AAAA$AAAA"} {
		if auth.CheckPassword(bad, pw) {
			t.Errorf("CheckPassword(%q) = true", bad)
		}
	}
}

func TestSignUpAndLogIn(t *testing.T) {
	a, _ := newAuth(t)
	ctx := context.Background()
	u, err := a.SignUp(ctx, "alice", pw)
	if err != nil || u.ID == 0 || u.Username != "alice" {
		t.Fatalf("SignUp = %+v, %v", u, err)
	}
	token, sess, err := a.LogIn(ctx, "alice", pw, false)
	if err != nil || token == "" || sess.Persistent || sess.UserID != u.ID {
		t.Fatalf("LogIn = %q, %+v, %v", token, sess, err)
	}
	if sess.TokenHash != auth.HashToken(token) || strings.Contains(sess.TokenHash, token) {
		t.Fatal("session must store only the hash of the token")
	}
	got, _, err := a.Authenticate(ctx, token)
	if err != nil || got.ID != u.ID {
		t.Fatalf("Authenticate = %+v, %v", got, err)
	}
	if _, _, err := a.LogIn(ctx, "alice", "wrong password!", false); !errors.Is(err, auth.ErrBadLogin) {
		t.Fatalf("wrong password err = %v, want ErrBadLogin", err)
	}
	if _, _, err := a.LogIn(ctx, "nobody", pw, false); !errors.Is(err, auth.ErrBadLogin) {
		t.Fatalf("unknown user err = %v, want ErrBadLogin", err)
	}
	if ruleMsg(t, func() error { _, err := a.SignUp(ctx, "x", pw); return err }()) == "" {
		t.Fatal("short username accepted")
	}
	if _, err := a.SignUp(ctx, "bob", "short"); err == nil {
		t.Fatal("short password accepted")
	}
}

func TestUsernameIgnoresCase(t *testing.T) {
	a, _ := newAuth(t)
	ctx := context.Background()
	a.SignUp(ctx, "alice", pw)
	if _, _, err := a.LogIn(ctx, "ALICE", pw, false); err != nil {
		t.Fatalf("LogIn(ALICE) = %v, want success", err)
	}
	if _, err := a.SignUp(ctx, "Alice", pw); !errors.Is(err, auth.ErrUsernameTaken) {
		t.Fatalf("SignUp(Alice) err = %v, want ErrUsernameTaken", err)
	}
}

func TestSessionExpiry(t *testing.T) {
	a, c := newAuth(t)
	ctx := context.Background()
	u, _ := a.SignUp(ctx, "alice", pw)
	short, s1, _ := a.StartSession(ctx, u.ID, false)
	long, s2, _ := a.StartSession(ctx, u.ID, true)
	if s1.ExpiresAt.Sub(s1.CreatedAt) != auth.ShortSession || s2.ExpiresAt.Sub(s2.CreatedAt) != auth.LongSession || !s2.Persistent {
		t.Fatalf("lifetimes = %v, %v", s1.ExpiresAt.Sub(s1.CreatedAt), s2.ExpiresAt.Sub(s2.CreatedAt))
	}
	c.t = c.t.Add(auth.ShortSession + time.Second)
	if _, _, err := a.Authenticate(ctx, short); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("short session after 12 h: err = %v, want ErrNoSession", err)
	}
	if _, _, err := a.Authenticate(ctx, long); err != nil {
		t.Fatalf("long session after 12 h: %v, want valid", err)
	}
	if n, err := a.CleanUp(ctx); err != nil || n != 1 {
		t.Fatalf("CleanUp = %d, %v; want 1", n, err)
	}
	c.t = c.t.Add(auth.LongSession)
	if _, _, err := a.Authenticate(ctx, long); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("long session after 30 days: err = %v, want ErrNoSession", err)
	}
	if _, _, err := a.Authenticate(ctx, ""); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("empty token: err = %v, want ErrNoSession", err)
	}
}

func TestLogInCreatesNewTokenAndLogOutEndsIt(t *testing.T) {
	a, _ := newAuth(t)
	ctx := context.Background()
	a.SignUp(ctx, "alice", pw)
	t1, _, _ := a.LogIn(ctx, "alice", pw, false)
	t2, _, _ := a.LogIn(ctx, "alice", pw, false)
	if t1 == t2 {
		t.Fatal("two logins gave the same token")
	}
	if err := a.LogOut(ctx, t1); err != nil {
		t.Fatalf("LogOut: %v", err)
	}
	if _, _, err := a.Authenticate(ctx, t1); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("logged-out token: err = %v, want ErrNoSession", err)
	}
	if _, _, err := a.Authenticate(ctx, t2); err != nil {
		t.Fatalf("other session ended by LogOut: %v", err)
	}
}

func TestChangePassword(t *testing.T) {
	a, _ := newAuth(t)
	ctx := context.Background()
	u, _ := a.SignUp(ctx, "alice", pw)
	current, _, _ := a.LogIn(ctx, "alice", pw, false)
	other, _, _ := a.LogIn(ctx, "alice", pw, true)

	if err := a.ChangePassword(ctx, u.ID, current, "wrong one!!", "new password 1"); !errors.Is(err, auth.ErrWrongPassword) {
		t.Fatalf("wrong current err = %v, want ErrWrongPassword", err)
	}
	if err := a.ChangePassword(ctx, u.ID, current, pw, "short"); ruleMsg(t, err) != "Use at least 10 characters." {
		t.Fatalf("short new password err = %v", err)
	}
	if err := a.ChangePassword(ctx, u.ID, current, pw, "new password 1"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if _, _, err := a.Authenticate(ctx, current); err != nil {
		t.Fatalf("current session ended: %v", err)
	}
	if _, _, err := a.Authenticate(ctx, other); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("other session still valid: %v", err)
	}
	if _, _, err := a.LogIn(ctx, "alice", "new password 1", false); err != nil {
		t.Fatalf("login with new password: %v", err)
	}
	if _, _, err := a.LogIn(ctx, "alice", pw, false); !errors.Is(err, auth.ErrBadLogin) {
		t.Fatalf("old password still works: %v", err)
	}
}

func TestDeleteAccount(t *testing.T) {
	a, _ := newAuth(t)
	ctx := context.Background()
	u, _ := a.SignUp(ctx, "alice", pw)
	token, _, _ := a.LogIn(ctx, "alice", pw, true)
	if err := a.DeleteAccount(ctx, u.ID, "bob"); !errors.Is(err, auth.ErrConfirmMismatch) {
		t.Fatalf("mismatch err = %v, want ErrConfirmMismatch", err)
	}
	if err := a.DeleteAccount(ctx, u.ID, " ALICE "); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}
	if _, _, err := a.Authenticate(ctx, token); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("session of deleted user: err = %v, want ErrNoSession", err)
	}
	if _, _, err := a.LogIn(ctx, "alice", pw, false); !errors.Is(err, auth.ErrBadLogin) {
		t.Fatalf("deleted user can log in: %v", err)
	}
}
```

- [ ] **Step 2: Run to see them fail**

Run: `go test ./internal/auth/`
Expected: FAIL to compile (`auth.ValidateUsername undefined` …).

- [ ] **Step 3: Create `internal/auth/rules.go`**

```go
package auth

import (
	"regexp"
	"unicode/utf8"
)

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{3,32}$`)

const (
	minPasswordChars = 10
	maxPasswordChars = 200
)

// ValidateUsername checks the characters and the length of a username.
func ValidateUsername(name string) error {
	if !usernamePattern.MatchString(name) {
		return &RuleError{Field: "username", Msg: "Use 3–32 letters, digits, - or _."}
	}
	return nil
}

// ValidatePassword checks the length of a password in characters.
func ValidatePassword(password string) error {
	n := utf8.RuneCountInString(password)
	if n < minPasswordChars {
		return &RuleError{Field: "password", Msg: "Use at least 10 characters."}
	}
	if n > maxPasswordChars {
		return &RuleError{Field: "password", Msg: "Use at most 200 characters."}
	}
	return nil
}
```

- [ ] **Step 4: Create `internal/auth/password.go`**

```go
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"
)

const (
	argonMemory  = 64 * 1024 // KiB
	argonTime    = 3
	argonThreads = 4
	argonSalt    = 16
	argonKey     = 32
)

var b64 = base64.RawStdEncoding

// HashPassword returns the argon2id hash of password in its standard text form.
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSalt)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKey)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// CheckPassword reports whether password matches the stored hash. It reads
// the parameters from the hash, so they can change later.
func CheckPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return false
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false
	}
	var memory, time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil || threads == 0 || time == 0 {
		return false
	}
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return false
	}
	key, err := b64.DecodeString(parts[5])
	if err != nil || len(key) == 0 {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(key)))
	return subtle.ConstantTimeCompare(got, key) == 1
}

// dummyHash is checked for unknown usernames, so the answer takes as long
// as for a real user.
var dummyHash = sync.OnceValue(func() string {
	h, _ := HashPassword("not a real password, only for timing")
	return h
})
```

- [ ] **Step 5: Create `internal/auth/service.go`**

```go
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	// ShortSession is the lifetime without "Keep me logged in".
	ShortSession = 12 * time.Hour
	// LongSession is the lifetime with "Keep me logged in".
	LongSession = 30 * 24 * time.Hour
)

// Service holds the account and session rules.
type Service struct {
	store Store
	now   func() time.Time
}

// Option changes a Service.
type Option func(*Service)

// WithClock replaces the clock (for tests).
func WithClock(now func() time.Time) Option {
	return func(s *Service) { s.now = now }
}

// NewService returns a Service that keeps accounts in store.
func NewService(store Store, opts ...Option) *Service {
	s := &Service{store: store, now: time.Now}
	for _, o := range opts {
		o(s)
	}
	return s
}

// HashToken returns the hex SHA-256 of a session token.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// SignUp creates an account. It checks the rules before the slow hash.
func (s *Service) SignUp(ctx context.Context, username, password string) (User, error) {
	if err := ValidateUsername(username); err != nil {
		return User{}, err
	}
	if err := ValidatePassword(password); err != nil {
		return User{}, err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return User{}, err
	}
	return s.store.CreateUser(ctx, username, hash, s.now())
}

// LogIn checks username and password and starts a new session.
func (s *Service) LogIn(ctx context.Context, username, password string, persistent bool) (string, Session, error) {
	if len(password) > 4*maxPasswordChars {
		return "", Session{}, ErrBadLogin // never hash huge input
	}
	u, err := s.store.UserByName(ctx, username)
	if errors.Is(err, ErrNoUser) {
		CheckPassword(dummyHash(), password)
		return "", Session{}, ErrBadLogin
	}
	if err != nil {
		return "", Session{}, err
	}
	if !CheckPassword(u.PasswordHash, password) {
		return "", Session{}, ErrBadLogin
	}
	return s.StartSession(ctx, u.ID, persistent)
}

// StartSession creates a session and returns its token.
func (s *Service) StartSession(ctx context.Context, userID int64, persistent bool) (string, Session, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", Session{}, fmt.Errorf("start session: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	now := s.now()
	life := ShortSession
	if persistent {
		life = LongSession
	}
	sess := Session{TokenHash: HashToken(token), UserID: userID, CreatedAt: now, ExpiresAt: now.Add(life), Persistent: persistent}
	if err := s.store.CreateSession(ctx, sess); err != nil {
		return "", Session{}, err
	}
	return token, sess, nil
}

// Authenticate returns the user of a valid session token.
func (s *Service) Authenticate(ctx context.Context, token string) (User, Session, error) {
	if token == "" {
		return User{}, Session{}, ErrNoSession
	}
	sess, u, err := s.store.SessionUser(ctx, HashToken(token), s.now())
	return u, sess, err
}

// LogOut ends the session of token.
func (s *Service) LogOut(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.store.DeleteSession(ctx, HashToken(token))
}

// ChangePassword sets a new password and ends all other sessions of the user.
func (s *Service) ChangePassword(ctx context.Context, userID int64, token, current, next string) error {
	if len(current) > 4*maxPasswordChars {
		return ErrWrongPassword
	}
	u, err := s.store.UserByID(ctx, userID)
	if err != nil {
		return err
	}
	if !CheckPassword(u.PasswordHash, current) {
		return ErrWrongPassword
	}
	if err := ValidatePassword(next); err != nil {
		return err
	}
	hash, err := HashPassword(next)
	if err != nil {
		return err
	}
	if err := s.store.SetPasswordHash(ctx, userID, hash); err != nil {
		return err
	}
	return s.store.DeleteOtherSessions(ctx, userID, HashToken(token))
}

// DeleteAccount deletes the user, their items, and their sessions, if confirm
// matches the username (case does not matter).
func (s *Service) DeleteAccount(ctx context.Context, userID int64, confirm string) error {
	u, err := s.store.UserByID(ctx, userID)
	if err != nil {
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(confirm), u.Username) {
		return ErrConfirmMismatch
	}
	return s.store.DeleteUser(ctx, userID)
}

// CleanUp deletes expired sessions.
func (s *Service) CleanUp(ctx context.Context) (int64, error) {
	return s.store.DeleteExpiredSessions(ctx, s.now())
}
```

- [ ] **Step 6: Run the tests**

Run: `go mod tidy && go test -count=1 ./internal/auth/` then `go test -count=1 ./... && go vet ./...`
Expected: all exit 0. (`go mod tidy` keeps `golang.org/x/crypto` now that it is imported.)

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum internal/auth
git commit -m "feat: account rules, argon2id passwords, and sessions"
```

---

### Task 3: Sessions in the web app, login, sign-up, logout, and isolation

**Files:**
- Create: `internal/web/session.go`, `internal/web/auth_handlers.go`, `internal/web/templates/auth.html`, `internal/web/static/auth.js`
- Modify: `internal/web/server.go`, `internal/web/handlers.go`, `internal/web/views.go`, `internal/web/templates/page.html`, `internal/web/static/app.js`, `internal/web/static/app.css`, `cmd/todo/main.go`, `internal/web/web_test.go`, `internal/web/review_fixes_test.go`, `internal/web/notify_test.go`, `internal/web/postpone_test.go`
- Test: `internal/web/auth_test.go` (new)

**Interfaces:**
- Consumes: `auth.Service` (Task 2), `sqlite.Store` as `auth.Store` (Task 1).
- Produces: `web.New(svc *todo.Service, accounts *auth.Service, log *slog.Logger) (http.Handler, error)`; in package `web`: `sessionCookie = "todo_session"`, `setSessionCookie(w, token string, s auth.Session)`, `clearSessionCookie(w)`, `(*server).protect(h http.HandlerFunc) http.Handler`, `currentUser(r) auth.User`, `sessionToken(r) string`, template `auth-page` layout; test helpers `newTestEnv(t) *testEnv` with fields `H http.Handler` (adds the tester's cookie), `Raw http.Handler`, `Svc *todo.Service`, `Auth *auth.Service`, `User auth.User`, `Token string`; `cookie(token) map[string]string`; `anon` headers.

- [ ] **Step 1: Test helpers** — replace `newTestApp` in `internal/web/web_test.go` with:

```go
// testEnv is an app with one logged-in user "tester" (ID 1).
type testEnv struct {
	H     http.Handler // adds the tester's session cookie unless the request has one or is anonymous
	Raw   http.Handler
	Svc   *todo.Service
	Auth  *auth.Service
	Store *sqlite.Store
	User  auth.User
	Token string
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	user, err := store.CreateUser(context.Background(), "tester", "x", time.Now())
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	accounts := auth.NewService(store)
	token, _, err := accounts.StartSession(context.Background(), user.ID, false)
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	svc := todo.NewService(store)
	raw, err := web.New(svc, accounts, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("web.New: %v", err)
	}
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(anonHeader) == "" {
			if _, err := r.Cookie("todo_session"); err != nil {
				r.AddCookie(&http.Cookie{Name: "todo_session", Value: token})
			}
		}
		r.Header.Del(anonHeader)
		raw.ServeHTTP(w, r)
	})
	return &testEnv{H: h, Raw: raw, Svc: svc, Auth: accounts, Store: store, User: user, Token: token}
}

func newTestApp(t *testing.T) (http.Handler, *todo.Service) {
	env := newTestEnv(t)
	return env.H, env.Svc
}

// anonHeader makes a test request without the tester's cookie.
const anonHeader = "X-Test-Anonymous"

var anon = map[string]string{anonHeader: "1"}

// cookie returns headers that send the session cookie of token.
func cookie(token string, extra ...map[string]string) map[string]string {
	h := map[string]string{"Cookie": "todo_session=" + token}
	for _, e := range extra {
		for k, v := range e {
			h[k] = v
		}
	}
	return h
}

// sessionFrom returns the todo_session cookie set by a response.
func sessionFrom(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == "todo_session" {
			return c
		}
	}
	t.Fatalf("no todo_session cookie in response; headers: %v", rec.Header())
	return nil
}
```

Remove the `store.CreateUser` lines added in Task 1 to the old `newTestApp` (the new helper creates the user). Add `"todo/internal/auth"` to the imports.

Update three existing tests that assumed the old CSRF rule (no `HX-Request` → 403). `http.CrossOriginProtection` allows requests without `Sec-Fetch-Site` and `Origin` (not a browser); it rejects browser requests marked cross-site or same-site:

```bash
perl -0pi -e 's/\n\t*\{"plain form post", nil\},//' internal/web/review_fixes_test.go
perl -pi -e 's|do\(t, h, "POST", "/notifications/claim", nil, nil\); rec.Code != http.StatusForbidden|do(t, h, "POST", "/notifications/claim", nil, map[string]string{"Sec-Fetch-Site": "cross-site"}); rec.Code != http.StatusForbidden|' internal/web/notify_test.go
perl -pi -e 's|\{"/items/1/postpone\?minutes=5", nil, http.StatusForbidden\}|{"/items/1/postpone?minutes=5", map[string]string{"Sec-Fetch-Site": "cross-site"}, http.StatusForbidden}|' internal/web/postpone_test.go
```

- [ ] **Step 2: Write the failing tests** — `internal/web/auth_test.go`:

```go
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
	assertContains(t, body, `<form method="post" action="/signup"`, "Create an account", "3–32 letters, digits, - or _",
		"At least 10 characters", `class="show-password"`, "There is no email. If you forget your password, ask the admin.",
		"Create account", `href="/login"`, `<script src="/static/auth.js" defer></script>`)

	rec := do(t, env.H, "POST", "/signup", url.Values{"username": {"bob"}, "password": {pw}}, anon)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("sign-up: %d %q, want 303 /", rec.Code, rec.Header().Get("Location"))
	}
	if c := sessionFrom(t, rec); c.MaxAge != 0 {
		t.Fatalf("sign-up session must not be persistent: Max-Age %d", c.MaxAge)
	}

	cases := []struct {
		form url.Values
		msg  string
	}{
		{url.Values{"username": {"BOB"}, "password": {pw}}, "This username is taken."},
		{url.Values{"username": {"b"}, "password": {pw}}, "Use 3–32 letters, digits, - or _."},
		{url.Values{"username": {"carol"}, "password": {"short"}}, "Use at least 10 characters."},
	}
	for _, c := range cases {
		rec := do(t, env.H, "POST", "/signup", c.form, anon)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%v: status %d, want 422", c.form, rec.Code)
		}
		assertContains(t, rec.Body.String(), c.msg, `value="`+c.form.Get("username")+`"`)
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
```

- [ ] **Step 3: Run to see them fail**

Run: `go test ./internal/web/`
Expected: FAIL to compile (`web.New` has too many arguments).

- [ ] **Step 4: Create `internal/web/session.go`**

```go
package web

import (
	"context"
	"errors"
	"net/http"

	"todo/internal/auth"
)

const sessionCookie = "todo_session"

type ctxKey int

const userKey ctxKey = 1

func setSessionCookie(w http.ResponseWriter, token string, s auth.Session) {
	c := &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode}
	if s.Persistent {
		c.MaxAge = int(s.ExpiresAt.Sub(s.CreatedAt).Seconds())
	}
	http.SetCookie(w, c)
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, Secure: true,
		SameSite: http.SameSiteLaxMode, MaxAge: -1})
}

// sessionToken returns the session token sent by the browser, or "".
func sessionToken(r *http.Request) string {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return ""
	}
	return c.Value
}

// currentUser returns the logged-in user. Only valid inside protect.
func currentUser(r *http.Request) auth.User {
	u, _ := r.Context().Value(userKey).(auth.User)
	return u
}

func userID(r *http.Request) int64 { return currentUser(r).ID }

// protect lets only requests with a valid session through. Others go to the
// log-in page: a redirect for pages, 401 with HX-Redirect for htmx and fetch.
func (s *server) protect(h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, _, err := s.accounts.Authenticate(r.Context(), sessionToken(r))
		if errors.Is(err, auth.ErrNoSession) {
			if isHTMX(r) {
				w.Header().Set("HX-Redirect", "/login")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		h(w, r.WithContext(context.WithValue(r.Context(), userKey, u)))
	})
}
```

- [ ] **Step 5: `server.go`** — remove `defaultUserID` and `sameOriginOnly`; add the field and routes:

```go
type server struct {
	svc      *todo.Service
	accounts *auth.Service
	tmpl     *template.Template
	log      *slog.Logger
	now      func() time.Time
}

// New returns the HTTP handler for the app.
func New(svc *todo.Service, accounts *auth.Service, log *slog.Logger) (http.Handler, error) {
	s := &server{svc: svc, accounts: accounts, log: log, now: time.Now}
```

Replace the route block and the `return` with:

```go
	mux := http.NewServeMux()
	mux.Handle("GET /static/", noDirListing(http.FileServerFS(staticFS)))
	mux.HandleFunc("GET /login", s.loginPage)
	mux.HandleFunc("POST /login", s.login)
	mux.HandleFunc("GET /signup", s.signupPage)
	mux.HandleFunc("POST /signup", s.signup)
	mux.Handle("POST /logout", s.protect(s.logout))
	mux.Handle("GET /{$}", s.protect(s.index))
	mux.Handle("POST /items", s.protect(s.addItem))
	mux.Handle("POST /items/{id}/toggle", s.protect(s.toggleItem))
	mux.Handle("POST /items/{id}/postpone", s.protect(s.postponeItem))
	mux.Handle("GET /items/{id}/edit", s.protect(s.editItem))
	mux.Handle("GET /items/{id}", s.protect(s.showItem))
	mux.Handle("PUT /items/{id}", s.protect(s.updateItem))
	mux.Handle("DELETE /items/{id}", s.protect(s.deleteItem))
	mux.Handle("POST /notifications/claim", s.protect(s.claimNotifications))
	// Rejects changing requests that a browser marks as coming from another site.
	return http.NewCrossOriginProtection().Handler(mux), nil
}
```

Add `"todo/internal/auth"` to the imports.

- [ ] **Step 6: Use the logged-in user everywhere**

```bash
perl -pi -e 's/defaultUserID/userID(r)/g' internal/web/handlers.go
```

In `views.go`, change `listView` to take the user ID (it has no request):

```go
func (s *server) listView(ctx context.Context, userID int64, hideDone bool) (listView, error) {
	items, err := s.svc.List(ctx, userID, hideDone)
	…
	open, done, err := s.svc.Counts(ctx, userID)
```

and update every call: `perl -pi -e 's/s\.listView\(r\.Context\(\), /s.listView(r.Context(), userID(r), /g' internal/web/handlers.go`.

Add `Username string` to `pageView`; in `index` render `pageView{Form: formView{Focus: true}, List: lv, Username: currentUser(r).Username}`.

- [ ] **Step 7: Header in `page.html`** — replace `<h1>Todo</h1>` with:

```html
<header class="top">
<h1>Todo</h1>
<nav class="user-nav" aria-label="Account">Signed in as <strong>{{.Username}}</strong> · <a href="/account">Account</a> · <form method="post" action="/logout" class="inline-form"><button type="submit" class="link-button">Log out</button></form></nav>
</header>
```

- [ ] **Step 8: Create `internal/web/templates/auth.html`**

```html
{{define "auth-head"}}<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.}} – Todo</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Source+Sans+3:wght@400;600;700&display=swap">
<link rel="stylesheet" href="/static/app.css">
<script src="/static/auth.js" defer></script>
</head>
<body>{{end}}

{{define "login"}}{{template "auth-head" "Log in"}}
<main class="auth-wrap">
<p class="auth-brand">Todo</p>
<div class="auth-card">
<h1>Log in</h1>
{{if .Notice}}<div class="alert ok" role="status">{{.Notice}}</div>{{end}}
{{if .Error}}<div class="alert" role="alert">{{.Error}}</div>{{end}}
<form method="post" action="/login" class="auth-form">
<div class="field"><label for="login-username">Username</label>
<input id="login-username" name="username" value="{{.Username}}" autocomplete="username" required autofocus></div>
<div class="field"><label for="login-password">Password</label>
<input id="login-password" name="password" type="password" autocomplete="current-password" required></div>
<label class="check-line"><input type="checkbox" name="keep" value="1"> Keep me logged in</label>
<button type="submit" class="btn-primary">Log in</button>
</form>
</div>
<p class="auth-foot">No account yet? <a href="/signup">Sign up</a></p>
</main>
</body>
</html>
{{end}}

{{define "signup"}}{{template "auth-head" "Sign up"}}
<main class="auth-wrap">
<p class="auth-brand">Todo</p>
<div class="auth-card">
<h1>Create an account</h1>
{{if .Error}}<div class="alert" role="alert">{{.Error}}</div>{{end}}
<form method="post" action="/signup" class="auth-form">
<div class="field"><label for="signup-username">Username</label>
<input id="signup-username" name="username" value="{{.Username}}" autocomplete="username" required autofocus{{if .UsernameError}} aria-invalid="true" aria-describedby="signup-username-error"{{end}}>
{{if .UsernameError}}<span class="field-error" id="signup-username-error">{{.UsernameError}}</span>{{end}}
<span class="hint">3–32 letters, digits, - or _</span></div>
<div class="field"><label for="signup-password">Password</label>
<div class="password-wrap"><input id="signup-password" name="password" type="password" autocomplete="new-password" required{{if .PasswordError}} aria-invalid="true" aria-describedby="signup-password-error"{{end}}>
<button type="button" class="show-password" data-target="signup-password" aria-pressed="false">Show</button></div>
{{if .PasswordError}}<span class="field-error" id="signup-password-error">{{.PasswordError}}</span>{{end}}
<span class="hint">At least 10 characters</span></div>
<p class="note">There is no email. If you forget your password, ask the admin.</p>
<button type="submit" class="btn-primary">Create account</button>
</form>
</div>
<p class="auth-foot">Already have an account? <a href="/login">Log in</a></p>
</main>
</body>
</html>
{{end}}
```

- [ ] **Step 9: Create `internal/web/static/auth.js`**

```js
// "Show" next to a password field switches it between hidden and readable.
document.addEventListener("click", (e) => {
  const button = e.target.closest && e.target.closest(".show-password");
  if (!button) {
    return;
  }
  const input = document.getElementById(button.dataset.target);
  const show = input.type === "password";
  input.type = show ? "text" : "password";
  button.textContent = show ? "Hide" : "Show";
  button.setAttribute("aria-pressed", show ? "true" : "false");
});
```

- [ ] **Step 10: Create `internal/web/auth_handlers.go`**

```go
package web

import (
	"errors"
	"net/http"

	"todo/internal/auth"
)

type loginView struct {
	Username string
	Error    string
	Notice   string
}

type signupView struct {
	Username      string
	Error         string
	UsernameError string
	PasswordError string
}

// alreadyLoggedIn sends a logged-in browser from the log-in and sign-up pages to the list.
func (s *server) alreadyLoggedIn(w http.ResponseWriter, r *http.Request) bool {
	if _, _, err := s.accounts.Authenticate(r.Context(), sessionToken(r)); err == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return true
	}
	return false
}

func (s *server) loginPage(w http.ResponseWriter, r *http.Request) {
	if s.alreadyLoggedIn(w, r) {
		return
	}
	view := loginView{}
	if r.URL.Query().Get("deleted") == "1" {
		view.Notice = "Your account was deleted."
	}
	s.render(w, r, http.StatusOK, part{"login", view})
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	username := r.PostFormValue("username")
	token, sess, err := s.accounts.LogIn(r.Context(), username, r.PostFormValue("password"), r.PostFormValue("keep") == "1")
	if errors.Is(err, auth.ErrBadLogin) {
		s.render(w, r, http.StatusUnprocessableEntity, part{"login", loginView{Username: username, Error: "Wrong username or password."}})
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	setSessionCookie(w, token, sess)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *server) signupPage(w http.ResponseWriter, r *http.Request) {
	if s.alreadyLoggedIn(w, r) {
		return
	}
	s.render(w, r, http.StatusOK, part{"signup", signupView{}})
}

func (s *server) signup(w http.ResponseWriter, r *http.Request) {
	username := r.PostFormValue("username")
	u, err := s.accounts.SignUp(r.Context(), username, r.PostFormValue("password"))
	var rule *auth.RuleError
	switch {
	case errors.Is(err, auth.ErrUsernameTaken):
		s.render(w, r, http.StatusUnprocessableEntity, part{"signup", signupView{Username: username, UsernameError: "This username is taken."}})
		return
	case errors.As(err, &rule):
		view := signupView{Username: username}
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

func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	if err := s.accounts.LogOut(r.Context(), sessionToken(r)); err != nil {
		s.serverError(w, r, err)
		return
	}
	clearSessionCookie(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
```

- [ ] **Step 11: `app.js`** — stop polling after a 401. Add `let claimStopped = false;` under `const NETWORK_ERROR = …`; change the first condition of `claimDue` to:

```js
  if (claimStopped || !canNotify || Notification.permission !== "granted") {
```

and before `if (!res.ok) {` add:

```js
  if (res.status === 401) {
    // Not logged in any more: stop asking until the page reloads.
    claimStopped = true;
    return;
  }
```

- [ ] **Step 12: `app.css`** — append:

```css
/* Header with the user */
.top { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
.user-nav { font-size: 14px; color: var(--muted); display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
.user-nav strong { color: var(--ink); }
.inline-form { display: inline; margin: 0; }
.link-button { font: inherit; color: var(--accent); background: none; border: none; padding: 0; cursor: pointer; text-decoration: underline; }

/* Log in, sign up, account */
.auth-wrap { max-width: 400px; margin: 0 auto; padding: 40px 16px; display: flex; flex-direction: column; gap: 20px; }
.auth-brand { margin: 0; text-align: center; font-size: 30px; font-weight: 700; }
.auth-card { background: var(--card); border: 1px solid var(--line); border-radius: 12px; padding: 24px; display: flex; flex-direction: column; gap: 16px; }
.auth-card h1 { margin: 0; font-size: 22px; }
.auth-form { display: flex; flex-direction: column; gap: 16px; }
.field { display: flex; flex-direction: column; gap: 6px; }
.field label { font-size: 14px; font-weight: 600; }
.field input {
  font: inherit; font-size: 16px; color: var(--ink); padding: 10px 12px;
  border: 2px solid var(--line); border-radius: 8px; background: var(--card); width: 100%;
}
.field input:focus { outline: none; border-color: var(--accent); }
.field input[aria-invalid="true"] { border-color: var(--error); }
.field .hint, .note { margin: 0; font-size: 13px; color: var(--muted); }
.note { background: var(--ground); border-radius: 8px; padding: 10px 12px; }
.field-error { font-size: 13px; color: var(--error); }
.password-wrap { position: relative; }
.password-wrap input { padding-right: 72px; }
.show-password {
  position: absolute; right: 6px; top: 50%; transform: translateY(-50%);
  font: inherit; font-size: 14px; font-weight: 600; color: var(--accent);
  background: none; border: none; padding: 8px; min-height: 36px; cursor: pointer;
}
.check-line { display: flex; align-items: center; gap: 10px; font-size: 15px; min-height: 32px; }
.check-line input { width: 20px; height: 20px; }
.btn-primary {
  font: inherit; font-size: 16px; font-weight: 600; color: #FFFFFF; background: var(--accent);
  border: none; border-radius: 8px; min-height: 48px; padding: 0 20px; cursor: pointer;
}
.btn-primary:hover { background: var(--accent-dark); }
.alert { background: #FCEDEC; color: var(--error); border: 1px solid #F2C4C0; border-radius: 8px; padding: 10px 12px; font-size: 14px; }
.alert.ok { background: #EAF5EC; color: #1E6B32; border-color: #BFE0C7; }
.auth-foot { margin: 0; text-align: center; font-size: 15px; color: var(--muted); }
```

- [ ] **Step 13: `cmd/todo/main.go`** — create the auth service, clean up sessions, pass it to `web.New` (add `"todo/internal/auth"` import):

```go
	accounts := auth.NewService(store)
	handler, err := web.New(todo.NewService(store), accounts, logger)
	if err != nil {
		return err
	}
```

and after `context.AfterFunc(ctx, stop)`:

```go
	go cleanSessions(ctx, accounts, logger)
```

with the new function:

```go
// cleanSessions deletes expired sessions now and then every hour.
func cleanSessions(ctx context.Context, accounts *auth.Service, logger *slog.Logger) {
	tick := time.NewTicker(time.Hour)
	defer tick.Stop()
	for {
		if _, err := accounts.CleanUp(ctx); err != nil && ctx.Err() == nil {
			logger.Error("clean up sessions", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
```

- [ ] **Step 14: Run all tests, vet, JS check**

Run: `go test -count=1 ./...`, then `go vet ./...`, then `node --check internal/web/static/app.js && node --check internal/web/static/auth.js`
Expected: all exit 0.

- [ ] **Step 15: Commit**

```bash
git add cmd internal/web
git commit -m "feat: log in, sign up, log out; items per user"
```

---

### Task 4: Account page — change password and delete account

**Files:**
- Modify: `internal/web/server.go` (routes), `internal/web/auth_handlers.go`, `internal/web/templates/auth.html`, `internal/web/static/app.css`
- Test: `internal/web/account_test.go` (new)

**Interfaces:**
- Consumes: `auth.Service.ChangePassword`, `DeleteAccount`; `protect`, `currentUser`, `sessionToken`, `clearSessionCookie` (Task 3); test helpers (Task 3).
- Produces: routes `GET /account`, `POST /account/password`, `POST /account/delete`; template `account`.

- [ ] **Step 1: Write the failing tests** — `internal/web/account_test.go`:

```go
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
```

- [ ] **Step 2: Run to see them fail**

Run: `go test ./internal/web/ -run 'Account|DeletedAccount|ChangePassword'`
Expected: FAIL (`/account` returns 404 after the session check).

- [ ] **Step 3: Routes** — in `server.go`, after the logout route:

```go
	mux.Handle("GET /account", s.protect(s.accountPage))
	mux.Handle("POST /account/password", s.protect(s.changePassword))
	mux.Handle("POST /account/delete", s.protect(s.deleteAccount))
```

- [ ] **Step 4: Handlers** — append to `auth_handlers.go`:

```go
type accountView struct {
	Username      string
	MemberSince   string
	PasswordError string
	PasswordOK    bool
	DeleteError   string
}

func (s *server) accountView(r *http.Request) accountView {
	u := currentUser(r)
	return accountView{Username: u.Username, MemberSince: u.CreatedAt.In(s.now().Location()).Format("2 Jan 2006")}
}

func (s *server) accountPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusOK, part{"account", s.accountView(r)})
}

func (s *server) changePassword(w http.ResponseWriter, r *http.Request) {
	view := s.accountView(r)
	err := s.accounts.ChangePassword(r.Context(), userID(r), sessionToken(r),
		r.PostFormValue("current_password"), r.PostFormValue("new_password"))
	var rule *auth.RuleError
	switch {
	case errors.Is(err, auth.ErrWrongPassword):
		view.PasswordError = "The current password is wrong."
	case errors.As(err, &rule):
		view.PasswordError = rule.Msg
	case err != nil:
		s.serverError(w, r, err)
		return
	default:
		view.PasswordOK = true
		s.render(w, r, http.StatusOK, part{"account", view})
		return
	}
	s.render(w, r, http.StatusUnprocessableEntity, part{"account", view})
}

func (s *server) deleteAccount(w http.ResponseWriter, r *http.Request) {
	err := s.accounts.DeleteAccount(r.Context(), userID(r), r.PostFormValue("confirm_username"))
	if errors.Is(err, auth.ErrConfirmMismatch) {
		view := s.accountView(r)
		view.DeleteError = "The username does not match."
		s.render(w, r, http.StatusUnprocessableEntity, part{"account", view})
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	clearSessionCookie(w)
	http.Redirect(w, r, "/login?deleted=1", http.StatusSeeOther)
}
```

- [ ] **Step 5: Template** — append to `auth.html`:

```html
{{define "account"}}{{template "auth-head" "Account"}}
<main class="account-wrap">
<div class="account-top"><a href="/">← Back to my list</a> <form method="post" action="/logout" class="inline-form"><button type="submit" class="link-button">Log out</button></form></div>
<h1>Account</h1>
<p class="account-who">Signed in as <strong>{{.Username}}</strong> · member since {{.MemberSince}}</p>
<section class="auth-card" aria-labelledby="pw-title">
<h2 id="pw-title">Change password</h2>
{{if .PasswordOK}}<div class="alert ok" role="status">Password changed. You were logged out on your other devices.</div>{{end}}
{{if .PasswordError}}<div class="alert" role="alert">{{.PasswordError}}</div>{{end}}
<form method="post" action="/account/password" class="auth-form">
<div class="field"><label for="current-password">Current password</label>
<input id="current-password" name="current_password" type="password" autocomplete="current-password" required></div>
<div class="field"><label for="new-password">New password</label>
<div class="password-wrap"><input id="new-password" name="new_password" type="password" autocomplete="new-password" required>
<button type="button" class="show-password" data-target="new-password" aria-pressed="false">Show</button></div>
<span class="hint">At least 10 characters</span></div>
<div><button type="submit" class="btn-primary">Change password</button></div>
</form>
</section>
<section class="auth-card danger-zone" aria-labelledby="del-title">
<h2 id="del-title">Delete account</h2>
<p>This deletes your account and all your items. It cannot be undone.</p>
{{if .DeleteError}}<div class="alert" role="alert">{{.DeleteError}}</div>{{end}}
<form method="post" action="/account/delete" class="auth-form">
<div class="field"><label for="confirm-username">Type your username to confirm</label>
<input id="confirm-username" name="confirm_username" autocomplete="off" required></div>
<div><button type="submit" class="btn-danger">Delete my account and all items</button></div>
</form>
</section>
</main>
</body>
</html>
{{end}}
```

- [ ] **Step 6: CSS** — append to `app.css`:

```css
/* Account page */
.account-wrap { max-width: 560px; margin: 0 auto; padding: 24px 16px 40px; display: flex; flex-direction: column; gap: 20px; }
.account-wrap h1 { margin: 0; font-size: 28px; }
.account-wrap h2 { margin: 0; font-size: 20px; }
.account-top { display: flex; align-items: center; justify-content: space-between; font-size: 15px; }
.account-who { margin: 0; font-size: 15px; color: var(--muted); }
.account-who strong { color: var(--ink); }
.danger-zone { border: 2px solid #F2C4C0; }
.danger-zone h2 { color: var(--error); }
.danger-zone p { margin: 0; font-size: 15px; }
.btn-danger {
  font: inherit; font-size: 15px; font-weight: 600; color: #FFFFFF; background: var(--error);
  border: none; border-radius: 8px; min-height: 44px; padding: 0 16px; cursor: pointer;
}
```

- [ ] **Step 7: Run all tests and vet**

Run: `go test -count=1 ./...` then `go vet ./...`
Expected: both exit 0.

- [ ] **Step 8: Commit**

```bash
git add internal/web
git commit -m "feat: account page with password change and account deletion"
```

---

### Task 5: Security headers, CSP, and htmx without eval

**Files:**
- Modify: `internal/web/server.go`, `internal/web/templates/page.html`, `internal/web/templates/item.html`, `internal/web/static/app.js`, tests `internal/web/edit_test.go`, `internal/web/due_inputs_test.go`, `internal/web/focus_typing_test.go`, `internal/web/review_fixes_test.go`
- Test: `internal/web/security_test.go` (new)

**Interfaces:**
- Consumes: `New` from Task 3.
- Produces: `securityHeaders(next http.Handler) http.Handler`; custom events `save-edit` and `cancel-edit` fired only by app.js.

- [ ] **Step 1: Update the existing tests that expect the old triggers and `hx-on`**

```bash
perl -pi -e "s/hx-trigger=\"keydown\[key=='Enter'\], save-edit\"/hx-trigger=\"save-edit\"/g" internal/web/edit_test.go internal/web/due_inputs_test.go
perl -pi -e "s/hx-trigger=\"keyup\[key=='Escape'\], cancel-edit\"/hx-trigger=\"cancel-edit\"/" internal/web/focus_typing_test.go
```

In `internal/web/review_fixes_test.go`, replace the whole functions `TestEditSavesOnKeydownEnter` and `TestPageShowsErrors` with:

```go
// Enter that opens edit mode must not also save it: app.js saves on keydown.
func TestEditSavesOnKeydownEnter(t *testing.T) {
	h, _ := newTestApp(t)
	js := do(t, h, "GET", "/static/app.js", nil, nil).Body.String()
	assertContains(t, js, `e.key === "Enter"`, `"save-edit"`, `e.key === "Escape"`, `"cancel-edit"`)
}

// Server and network errors show a short message to the user.
func TestPageShowsErrors(t *testing.T) {
	h, _ := newTestApp(t)
	body := do(t, h, "GET", "/", nil, nil).Body.String()
	assertContains(t, body,
		`<p id="error" class="error" role="alert"></p>`,
		`{"code":"5..","swap":true,"error":true,"target":"#error","swapOverride":"innerHTML"}`)
	js := do(t, h, "GET", "/static/app.js", nil, nil).Body.String()
	assertContains(t, js, `"htmx:sendError"`, "Cannot reach the server.", `"htmx:afterRequest"`)
}
```

- [ ] **Step 2: Write the failing tests** — `internal/web/security_test.go`:

```go
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
```

- [ ] **Step 3: Run to see them fail**

Run: `go test ./internal/web/`
Expected: FAIL in `TestSecurityHeaders`, `TestNoEvalInPages`, `TestEditSavesOnKeydownEnter`, `TestPageShowsErrors`, and the trigger tests.

- [ ] **Step 4: Headers** — in `server.go` add:

```go
const contentSecurityPolicy = "default-src 'self'; style-src 'self' https://fonts.googleapis.com; " +
	"font-src https://fonts.gstatic.com; img-src 'self' data:; frame-ancestors 'none'; base-uri 'none'; form-action 'self'"

// securityHeaders adds the CSP and other protective headers to every response.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", contentSecurityPolicy)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}
```

and change the last line of `New` to:

```go
	return securityHeaders(http.NewCrossOriginProtection().Handler(mux)), nil
```

- [ ] **Step 5: `page.html`** — in the `htmx-config` JSON, insert `"allowEval":false,"includeIndicatorStyles":false,` right after `{"historyCacheSize":0,`; replace the `<body …>` tag (with its two `hx-on` attributes) by `<body>`.

- [ ] **Step 6: `item.html`** — in `item-edit`, change `hx-trigger="keyup[key=='Escape'], cancel-edit"` to `hx-trigger="cancel-edit"` and `hx-trigger="keydown[key=='Enter'], save-edit"` to `hx-trigger="save-edit"`.

- [ ] **Step 7: `app.js`** — append:

```js
// 7. Enter saves and Escape cancels an edit row. (These were htmx trigger
// filters, which need eval; the CSP does not allow eval.)
document.addEventListener("keydown", (e) => {
  const row = e.target.closest && e.target.closest("li.editing");
  if (!row) {
    return;
  }
  if (e.key === "Enter") {
    e.preventDefault();
    htmx.trigger(row.querySelector(".edit"), "save-edit");
  } else if (e.key === "Escape") {
    e.preventDefault();
    htmx.trigger(row, "cancel-edit");
  }
});

// 8. The error line under the title: show network errors, clear it after a
// successful request. (These were hx-on attributes.)
document.addEventListener("htmx:sendError", () => {
  const line = document.getElementById("error");
  if (line) {
    line.textContent = NETWORK_ERROR;
  }
});

document.addEventListener("htmx:afterRequest", (e) => {
  const line = document.getElementById("error");
  if (line && e.detail.successful) {
    line.textContent = "";
  }
});
```

(The existing capture-phase listener for Enter on `.clear-due` stops that event before it reaches this listener, so "Clear" still only clears.)

- [ ] **Step 8: Run all tests, vet, JS check**

Run: `go test -count=1 ./...`, `go vet ./...`, `node --check internal/web/static/app.js`
Expected: all exit 0.

- [ ] **Step 9: Commit**

```bash
git add internal/web
git commit -m "feat: security headers and CSP; htmx without eval"
```

---

## Final check (after Task 5)

- Whole-branch review by a fresh reviewer (per the chosen execution method).
- Browser check by the user (`go build ./cmd/todo && ./todo -addr 127.0.0.1:8812 -db /tmp/todo-users.db`):
  1. `/` redirects to the log-in page. Sign up "alice" with "Show" used once; you land on your empty list with "Signed in as alice".
  2. Add items, edit with Enter/Escape (still works without eval), postpone, notifications — all as before.
  3. Log out; log in with "ALICE" and "Keep me logged in"; close and reopen the browser → still logged in.
  4. In a private window, sign up "bob": bob sees none of alice's items.
  5. Account page: wrong current password → message; change password → the other browser is logged out.
  6. Delete bob's account with "bob" typed → log-in page says "Your account was deleted."
  7. Your real database: start the app on it without the flag → it refuses with the item count; only `-delete-old-items` deletes them.
  8. Browser console shows no CSP errors on all pages.
- Then ask: merge `users` into `main` locally, or keep the branch. Plan 2 (spam checks, login limits, `-trust-proxy`, admin command line) follows.

## Spec coverage

| Spec section | Task |
|---|---|
| 3 Data, version 2, start rule | 1 |
| 4 Accounts, argon2id | 2 |
| 5 Sessions, middleware, 401/303 | 2, 3 |
| 6 Cross-site, headers, CSP, htmx without eval | 3 (CrossOriginProtection), 5 |
| 6 Sign-up spam checks, login limits, `-trust-proxy` | Plan 2 |
| 7 Pages and routes | 3, 4 |
| 8 Admin command line | Plan 2 |
| 9 Edge cases | 2 (case, expiry), 3 (logout in other tab via 303/401), 4 (deleted user) |
| 10 Testing | every task; browser check in Final check |
