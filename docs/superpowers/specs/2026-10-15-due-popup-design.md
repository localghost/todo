# Due date: a calendar icon and a pop-up on each item

Note (2026-10-21): the calendar icon and the + now always show on every row, also on a computer
(canvas page "16 · Labels", board "NEW: calendar icon and + always shown", option A).
Note (2026-10-21): the add form has no due date fields any more. Set a due date with the calendar
icon on the row (canvas page "10 · Due date", board "NEW: add form without the due date").

Status: approved and built (2026-10-15). Replaces the Due fields of edit mode in
`2026-10-01-due-dates-design.md`. The add form does not change.
Mockups: canvas "Todo app UI proposal", page "10 · Due date", board "NEW: calendar icon and due
date pop-up", option A.

## What changes

### Row

- The gray line of a row is, in this order: the due text, the calendar icon, the labels, the +.
- The due text becomes a button. It keeps its look and color (red for overdue). A pointer on it
  shows an underline. Its tooltip is "Change the due date".
- The calendar icon is a 12 px gray (`#8C8A84`) calendar outline in the same 16 px box as the +,
  with a 24 px click area. On a touch screen the box is 20 px and the click area is 32 px.
  Pointing at it shows the light gray disc of the +. While its pop-up is open it is blue.
- On a computer the icon shows when the pointer is on the row or focus is in it. When it is
  hidden, it keeps its place, so nothing moves on hover (option A). On a touch screen it always
  shows.
- The icon is on every row: with or without a date, open, done, and overdue. The "+5 min" …
  "+30 min" buttons of an overdue row stay as they are.
- Its accessible name is "Set the due date of “text”". It has `aria-haspopup="dialog"` and
  `aria-expanded`.

### Pop-up

- A click on the icon or on the due text opens the pop-up right under the icon, with a pointer at
  the icon. A second click on the same icon or due text closes it.
- If the pop-up does not fit to the right of the icon, it opens to the left. If it fits neither
  way (a narrow phone), it moves in from the screen edge, and the pointer still points at the
  icon. This is the same placement as the label list.
- Only one pop-up is open at a time. Opening the due pop-up closes an open label list, and the
  other way around.
- Contents, from top to bottom:
  1. the title "Due date";
  2. the date field and the time field (the same `due-time` field as the add form: 24-hour
     `HH:MM`, optional);
  3. the hint "Leave the time empty for “all day”", or the error message in its place;
  4. the Save button, and the Remove date button. Remove date shows only when the item has a
     due date.
- The fields show the item's current date and time in the browser's time zone. A row without a
  date shows empty fields.
- When it opens, the focus goes to the date field. Enter in a field saves. Tab and Shift+Tab move
  between the fields and the buttons.
- Esc closes the pop-up without saving, and the focus goes back to the icon. A click outside, or
  focus that goes out of the pop-up, also closes it without saving.
- Save stores the date and time. Remove date removes the due date. In both cases the pop-up
  closes, the row shows the new due text, and the focus goes back to the calendar icon of the
  row.
- If the change moves the row, the whole list updates. For example, an item that becomes
  overdue moves to the top, and an overdue item that gets a later date leaves the top.
- A wrong value keeps the pop-up open with the values the user typed. The message comes from the
  existing due date rules, for example "Pick a day for the due date." for a time without a date,
  or "Please choose a valid date and time.". As in edit mode now, the date field gets
  `aria-invalid` and a red border, and the message is linked to it by `aria-describedby`. The
  time field also turns red on its own when its value does not match `HH:MM`.
- If the row changes while the pop-up is open (for example, another request answers with the
  row), the pop-up closes.

### Edit mode

- A click on the text opens edit mode with only the text field. The Due line and its Clear
  button go away. The hint is "Enter to save · Esc to cancel".
- Saving the text does not change the due date or the labels.

### Add form

- No change. It keeps its Due fields.

## Routes

| Route | Does |
| --- | --- |
| `GET /items/{id}/due` | The pop-up for the calendar icon (htmx fragment). The fields show the current due date and time in the browser's time zone, and a hidden `due_tz` field holds that zone. |
| `PUT /items/{id}/due` | Form fields `due_date`, `due_time`, `due_tz`. Sets the due date; both fields empty removes it. Answers the row and the toolbar, or the whole list if the row moves (`placeChanged`). A wrong value answers 422 with the pop-up and its message (`HX-Retarget` the pop-up). An unknown item answers 404, as the other item routes do. |

`PUT /items/{id}` changes only the text now. It ignores `due_date` and `due_time`.

## Rules

- `todo.Service.SetDue(ctx, userID, id, date, clock, loc)` changes only the due date. It uses the
  existing `parseDue` rules and messages, in the zone `loc` (the `due_tz` field, or the
  browser's zone if that field is missing or wrong). A due date that changes resets "notified",
  so the browser can remind again. The same date changes nothing.
- `todo.Service.EditText(ctx, userID, id, text)` changes only the text. Empty text is
  `ErrEmptyText`, as now. The due date, its "notified" state, and the labels stay.
- `todo.Service.Edit` and `EditIn` go away. Their tests move to the two new functions.

## Code

- `app.js`: the open, close and place code of the label list becomes shared code for both
  pop-ups. The search and option code stays only for the label list. The due pop-up adds Enter
  to save and "close when the focus leaves". The label list keeps "Tab closes it".
- `item.html`: the due text becomes a button, the calendar icon follows it, and edit mode loses
  the Due line. A new template `due-picker` is the pop-up.
- The README line about due dates in "What it does" names the calendar icon.
