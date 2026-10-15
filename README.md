# Todo

A small todo list for the browser. Everyone has their own account and their own list.

**Open the app: https://toodoo.fly.dev**

## What it does

- Add, edit, check off and delete items. Hide the done ones.
- Give an item a due date and time when you add it, or later with the calendar icon on each
  item (or a click on its due date). The browser can remind you, and you can postpone by
  5, 10, 15 or 30 minutes.
- Overdue items move to the top of the list.
- Drag items by the grip (⋮⋮) to change their order, or use the arrow keys. Overdue items
  stay at the top.
- Put labels such as "work" or "home" on items with the small + on each item, which opens a
  searchable list of your labels; point at a label to remove it with its ×. Click a label to see
  only its items. Rename, recolor or delete labels on the Labels page.
- Times are shown in your browser's time zone.

## Run it yourself

You need Go 1.27.

    go run ./cmd/todo

Then open http://127.0.0.1:8811. Settings are in `config.yaml`. To run it on Fly.io, see
[docs/deploy.md](docs/deploy.md).

## License

Apache License 2.0, see [LICENSE](LICENSE).
