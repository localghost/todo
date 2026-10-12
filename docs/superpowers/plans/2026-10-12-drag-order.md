# Drag-and-drop order Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The user can drag items (or move them with Arrow Up and Arrow Down) to set their own order. The server saves the order at once.

**Architecture:** A new store method renumbers the user's `position` values in one transaction. A service method blocks moves of overdue items. A new route `POST /items/{id}/move` answers `204` on success. In the browser, a vendored SortableJS file handles mouse and touch. A new script `order.js` sends the move requests in order and handles the keyboard.

**Tech Stack:** Go 1.27, `net/http`, `html/template`, SQLite (`modernc.org/sqlite`), htmx 2.0.11, SortableJS 1.15.6 (vendored), Node (only for the script test, which skips without Node).

**Spec:** `docs/superpowers/specs/2026-10-12-drag-order-design.md`

**Worktree:** `/home/kostrzewa/claude-personal/todo-worktrees/drag-order`, branch `drag-order`. Run every command from this directory.

**Execution method (chosen by the user):** Native. Use superpowers:executing-plans. One fresh reviewer checks the whole branch at the end.

**Context:** The `items` table already has a `position` column, and `List` sorts by `position, id`. New items get `max(position) + 1`. The first spec (`2026-10-01-todo-app-design.md`, section 10, item 3) planned this feature with SortableJS. Overdue items are sorted to the top in the web layer by `todo.OverdueFirst`, and they must stay there.

## Global Constraints

- Go 1.27. No new Go modules.
- No schema change. `currentVersion` in `internal/store/sqlite/sqlite.go` stays `3`.
- The Content Security Policy (`contentSecurityPolicy` in `internal/web/server.go`) does not change. All scripts are served from `/static/`.
- Overdue items stay pinned at the top. They cannot move, and nothing can be dropped between them.
- Only the grip button (`class="grip"`) starts a drag. Its label is `Move: <item text>`.
- The screen reader text after a keyboard move is `Moved to N of M`. N and M count the normal rows (rows that are not overdue).
- New items still go to the end of the list.
- UI text and docs follow the user's text rules: CEFR B2 English, short sentences.
- Commit messages follow the repo style (`feat: …`, `test: …`, `docs: …`) and end with the line `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- The commit steps in this plan run because the user approves this plan. Do not push, and do not open a PR.

## Review Focus

1. The session ends, then the user drags a row: the server answers `401`. The page must reload (and so show the log-in page), not stay with a wrong order. Test: Task 3, `TestMoveNeedsLogin`.
2. "Hide done" is on, the user moves an item, then shows the done items again: each done item must stay next to the rows it was next to before. Test: Task 3, `TestMoveWithHideDoneKeepsHiddenItemsInPlace`.
3. Fast Arrow presses on one grip: the server must get the moves in the same order as the page. Check: Task 5, manual step 3 (the queue in `order.js` has no automatic test).
4. SortableJS sets styles from script while it drags. The CSP must not block this. Check: Task 5, manual step 1 (no CSP error in the browser console).
5. On a phone, a swipe on the item text must still scroll the page and must not start a drag. Check: Task 5, manual step 2.

---

### Task 1: Store method `Move`

**Files:**
- Modify: `internal/todo/todo.go` (the `Store` interface)
- Modify: `internal/store/sqlite/sqlite.go` (imports, new methods after `UpdateItem`)
- Create: `internal/store/sqlite/move_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces: `todo.Store.Move(ctx context.Context, userID, id, afterID int64) error` and `(*sqlite.Store).Move` with the same signature. `afterID` 0 means the first place. It returns `todo.ErrNotFound` if `id`, or a non-zero `afterID`, is not an item of the user.

- [ ] **Step 1: Write the failing tests**

Create `internal/store/sqlite/move_test.go`:

```go
package sqlite_test

import (
	"context"
	"errors"
	"testing"

	"todo/internal/store/sqlite"
	"todo/internal/todo"
)

// order returns the texts of user 1's items in list order.
func order(t *testing.T, s *sqlite.Store, hideDone bool) string {
	t.Helper()
	items, err := s.List(context.Background(), 1, hideDone)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	return texts(items)
}

func TestMovePlacesItem(t *testing.T) {
	ctx := context.Background()
	for _, c := range []struct {
		name, move, after, want string
	}{
		{"first", "c", "", "c,a,b"},
		{"middle", "a", "b", "b,a,c"},
		{"end", "a", "c", "b,c,a"},
		{"after itself", "b", "b", "a,b,c"},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := newStore(t)
			ids := map[string]int64{"": 0}
			for _, text := range []string{"a", "b", "c"} {
				ids[text] = mustCreate(t, s, text).ID
			}
			if err := s.Move(ctx, 1, ids[c.move], ids[c.after]); err != nil {
				t.Fatalf("Move: %v", err)
			}
			if got := order(t, s, false); got != c.want {
				t.Fatalf("order = %s, want %s", got, c.want)
			}
		})
	}
}

func TestMoveUnknownItems(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	mustCreate(t, s, "a")
	b := mustCreate(t, s, "b")
	if _, err := s.CreateUser(ctx, "bob", "x", t0); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	bobs, err := s.Create(ctx, 2, todo.Change{Text: "bob's"}, t0)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	for _, c := range []struct {
		name      string
		id, after int64
	}{
		{"unknown id", 999, 0},
		{"unknown after", b.ID, 999},
		{"other user's id", bobs.ID, 0},
		{"other user's after", b.ID, bobs.ID},
	} {
		if err := s.Move(ctx, 1, c.id, c.after); !errors.Is(err, todo.ErrNotFound) {
			t.Errorf("%s: err = %v, want ErrNotFound", c.name, err)
		}
	}
	if got := order(t, s, false); got != "a,b" {
		t.Fatalf("order = %s, want a,b unchanged", got)
	}
}

func TestMoveKeepsHiddenDoneItemsInPlace(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	a := mustCreate(t, s, "a")
	done := mustCreate(t, s, "done")
	mustCreate(t, s, "b")
	c := mustCreate(t, s, "c")
	if _, err := s.SetDone(ctx, 1, done.ID, true, t0); err != nil {
		t.Fatalf("SetDone: %v", err)
	}
	// The page hides "done" and shows a, b, c. The user drags c after a.
	if err := s.Move(ctx, 1, c.ID, a.ID); err != nil {
		t.Fatalf("Move: %v", err)
	}
	if got := order(t, s, true); got != "a,c,b" {
		t.Fatalf("visible order = %s, want a,c,b", got)
	}
	if got := order(t, s, false); got != "a,c,done,b" {
		t.Fatalf("full order = %s, want a,c,done,b", got)
	}
}

func TestCreateAfterMoveGoesToEnd(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	a := mustCreate(t, s, "a")
	mustCreate(t, s, "b")
	if err := s.Move(ctx, 1, a.ID, 0); err != nil {
		t.Fatalf("Move: %v", err)
	}
	mustCreate(t, s, "new")
	if got := order(t, s, false); got != "a,b,new" {
		t.Fatalf("order = %s, want a,b,new", got)
	}
}
```

Note: `texts`, `newStore`, `mustCreate`, and `t0` already exist in `internal/store/sqlite/sqlite_test.go`. `newStore` creates user 1 ("alice").

- [ ] **Step 2: Run the tests to check that they fail**

Run: `go test ./internal/store/sqlite/ -run 'TestMove|TestCreateAfterMove'`
Expected: FAIL to compile with `s.Move undefined`.

- [ ] **Step 3: Add the method to the interface**

In `internal/todo/todo.go`, add to the `Store` interface, after `Delete`:

```go
	Delete(ctx context.Context, userID, id int64) error
	// Move puts item id right after item afterID in the user's list, or
	// first if afterID is 0.
	Move(ctx context.Context, userID, id, afterID int64) error
	Counts(ctx context.Context, userID int64) (open, done int, err error)
```

- [ ] **Step 4: Write the implementation**

In `internal/store/sqlite/sqlite.go`, add `"slices"` to the imports (between `"path/filepath"` and `"strings"`). Add these functions right after `UpdateItem`:

```go
// Move puts item id right after item afterID (0: first) and writes the
// positions 1, 2, 3, … for all items of the user, in the new order.
func (s *Store) Move(ctx context.Context, userID, id, afterID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("move item: %w", err)
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT id FROM items WHERE user_id = ? ORDER BY position, id`, userID)
	if err != nil {
		return fmt.Errorf("move item: %w", err)
	}
	var ids []int64
	for rows.Next() {
		var x int64
		if err := rows.Scan(&x); err != nil {
			rows.Close()
			return fmt.Errorf("move item: %w", err)
		}
		ids = append(ids, x)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("move item: %w", err)
	}
	moved, ok := moveAfter(ids, id, afterID)
	if !ok {
		return todo.ErrNotFound
	}
	for i, x := range moved {
		if _, err := tx.ExecContext(ctx, `UPDATE items SET position = ? WHERE id = ?`, i+1, x); err != nil {
			return fmt.Errorf("move item: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("move item: %w", err)
	}
	return nil
}

// moveAfter returns ids with id moved right after afterID (0: first). It
// returns false if id, or a non-zero afterID, is not in ids.
func moveAfter(ids []int64, id, afterID int64) ([]int64, bool) {
	if !slices.Contains(ids, id) || (afterID != 0 && !slices.Contains(ids, afterID)) {
		return nil, false
	}
	if afterID == id {
		return ids, true
	}
	rest := slices.DeleteFunc(slices.Clone(ids), func(x int64) bool { return x == id })
	at := 0
	if afterID != 0 {
		at = slices.Index(rest, afterID) + 1
	}
	return slices.Insert(rest, at, id), true
}
```

The method does not change `updated_at`, because a move does not change the item.

- [ ] **Step 5: Run the tests to check that they pass**

Run: `go test ./internal/store/sqlite/ -run 'TestMove|TestCreateAfterMove' -v`
Expected: PASS for all four tests.

Run: `go build ./... && go vet ./...`
Expected: no output. (The only `todo.Store` implementation is `*sqlite.Store`. No test fakes exist.)

- [ ] **Step 6: Commit**

```bash
git add internal/todo/todo.go internal/store/sqlite/sqlite.go internal/store/sqlite/move_test.go
git commit -m "feat: the store moves an item after another item

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Service method `Move` and `ErrCannotMove`

**Files:**
- Modify: `internal/todo/todo.go` (the error `var` block)
- Modify: `internal/todo/service.go` (new method after `Delete`)
- Create: `internal/todo/move_test.go`

**Interfaces:**
- Consumes: `todo.Store.Move(ctx, userID, id, afterID int64) error` from Task 1.
- Produces: `(*todo.Service).Move(ctx context.Context, userID, id, afterID int64, now time.Time) error` and `todo.ErrCannotMove`. `now` carries the browser's zone.

- [ ] **Step 1: Write the failing tests**

Create `internal/todo/move_test.go`:

```go
package todo_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"todo/internal/todo"
)

// listOrder returns the texts of user 1's items in stored order.
func listOrder(t *testing.T, svc *todo.Service) []string {
	t.Helper()
	items, err := svc.List(context.Background(), 1, false)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	out := []string{}
	for _, it := range items {
		out = append(out, it.Text)
	}
	return out
}

func TestMoveChangesOrder(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	svc.Add(ctx, 1, "a", "", "")
	b, _ := svc.Add(ctx, 1, "b", "", "")
	if err := svc.Move(ctx, 1, b.ID, 0, time.Now()); err != nil {
		t.Fatalf("Move: %v", err)
	}
	if got := listOrder(t, svc); len(got) != 2 || got[0] != "b" || got[1] != "a" {
		t.Fatalf("order = %v, want [b a]", got)
	}
}

func TestMoveOverdueItemFails(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	svc.Add(ctx, 1, "a", "", "")
	late, _ := svc.Add(ctx, 1, "late", "2020-01-01", "08:00")
	if err := svc.Move(ctx, 1, late.ID, 0, time.Now()); !errors.Is(err, todo.ErrCannotMove) {
		t.Fatalf("Move(overdue) err = %v, want ErrCannotMove", err)
	}
	if got := listOrder(t, svc); len(got) != 2 || got[0] != "a" || got[1] != "late" {
		t.Fatalf("order = %v, want [a late] unchanged", got)
	}
}

func TestMoveAllDayUsesZoneOfNow(t *testing.T) {
	warsaw, err := time.LoadLocation("Europe/Warsaw")
	if err != nil {
		t.Fatal(err)
	}
	svc := newService(t)
	ctx := context.Background()
	svc.Add(ctx, 1, "a", "", "")
	rent, err := svc.AddIn(ctx, 1, "rent", "2026-10-01", "", warsaw)
	if err != nil {
		t.Fatal(err)
	}
	// 23:30 on the due day in Warsaw: not overdue yet, so it can move.
	if err := svc.Move(ctx, 1, rent.ID, 0, time.Date(2026, 10, 1, 23, 30, 0, 0, warsaw)); err != nil {
		t.Fatalf("Move on the due day: %v", err)
	}
	// 00:30 on the next day in Warsaw: overdue, so it cannot move.
	if err := svc.Move(ctx, 1, rent.ID, 0, time.Date(2026, 10, 2, 0, 30, 0, 0, warsaw)); !errors.Is(err, todo.ErrCannotMove) {
		t.Fatalf("Move on the next day err = %v, want ErrCannotMove", err)
	}
}

func TestMoveUnknownItem(t *testing.T) {
	svc := newService(t)
	if err := svc.Move(context.Background(), 1, 999, 0, time.Now()); !errors.Is(err, todo.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
```

Note: `newService` exists in `internal/todo/service_test.go`. It creates user 1 with a real SQLite store.

- [ ] **Step 2: Run the tests to check that they fail**

Run: `go test ./internal/todo/ -run TestMove`
Expected: FAIL to compile with `svc.Move undefined` and `undefined: todo.ErrCannotMove`.

- [ ] **Step 3: Write the implementation**

In `internal/todo/todo.go`, add to the `var` block:

```go
var (
	// ErrEmptyText means the text is empty after whitespace is removed.
	ErrEmptyText = errors.New("todo: text is empty")
	// ErrNotFound means the item does not exist or belongs to another user.
	ErrNotFound = errors.New("todo: item not found")
	// ErrCannotMove means the item is overdue; overdue items stay at the top.
	ErrCannotMove = errors.New("todo: an overdue item cannot move")
)
```

In `internal/todo/service.go`, add after `Delete`:

```go
// Move puts item id after item afterID (0: first). now carries the browser's
// zone, because the overdue rule uses it for all-day items.
func (s *Service) Move(ctx context.Context, userID, id, afterID int64, now time.Time) error {
	it, err := s.store.Get(ctx, userID, id)
	if err != nil {
		return err
	}
	if it.Overdue(now) {
		return ErrCannotMove
	}
	return s.store.Move(ctx, userID, id, afterID)
}
```

- [ ] **Step 4: Run the tests to check that they pass**

Run: `go test ./internal/todo/ -v -run TestMove`
Expected: PASS for all four tests.

- [ ] **Step 5: Commit**

```bash
git add internal/todo/todo.go internal/todo/service.go internal/todo/move_test.go
git commit -m "feat: the service moves items; overdue items cannot move

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Route, handler, and templates

**Files:**
- Modify: `internal/web/server.go` (one route after `POST /items/{id}/postpone`)
- Modify: `internal/web/handlers.go` (new `moveItem`, `parseAfter`; `renderList` gets a status variant)
- Modify: `internal/web/templates/item.html` (grip, `overdue-row`, grip space in the edit row)
- Modify: `internal/web/templates/list.html` (`#move-status`)
- Modify: `internal/web/static/app.css` (grip style)
- Create: `internal/web/move_test.go`

**Interfaces:**
- Consumes: `(*todo.Service).Move(ctx, userID, id, afterID int64, now time.Time) error`, `todo.ErrCannotMove`, `todo.ErrNotFound`.
- Produces for Task 4:
  - `POST /items/{id}/move`, form field `after` (an item id, or empty for the first place). Answers: `204` (saved), `404` or `422` (whole list, `HX-Retarget: #list-section`, `HX-Reswap: outerHTML`), `401` (no session), `500`.
  - Each normal row: `<li id="item-N" class="item…">` with `<button class="grip" type="button" aria-label="Move: TEXT" …>` as its first child.
  - Each overdue row: class `overdue-row` on the `<li>`, and `<span class="grip-space" aria-hidden="true"></span>` instead of the grip.
  - The edit row: `<span class="grip-space" aria-hidden="true"></span>`, no grip.
  - `<p id="move-status" class="visually-hidden" aria-live="polite"></p>` inside `#list-section`.

- [ ] **Step 1: Write the failing tests**

Create `internal/web/move_test.go`:

```go
package web_test

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func moveForm(after int64) url.Values {
	if after == 0 {
		return url.Values{"after": {""}}
	}
	return url.Values{"after": {strconv.FormatInt(after, 10)}}
}

func moveURL(id int64) string { return "/items/" + strconv.FormatInt(id, 10) + "/move" }

// svcOrder returns the texts of the tester's items in stored order.
func svcOrder(t *testing.T, env *testEnv) string {
	t.Helper()
	items, err := env.Svc.List(context.Background(), env.User.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, it := range items {
		out = append(out, it.Text)
	}
	return strings.Join(out, ",")
}

func TestMoveSavesOrder(t *testing.T) {
	env := newTestEnv(t)
	a := mustAdd(t, env.Svc, "a")
	mustAdd(t, env.Svc, "b")
	c := mustAdd(t, env.Svc, "c")

	rec := do(t, env.H, "POST", moveURL(c.ID), moveForm(0), htmxHeaders)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("status = %d, body %q; want 204 and no body", rec.Code, rec.Body.String())
	}
	if got := svcOrder(t, env); got != "c,a,b" {
		t.Fatalf("order = %s, want c,a,b", got)
	}
	do(t, env.H, "POST", moveURL(c.ID), moveForm(a.ID), htmxHeaders)
	if got := svcOrder(t, env); got != "a,c,b" {
		t.Fatalf("order = %s, want a,c,b", got)
	}
}

func TestMoveNotFoundAnswersWithList(t *testing.T) {
	env := newTestEnv(t)
	a := mustAdd(t, env.Svc, "a")
	mustAdd(t, env.Svc, "b")
	other, err := env.Store.CreateUser(context.Background(), "other", "x", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	theirs, err := env.Svc.Add(context.Background(), other.ID, "theirs", "", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name, target string
		form         url.Values
	}{
		{"unknown id", moveURL(999), moveForm(0)},
		{"bad id", "/items/x/move", moveForm(0)},
		{"bad after", moveURL(a.ID), url.Values{"after": {"x"}}},
		{"unknown after", moveURL(a.ID), moveForm(999)},
		{"other user's id", moveURL(theirs.ID), moveForm(0)},
		{"other user's after", moveURL(a.ID), moveForm(theirs.ID)},
	} {
		rec := do(t, env.H, "POST", c.target, c.form, htmxHeaders)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", c.name, rec.Code)
			continue
		}
		if rec.Header().Get("HX-Retarget") != "#list-section" || rec.Header().Get("HX-Reswap") != "outerHTML" {
			t.Errorf("%s: headers = %v, want the list retarget", c.name, rec.Header())
		}
		assertContains(t, rec.Body.String(), `id="list-section"`)
	}
	if got := svcOrder(t, env); got != "a,b" {
		t.Fatalf("order = %s, want a,b unchanged", got)
	}
}

func TestMoveOverdueAnswers422WithList(t *testing.T) {
	env, ids := overdueEnv(t)
	rec := do(t, env.H, "POST", moveURL(ids["Call the dentist"]), moveForm(0), warsawHTMX())
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	if rec.Header().Get("HX-Retarget") != "#list-section" {
		t.Fatalf("HX-Retarget = %q, want #list-section", rec.Header().Get("HX-Retarget"))
	}
	assertOrder(t, rec.Body.String(), "Pay the rent", "Call the dentist", "Return library books", "Water the plants")
}

func TestMoveNeedsLogin(t *testing.T) {
	env := newTestEnv(t)
	a := mustAdd(t, env.Svc, "a")
	rec := do(t, env.H, "POST", moveURL(a.ID), moveForm(0), map[string]string{"HX-Request": "true", anonHeader: "1"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestMoveWithHideDoneKeepsHiddenItemsInPlace(t *testing.T) {
	env := newTestEnv(t)
	// Long texts, so assertOrder (">TEXT<") cannot match other page text.
	a := mustAdd(t, env.Svc, "Alpha task")
	done := mustAdd(t, env.Svc, "Finished task")
	mustAdd(t, env.Svc, "Beta task")
	c := mustAdd(t, env.Svc, "Gamma task")
	do(t, env.H, "POST", "/items/"+strconv.FormatInt(done.ID, 10)+"/toggle", nil, htmxHeaders)
	do(t, env.H, "POST", "/settings/hide-done", url.Values{"hide_done": {"1"}}, htmxHeaders)

	// The page shows Alpha, Beta, Gamma. The user drags Gamma after Alpha.
	if rec := do(t, env.H, "POST", moveURL(c.ID), moveForm(a.ID), htmxHeaders); rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	do(t, env.H, "POST", "/settings/hide-done", url.Values{"hide_done": {"0"}}, htmxHeaders)
	body := do(t, env.H, "GET", "/", nil, nil).Body.String()
	assertOrder(t, body, "Alpha task", "Gamma task", "Finished task", "Beta task")
}

func TestGripOnNormalRowsOnly(t *testing.T) {
	env, ids := overdueEnv(t)
	do(t, env.H, "POST", "/items/"+strconv.FormatInt(ids["Return library books"], 10)+"/toggle", nil, warsawHTMX())
	body := do(t, env.H, "GET", "/", nil, map[string]string{"Cookie": "todo_tz=Europe/Warsaw"}).Body.String()

	// A done row and an open row that is not overdue have a grip.
	assertContains(t, body, `aria-label="Move: Return library books"`, `aria-label="Move: Water the plants"`,
		`<p id="move-status" class="visually-hidden" aria-live="polite"></p>`)
	// Overdue rows have no grip, and they carry the class overdue-row.
	assertNotContains(t, body, `aria-label="Move: Call the dentist"`, `aria-label="Move: Pay the rent"`)
	assertContains(t, body, `id="item-`+strconv.FormatInt(ids["Call the dentist"], 10)+`" class="item overdue-row"`)

	edit := do(t, env.H, "GET", "/items/"+strconv.FormatInt(ids["Water the plants"], 10)+"/edit", nil, warsawHTMX()).Body.String()
	assertNotContains(t, edit, `class="grip"`)
	assertContains(t, edit, `class="grip-space"`)
}
```

Notes for the implementer:
- `overdueEnv`, `warsawHTMX`, and `assertOrder` exist in `internal/web/overdue_order_test.go`. `assertOrder` looks for `>TEXT<` in the body, so the item texts must stay inside their own tag.
- In `overdueEnv`, "Call the dentist" and "Pay the rent" are overdue. "Return library books" and "Water the plants" are not.
- `newTestEnv`, `mustAdd`, `do`, `htmxHeaders`, `anonHeader`, `assertContains`, and `assertNotContains` exist in `internal/web/web_test.go`.

- [ ] **Step 2: Run the tests to check that they fail**

Run: `go test ./internal/web/ -run 'TestMove|TestGrip'`
Expected: FAIL. The move requests get `405` (no such route), and the grip text is missing.

- [ ] **Step 3: Add the route**

In `internal/web/server.go`, after the postpone route:

```go
	mux.Handle("POST /items/{id}/postpone", s.protect(s.postponeItem))
	mux.Handle("POST /items/{id}/move", s.protect(s.moveItem))
```

- [ ] **Step 4: Add the handler**

In `internal/web/handlers.go`, change `renderList` so that a status can be passed. Replace the existing `renderList` with:

```go
// renderList answers with the whole list section instead of one row, for a
// change that moves an item into or out of the overdue group at the top.
func (s *server) renderList(w http.ResponseWriter, r *http.Request, hideDone bool) {
	s.renderListStatus(w, r, hideDone, http.StatusOK)
}

// renderListStatus is renderList with another status code.
func (s *server) renderListStatus(w http.ResponseWriter, r *http.Request, hideDone bool, status int) {
	lv, err := s.listView(r, hideDone)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	w.Header().Set("HX-Retarget", "#list-section")
	w.Header().Set("HX-Reswap", "outerHTML")
	s.render(w, r, status, part{"list", lv})
}
```

Add after `postponeItem`:

```go
// moveItem puts an item right after the item in the form field "after"
// (empty: first). The page has already moved the row, so success is 204.
// On an error the page gets the whole list, to show the true order.
func (s *server) moveItem(w http.ResponseWriter, r *http.Request) {
	hideDone := currentUser(r).HideDone
	id, ok := parseID(r)
	after, okAfter := parseAfter(r.FormValue("after"))
	if !ok || !okAfter {
		s.renderListStatus(w, r, hideDone, http.StatusNotFound)
		return
	}
	err := s.svc.Move(r.Context(), userID(r), id, after, s.nowFor(r))
	switch {
	case errors.Is(err, todo.ErrNotFound):
		s.renderListStatus(w, r, hideDone, http.StatusNotFound)
	case errors.Is(err, todo.ErrCannotMove):
		s.renderListStatus(w, r, hideDone, http.StatusUnprocessableEntity)
	case err != nil:
		s.serverError(w, r, err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// parseAfter reads the form field "after": empty means 0, the first place.
func parseAfter(v string) (int64, bool) {
	if v == "" {
		return 0, true
	}
	id, err := strconv.ParseInt(v, 10, 64)
	return id, err == nil && id > 0
}
```

`strconv` is already imported in `handlers.go`.

- [ ] **Step 5: Change the templates**

In `internal/web/templates/item.html`, add a `grip-icon` template after `x-icon`:

```html
{{define "grip-icon"}}<svg width="12" height="16" viewBox="0 0 12 16" fill="currentColor" aria-hidden="true"><circle cx="3" cy="3" r="1.5"/><circle cx="9" cy="3" r="1.5"/><circle cx="3" cy="8" r="1.5"/><circle cx="9" cy="8" r="1.5"/><circle cx="3" cy="13" r="1.5"/><circle cx="9" cy="13" r="1.5"/></svg>{{end}}
```

Change the start of the `item` template. `$d` must be known before the `<li>`, so the `{{$d := due .}}` line moves up:

```html
{{define "item"}}{{$d := due .}}<li id="item-{{.ID}}" class="item{{if .Done}} done{{end}}{{if $d.Overdue}} overdue-row{{end}}">
{{if $d.Overdue}}<span class="grip-space" aria-hidden="true"></span>{{else}}<button class="grip" type="button" aria-label="Move: {{.Text}}" title="Drag to move, or use the arrow keys">{{template "grip-icon"}}</button>{{end}}
<button id="check-{{.ID}}" class="check" type="button" aria-label="{{if .Done}}Mark not done{{else}}Mark done{{end}}: {{.Text}}"
```

Then remove the old `{{$d := due .}}` in front of the meta line. It becomes:

```html
{{if $d.Text}}<span class="meta"><span class="due{{if $d.Overdue}} overdue{{end}}">{{$d.Text}}</span></span>{{end}}
```

In the `item-edit` template, add the grip space as the first child of the `<li>`, before the disabled check button:

```html
  hx-get="/items/{{.Item.ID}}" hx-trigger="cancel-edit" hx-target="this" hx-swap="outerHTML" hx-sync="this:drop">
<span class="grip-space" aria-hidden="true"></span>
<button class="check" type="button" disabled aria-label="Mark done (finish editing first)">
```

In `internal/web/templates/list.html`, add the live region after the `<ul>`:

```html
<ul id="items">{{range .Items}}{{template "item" .}}{{end}}</ul>
<p id="move-status" class="visually-hidden" aria-live="polite"></p>
</section>{{end}}
```

- [ ] **Step 6: Add the grip style**

In `internal/web/static/app.css`, after the `.check, .delete { … }` rule:

```css
/* Drag order (spec 2026-10-12): only the grip starts a drag. touch-action
   lets a finger on the grip drag the row instead of scrolling the page. */
.grip, .grip-space { width: 24px; height: 44px; flex-shrink: 0; margin-top: -2px; }
.grip {
  display: flex; align-items: center; justify-content: center;
  background: none; border: none; padding: 0;
  color: var(--muted); cursor: grab; touch-action: none;
}
```

- [ ] **Step 7: Run the tests to check that they pass**

Run: `go test ./internal/web/ -run 'TestMove|TestGrip' -v`
Expected: PASS for all six tests.

Run: `go test ./...`
Expected: all PASS. Some existing tests check row HTML. If one fails only because of the new grip markup, update that test's expected HTML, and say so in the task report. Do not change the behavior to make an old test pass.

- [ ] **Step 8: Commit**

```bash
git add internal/web/server.go internal/web/handlers.go internal/web/templates/item.html internal/web/templates/list.html internal/web/static/app.css internal/web/move_test.go
git commit -m "feat: POST /items/{id}/move and a grip on each normal row

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: Browser: SortableJS and `order.js`

**Files:**
- Create: `internal/web/static/Sortable.min.js` (vendored)
- Create: `internal/web/static/order.js`
- Modify: `internal/web/templates/page.html` (two script tags)
- Modify: `internal/web/static/app.js` (section 10: no list refresh during a drag)
- Modify: `internal/web/static/app.css` (drag styles)
- Create: `internal/web/order_script_test.go`

**Interfaces:**
- Consumes: the route, the answers, and the markup from Task 3. `NETWORK_ERROR` (a top-level `const` in `app.js`, so `order.js` must load after `app.js`).
- Produces: the body class `dragging` while a drag runs (read by `app.js` section 10).

- [ ] **Step 1: Vendor SortableJS**

```bash
curl -fsSL -o internal/web/static/Sortable.min.js https://cdn.jsdelivr.net/npm/sortablejs@1.15.6/Sortable.min.js
curl -fsSL https://unpkg.com/sortablejs@1.15.6/Sortable.min.js | cmp - internal/web/static/Sortable.min.js && echo same
head -c 120 internal/web/static/Sortable.min.js; echo
sha256sum internal/web/static/Sortable.min.js
```

Expected: `same`, and the first line starts with `/*! Sortable 1.15.6 - MIT`. Write the SHA-256 into the commit message of this task.

- [ ] **Step 2: Write the failing tests**

Create `internal/web/order_script_test.go`:

```go
package web_test

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPageLoadsOrderScripts(t *testing.T) {
	env := newTestEnv(t)
	body := do(t, env.H, "GET", "/", nil, nil).Body.String()
	tags := []string{
		`<script src="/static/Sortable.min.js" defer></script>`,
		`<script src="/static/app.js" defer></script>`,
		`<script src="/static/order.js" defer></script>`,
	}
	last := -1
	for _, tag := range tags {
		i := strings.Index(body, tag)
		if i < 0 || i < last {
			t.Fatalf("want the script tags in this order: %v\nbody:\n%s", tags, body)
		}
		last = i
	}
	for _, path := range []string{"/static/Sortable.min.js", "/static/order.js"} {
		if rec := do(t, env.H, "GET", path, nil, nil); rec.Code != http.StatusOK {
			t.Errorf("GET %s status = %d, want 200", path, rec.Code)
		}
	}
	assertContains(t, do(t, env.H, "GET", "/static/app.js", nil, nil).Body.String(),
		`document.body.classList.contains("dragging")`)
}

// orderHarness runs order.js with a fake document and prints one JSON line
// per case. A list is a string like "o1 o2 3 4": "o" marks an overdue row.
const orderHarness = `
const src = require("fs").readFileSync(process.argv[2], "utf8");
const document = { addEventListener() {} };
const fns = new Function("document", src + "\nreturn { afterFor, stepTarget };")(document);
function rows(spec) {
  const out = spec.split(" ").map((s) => ({
    id: "item-" + s.replace("o", ""),
    classList: { contains: (c) => c === "item" || (c === "overdue-row" && s.startsWith("o")) },
  }));
  out.forEach((r, i) => {
    r.previousElementSibling = out[i - 1] || null;
    r.nextElementSibling = out[i + 1] || null;
  });
  return out;
}
function show(node) {
  if (node === undefined) return "none";
  if (node === null) return "end";
  return node.id;
}
const r = rows("o1 o2 3 4 5");
console.log(JSON.stringify({ name: "after: first normal row", got: fns.afterFor(r[2]) }));
console.log(JSON.stringify({ name: "after: second normal row", got: fns.afterFor(r[3]) }));
console.log(JSON.stringify({ name: "after: first row, no overdue", got: fns.afterFor(rows("7 8")[0]) }));
console.log(JSON.stringify({ name: "up: below overdue", got: show(fns.stepTarget(r[2], -1)) }));
console.log(JSON.stringify({ name: "up: middle", got: show(fns.stepTarget(r[3], -1)) }));
console.log(JSON.stringify({ name: "down: middle", got: show(fns.stepTarget(r[2], 1)) }));
console.log(JSON.stringify({ name: "down: second last", got: show(fns.stepTarget(r[3], 1)) }));
console.log(JSON.stringify({ name: "down: last", got: show(fns.stepTarget(r[4], 1)) }));
`

func TestOrderScriptHelpers(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	harness := filepath.Join(t.TempDir(), "harness.js")
	if err := os.WriteFile(harness, []byte(orderHarness), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(node, harness, "static/order.js").CombinedOutput()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, out)
	}
	want := map[string]string{
		"after: first normal row":      "",
		"after: second normal row":     "3",
		"after: first row, no overdue": "",
		"up: below overdue":            "none",
		"up: middle":                   "item-3",
		"down: middle":                 "item-5",
		"down: second last":            "end",
		"down: last":                   "none",
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != len(want) {
		t.Fatalf("got %d results, want %d:\n%s", len(lines), len(want), out)
	}
	for _, line := range lines {
		var got struct{ Name, Got string }
		if err := json.Unmarshal([]byte(line), &got); err != nil {
			t.Fatalf("%q: %v", line, err)
		}
		if w, ok := want[got.Name]; !ok || got.Got != w {
			t.Errorf("%s: got %q, want %q", got.Name, got.Got, w)
		}
	}
}
```

- [ ] **Step 3: Run the tests to check that they fail**

Run: `go test ./internal/web/ -run 'TestPageLoadsOrderScripts|TestOrderScriptHelpers'`
Expected: FAIL. The script tags are missing, and Node cannot read `static/order.js`.

- [ ] **Step 4: Write `order.js`**

Create `internal/web/static/order.js`:

```js
// Drag-and-drop and keyboard order (spec 2026-10-12-drag-order-design.md).
// Only the grip starts a drag. Overdue rows (.overdue-row) stay at the top:
// they cannot move, and nothing can be dropped between them. This file loads
// after app.js and uses its NETWORK_ERROR.

// isNormal reports whether row is a list row that is not overdue.
function isNormal(row) {
  return !!row && row.classList.contains("item") && !row.classList.contains("overdue-row");
}

// afterFor returns the "after" value for row at its current place: the id of
// the nearest normal row above it, or "" for the first place.
function afterFor(row) {
  for (let prev = row.previousElementSibling; prev; prev = prev.previousElementSibling) {
    if (isNormal(prev)) {
      return prev.id.slice("item-".length);
    }
  }
  return "";
}

// stepTarget returns the node to insert row before, for one step up (-1) or
// down (+1). null means the end of the list; undefined means no move.
function stepTarget(row, dir) {
  if (dir < 0) {
    const prev = row.previousElementSibling;
    return isNormal(prev) ? prev : undefined;
  }
  const next = row.nextElementSibling;
  return next ? next.nextElementSibling : undefined;
}

// Move requests go out one at a time and in order, so fast key presses
// reach the server in the same order as on the page.
let moveQueue = Promise.resolve();

function sendMove(row, putBack) {
  const id = row.id.slice("item-".length);
  const body = new URLSearchParams({ after: afterFor(row) });
  moveQueue = moveQueue.then(() => postMove(id, body, putBack));
}

async function postMove(id, body, putBack) {
  const errorLine = document.getElementById("error");
  let res;
  try {
    res = await fetch(`/items/${id}/move`, { method: "POST", headers: { "HX-Request": "true" }, body });
  } catch (err) {
    putBack();
    if (errorLine) {
      errorLine.textContent = NETWORK_ERROR;
    }
    return;
  }
  if (res.status === 204) {
    if (errorLine) {
      errorLine.textContent = "";
    }
    return;
  }
  if (res.status === 401) {
    // Not logged in any more: the reload shows the log-in page.
    location.reload();
    return;
  }
  if (res.status === 404 || res.status === 422) {
    // The page shows an old state (another tab, or the item became overdue).
    // Load the true order. app.js section 11 keeps an open edit row.
    htmx.ajax("GET", "/", { target: "#list-section", swap: "outerHTML" });
    return;
  }
  putBack();
  if (errorLine) {
    errorLine.textContent = "Something went wrong. Please try again.";
  }
}

// Say the new place of row to a screen reader, for example "Moved to 3 of 7".
function announce(row) {
  const status = document.getElementById("move-status");
  const rows = Array.from(row.parentElement.children).filter(isNormal);
  if (status) {
    status.textContent = `Moved to ${rows.indexOf(row) + 1} of ${rows.length}`;
  }
}

// Start SortableJS on the list. htmx replaces the list after some answers,
// so this runs again for each new list.
function startSortable(list) {
  if (!list || typeof Sortable === "undefined" || Sortable.get(list)) {
    return;
  }
  Sortable.create(list, {
    handle: ".grip",
    draggable: "li.item:not(.overdue-row)",
    ghostClass: "drag-ghost",
    chosenClass: "drag-chosen",
    animation: 150,
    onMove: (e) => !e.related.classList.contains("overdue-row"),
    onStart: () => document.body.classList.add("dragging"),
    onEnd: (e) => {
      document.body.classList.remove("dragging");
      if (e.oldIndex === e.newIndex) {
        return; // dropped at the start place: nothing changed
      }
      const row = e.item;
      const oldIndex = e.oldIndex;
      sendMove(row, () => {
        row.remove();
        list.insertBefore(row, list.children[oldIndex] || null);
      });
    },
  });
}

document.addEventListener("htmx:load", (e) => {
  const elt = e.detail.elt;
  startSortable(elt.id === "items" ? elt : elt.querySelector && elt.querySelector("#items"));
});
document.addEventListener("DOMContentLoaded", () => startSortable(document.getElementById("items")));

// Arrow Up and Arrow Down on a focused grip move the row one place.
document.addEventListener("keydown", (e) => {
  if (!e.target.matches || !e.target.matches(".grip") || (e.key !== "ArrowUp" && e.key !== "ArrowDown")) {
    return;
  }
  e.preventDefault();
  const row = e.target.closest("li.item");
  const before = stepTarget(row, e.key === "ArrowUp" ? -1 : 1);
  if (before === undefined) {
    return;
  }
  const list = row.parentElement;
  const oldNext = row.nextElementSibling;
  list.insertBefore(row, before);
  row.querySelector(".grip").focus(); // a moved node can lose the focus
  announce(row);
  sendMove(row, () => list.insertBefore(row, oldNext));
});
```

- [ ] **Step 5: Load the scripts**

In `internal/web/templates/page.html`:

```html
<script src="/static/tz.js" defer></script>
<script src="/static/htmx.min.js" defer></script>
<script src="/static/Sortable.min.js" defer></script>
<script src="/static/app.js" defer></script>
<script src="/static/order.js" defer></script>
```

- [ ] **Step 6: No overdue refresh during a drag**

In `internal/web/static/app.js`, section 10, change `refreshForOverdue`. Also add one sentence to the section comment: "Never during a drag (order.js sets the body class "dragging")."

```js
function refreshForOverdue(next) {
  if (document.querySelector(".item.editing") || document.body.classList.contains("dragging")) {
    overdueTimer = setTimeout(() => refreshForOverdue(next), 30000);
    return;
  }
```

- [ ] **Step 7: Add the drag styles**

In `internal/web/static/app.css`, after the `.grip` rule from Task 3:

```css
.dragging, .dragging * { cursor: grabbing; }
.drag-ghost { opacity: 0.4; }
.drag-chosen { background: var(--card); }
```

- [ ] **Step 8: Run the tests to check that they pass**

Run: `go test ./internal/web/ -run 'TestPageLoadsOrderScripts|TestOrderScriptHelpers' -v`
Expected: PASS for both. (`TestOrderScriptHelpers` skips if Node is not installed. On this machine Node v24 is installed.)

Run: `go test ./...`
Expected: all PASS, also `TestSecurityHeaders` (the CSP did not change).

- [ ] **Step 9: Commit**

```bash
git add internal/web/static/Sortable.min.js internal/web/static/order.js internal/web/templates/page.html internal/web/static/app.js internal/web/static/app.css internal/web/order_script_test.go
git commit -m "feat: drag rows by the grip or move them with the arrow keys

Vendor SortableJS 1.15.6 (MIT), sha256 <the value from Step 1>.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: Docs and the manual browser check

**Files:**
- Modify: `README.md` ("What it does")
- Modify: `docs/superpowers/specs/2026-10-01-todo-app-design.md` (order rules, section 10)
- Modify: `docs/superpowers/specs/2026-10-12-drag-order-design.md` (three small fixes, see Step 3)

**Interfaces:**
- Consumes: the finished feature from Tasks 1–4.
- Produces: up-to-date docs.

- [ ] **Step 1: Update the README**

In `README.md`, "What it does", add a line after "Overdue items move to the top of the list.":

```markdown
- Overdue items move to the top of the list.
- Drag items by the grip (⋮⋮) to change their order, or use the arrow keys. Overdue items
  stay at the top.
```

- [ ] **Step 2: Update the first spec**

In `docs/superpowers/specs/2026-10-01-todo-app-design.md`, after the "Since 2026-10-02: overdue open items come first…" line in the order rules, add:

```markdown
- Since 2026-10-12: the user can drag items to change the order. A move renumbers the user's `position` values. See `2026-10-12-drag-order-design.md`.
```

In section 10, item 3, add at the end: "Done on 2026-10-12, see `2026-10-12-drag-order-design.md`."

- [ ] **Step 3: Make the drag spec match the code**

In `docs/superpowers/specs/2026-10-12-drag-order-design.md`:

1. Section 4, "Browser": the code is in a new file `static/order.js`, not in a new section 13 of `app.js`. `order.js` loads after `app.js`. In `page.html`, `Sortable.min.js` loads before `app.js`.
2. Section 5, point 5: SortableJS has no Escape key to cancel a drag. The text becomes: "A drop at the start place sends no request."
3. Section 5, points 1 and 2: the page does not use the list in the `404` and `422` answers. It loads the list again with `GET /`, the same call as the overdue refresh, so `app.js` section 11 keeps an open edit row.
4. Section 6, point 2: the test checks that the order does not change (there is no fake store, so it cannot check that the store was not called).

- [ ] **Step 4: Manual check in a real browser**

Run: `go run ./cmd/todo` and open http://127.0.0.1:8811. Sign up a test user. Add five items, and give one item a due time in the past.

1. Desktop width: drag a row by the grip to three places (top, middle, end). Reload: the order is the same. The overdue row stays at the top, and a drop above it is not possible. The browser console shows no CSP error.
2. Phone width with touch emulation (DevTools device mode): drag by the grip. A swipe on the item text scrolls the page and does not start a drag. A tap on the text opens the edit row.
3. Keyboard: Tab to a grip and press Arrow Down three times fast, then reload. The order is the same as before the reload. At the top of the normal group, Arrow Up does nothing. Check the text of `#move-status`, for example "Moved to 4 of 5".
4. Turn on "Hide done" with one done item in the middle. Move a visible row, then turn "Hide done" off. The done item is still between the same rows.
5. Stop the server and drag a row. The row goes back to its old place, and the error line says "Cannot reach the server. Please try again."

Write the result of each check in the task report. If a check fails, stop and report it. Do not change the spec to make it pass.

- [ ] **Step 5: Run all tests**

Run: `go test ./... && go vet ./...`
Expected: all PASS, no vet output.

- [ ] **Step 6: Commit**

```bash
git add README.md docs/superpowers/specs/2026-10-01-todo-app-design.md docs/superpowers/specs/2026-10-12-drag-order-design.md
git commit -m "docs: drag-and-drop order in the README and the specs

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

## Verification (end to end)

1. `go test ./... && go vet ./...` passes in the worktree.
2. The manual browser check in Task 5, Step 4 passes, all five points.
3. `git log --oneline main..drag-order` shows the spec commit and five task commits.
4. After approval, copy this plan to `docs/superpowers/plans/2026-10-12-drag-order.md` in the worktree and commit it, before Task 1.
