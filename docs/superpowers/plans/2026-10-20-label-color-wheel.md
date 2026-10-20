# Label color wheel Implementation Plan

**Goal:** Replace the eight fixed color dots on the labels page with a color button next to the
name field that opens a pop-up with a color wheel; a label can have any `#rrggbb` color.

**Spec:** `docs/superpowers/specs/2026-10-20-label-color-wheel-design.md`. Design: canvas "Todo app
UI proposal", page "16 · Labels", board "NEW: color wheel pop-up on the labels page".

## Tasks

- [x] **Model.** `todo.LabelColors` becomes the eight old colors as hex (new labels still get them in
  turn). `todo.CleanColor` accepts `#rrggbb` in any case and returns it in lower case;
  `UpdateLabel` refuses anything else with "Pick a color.".
- [x] **Schema v5.** `migrateToV5` maps the color names `gray` … `purple` to their hex colors (the
  old text colors' hue and saturation at 50 % lightness). Test: a v4 file with names upgrades,
  a hex color stays.
- [x] **Chips.** Templates write `data-color="{{.Color}}"` instead of the `c-<name>` classes.
  `static/chips.js` (on the list and the labels page) sets `--l-bg`, `--l-bd`, `--l-fg`, `--l-c`
  from it, and again for nodes htmx adds. CSS keeps gray as the fallback.
- [x] **Labels page.** Name field and color button on one line (`.label-name-line`), a hidden
  `color` input, one `#color-picker` pop-up (a `.picker`) that `static/labels.js` moves under the
  clicked button: wheel (pointer and arrow keys), preview chip, Done, Cancel; Esc and a press
  outside cancel. The swatch CSS goes.
- [x] **Docs.** Spec, this plan, the labels spec's Colour row, README.
- [x] **Check.** `go vet ./...`, `go test ./...`, and the page in Chromium at 1000 px and 390 px.
