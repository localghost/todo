# Overdue items at the top — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.

## Context

The user wants overdue items at the top of the list. Decisions: most overdue first; the
other items keep today's order (`position, id`); look = **option A** from canvas page 14
("just moved", no new elements); rows move live (when an item becomes overdue while the page
is open, and back after postpone/done/edit).

Worktree `~/claude-personal/todo-worktrees/overdue-top`, branch `overdue-top`, from `main`
(`981ac71`). Native execution, one commit per task, then a fresh whole-branch review. No push.

## Design

1. **Rule in one place (todo):** `func (it Item) Overdue(now time.Time) bool` — open item,
   timed: `now` after the due moment; all-day: on or after the start of the next day in
   `now`'s zone. `web/due.go` `isOverdue` is replaced by it. `OverdueFirst(items, now)` —
   stable: overdue items first, sorted by due moment (then id); the rest keep their order.
   `NextOverdue(items, now) (time.Time, bool)` — the earliest future moment when an open
   item becomes overdue (timed: its due moment; all-day: next midnight after its day, in
   `now`'s zone).
2. **List render (web):** `listView` gets `now` (= `s.now().In(zoneFor(r))`), orders with
   `OverdueFirst`, and sets `NextOverdue` (Unix ms). `<section id="list-section"
   data-next-overdue="…">` (attribute only when there is one).
3. **Moves after an action:** add, toggle, postpone and save-edit compare the item's overdue
   state before and after (add: "before" = not in the list). If it changed, the response is
   the whole list (`HX-Retarget: #list-section`, `HX-Reswap: outerHTML`, part `list`);
   otherwise the row as today. Focus stays right: app.js already focuses `check-<id>`, which
   exists in the new list.
4. **Live move (app.js):** after load and after every swap, read `data-next-overdue`; set one
   timer (cleared on each swap) for that moment + 1 s, at most 6 hours ahead. When it fires:
   if an edit row is open (`.item.editing`), try again in 30 s; else
   `htmx.ajax("GET", "/", {target: "#list-section", swap: "outerHTML"})` (uses the existing
   htmx `GET /` fragment branch, which gets a caller again).
5. **Docs:** todo-app spec (list order section), due-dates spec (overdue moves to top).

## Review Focus

1. All-day items become overdue at midnight in the browser's zone, not the server's. → Task 1 test with two zones.
2. A done item is never in the overdue group, even if its due time is past. → Task 1 test.
3. Marking an overdue item done (or postponing it) moves it back to its normal place in the response. → Task 2 test.
4. The live refresh never destroys an open edit row. → Task 3 static check + browser check.
5. With done items hidden, the order and the moves work the same. → Task 2 test.

---

### Task 1: overdue rule and order in todo

**Files:** `internal/todo/overdue.go` (new) + `overdue_test.go`, `internal/web/due.go` (use `Item.Overdue`). Copy this plan to `docs/superpowers/plans/2026-10-02-overdue-top.md`; ledger.

- [ ] Step 1: failing tests: `Overdue` for timed (before/after due), all-day (still due on its day 23:59 in Warsaw; overdue at 00:00 the next day in Warsaw; not yet overdue at that moment in New York), done items never overdue; `OverdueFirst` on 5 items (2 overdue in wrong order, 1 done-and-past, 2 others) → overdue by due time, then the rest in input order; `NextOverdue` returns the earliest future moment (timed and all-day), false if none. Run → FAIL.
- [ ] Step 2: implement; `web/due.go` calls `it.Overdue(now)`; existing due-label tests stay green.
- [ ] Step 3: `go test -count=1 ./...`, `go vet ./...` → exit 0. Commit `feat: overdue rule and overdue-first order in todo`.

### Task 2: list order and moves in web

**Files:** `internal/web/views.go`, `internal/web/handlers.go`, `internal/web/templates/list.html`, new `internal/web/overdue_order_test.go`.

- [ ] Step 1: failing tests (fixed clock, Warsaw cookie): `GET /` lists the overdue items first, most overdue first, then the others in creation order (check text positions); `data-next-overdue` = the next due moment in ms (absent without one); toggle an overdue item → response has `HX-Retarget: #list-section` and the item is after the remaining overdue item; toggle a not-overdue item → row only (no retarget); postpone an overdue item → full list, item in its normal place; add an item with a past due time → full list with it at the top; the same order with hide done on. Run → FAIL.
- [ ] Step 2: implement design points 2–3.
- [ ] Step 3: `go test -race -count=1 ./...`, `go vet ./...` → exit 0. Commit `feat: overdue items at the top of the list`.

### Task 3: live move and docs

**Files:** `internal/web/static/app.js`, static test, specs.

- [ ] Step 1: failing static test: app.js contains `data-next-overdue` handling, `htmx.ajax("GET", "/"`, the `.item.editing` guard, and a 6-hour cap. Run → FAIL.
- [ ] Step 2: implement design point 4; `node --check`; docs (design point 5).
- [ ] Step 3: `go test -count=1 ./...`, `go vet ./...` → exit 0. Commit `feat: the list refreshes when an item becomes overdue; docs`.

## Final check

Fresh whole-branch review with the Review Focus. Browser check (user): two overdue items are
at the top, most overdue first; postpone one → it moves back down; mark one done → it moves to
its normal place; add an item due 1 minute from now, wait → it moves to the top by itself; with
an edit row open at that moment, the row stays until you finish. Then the merge question.
