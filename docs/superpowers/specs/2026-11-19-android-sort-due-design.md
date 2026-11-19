# Android: sort by hand or by due date

Date: 2026-11-19
Status: approved 2026-11-19 (S3, the sort button); built.
Issue: #17 (and #1, overdue items in the hand order)
Builds on: `2026-10-22-android-gestures-design.md` (hold to move)
Mockup: canvas "Todo app UI proposal", page "24 · Android sort".

## 1. Goal

The Android app gets the two orders of the web app: **by hand** (the order you make by dragging)
and **by due date**. The choice is one setting per user, shared with the web: switch it on the
phone and the web shows the same order, and the other way round.

The order rules are the web's (`todo.DueOrder`), not a copy of them in Kotlin:

1. The overdue items, the most overdue first.
2. The other items with a due date, the earliest first. An all-day item counts as due at the
   end of its day, so it comes after the items with a time on that day.
3. The items without a due date.

Items with the same due moment, and the items without a due date, keep the hand order. Done
items keep their place in this order too.

## 2. Where the switch is

**S3 (recommended): a sort button right of the "All · Today" switch.** It shows the order that is
on, with a sort icon: "By hand" (white) or "By due date" (light blue). One tap switches. It uses
the empty space next to the switch, so the top bar and the list height do not change. TalkBack
reads "Sorted by hand. Tap to sort by due date" (and the other way round).

**S1: a line in the ⋮ menu**, above "Hide done items", saying what a tap does ("Sort by due
date" / "Sort by hand"), plus a gray line above the list in due date order: "Sorted by due date.
To move items, choose “Sort by hand” in the ⋮ menu." Two taps to switch.

**S2: a sort icon in the top bar**, left of the label filter, on a light blue circle in due date
order. One tap, but a fourth icon in the top bar, and an icon says less than words.

## 3. In due date order

- Holding a row does not lift it. It gives the short double buzz of a refused hold and a
  snackbar "Sorted by due date. Tap “By due date” to sort by hand and move items." Swipes work
  as before. (S1 and S2 also get the gray line of section 2.)
- A new item, a new or removed due date, a postponed item, and done / not done go to their
  place at once (the app loads the list again after each change, as now).
- An item that becomes overdue moves to the top when the list loads again: on the next change,
  pull to refresh, or when the app comes back to the screen.
- The Today view and the label filter work in both orders.
- Without a connection the saved list keeps the order it had at the last load.

## 4. By hand (fixes #1)

The web keeps overdue items where you put them (since ea45790). The app still keeps them at the
top and refuses to lift them ("Overdue items stay at the top."). With this change the app does
what the web does: in the hand order an overdue row lifts and moves like any other row, and the
snackbar goes away. The date stays red.

## 5. API

- `GET /api/v1/me` and `PUT /api/v1/settings` get `sort_due` (bool), like `hide_done`.
- `GET /api/v1/items` answers in due date order when the user's `sort_due` is on, and the answer
  gets `"sort_due": true|false`, so the app knows which order it shows.
- `POST /api/v1/items/{id}/move` answers 409 `sort_due` ("The list is sorted by due date.") while
  the setting is on, as the web refuses a move then. The app then loads the list again.
- An older app (1.1.0) ignores the new field. While `sort_due` is on it would show the due date
  order with drag on, and a drag gets the 409 and an error snackbar. That is acceptable for the
  short time until the update.

No schema change: `users.sort_due` exists.

## 6. Tests and docs

- Go: API tests for `sort_due` in `/me`, `/settings`, the items order and the refused move.
- Android: view model tests for the setting and the refused hold; Paparazzi shots of the menu and
  the due date order with the sort button.
- User guide (Android part) and the README feature list. Android screenshots with
  `mise run android:shots`.
- The API change needs a deploy before the new app is used; the APK is built and signed on
  Zbyszek's computer.
