# Labels: the + on each item

Status: approved and built (2026-10-14). Replaces the Labels field of `2026-10-14-labels-design.md`.
Mockups: canvas "Todo app UI proposal", page "16 · Labels", the boards marked NEW.

## What changes

- The add form and edit mode lose the Labels field and its suggestion buttons.
- Each row's grey line ends with a small grey **+** (9 px, `#8C8A84`, no circle; a 24 px click
  area, 32 px on a touch screen). On a computer it shows when the pointer is on the row or focus is
  in it, also on a row without labels. On a touch screen it always shows. Pointing at it shows a
  light grey disc; while its list is open it is blue. Source: option C on the canvas board "NEW:
  smaller gray +" (2026-10-14), which replaced the 24 px dashed blue circle.
- Clicking + opens a small list right under it, with a pointer at the +. At the top is a search
  field; below it, one per line, the user's labels that the item does not have. Typing narrows the
  list. A name that matches no label adds a last line "Create “name”".
- Choosing a line (click, or ↑ ↓ and Enter) adds **one** label and closes the list; focus goes back
  to the +. For another label the user clicks + again. Esc, Tab or a click outside closes the list.
- If the list does not fit to the right of the +, it opens to the left; if it fits neither way (a
  narrow phone), it moves in from the screen edge and the pointer still points at the +.
- An item with 5 labels shows "An item can have at most 5 labels. Remove one first." in the list.
- Each chip has a **×** that removes the label from the item (the label itself stays). On a
  computer the × shows when the pointer is on the chip or focus is in it; on a touch screen always.
- While the list is filtered by a label, new items get that label, and a line under the add form
  says so: "New items get the label …".

## Routes

| Route | Does |
| --- | --- |
| `GET /items/{id}/labels` | The list for the + (htmx fragment). |
| `POST /items/{id}/labels` | Form field `name`: adds that label, making it if needed. Answers the row and the toolbar. A bad name answers 422 with the list and its message (`HX-Retarget` the list). |
| `DELETE /items/{id}/labels/{label}` | Takes the label off the item. A row that loses the filter's label leaves the filtered list. |

`POST /items` ignores a `labels` field now; `PUT /items/{id}` keeps the item's labels.

## Rules

`todo.Service.AddLabel` and `RemoveLabel` hold the rules: the name is cleaned as for a rename (no
comma, at most 30 characters), a name the item has already (ignoring case) changes nothing, at most
5 labels per item, at most 50 labels per user.
