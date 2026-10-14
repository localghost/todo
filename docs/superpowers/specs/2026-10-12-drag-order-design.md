# Drag-and-drop order — design spec

Date: 2026-10-12
Status: waiting for review
Builds on: `2026-10-01-todo-app-design.md` (section 10, future work item 3) and `2026-10-01-due-dates-design.md` (overdue items at the top)

## 1. Goal and scope

The user can drag items in the list to set their own order. The order is saved on the server at once. It is the same in every tab and after a reload. A mouse, a touch screen, and the keyboard can all change the order.

These items are not part of this feature:

- Drag between lists or between users.
- Sort buttons (for example "sort by due date").
- Drag of overdue items.
- Browser tests (for example Playwright). The first spec has none, and this feature keeps it that way.

## 2. Decisions

| Topic | Decision |
|---|---|
| Overdue items | They stay pinned at the top, sorted by due time. They cannot be dragged, and nothing can be dropped into their group. |
| Start a drag | Only from a grip button (⋮⋮) on the left of each row. Click-to-edit and page scroll on phones work as before. |
| Keyboard | Focus the grip with Tab. Arrow Up and Arrow Down move the item one place. |
| Browser code | SortableJS, vendored into `internal/web/static/` (MIT license). The CSP allows only scripts from our own server, so a CDN is not possible. |
| What the browser sends | "Put item X after item Y" (or first). Not the full order, because with "Hide done" on, the page does not show all items. |
| How the server saves | It renumbers all positions of the user in one transaction. The lists are small, so this is cheap. |
| Schema | No change. The `position` column already exists, and the list already sorts by `position, id`. |

## 3. Behavior

A "normal row" is any row that is not overdue. Done rows are normal rows.

1. Each normal row gets a grip button (⋮⋮) on the left, before the checkbox. Only the grip starts a drag.
2. Overdue rows have no grip. They stay on top, sorted by due time, as now. A drop above the first normal row puts the item first in the normal group.
3. Done rows have a grip and can move like open rows. Done items keep their place in the list, as now.
4. A row in edit mode has no grip. While a drag runs, the other rows move away to show the drop place.
5. New items still go to the end of the list (`position = max(position) + 1`).
6. If an item stops being overdue (done, postponed, or a changed due date), it goes back to its saved place. Its `position` does not change while it is overdue.
7. Keyboard: Arrow Up on a focused grip moves the item one place up, and Arrow Down moves it one place down. Focus stays on the grip of the moved row. At the top or the bottom of the normal group, the key does nothing.
8. A screen reader hears the result, for example "Moved to 3 of 7". The numbers count the rows of the normal group. The grip label is "Move: <item text>".
9. If "Hide done" is on, the user moves items between the visible rows. The hidden done items keep their order relative to the rows around them.

## 4. Parts

### Store (`internal/store/sqlite`)

A new method goes into the `todo.Store` interface:

```go
// Move puts item id right after item afterID in the user's list, or first
// if afterID is 0, and renumbers the user's positions.
Move(ctx context.Context, userID, id, afterID int64) error
```

The SQLite store does these steps in one transaction:

1. Read the user's item ids in list order (`ORDER BY position, id`).
2. If `id` is not in the list, return `todo.ErrNotFound`. If `afterID` is not 0 and not in the list, return `todo.ErrNotFound`. Items of another user are not in the list.
3. If `afterID` equals `id`, change nothing.
4. Take `id` out of the list. Put it right after `afterID`, or first if `afterID` is 0.
5. Write `position` 1, 2, 3, … for all items of the user, in the new order.

The method does not change `updated_at`, because a move does not change the item.

### Service (`internal/todo`)

```go
// ErrCannotMove means the item is overdue at now; overdue items stay at the top.
var ErrCannotMove = errors.New("todo: an overdue item cannot move")

// Move puts item id after item afterID (0: first). now carries the browser's
// zone, because the overdue rule uses it for all-day items.
func (s *Service) Move(ctx context.Context, userID, id, afterID int64, now time.Time) error
```

The service reads the item. If it is overdue at `now`, the service returns `ErrCannotMove`. Else it calls `store.Move`.

### Route (`internal/web`)

`POST /items/{id}/move`, behind `s.protect` like the other item routes. The form field `after` holds an item id, or is empty for the first place. The handler passes `s.nowFor(r)` to the service.

| Result | Answer |
|---|---|
| The move worked | `204 No Content`. The page has already moved the row. |
| Bad id in the URL or in `after` | `404` with the whole list, `HX-Retarget: #list-section`, `HX-Reswap: outerHTML` |
| `todo.ErrNotFound` | The same as a bad id |
| `todo.ErrCannotMove` | `422` with the whole list, the same headers |
| Other error | `s.serverError`, as in the other handlers |

The existing `htmx-config` in `page.html` already swaps `404` and `422` answers and does not swap `204`. It needs no change.

### Templates

- `item.html`: the `item` template gets the grip button on normal rows: `<button class="grip" type="button" aria-label="Move: {{.Text}}" aria-describedby="move-help">`. A screen reader does not always read `title`, so `aria-describedby` gives the arrow-key hint. An overdue row gets no grip and the class `overdue-row` on its `<li>`. The `item-edit` template gets no grip.
- `list.html`: a visually hidden live region `<p id="move-status" class="visually-hidden" aria-live="polite"></p>` inside `#list-section`. Next to it, a visually hidden help text `<p id="move-help" class="visually-hidden">Use the Arrow Up and Arrow Down keys to move the item.</p>`.
- `page.html`: `<script src="/static/Sortable.min.js" defer></script>` before `app.js`, and `<script src="/static/order.js" defer></script>` after `app.js`. The `htmx-config` sets `attributesToSettle` to `class`, `width`, and `height`, without `style`. After a drag, SortableJS leaves a `style` attribute on the rows. Without this setting, htmx copies it to the new rows during a swap, and the CSP blocks it with a console error.
- `app.css`: the grip style, the drag styles from SortableJS (the ghost row and the chosen row), and `.visually-hidden` if it does not exist yet.

### Browser (`internal/web/static`)

- `Sortable.min.js`: one release file of SortableJS. The plan picks the exact version and checks its checksum. The MIT license note stays at the top of the file.
- `order.js`, a new file for the drag and keyboard order. It loads after `app.js` and uses its `NETWORK_ERROR`. It sets the body class `dragging` while a drag runs:
  1. Start SortableJS on `#items` at page load and again after each htmx swap that replaces `#list-section`. Options: `handle: ".grip"`, only rows without `.overdue-row` can be dragged, and an `onMove` check that stops a drop above an overdue row. The edit row has no grip, so the handle already stops a drag of it. The `draggable` option does not skip `.editing`: SortableJS counts `oldIndex` and `newIndex` only among draggable rows, so a move past the edit row would keep the same index and would not be sent.
  2. On a drop at a new place, find the nearest normal row above the dropped row. Send its id as `after`, or empty if there is none.
  3. Arrow Up and Arrow Down on a focused grip: move the row in the page, keep the focus on its grip, write "Moved to N of M" into `#move-status`, and send the same request.
  4. Send move requests one at a time and in order. The script keeps a small queue and sends the next request only after the last one has finished. So fast key presses cannot reach the server in the wrong order. A request that gets no answer in 10 seconds fails like a network error, so it cannot stop the queue.

## 5. Errors and edge cases

1. The item or the `after` item is gone, for example deleted in another tab: the server answers `404` with the whole list. The page loads the list again with `GET /` (the same call as the overdue refresh, so `app.js` section 11 keeps an open edit row) and shows the true order.
2. The item became overdue between page load and drop: the server answers `422` with the whole list. The page loads the list again, as in point 1. The item then shows at the top, as an overdue item.
3. A bad id in the URL or in `after`: the same answer as point 1.
4. A network error, a request with no answer in 10 seconds, or a `5xx` answer: the script puts the row back at its old place. For a network error, the error line says "Cannot reach the server. Please try again." For a `5xx` answer, it says "Something went wrong. Please try again."
5. A drop at the start place: no request goes out. (SortableJS has no Escape key to cancel a drag.)
6. A `401` answer (the session ended): the page reloads, so the server shows the log-in page.
7. The overdue refresh timer (section 10 in `app.js`) fires during a drag: it waits 30 seconds and tries again, the same as it does now while an edit row is open.
8. htmx removes the list during a drag, for example after a `404` answer: on `htmx:beforeCleanupElement` for `#items`, the script stops SortableJS on the old list and removes the body class `dragging`. The drag on the old list sends no request. Without this, `dragging` could stay on and stop the overdue refresh.
8. An edit row is open while the user drags another row: a whole-list answer keeps the edit row, because section 11 in `app.js` already does this.

## 6. Testing

1. Store, with real SQLite in `t.TempDir()`:
   - Move to the first place, to the middle, and to the end.
   - A move after itself changes nothing.
   - An unknown `id`, an unknown `afterID`, and another user's item return `todo.ErrNotFound`, and the order does not change.
   - Hidden done items keep their order relative to the other items.
   - A new item still goes to the end after a move.
2. Service:
   - An overdue item returns `ErrCannotMove`, and the order does not change. (The tests use a real store, so they check the order, not the store calls.)
   - An all-day item uses the zone of `now`: it can move on its due day and cannot move on the next day.
3. Web:
   - `POST /items/{id}/move` answers `204` and saves the order.
   - The `404` and `422` answers carry the whole list and the retarget headers.
   - The grip is on normal rows and done rows, but not on overdue rows or edit rows.
   - The page has `#move-status`.
   - `/static/Sortable.min.js` is served, and the page loads it before `app.js`.
   - The CSP test does not change.
4. Browser, manual check after the build:
   - Drag with the mouse at desktop width.
   - Drag with touch emulation at phone width.
   - Move items with Arrow Up and Arrow Down, with a screen reader text check in the live region.
   - Drag with "Hide done" on, then show done items and check their place.

## 7. Docs

- `README.md`: a new line in "What it does", for example "Drag items to change their order. Overdue items stay at the top."
- `2026-10-01-todo-app-design.md`: a "Since 2026-10-12" line in the order rules, with a link to this spec.
