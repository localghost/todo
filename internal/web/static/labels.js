// The color wheel pop-up of the labels page (spec
// 2026-10-20-label-color-wheel-design.md). The round button after a name
// opens it. The angle on the wheel is the hue, the distance from the middle
// the strength; the middle is gray. Done puts the color in the row's form,
// which Save stores; Cancel, Esc or a click outside puts the color back.
// Needs chips.js (hexToHsl, hslToHex).
"use strict";

const colorPicker = document.getElementById("color-picker");
let colorButton = null; // the button whose pop-up is open
let startColor = "";    // the color when the pop-up opened
let hue = 0, strength = 0, dragging = false;

function rowChip(button) {
  return button.closest(".label-row").querySelector(".label-head .chip");
}

function currentColor() {
  return hslToHex(hue, strength, 0.5);
}

function showColor() {
  const color = currentColor();
  const ring = colorPicker.querySelector(".wheel-ring");
  const a = hue * Math.PI / 180;
  ring.style.left = (50 + 50 * strength * Math.sin(a)) + "%";
  ring.style.top = (50 - 50 * strength * Math.cos(a)) + "%";
  ring.style.background = color;
  const wheel = colorPicker.querySelector(".wheel");
  wheel.setAttribute("aria-valuenow", String(Math.round(hue) % 360));
  wheel.setAttribute("aria-valuetext", "hue " + Math.round(hue) % 360 + " degrees, strength " + Math.round(strength * 100) + " percent");
  colorPicker.querySelector(".color-preview .chip").dataset.color = color;
  rowChip(colorButton).dataset.color = color;
}

// Open to the left of the button when the pop-up does not fit on the right.
// If it fits neither way, move it in from the left edge.
function placeColorPicker() {
  const width = document.documentElement.clientWidth;
  colorPicker.classList.remove("flip");
  colorPicker.style.left = "";
  colorPicker.style.removeProperty("--caret");
  if (colorPicker.getBoundingClientRect().right <= width - 8) {
    return;
  }
  colorPicker.classList.add("flip");
  if (colorPicker.getBoundingClientRect().left >= 8) {
    return;
  }
  const b = colorButton.getBoundingClientRect();
  colorPicker.classList.remove("flip");
  colorPicker.style.left = (8 - b.left) + "px";
  colorPicker.style.setProperty("--caret", (b.left + b.width / 2 - 8 - 6) + "px");
}

function openColorPicker(button) {
  closeColorPicker(false, false);
  colorButton = button;
  startColor = button.parentElement.querySelector('input[name="color"]').value;
  const hsl = hexToHsl(startColor) || [0, 0, 0.5];
  hue = hsl[0];
  strength = hsl[1];
  colorPicker.querySelector(".color-preview .chip").textContent = button.dataset.name;
  button.parentElement.append(colorPicker);
  colorPicker.hidden = false;
  button.setAttribute("aria-expanded", "true");
  placeColorPicker();
  showColor();
  colorPicker.querySelector(".wheel").focus();
}

// closeColorPicker keeps the new color if keep is true, else puts the old one
// back. refocus puts the focus back on the color button.
function closeColorPicker(keep, refocus) {
  if (!colorButton) {
    return;
  }
  const button = colorButton;
  const color = keep ? currentColor() : startColor;
  button.parentElement.querySelector('input[name="color"]').value = color;
  button.dataset.color = color;
  rowChip(button).dataset.color = color;
  colorPicker.hidden = true;
  button.setAttribute("aria-expanded", "false");
  colorButton = null;
  dragging = false;
  if (refocus) {
    button.focus();
  }
}

function pickAt(e) {
  const r = e.currentTarget.getBoundingClientRect();
  const dx = (e.clientX - r.left) / r.width - 0.5, dy = (e.clientY - r.top) / r.height - 0.5;
  hue = (Math.atan2(dx, -dy) * 180 / Math.PI + 360) % 360;
  strength = Math.min(1, 2 * Math.hypot(dx, dy));
  showColor();
}

if (colorPicker) {
  const wheel = colorPicker.querySelector(".wheel");
  wheel.addEventListener("pointerdown", (e) => {
    e.preventDefault();
    wheel.setPointerCapture(e.pointerId);
    wheel.focus();
    dragging = true;
    pickAt(e);
  });
  wheel.addEventListener("pointermove", (e) => {
    if (dragging) pickAt(e);
  });
  wheel.addEventListener("pointerup", () => { dragging = false; });
  wheel.addEventListener("pointercancel", () => { dragging = false; });
  // ← → turn the hue, ↑ ↓ change the strength; with Shift in bigger steps.
  wheel.addEventListener("keydown", (e) => {
    const step = e.shiftKey ? 15 : 5;
    switch (e.key) {
      case "ArrowLeft": hue = (hue - step + 360) % 360; break;
      case "ArrowRight": hue = (hue + step) % 360; break;
      case "ArrowUp": strength = Math.min(1, strength + step / 100); break;
      case "ArrowDown": strength = Math.max(0, strength - step / 100); break;
      case "Enter": e.preventDefault(); closeColorPicker(true, true); return;
      default: return;
    }
    e.preventDefault();
    showColor();
  });
  colorPicker.addEventListener("keydown", (e) => {
    if (e.key === "Escape") {
      e.preventDefault();
      closeColorPicker(false, true);
    }
  });
  colorPicker.querySelector("[data-color-done]").addEventListener("click", () => closeColorPicker(true, true));
  colorPicker.querySelector("[data-color-cancel]").addEventListener("click", () => closeColorPicker(false, true));

  document.addEventListener("click", (e) => {
    const button = e.target.closest && e.target.closest(".color-btn");
    if (!button) {
      return;
    }
    if (button === colorButton) closeColorPicker(false, true);
    else openColorPicker(button);
  });
  // A press outside the pop-up and its button puts the color back.
  document.addEventListener("pointerdown", (e) => {
    if (colorButton && !colorPicker.contains(e.target) && !colorButton.contains(e.target)) {
      closeColorPicker(false, false);
    }
  });
  window.addEventListener("resize", () => {
    if (colorButton) placeColorPicker();
  });
}
