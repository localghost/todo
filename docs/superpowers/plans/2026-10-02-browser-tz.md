# Due dates in the browser's time zone — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.

## Context

Due dates are stored in UTC (correct), but they are read from the form and shown in the
server's time zone (`time.Local`; on Fly `TZ=Europe/Warsaw`). The user wants them shown and
entered in the browser's local time. Decision: all-day items keep today's model (09:00 in
the zone where they are saved, stored as a UTC moment); far-west zones may see the day before
— accepted for this version.

Worktree `~/claude-personal/todo-worktrees/browser-tz`, branch `browser-tz`, from `main`
(`1c16596`). Native execution, one commit per task, then a fresh whole-branch review. No push.
No visible UI change, so no Claude Design page.

## Design

1. **Browser → server:** new `static/tz.js`, loaded (defer) on every page (`page.html`,
   `auth.html`). It reads `Intl.DateTimeFormat().resolvedOptions().timeZone` and, if the
   cookie `todo_tz` differs, sets `todo_tz=<zone>; Path=/; Max-Age=31536000; SameSite=Lax`
   (+ `Secure` on https). On the list page, if `<body data-tz>` (the zone the server used)
   differs from the browser zone **and** the script just changed the cookie, it reloads once.
   No loop: after one reload the cookie already matches, so there is no second reload, even if
   the server rejected the zone.
2. **Server zone per request:** `zoneFor(r)` reads `todo_tz`; accepts at most 64 characters
   matching `^[A-Za-z0-9_+-]+(/[A-Za-z0-9_+-]+)*$`, not `Local`; `time.LoadLocation`, cached
   in a `sync.Map`. Anything else → the server's zone (`s.now().Location()`).
   `cmd/todo/main.go` imports `_ "time/tzdata"` so every zone loads, also in distroless.
3. **todo service:** `Add` and `Edit` get a `loc *time.Location` parameter (nil = the
   service default, today's behavior) for reading date and time from the form.
4. **Rendering:** templates are parsed once into a base set that is never executed. Each
   `render` clones it and binds `added`, `due`, and a new `zone` func to the request's zone
   (`now := s.now().In(loc)`). Also in the request zone: edit-row inputs (`dueInputs`),
   notification text (`notifyText`), account "member since". `page.html` gets
   `<body data-tz="{{zone}}">`.
5. **Docs:** due-dates spec ("server local time" → "browser zone, fallback server zone");
   hosting spec and `fly.toml` comment: `TZ` is now only the fallback.

## Review Focus

1. Bad cookie values (`../../etc/passwd`, `Local`, `/abs`, 1 KB string, `Mars/Base`) fall back
   to the server zone, with no error page and no file read outside the zone database. → Task 2 table test.
2. No reload loop when the server rejects the browser's zone. → Task 3 JS logic test (static checks) + browser check.
3. "Today" / "Overdue" boundaries use the request zone. → Task 2 test with a clock near midnight.
4. An item saved in one zone shows the same moment in another zone. → Task 2 test (save in Tokyo, read in Warsaw).
5. Per-request template clones are safe under parallel requests. → Task 2 `-race` test with 20 parallel GETs in 2 zones.

---

### Task 1: todo service takes the zone

**Files:** `internal/todo/service.go`, `internal/todo/*_test.go`, callers in `internal/web/handlers.go` (pass `nil` for now). Copy this plan to `docs/superpowers/plans/2026-10-02-browser-tz.md`; ledger.

- [ ] Step 1: failing test: `Add(ctx, user, "x", "2026-10-03", "14:00", tokyo)` stores `2026-10-03T05:00:00Z`; `Edit` the same; `nil` keeps the service default. Run → FAIL (compile).
- [ ] Step 2: add the parameter; `loc == nil` → `s.loc`. Update callers and existing tests (`nil`).
- [ ] Step 3: `go test -count=1 ./...`, `go vet ./...` → exit 0. Commit `feat: todo service reads due dates in a given zone`.

### Task 2: web uses the browser's zone

**Files:** `internal/web/server.go` (base templates, `render`, `zoneFor`), `internal/web/handlers.go`, `internal/web/due.go`/`added.go` callers, `internal/web/auth_handlers.go`, `internal/web/templates/page.html`, `cmd/todo/main.go` (tzdata), new `internal/web/zone_test.go`.

- [ ] Step 1: failing tests (`zone_test.go`, helper: request with cookie `todo_tz`):
  - fixed clock `2026-10-02 23:30 Europe/Warsaw`; item due `2026-10-03 00:30` Warsaw → with cookie `Europe/Warsaw` the label says tomorrow-style text; with `Europe/London` (22:30, due 23:30) it says today. (Boundary test, Focus 3.)
  - POST add with `Asia/Tokyo` and 14:00 → stored `05:00Z`; GET with `Europe/Warsaw` shows 07:00. (Focus 4.)
  - edit row inputs show the time in the cookie zone.
  - `<body data-tz="Asia/Tokyo">` with that cookie; without a cookie → the server zone.
  - bad values (Focus 1 list) → 200 and the server zone in `data-tz`.
  - 20 parallel GETs, half Tokyo half Warsaw, each body has its own `data-tz` (run with `-race`). (Focus 5.)
  - notification JSON text uses the cookie zone.
  Run → FAIL.
- [ ] Step 2: implement design points 2–4 for the web layer; handlers pass `zoneFor(r)` to `Add`/`Edit`.
- [ ] Step 3: `go test -race -count=1 ./...`, `go vet ./...` → exit 0. Commit `feat: show and read due dates in the browser's time zone`.

### Task 3: tz.js and docs

**Files:** new `internal/web/static/tz.js`, `page.html`, `auth.html`, static test, specs, `fly.toml` comment.

- [ ] Step 1: failing tests: both pages include `<script src="/static/tz.js" defer></script>`; `tz.js` contains `resolvedOptions().timeZone`, `todo_tz=`, `SameSite=Lax`, and reloads only when it changed the cookie (the reload call sits inside the `changed` branch — check by a simple pattern). Run → FAIL.
- [ ] Step 2: write `tz.js` (IIFE, no eval, works without htmx); add script tags; docs.
- [ ] Step 3: `node --check internal/web/static/tz.js`; `go test -count=1 ./...`, `go vet ./...` → exit 0. Commit `feat: browser sends its time zone; docs`.

## Final check

Fresh whole-branch review with the Review Focus. Browser check (user): open the app → no
visible change in Poland; change the system or browser zone (for example DevTools → Sensors →
Location → Tokyo) → reload → due times move by 7 hours; add an item at 14:00 Tokyo → switch
back → it shows 07:00. Then the merge question.
