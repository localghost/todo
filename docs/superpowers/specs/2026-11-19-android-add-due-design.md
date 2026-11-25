# Android: a due date on a new item

Date: 2026-11-19
Status: approved 2026-11-19 (D3); built.
Builds on: `2026-10-21-android-app-design.md` (the add field), the Android Today view (#15),
`2026-10-15-due-popup-design.md` (the web's due date for a new item)
Mockup: canvas "Todo app UI proposal", page "25 · Android due date on add".

## 1. Where we are

- **Web: already done.** The calendar icon under the add field opens the due date pop-up (date,
  time, repeat) for the new item, and the date can also be typed at the end of the text
  ("Call the dentist due tomorrow 10:00", "Pay the rent @12 Nov"). Nothing changes on the web.
- **Android: only in the Today view.** There the new item gets today, all day, as a "Today ×"
  chip in front of the label chips. In the "All" view there is no way to give a new item a date:
  you add it, tap the row, and set the date in the item sheet. Two extra steps for each item.
- **API:** `POST /api/v1/items` takes `due_date` (all day) only. The server's `todo.Input`
  already has `DueTime` and `Repeat`, so the change is small.

## 2. Goal

While you type a new item on the phone you can give it a date, and a time if you want, in one
or two taps. The common cases (today, tomorrow) take one tap. The rules are the web's: no time
means all day, a past date is allowed and shows red.

Repeat stays out: the Android app does not show or set repeat rules anywhere yet (also not in
the item sheet). It can come with a separate "repeat on Android" change.

## 3. Options

All three put the result in the same place: a date chip, first in the chip row under the field
(where "Today ×" is now), blue when set, with × to remove it. They differ in how you pick.

**D1 (recommended): a "Date" chip in the chip row, with quick choices.**
- While the keyboard is open, the chip row always starts with an outlined chip "📅 Date" (also
  when you have no labels, so the row now always shows while you type).
- Tap it: a small menu opens above the chip: **Today**, **Tomorrow**, **Pick a date and time…**.
  Today and Tomorrow set the date at once (all day). "Pick a date and time…" opens the date
  picker of the item sheet, and after OK its time picker at once ("All day" keeps no time).
  (Changed 2026-11-25 at Zbyszek's request: one go for a date and a time.)
- With a date, the chip turns blue: "Tomorrow ×". Tap the chip (not the ×) for the menu again,
  now with one more line, **Set a time…** (the time picker of the item sheet, with its "All day"
  button). Then the chip says "Tomorrow, 10:00 ×".
- The × removes the date (and time) from the new item. In the Today view the chip starts as
  "Today ×", as now; the × turns it back into "Date".
- After a pick the focus goes back to the field, so the keyboard and the chip row come back and
  you can keep typing.
- Cost: none in the layout. The date sits with the labels, as on the web.

**D2: a calendar button between the field and +.**
- Always shown, also without the keyboard. Tap: the date picker opens at once, then the chip
  row shows "12 Nov ×"; tapping the chip opens the time picker.
- One fewer step for a far date, but no one-tap "Tomorrow", and the field loses 48 dp of width
  (about 5 characters on a 360 dp phone).

**D3: D1 plus the typed date of the web.**
- As D1, and the end of the text is read as on the web: "due tomorrow 10:00", "@fri".
  The words get a blue underline (Compose has no dotted underline, so the app draws the words
  blue with a plain underline) and the chip shows the date. The × on the chip keeps
  the words as text (the app does not take them again), as on the web.
- The Kotlin parser is a port of `parseTypedDue` (app.js) and is tested against the same cases
  (`internal/web/testdata/typed-due.json`), so the two cannot drift.
- Most work of the three. It can also come later, on top of D1.

## 4. Details (all options)

- The chip text uses the words of the rows: "Today", "Tomorrow", "Yesterday", "23 Nov", "12 Jan 2027",
  with ", 10:00" for a time. A date in the past is red ("Yesterday ×"), as on the web.
- After the add, the field, the labels and the date go back to the start (none, or "Today" in
  the Today view). If the add fails, everything stays, as now.
- An item added in the Today view with a later date (for example tomorrow) is added, and does
  not show in the view. A short snackbar says so: "Added. It is due tomorrow, so it is not in
  Today." (D1/D2/D3 alike.)
- The new item goes where it goes now: at the end in the hand order, at its place in the due
  date order (see the parallel design "Android: sort by hand or by due date").
- A due time in the future sets a reminder at once, as with a date set in the item sheet.
- TalkBack: the chip reads "Due date: none. Double tap to pick" or "Due date: tomorrow,
  10:00. Double tap to change"; the × reads "Remove the due date from the new item".

## 5. API

- `POST /api/v1/items` gets `due_time` ("HH:MM" or "HHMM", empty = all day), next to
  `due_date`, in the phone's zone (the `Time-Zone` header), like `PUT /api/v1/items/{id}/due`.
  A time without a date is refused with `bad_due` (422).
- Old app versions send no `due_time` and work as before.

## 6. Tests

- API: add with date and time, with date only, with time only (refused), in another zone.
- Android unit tests: the chip text (today, tomorrow, weekday, other year, past, with time),
  the reset after add, the Today view start value.
- Paparazzi screenshots: the chip row with "Date", the quick menu, "Tomorrow, 10:00 ×", the red
  past date. (D3 also: the parser against the shared cases, and the dotted line.)
- `docs/user-guide.md` (Android section) and the README feature list get the new step, with a
  new `android-addDue` screenshot.

## 7. Open questions

1. D1, D2 or D3?
2. Should the quick menu have more lines (for example "Next Monday", "In a week"), or stay at
   Today / Tomorrow / Pick a date?
