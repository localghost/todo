// The color wheel of the label list (spec 2026-10-20-label-list-manage-design.md;
// first on the Labels page, spec 2026-10-20-label-color-wheel-design.md). The
// angle is the hue, the distance from the middle the strength; the middle is
// gray. Needs chips.js (hexToHsl, hslToHex).
"use strict";

// wheelPick returns [hue, strength] for a point dx, dy from the middle of
// the wheel, in parts of its width (-0.5 to 0.5).
function wheelPick(dx, dy) {
  return [(Math.atan2(dx, -dy) * 180 / Math.PI + 360) % 360, Math.min(1, 2 * Math.hypot(dx, dy))];
}

// wheelRing returns the place of the ring for hue and strength, in percent
// of the wheel's size from its left and top edge.
function wheelRing(hue, strength) {
  const a = hue * Math.PI / 180;
  return [50 + 50 * strength * Math.sin(a), 50 - 50 * strength * Math.cos(a)];
}

// makeWheel makes el a wheel. onChange gets each color the user picks.
// set(hex) moves the wheel to a color; color() returns the current one.
function makeWheel(el, onChange) {
  const ring = el.querySelector(".wheel-ring");
  let hue = 0, strength = 0, dragging = false;
  const color = () => hslToHex(hue, strength, 0.5);

  function show(notify) {
    const [left, top] = wheelRing(hue, strength);
    ring.style.left = left + "%";
    ring.style.top = top + "%";
    ring.style.background = color();
    el.setAttribute("aria-valuenow", String(Math.round(hue) % 360));
    el.setAttribute("aria-valuetext", "hue " + Math.round(hue) % 360 + " degrees, strength " + Math.round(strength * 100) + " percent");
    if (notify) onChange(color());
  }

  function pickAt(e) {
    const r = el.getBoundingClientRect();
    [hue, strength] = wheelPick((e.clientX - r.left) / r.width - 0.5, (e.clientY - r.top) / r.height - 0.5);
    show(true);
  }

  el.addEventListener("pointerdown", (e) => {
    e.preventDefault();
    el.setPointerCapture(e.pointerId);
    el.focus();
    dragging = true;
    pickAt(e);
  });
  el.addEventListener("pointermove", (e) => {
    if (dragging) pickAt(e);
  });
  el.addEventListener("pointerup", () => { dragging = false; });
  el.addEventListener("pointercancel", () => { dragging = false; });
  // ← → turn the hue, ↑ ↓ change the strength; with Shift in bigger steps.
  el.addEventListener("keydown", (e) => {
    const step = e.shiftKey ? 15 : 5;
    switch (e.key) {
      case "ArrowLeft": hue = (hue - step + 360) % 360; break;
      case "ArrowRight": hue = (hue + step) % 360; break;
      case "ArrowUp": strength = Math.min(1, strength + step / 100); break;
      case "ArrowDown": strength = Math.max(0, strength - step / 100); break;
      default: return;
    }
    e.preventDefault();
    show(true);
  });

  return {
    set(hex) {
      const hsl = hexToHsl(hex) || [0, 0, 0.5];
      hue = hsl[0];
      strength = hsl[1];
      dragging = false;
      show(false);
    },
    color,
  };
}
