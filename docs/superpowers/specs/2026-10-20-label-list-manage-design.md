# Labels: several labels, colors and delete in the label list

Status: approved and built (2026-10-20).
Mockup: canvas "Todo app UI proposal", page "16 · Labels", board "NEW: label list – several
labels, color wheel, × deletes (no Labels page)".

This spec replaces the Labels page (`2026-10-14-labels-design.md`, the page part, and
`2026-10-20-label-color-wheel-design.md`). It changes the label list under + from
`2026-10-14-label-plus-design.md`.

## Why

The user wants to manage labels where they use them: in the list under + on an item. A separate
Labels page is one more place to go. So the page goes away, and the list under + gets the color
and delete actions. The user also wants to give an item several labels without opening the list
again for each one.

## What changes

### The list under +

- The list shows **all** the user's labels, not only the labels the item does not have.
- A label that the item has shows a blue **check** on the right.
- A click on a name **adds** the label to the item, or **takes it off** if the item has it. The
  list stays open. The search field gets empty and keeps the focus.
- Esc, a click outside the list, or a click on + closes the list. Focus that leaves the list
  closes it too.
- When the item has 5 labels, the names of the other labels are gray and do nothing. A line
  under the list says "An item can have at most 5 labels." The Create line does not show.

### Color

- Each line starts with a round **color circle** (14 px; 22 px on a touch screen; a larger
  click area). A click on it shows the **color wheel in the same pop-up**, in place of the list
  (option A on the canvas).
- The wheel view has a title "Color of “work”", a ← button, the wheel, a preview chip, and the
  buttons Done and Cancel. It works like the wheel of the old Labels page: the angle is the hue,
  the distance from the middle is the strength, arrow keys move it.
- While the user picks, the chips of that label on the page show the new color.
- **Done** saves the color at once and goes back to the list. **Cancel**, ← and Esc put the old
  color back and go back to the list. Esc in the wheel view does not close the pop-up.

### New label

- The Create line ("Create “gym”") has a color circle too. It shows the color that the new label
  will get: the next color of the fixed list (`todo.LabelColors`), as today.
- A click on that circle shows the wheel with the title "Color of the new label “gym”". Done
  goes back to the list, and the circle shows the chosen color. Nothing is saved yet.
- A click on "Create “gym”" makes the label with the circle's color and adds it to the item.
- If the user changes the typed name, the chosen color stays for the new name.

### Delete

- On a computer, an **×** shows at the right end of a line when the pointer is on the line or
  the focus is in it. On a touch screen the × always shows (36 px).
- A click on × changes the line to a question: "Delete “urgent”? It comes off 1 item. The items
  stay." with the buttons **Delete** and **Cancel**. With no items it says "No items use it."
- Delete deletes the label: it comes off all items, and the list shows without it. Cancel shows
  the line again.

### No rename

There is no way to rename a label. To change a name, the user deletes the label and creates a
new one.

### The Labels page goes away

- The link "Labels" in the top bar goes away.
- `GET /labels` sends the browser to `/` (old bookmarks keep working).
- `POST /labels/{id}` and `POST /labels/{id}/delete` go away, with `labels.html`.
- The chip × on a row stays: it takes the label off that item, as today.

## Routes

| Route | Does |
| --- | --- |
| `GET /items/{id}/labels` | The list for + (htmx fragment): all labels, with checks. |
| `POST /items/{id}/labels` | Form fields `name` and, for a new label, `color` (optional, `#rrggbb`). Adds the label, making it if needed. A `color` for a label that exists already changes nothing. |
| `DELETE /items/{id}/labels/{label}` | Takes the label off the item. |
| `PUT /labels/{id}/color` | Form field `color`. Saves the color. Answers 204. A bad color answers 422, an unknown label 404. |
| `DELETE /labels/{id}` | Deletes the label. Answers the list section. |
| `GET /labels` | Redirect to `/`. |

### Answers that keep the list open

- A click on a name sends `POST` or `DELETE /items/{id}/labels…`. The answer is the row, as
  today. The page moves the open list into the new row, so it stays on the screen. Then it loads
  the list again (`GET /items/{id}/labels`) and puts the focus in the search field.
- The chip × on a row works as today: its answer is the row, and no list opens.
- If the row leaves a filtered list (the user took off the filter's label), the list closes.
- `DELETE /labels/{id}` answers the whole list section (as "Hide done" does). The page puts the
  open list back into the row of its item in the same way. If the deleted label is the list's
  filter, the answer sends the browser to `/` instead (`HX-Redirect`).
- `PUT /labels/{id}/color` answers 204 with no body. The page itself changes `data-color` on
  each element of that label (chips on rows, the filter chip, the circle). Each such element
  gets `data-label="{id}"`. If the save fails, the page puts the old color back.

## Rules

- `todo.Service.AddLabel` gets a `color` argument. If the label is new and the color is a good
  `#rrggbb`, the label gets that color; else it gets the next fixed color.
- The name and limit rules stay: at most 30 characters, no comma, at most 5 labels per item and
  50 per user. Errors show in the list as today. The 50-labels message changes from "You have 50
  labels. Delete one on the Labels page first." to "You have 50 labels. Delete one first (× on
  its line)."
- `UpdateLabel` (name and color) becomes `SetLabelColor` (color only), because rename goes away.

## Keyboard and screen readers

- The list is a plain list of lines, no longer a `listbox`. Each name is a toggle button with
  `aria-pressed` ("true" when the item has the label). The color circle and the × are buttons
  with `aria-label` "Color of “work”" and "Delete label “work”".
- Tab moves through the search field and the buttons of the pop-up. Focus that leaves the pop-up
  closes it.
- In the search field, ↓ moves the focus to the first shown name; ↑ and ↓ move between names.
  Enter in the search field toggles the first shown line, or creates the typed label.
- The delete question has `role="alert"`; its Delete button gets the focus. Cancel puts the
  focus back on the line's ×.
- The wheel is a `slider` as on the old Labels page. When the wheel view opens, it gets the focus.
  When it closes, the focus goes back to the color circle that opened it.

## Code

- `item.html`: the `label-picker` template shows the new lines, the check, the circles, the ×,
  the Create line with its circle, and the wheel view (hidden until used).
- `app.js`: choose toggles and keeps the list open; the keyboard rules above; the delete
  question (built in the page); the color requests.
- The wheel code moves from `labels.js` to a file that the main page loads (for example
  `wheel.js`). `labels.js` and the labels page CSS go away.
- `README.md`: "What it does" says that the list under + adds several labels and changes and
  deletes labels; the Labels page is gone.

## Tests

- Web: the list shows all labels with `aria-pressed`; create with a color; `PUT /labels/{id}/color` (good, bad, other user's label); `DELETE
  /labels/{id}` (answer with the list open, filter label gives `HX-Redirect`); `GET /labels`
  redirects; the top bar has no "Labels" link.
- Service: `AddLabel` with a color for a new label and for an existing label;
  `SetLabelColor`.
- A script test (as in `order_script_test.go`) checks the wheel math in `wheel.js` with node.
- By hand: run the app and try each frame of the canvas board on a computer and at 375 px.
