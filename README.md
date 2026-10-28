# Todo

A small todo list for the browser. Everyone has their own account and their own list.

**Open the app: https://toodoo.fly.dev**

User guide: https://toodoo.fly.dev/guide (no login needed; the text is in
[docs/user-guide.md](docs/user-guide.md)).

Privacy policy: https://toodoo.fly.dev/privacy (the text is in
[docs/privacy-policy.md](docs/privacy-policy.md)).

## What it does

- Add, edit, check off and delete items. Hide the done ones, or clear them all at once.
- Give an item a due date and time with the calendar icon on each item (or a click on its
  due date). Type the time as digits: 1430 becomes 14:30. The
  browser can remind you, and you can postpone by 5, 10, 15 or 30 minutes.
- Overdue items move to the top of the list.
- Drag items by the grip (⋮⋮) to change their order, or use the arrow keys. Overdue items
  stay at the top.
- Put labels such as "work" or "home" on items with the small + on each item. It opens a
  searchable list of all your labels: click names to add or remove several labels, click a
  label's color circle to pick any color from a color wheel, and use its × to delete it. Point
  at a label on an item to remove it with its ×. Click a label to see only its items.
- Times are shown in your browser's time zone.
- An Android app on Google Play (https://play.google.com/store/apps/details?id=dev.toodoo) with the same list: add, edit, swipe right to check off, swipe left to delete
  (both with Undo), hold a row to drag it, due dates, labels and the label filter, label chips
  under the add field and in the item sheet to label an item with one tap, creating a label from
  the label list at the top, and
  reminders on the phone (also when the app is closed), with Done and Postpone buttons. See [docs/android.md](docs/android.md).

## Run it yourself

You need Go 1.27.

    go run ./cmd/todo

Then open http://127.0.0.1:8811. Settings are in `config.yaml`. To run it on Fly.io, see
[docs/deploy.md](docs/deploy.md).

To build the Android app you need Java 17 and the Android SDK; then `mise run android:build`.
See [docs/android.md](docs/android.md).

## License

Apache License 2.0, see [LICENSE](LICENSE).
