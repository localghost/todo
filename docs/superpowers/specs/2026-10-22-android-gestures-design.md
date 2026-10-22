# Android: swipe right for done, hold to move

Date: 2026-10-22
Status: draft for Zbyszek's review. Nothing is built yet.
Builds on: `2026-10-21-android-app-design.md` (section 4.1, the list)
Mockup: canvas "Todo app UI proposal", page "18 · Swipe and hold".

## 1. Goal

The list rows lose the round check box and the grip (⋮⋮). The text starts at the left edge of
the row, so long items get one more word per line.

- Swipe a row **right** to mark it done (or not done, if it is done).
- Swipe a row **left** to delete it, as today.
- **Hold** a row (anywhere on it) to lift it, then drag it up or down.
- A short tap still opens the item sheet.

The web app does not change. No API or schema change.

## 2. The row

- Text, then the second line with the due date and label chips, as today.
- Done rows: gray and struck through, as today. With no check box, the strike-through is what
  tells done from open, and TalkBack says "done" (section 6).
- Overdue rows: the due date stays red. The red ring of the check box goes away with the box.

## 3. Swipe right

- While the finger moves, the row slides right and shows what is under it:
  - before the line (40 % of the row width): light blue (`#DCE5FA`), a small check, "Done";
  - past the line: full blue (`#2450C7`), a bigger white check, "Done", and one haptic tick
    when the line is crossed (and again if the finger goes back over it).
- Let go before the line: the row springs back, nothing changes.
- Let go past the line: the row slides back into place, turns gray and struck through, and the
  counter changes ("4 open · 2 done"). With "Hide done" on, the row slides out of the list.
- On a done row the same swipe shows gray (`#5C5A55`), an "undo" arrow and "Not done".
- Swipe left (delete) stays as it is: red, trash icon, "Delete".
- Blue for done and red for delete differ in lightness too, so they are not told apart by hue
  alone.

## 4. Undo

- After a swipe right a snackbar shows "“Send invoice to Kowalski” done" (or "… not done") with
  "Undo", for 5 seconds, like the delete snackbar.
- Unlike delete, the change is sent to the server right away (it is cheap to undo and the web
  should see it at once). Undo sends the opposite change.
- A new swipe while a snackbar shows replaces it; the earlier change stays.

## 5. Hold to move

- Hold a row still for the system long-press time (about half a second): a haptic tick, the row
  lifts (shadow, a 2 dp blue outline, slightly wider) and follows the finger up and down. The
  other rows make room. Let go to drop it; the new order is sent as today.
- Gesture rules: if the finger moves sideways before the hold time, it is a swipe; if it holds
  still first, it is a drag and sideways moves are ignored. Moving up or down before the hold
  time scrolls the list, as now.
- Near the top or bottom edge, the list scrolls while a row is lifted.
- Overdue rows do not lift (they stay at the top, as on the web). Holding one gives a short
  double buzz and a snackbar "Overdue items stay at the top." They can still be swiped.
- With a label filter on, drag works as today (the order is the user's one order).

## 6. Without gestures

Swipes and holds are hard to find and hard for some hands, so each has another way:

- **TalkBack:** the row is one node, read as "Send invoice to Kowalski, not done, due today
  14:30, money, work". Its actions: "Mark as done" / "Mark as not done", "Move up",
  "Move down" (not on overdue rows), "Delete", and double tap to edit. "Move up/down" and
  "Delete" exist today; "Mark as done" is new.
- **Item sheet:** a "Mark as done" (or "Mark as not done") button above Delete / Cancel / Save.
  It saves at once and closes the sheet.
- **Reminder notification:** keeps its "Done" button.
- **First start after the update:** a blue hint above the list: "Swipe a row right to mark it
  done, left to delete it. Hold a row to move it." with "Got it". It shows once.
- Switch Access and keyboards reach the same custom actions.

## 7. Changes

- `ListScreen.kt`: remove `CheckCircle` and the grip box; `SwipeToDismissBox` with both
  directions (start-to-end toggles and snaps back, never dismisses); the long-press drag moves
  from the grip to the whole row; the new "Mark as done" accessibility action; the hint; the
  done/not done snackbar with Undo; the overdue hold message.
- `ListViewModel.kt`: `toggle` returns the old state for Undo.
- `ItemSheet.kt`: the "Mark as done" button.
- Paparazzi screenshots (`mise run android:shots`) and the unit tests for the row actions.
- `docs/user-guide.md` (Android section), `README.md` ("What it does"), `docs/android.md`.

## 8. Questions for you

1. Blue for done, or another color (green would match "done" better but is close to red for
   some color-blind users)?
2. Swipe right sends the change at once with Undo (recommended), or waits for the snackbar to
   go, like delete?
3. Keep the one-time hint, or no hint?
