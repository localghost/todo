# Todo app — design spec

Date: 2026-10-01
Status: waiting for review

## 1. Goal and scope

The app is a todo list for personal use. It runs in the browser and is written in Go.

The first version has these features:

- Add an item with an input field at the top of the page.
- Edit the text of an item in the list.
- Mark an item as done. The text of a done item is crossed out.
- Mark a done item as not done again.
- Delete an item.
- Hide or show done items with one button.

Later, the app may support more than one user. The design makes this easy, but the first version has no login.

These items are not part of the first version:

- Login and user accounts.
- A JSON API.
- Drag-and-drop to change the order of items.
- Offline use.

## 2. Decisions and reasons

| Topic | Decision | Reason |
|---|---|---|
| Language | Go 1.22 (installed: 1.22.2) | User choice. Go 1.22 `http.ServeMux` supports method and `{id}` patterns, so no router library is necessary. |
| Storage | SQLite, one file on disk | Data stays after a restart. Easy to add users later. |
| SQLite driver | Pure-Go driver (`modernc.org/sqlite`) | No C compiler is necessary. During implementation, pin a driver version that still supports Go 1.22. |
| Frontend | htmx, with HTML made on the server by Go `html/template` | Mostly Go code and very little JavaScript. No build step. |
| JSON API | Not now. Add later on the same service. | YAGNI. A clean service layer makes the API a small change later. |

htmx keeps both future paths open:

1. A JSON API for other clients (mobile app, CLI, scripts) is a set of new handlers on the same `todo.Service`.
2. A move to a JavaScript frontend later only replaces the `web` package. The service and the store do not change.

Both paths work only if all business rules stay in `todo.Service`. Section 4 makes this a rule.

## 3. Run

The app is one binary. Build it with `go build ./cmd/todo`.

| Flag | Default | Meaning |
|---|---|---|
| `-addr` | `127.0.0.1:8811` | Address and port to listen on |
| `-db` | `todo.db` | Path to the SQLite database file |

The binary embeds the HTML templates, the CSS file, and `htmx.min.js` with `go:embed`. So the binary needs no other files next to it.

If the app cannot open the database, it writes a clear error message and stops.

On Ctrl+C (SIGINT) or SIGTERM, the server stops with `http.Server.Shutdown`. Open requests can finish first.

## 4. Architecture

```
todo/
├── cmd/todo/main.go          # flags, open DB, start and stop server
├── internal/
│   ├── todo/                 # Item type, Service (all rules), Store interface
│   ├── store/sqlite/         # SQLite implementation of Store, schema setup
│   └── web/                  # HTTP handlers, templates, static files
│       ├── templates/
│       └── static/           # app.css, htmx.min.js
└── go.mod                    # module name: todo
```

Each package has one job:

1. `internal/todo` holds the rules, for example "trim spaces" and "text must not be empty". It does not know about HTTP, HTML, or SQL.
2. `internal/store/sqlite` holds only SQL queries and the schema setup.
3. `internal/web` reads requests, calls the service, and renders HTML.
4. `cmd/todo` connects the parts and starts the server.

Rule: handlers in `internal/web` contain no business rules. They only read input, call one service method, and render the result.

The service API:

```go
type Item struct {
    ID        int64
    UserID    int64
    Text      string
    Done      bool
    Position  int64
    CreatedAt time.Time
    UpdatedAt time.Time
}

func (s *Service) List(ctx context.Context, userID int64, hideDone bool) ([]Item, error)
func (s *Service) Get(ctx context.Context, userID, id int64) (Item, error)
func (s *Service) Add(ctx context.Context, userID int64, text string) (Item, error)
func (s *Service) UpdateText(ctx context.Context, userID, id int64, text string) (Item, error)
func (s *Service) Toggle(ctx context.Context, userID, id int64) (Item, error)
func (s *Service) Delete(ctx context.Context, userID, id int64) error
func (s *Service) Counts(ctx context.Context, userID int64) (open, done int, err error)
```

The service returns two known errors:

- `todo.ErrEmptyText` if the text is empty after trimming spaces.
- `todo.ErrNotFound` if the item does not exist or belongs to another user.

## 5. Data model

```sql
CREATE TABLE IF NOT EXISTS users (
  id   INTEGER PRIMARY KEY,
  name TEXT NOT NULL
);
INSERT OR IGNORE INTO users (id, name) VALUES (1, 'default');

CREATE TABLE IF NOT EXISTS items (
  id         INTEGER PRIMARY KEY,
  user_id    INTEGER NOT NULL REFERENCES users(id),
  text       TEXT    NOT NULL,
  done       INTEGER NOT NULL DEFAULT 0,
  position   INTEGER NOT NULL,
  created_at TEXT    NOT NULL,
  updated_at TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS items_user_position ON items (user_id, position);
```

Rules:

- The schema setup runs at each start. It is idempotent: it can run more than once without errors or changes.
- All service methods take a `userID`. In the first version, the `web` package always passes `1`.
- A new item gets `position = max(position) + 1` for its user. So new items go to the end of the list.
- The list is sorted by `position`. Done items keep their place.
- Times are stored as UTC text in RFC 3339 format.

## 6. UI

The approved UI proposal is in Claude Design: https://claude.ai/artifact/7mnRhC4Jd8jbnbAhjN7Woq

The canvas has 7 pages: clickable prototype, main list, done hidden, inline edit, add error, empty state, and phone width.

The page layout, from top to bottom:

1. The title "Todo".
2. The add form: an input field ("What needs to be done?") and an "Add" button. Enter also adds the item.
3. The toolbar: the counter "N open · M done" on the left, and the button "Hide done (M)" or "Show done (M)" on the right.
4. The list. Each row has a checkbox, the text, and a delete button `×`.

The look: one light theme, one blue accent (`#2450C7`), and the font Source Sans 3. The content column is at most 560 px wide. All click targets are at least 44 px. The page works at phone width (390 px).

Approved UI decisions:

| Topic | Decision |
|---|---|
| Delete button `×` | Always visible, also on desktop. No confirm dialog. |
| Done item | Checkbox is filled. Text is crossed out and gray. |
| Check an item while done items are hidden | The row disappears at once. |
| Start edit | Click the text. It changes to an input field. |
| Save edit | Enter, or a click outside the input (blur). |
| Cancel edit | Esc. The old text comes back. |
| Order of done items | They keep their place in the list. |
| Empty list, no items at all | Show "No items yet" and "Type a task above and press Enter." |
| Empty list, all items done and hidden | Show "All done" and "Done items are hidden. Use "Show done" to see them." |

Rule for this project: each new UI/UX decision is first proposed in Claude Design and approved by the user.

## 7. Routes and htmx behavior

| Action | Route | Response |
|---|---|---|
| Show page | `GET /` (optional `?hide_done=1`) | Full page |
| Add item | `POST /items` (form field `text`) | New row, added at the end of the list. The input is cleared. |
| Mark done or not done | `POST /items/{id}/toggle` | Updated row. If done items are hidden and the item is now done: empty response, and htmx removes the row. |
| Start edit | `GET /items/{id}/edit` | Row with an input field |
| Save edit | `PUT /items/{id}` (form field `text`) | Normal row with the new text |
| Cancel edit | `GET /items/{id}` | Normal row with the old text |
| Delete | `DELETE /items/{id}` | Empty response. htmx removes the row. |
| Show or hide done items | `GET /?hide_done=1` or `GET /` with `hx-push-url` | The list body and toolbar. The server sends this fragment if the request has the `HX-Request` header. Without the header, it sends the full page. |
| Static files | `GET /static/...` | CSS and htmx, from the embedded files |

The htmx requests send form data (`application/x-www-form-urlencoded`), the htmx default. Handlers read it with `r.FormValue`.

The current hide state is in the URL (`?hide_done=1`). A page reload keeps it. Requests that change items send the hide state as a form value (`hide_done`), so the server knows if a done row must disappear.

Out-of-band swaps (`hx-swap-oob`):

- After add, toggle, and delete, the response also contains the toolbar. So the counter and the button label are always correct.
- If the visible list becomes empty, or stops being empty, the response also contains the empty-state block.

Edit triggers on the edit input:

- Save: `hx-trigger="keyup[key=='Enter'], blur"` with `hx-put="/items/{id}"`.
- Cancel: `keyup[key=='Escape']` loads `GET /items/{id}`.

## 8. Errors

| Case | Status | What the user sees |
|---|---|---|
| Empty text on add | `422` | A red border on the input and the message "Please type some text first." under it |
| Empty text on edit | `422` | The input stays open, with a red border and the message "Text cannot be empty." |
| Item not found (for example deleted in another tab) | `404` | The row is removed from the page |
| Unexpected error (for example a database error) | `500` | A short error message. The server logs the details with `log/slog`. |
| Database cannot be opened at start | — | The app writes an error and stops with exit code 1 |

htmx does not swap `4xx` and `5xx` responses by default. The page configures htmx to swap `422` and `404` responses, so the server can send the error HTML and the row removal.

## 9. Testing

The work follows TDD: write a failing test first, then the code. The tests use only the Go standard library (`testing`, `net/http/httptest`).

| Layer | What the tests check | How |
|---|---|---|
| `internal/todo` | Trim spaces, reject empty text, new item at the end, toggle, delete, item of another user is "not found" | Real SQLite in `t.TempDir()` |
| `internal/store/sqlite` | Schema setup runs twice without errors, order by `position`, `hide_done` filter | Real SQLite in `t.TempDir()` |
| `internal/web` | Status codes `200`, `422`, `404`. Each response has the expected row, toolbar, and empty state. The out-of-band toolbar is present. | `httptest` with the real router and a temporary database |
| Manual | The full flow in the browser at `127.0.0.1:8811` | Run the binary and repeat the flows from the prototype |

Commands:

- `go test ./...` runs all tests.
- `go vet ./...` checks the code.

There are no browser tests (for example Playwright) in the first version.

## 10. Future work

These items are not in the first version. The design prepares for them:

1. JSON API: add `internal/web/api.go` with `/api/items` routes on the same `todo.Service`.
2. More users: add login and sessions in `internal/web`. Pass the real user ID to the service. The service and the store do not change.
3. Drag-and-drop order: use SortableJS with htmx. Add a service method that saves new `position` values.

---

Co-authored with [Claude Code](https://claude.com/claude-code).
