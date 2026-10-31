# TooDoo

A small todo list for the browser. Everyone has their own account and their own list.

**Open the app: https://toodoo.fly.dev**

User guide: https://toodoo.fly.dev/guide (no login needed; the text is in
[docs/user-guide.md](docs/user-guide.md)).

Privacy policy: https://toodoo.fly.dev/privacy (the text is in
[docs/privacy-policy.md](docs/privacy-policy.md)).

## What it does

- Add, edit, check off and delete items. After a delete, Undo shows for 8 seconds. Hide the
  done ones, or clear them all at once.
- Search the items with the magnifying glass or `/`. The search is fuzzy: "jul" finds "Julek",
  and Polish letters do not matter.
- Add notes to an item, for example an address, a phone number or a link. The notes icon on
  each item opens them, and links in them can be clicked. The × on the notes chip removes them,
  with Undo.
- Repeat an item every day, week, month or year. When you check it off, its date moves to the
  next time.
- Give a new item its due date and labels with the calendar icon and the + under the add
  field, as on each item.
- Give an item a due date and time with the calendar icon on each item. The date then shows
  as a chip, and its × removes it. Type the time as digits: 1430 becomes 14:30. The
  browser can remind you, and you can postpone by 5, 10, 15 or 30 minutes.
- Drag items by the grip (⋮⋮) to change their order, or use the arrow keys. Overdue items
  turn red and stay where you put them.
- Sort the list by due date with one click: overdue items first, then the nearest date, items
  without a date at the end. "Sort by hand" brings your own order back. The app remembers the
  choice.
- Put labels such as "work" or "home" on items with the small + on each item. It opens a
  searchable list of all your labels: click names to add or remove several labels, or create a
  new one. Point at a label on an item to remove it with its ×. Click a label to see only its
  items.
- Manage labels on the Labels page (the "Labels" link at the top): create a label, pick any
  color from a color wheel, delete a label, and see how many items use each one.
- Times are shown in your browser's time zone.
- An Android app on Google Play (https://play.google.com/store/apps/details?id=com.codeleet.toodoo) with the same list: add, edit, swipe right to check off, swipe left to delete
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
