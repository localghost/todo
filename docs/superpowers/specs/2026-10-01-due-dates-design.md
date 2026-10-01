# Due dates and notifications — design spec

Date: 2026-10-01
Status: waiting for review
Builds on: `2026-10-01-todo-app-design.md`

## 1. Goal and scope

Each item can have a due date: a day and an optional time. When an item becomes due, the browser shows a notification. This works while the todo page is open in a browser tab. The tab can be in the background.

The feature has these parts:

- Set a due date when you add an item, and change or remove it in edit mode.
- See the due date in the list. Overdue open items are red.
- Get one browser notification per item, at the due time.
- Turn notifications on with one click.

These items are not part of this feature:

- Reminders before the due time.
- Repeated notifications for overdue items.
- Notifications when no todo tab is open (service worker and Web Push).
- Sorting or filtering by due date.

## 2. Decisions

| Topic | Decision |
|---|---|
| Notification type | Browser notification (the Notification API) |
| When the page must be open | A todo tab must be open. A background tab is enough. |
| Timing | One notification per item, at the due time |
| Missed while no tab was open | The notification comes when the page is open again |
| Who decides what to notify | The server (approach A). The page asks every 30 seconds. The server returns due items and marks them as notified in the same step. |
| Due date | A day, plus an optional time. Without a time: due at 09:00 local time ("all-day"). |
| Set the date in an existing item | In edit mode, a "Due" line under the text field (canvas page 10, part 1, option A) |
| Show the date | In the grey line under the text, after the added date (part 2, option A) |
| Permission | A bar under the toolbar, only when useful (part 3) |
| Set the date when adding | The add form always shows a "Due" line under the text field (part 5, option A) |

UI reference: canvas page "10 · Due date" in https://claude.ai/artifact/7mnRhC4Jd8jbnbAhjN7Woq

## 3. Data

The `items` table gets three new columns:

| Column | Meaning |
|---|---|
| `due_at TEXT NULL` | The due moment in UTC, in RFC 3339 format. `NULL` means no due date. |
| `due_all_day INTEGER NOT NULL DEFAULT 0` | `1` if no time was given |
| `notified_at TEXT NULL` | When the item was claimed for a notification. `NULL` means not yet. |

The schema setup adds each column at start, but only if it is missing. It reads `PRAGMA table_info(items)` and runs `ALTER TABLE items ADD COLUMN …` for the missing ones. Existing data stays, and the setup stays idempotent.

`todo.Item` gets three new fields:

```go
DueAt      *time.Time // nil: no due date
DueAllDay  bool
NotifiedAt *time.Time // nil: not notified yet
```

## 4. Rules (service)

The service reads two text inputs, in the server's local time zone:

- `due_date` in the format `YYYY-MM-DD`, for example `2026-10-03`.
- `due_time` in the format `HH:MM`, for example `14:00`. It is optional.

The browser's date and time inputs send these formats.

| Input | Result |
|---|---|
| Both empty | No due date |
| Date only | Due at 09:00 local time on that day, all-day |
| Date and time | Due at that time |
| Time only | `ErrBadDue` with the message "Pick a day for the due date." |
| Bad format | `ErrBadDue` with the message "Please choose a valid date and time." |

A date in the past is allowed. The item then notifies on the next poll.

Each change of the due date (set, change, or remove) sets `notified_at` to `NULL`. So a changed item can notify again.

Service methods:

```go
func (s *Service) Add(ctx context.Context, userID int64, text, dueDate, dueTime string) (Item, error)
func (s *Service) Edit(ctx context.Context, userID, id int64, text, dueDate, dueTime string) (Item, error)
func (s *Service) ClaimDue(ctx context.Context, userID int64, now time.Time) ([]Item, error)
```

- `Add` checks the text and the due date before it writes anything.
- `Edit` replaces `UpdateText`. It checks the text and the due date before it writes anything. If nothing changed, it does not write (the existing rule).
- `ClaimDue` runs in one database transaction. It finds open items of the user with `due_at <= now` and `notified_at IS NULL`. It sets `notified_at = now` on them and returns them. If two tabs ask at the same time, each item goes to one tab only.

`ErrBadDue` is a new error type that carries the message for the user. `errors.Is(err, todo.ErrBadDue)` works for both messages.

## 5. Labels

The due label goes after the added date in the grey line, for example "Added 28 Sep · Due 3 Oct".

| Case | Label |
|---|---|
| Due today, with time | Due today, 14:00 |
| Due tomorrow, with time | Due tomorrow, 14:00 |
| Due later this year | Due 3 Oct (with ", 14:00" if a time is set) |
| Due in another year | Due 3 Oct 2027 (with the time if set) |
| All-day item | Same as above, but never with a time |
| Overdue, open item | Overdue since today, 09:00 · Overdue since yesterday · Overdue since 28 Sep (red) |
| Done item | The normal "Due …" label in grey, never red |

An item with a time is overdue when `now > due_at`. An all-day item is overdue only after its day ends. Its notification still comes at 09:00 on that day, as a reminder.

## 6. UI

### Add form

The add form always shows a second line under the text field: "Due [date] [time]". Both inputs are empty by default, and both are optional. Enter in any field of the form adds the item.

After a successful add, `app.js` clears each field only if it still holds the value that was sent. This is the existing rule for the text field, extended to the two due fields.

If the due input is wrong, the server answers `422`. The error message shows under the due line, the form keeps all the values, and the cursor goes to the date field.

### Edit row

The edit row shows the text field and, under it, the line "Due [date] [time] Clear". "Clear" empties both due fields.

| Action | Result |
|---|---|
| Enter in any field of the row | Save the text and the due date |
| Focus leaves the row (a click or Tab outside it) | Save. This replaces the old blur trigger of the text field, because moving between the fields of the row must not save. |
| Esc | Cancel |
| Focus leaves the row with empty text | Cancel (existing rule) |
| Wrong due input | `422`. The row stays open with the message under the due line, and the cursor goes to the date field. |

### Permission bar

A bar `#notify-bar` sits under the toolbar. The server renders it with `data-has-due="true"` or `"false"`. The value is `true` if at least one open item has a due date. The bar is updated out-of-band together with the toolbar.

`app.js` fills the bar based on the browser's permission:

| Permission | Bar content |
|---|---|
| `default` (not asked yet), and `data-has-due="true"` | "Get a notification when an item is due." and the button "Turn on notifications" |
| `default`, and `data-has-due="false"` | Empty, so the bar is hidden |
| `denied` | Grey text: "Notifications are blocked in your browser settings." |
| `granted` | Empty, so the bar is hidden |

The button calls `Notification.requestPermission()`. Browsers allow this only after a click. After the answer, the bar updates. If the answer is "granted", polling starts at once.

## 7. Routes

| Route | Change |
|---|---|
| `POST /items` | Also reads `due_date` and `due_time` |
| `PUT /items/{id}` | Also reads `due_date` and `due_time` |
| `POST /notifications/claim` | New. Returns JSON. |

The claim response is a JSON list:

```json
[{"id": 3, "title": "Todo: due now", "text": "Call the dentist", "due": "Due today, 14:00"}]
```

An empty list is `[]`. The route is a changing request, so the same-origin check applies. `app.js` sends the header `HX-Request: true`, like htmx requests.

## 8. Notifications (app.js)

`app.js` asks `POST /notifications/claim` at these moments:

- When the page loads.
- Every 30 seconds.
- When the tab becomes visible again (`visibilitychange`).

It asks only if `Notification.permission` is `"granted"`. Otherwise the server would mark items as notified that you never saw.

For each returned item, it shows one notification:

```js
new Notification("Todo: due now", { body: "Call the dentist — Due today, 14:00", tag: "todo-3" })
```

The server chooses the title and the due text. If the item is overdue and more than 5 minutes have passed since its due time, the title is "Todo: overdue" and the text is the overdue label, for example "Overdue since yesterday, 14:00". Otherwise the title is "Todo: due now" and the text is the "Due …" label.

A click on the notification calls `window.focus()`, so the todo tab comes to the front.

### Postpone

Overdue open items show four small buttons under the grey line: "+5 min", "+10 min", "+15 min", "+30 min" (canvas page "11 · Postpone", option B, only on overdue items). Other items show none.

A click sends `POST /items/{id}/postpone?minutes=10`. The new due time is the later of the current due time and now, plus the amount. For an overdue item this is now plus the amount. The item becomes a timed item (`due_all_day = 0`), and `notified_at` is reset, so it notifies again at the new time. The answer is the updated row plus the out-of-band toolbar and permission bar. After the click, focus moves to the row's checkbox, because the buttons disappear.

| Case | Status |
|---|---|
| Amount not 5, 10, 15, or 30 | `400` |
| Item unknown | `404` (the row is removed) |
| Item done or without a due date | `422` (the row is shown unchanged) |

## 9. Errors and edge cases

| Case | Behavior |
|---|---|
| The claim request fails (server stopped) | No notification. The next poll tries again. The existing error line shows the network error. |
| An item is deleted or done before the due time | `ClaimDue` does not return it |
| Two tabs are open | Each item goes to one tab only |
| An item is undone after it was notified | No new notification. Only a change of the due date resets it. |
| Server and browser are in different time zones | The server's local time is used for the labels and for 09:00 |

## 10. Testing

| Layer | What the tests check |
|---|---|
| Service | Date and time parsing, the 09:00 rule, removing a due date, `notified_at` reset on change, no write when nothing changed, `ErrBadDue` messages |
| Store | Migration of a database file from before this feature (old columns only, existing items keep their data). `ClaimDue` skips done items, future items, and notified items. A second claim returns nothing. |
| Labels | A table test, like the one for `addedLabel`: today, tomorrow, later, other year, all-day, overdue, done |
| Handlers | Add and edit with a due date, `422` for bad input, the claim JSON, the same-origin check on claim, `data-has-due` on the bar |
| Browser (user) | The pop-up, the permission bar, saving when focus leaves the edit row, the add form |

---

Co-authored with [Claude Code](https://claude.com/claude-code).
