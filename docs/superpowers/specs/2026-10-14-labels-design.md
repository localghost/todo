# Labels: design spec

Date: 2026-10-14
Status: approved and built
Builds on: `2026-10-01-todo-app-design.md`, `2026-10-12-drag-order-design.md`

## 1. Goal and scope

Each user can put labels such as "work", "home" or "urgent" on items. Labels show as small coloured chips on the row. A click on a chip shows only the items with that label. A labels page lets the user rename, recolour and delete labels.

These items are not part of this feature:

- A built-in priority with a meaning or an order. "Urgent" is just a label.
- Filtering by more than one label at once (AND / OR).
- Sorting by label. The order stays the drag order, with overdue items on top.
- Inline `#tag` syntax in the item text.
- Shared labels between users.

## 2. Decisions

| Topic | Decision |
|---|---|
| Owner | Labels belong to one user. Two users can both have "work". |
| Name | 1 to 30 characters after whitespace is collapsed (same rule as item text). Unique per user, ignoring case: "Work" and "work" are the same label. The first spelling is kept. |
| Colour | One of 8 fixed colours (gray, red, orange, yellow, green, teal, blue, purple). A new label gets the next colour in turn. The user can change it on the labels page. No free hex colours: a fixed set maps to CSS classes, works in dark and light mode, and keeps contrast readable. |
| Limits | At most 5 labels per item and 50 labels per user. Over the limit, the form shows an error, like an empty text does. |
| How to set labels | A "Labels" text field in the add form and in edit mode, below "Due". Names are separated by commas, for example `work, urgent`. A name that does not exist yet creates the label. Under the field, the user's existing labels show as small buttons; a click adds that name to the field (a few lines in `app.js`; the field also works without it). |
| On the row | Chips go in the meta line, after the due date. Each chip is a link to the filtered list. |
| Filter | `/?label=<id>`. htmx requests from a filtered page carry the page address in `HX-Current-URL`, so row and toolbar answers are filtered too. The toolbar shows "Label: work ×"; × goes back to the full list. The filter lives in the URL, not in the user settings, so a reload or a bookmark keeps it, and a new tab starts unfiltered. It works together with "Hide done". |
| Adding while filtered | The add form fills in the active label, so the new item shows in the filtered list. |
| Drag while filtered | Works like drag with "Hide done" on: the browser sends "put X after Y", and hidden items keep their place relative to the rows around them. No change to the move code. |
| Counter | With a filter, the counter counts only the matching items ("2 open · 1 done"). |
| Unused labels | They stay until the user deletes them on the labels page, so a label is not lost when its last item is deleted. |
| Delete a label | Removes the label from all its items. The items stay. The page asks for confirmation first. |
| Rename to an existing name | Refused with an error message. No merging in this step. |
| Done and overdue items | Keep their labels and chips. The overdue rule does not change. |

## 3. Pages and routes

| Route | What it does |
|---|---|
| `GET /?label=<id>` | The list, filtered. An unknown id or another user's id shows the full list. |
| `POST /items`, `PUT /items/{id}` | Gain a `labels` form field (comma list). |
| `GET /labels` | Labels page: each label with its chip, item count, a name field, a colour select, Save and Delete. Linked from the header: "Signed in as … · Labels · Account · Log out". |
| `POST /labels/{id}` | Save name and colour. |
| `GET /labels?delete=<id>` | The labels page with the delete question in that row. No JavaScript is needed. |
| `POST /labels/{id}/delete` | Delete the label. |

The labels page is a plain form page like the account page, with htmx where the rest of the app uses it.

On a wide screen each label row has name, colour dots, Save and Delete on one line. Below 480 px each row stacks: chip and count, name field at full width, colour dots spread across the row (36 px tap areas; eight 44 px targets do not fit at 390 px), then Save and Delete as two equal buttons.

## 4. Parts

### Model (`internal/todo`)

```go
type Label struct {
    ID     int64
    UserID int64
    Name   string
    Color  string // one of LabelColors
}

type Item struct {
    // … as now …
    Labels []Label // sorted by name
}

type Change struct {
    // … as now …
    Labels     []string // names; missing labels are created
    KeepLabels bool     // UpdateItem only: leave the labels alone (Edit, Postpone)
}

type Input struct { Text, DueDate, DueTime, Labels string } // the form fields
```

The service cleans and checks the names (length, limit, duplicates in one field ignoring case). All rules stay in `todo.Service`, as the first spec requires.

New `Store` methods: `Labels(ctx, userID)` (with item counts), `Label`, `UpdateLabel`, `DeleteLabel`, and `ListLabel` / `CountsLabel`, which are `List` / `Counts` for one label. `Create` and `UpdateItem` save the labels in the same transaction as the item and create missing labels. New service methods `AddItem` and `EditItem` take an `Input`; `Add` and `Edit` keep working and leave the labels alone.

### Schema (version 4)

```sql
CREATE TABLE labels (
    id         INTEGER PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name       TEXT    NOT NULL COLLATE NOCASE,
    color      TEXT    NOT NULL,
    created_at TEXT    NOT NULL,
    UNIQUE (user_id, name)
);
CREATE TABLE item_labels (
    item_id  INTEGER NOT NULL REFERENCES items(id)  ON DELETE CASCADE,
    label_id INTEGER NOT NULL REFERENCES labels(id) ON DELETE CASCADE,
    PRIMARY KEY (item_id, label_id)
);
CREATE INDEX item_labels_label ON item_labels (label_id);
```

`migrateToV4` follows the `migrateToV3` pattern. Old items get no labels. Deleting an item, a label or an account removes the links through the cascades.

`List` loads the items as now, then the labels of those items in one second query, and attaches them. Lists are small, so two queries are cheap and keep `scanItem` simple.

### Web (`internal/web`)

- `item.html`: chips in the meta line; edit mode gets the Labels field.
- `form.html`: Labels field and the suggestion buttons.
- `list.html`: the toolbar shows the active filter.
- New `labels.html` page and handlers.
- `app.css`: one class per colour (soft fill, dark text, thin border). The app has only a light theme.

### Safety

- Every label query has `user_id = ?`, like items. A label id from another user acts like an unknown id.
- Names are shown through `html/template`, so they are escaped.
- Colours come from the fixed list, so a class name never holds user text.
- The new POST routes are behind `protect`, like the other routes.

## 5. UI

Approved in Claude Design, page "16 · Labels" of the Todo app UI proposal canvas.

## 6. Tests

Same style as now (Go tests, no browser tests):

- Store: create with labels, reuse ignoring case, update replaces the labels, postpone keeps them, delete cascades, filter by label, counts with a filter, another user's label is never visible.
- Service: name cleaning, length, the 5 and 50 limits, duplicates in one field.
- Web: chips on the row, the filter and its × link, add while filtered, the labels page (rename, recolour, rename clash, delete), and the migration from version 3 with existing items.

## 7. README

"What it does" gets one line: "Put labels on items and show only the items with one label."
