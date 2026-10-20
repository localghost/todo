# Labels page: a color wheel pop-up

Note (2026-10-20): the Labels page is gone; see `2026-10-20-label-list-manage-design.md`.

Date: 2026-10-20
Status: approved and built (2026-10-20)
Changes: `2026-10-14-labels-design.md` (the "Colour" row and the labels page)
Mockup: canvas "Todo app UI proposal", page "16 · Labels", board "NEW: color wheel pop-up".

## What changes

- The labels page no longer shows the eight color dots.
- Right after the name field of each label is a **color button**: a 26 px round swatch in the
  label's color with a thin border (36 px on a phone). Its name for screen readers is
  "Color of “work”", and pointing at it shows "Change the color".
- A click on it opens a **pop-up** under the button, with a pointer at it, like the due date
  pop-up. In it:
  - the title "Color",
  - a **color wheel** (168 px): the angle picks the hue, the distance from the middle picks how
    strong the color is. The middle is gray. A small ring marks the current color.
  - a **preview chip** with the label's name in the new color,
  - **Done** and **Cancel**.
- Clicking or dragging on the wheel moves the ring, and the preview chip and the chip at the top of
  the row change at once. Arrow keys move the ring too: ← → turn the hue, ↑ ↓ change the strength.
- **Done** closes the pop-up and keeps the new color in the form; **Save** on the row stores it,
  together with the name, as today. **Cancel**, Esc, or a click outside closes the pop-up and puts
  the color back. Focus goes back to the color button.
- On a wide screen Save and Delete sit at the right edge of the row (asked 2026-10-20).
- On a phone the button stays on the same line as the name field, and the pop-up opens to the left
  of it so it fits on the screen. The wheel is a bit larger there (200 px).

## Colors

- A label's color is stored as a hex color, `#rrggbb`, taken from the wheel (the hue and strength
  at mid lightness). Any color the wheel can make is allowed; the server checks the format only.
- The chip is made from that one color: a very light fill (lightness 96 %), a light border (87 %),
  and dark text in the same hue. The text is darkened until it has at least 7:1 contrast with the
  fill, like the old chips, so a light yellow still reads well.
- A new label still gets a color by itself, in turn from the eight colors used so far (now as hex
  values). They are only a starting point and are not shown anywhere.
- **Upgrade (schema v5):** the eight color names in the database become their hex colors, so
  every label keeps the look it has today.

## How chips get their color

The page's security policy blocks inline `style` attributes. So each chip, label dot and color
button carries the color as `data-color="#rrggbb"`, and `static/chips.js` sets the shades as CSS
variables (`--l-bg`, `--l-bd`, `--l-fg`, `--l-c`), also on rows that htmx swaps in later (a
`MutationObserver`). Without JavaScript a chip shows gray. `static/labels.js` runs the pop-up.

## Not in this step

- Typing a hex value, an eyedropper, or a lightness slider.
- A list of recent or suggested colors.
