// Label colors (spec 2026-10-20-label-color-wheel-design.md). A label has one
// color, #rrggbb. Each element with data-color gets a chip's shades from it as
// CSS variables: a light fill (--l-bg), a light border (--l-bd), text dark
// enough for 7:1 contrast on the fill (--l-fg), and the color itself (--l-c).
// The page's security policy blocks style attributes, not styles set from here.
"use strict";

// hexToHsl returns [hue in degrees, saturation 0..1, lightness 0..1], or
// null if hex is not #rrggbb.
function hexToHsl(hex) {
  if (!/^#[0-9a-fA-F]{6}$/.test(hex || "")) {
    return null;
  }
  const [r, g, b] = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16) / 255);
  const max = Math.max(r, g, b), min = Math.min(r, g, b), l = (max + min) / 2, d = max - min;
  if (d === 0) {
    return [0, 0, l];
  }
  const s = d / (1 - Math.abs(2 * l - 1));
  let h;
  if (max === r) h = ((g - b) / d) % 6;
  else if (max === g) h = (b - r) / d + 2;
  else h = (r - g) / d + 4;
  return [(h * 60 + 360) % 360, s, l];
}

function hslToRgb(h, s, l) {
  const a = s * Math.min(l, 1 - l);
  const f = (n) => {
    const k = (n + h / 30) % 12;
    return l - a * Math.max(-1, Math.min(k - 3, 9 - k, 1));
  };
  return [f(0), f(8), f(4)];
}

function rgbToHex(rgb) {
  return "#" + rgb.map((v) => Math.round(v * 255).toString(16).padStart(2, "0")).join("");
}

// hslToHex is the color the wheel shows at hue h and strength s.
function hslToHex(h, s, l) {
  return rgbToHex(hslToRgb(h, s, l));
}

function luminance(rgb) {
  const [r, g, b] = rgb.map((x) => (x <= 0.04045 ? x / 12.92 : Math.pow((x + 0.055) / 1.055, 2.4)));
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

// labelShades returns the shades of a chip in color hex, or null.
function labelShades(hex) {
  const hsl = hexToHsl(hex);
  if (!hsl) {
    return null;
  }
  const [h, s] = hsl;
  const bg = hslToRgb(h, s, 0.96), lbg = luminance(bg);
  let fg = hslToRgb(h, Math.min(s, 0.8), 0.05);
  for (let l = 0.40; l > 0.05; l -= 0.01) {
    const c = hslToRgb(h, Math.min(s, 0.8), l);
    if ((lbg + 0.05) / (luminance(c) + 0.05) >= 7) {
      fg = c;
      break;
    }
  }
  return { bg: rgbToHex(bg), bd: rgbToHex(hslToRgb(h, s, 0.87)), fg: rgbToHex(fg), c: hex.toLowerCase() };
}

function paintColor(el) {
  const shades = labelShades(el.dataset.color);
  for (const k of ["bg", "bd", "fg", "c"]) {
    if (shades) el.style.setProperty("--l-" + k, shades[k]);
    else el.style.removeProperty("--l-" + k);
  }
}

function paintColors(root) {
  if (root.matches && root.matches("[data-color]")) {
    paintColor(root);
  }
  if (root.querySelectorAll) {
    root.querySelectorAll("[data-color]").forEach(paintColor);
  }
}

paintColors(document);
// Rows, toolbars and label lists that htmx swaps in, and colors that change.
new MutationObserver((records) => {
  for (const r of records) {
    if (r.type === "attributes") paintColor(r.target);
    else r.addedNodes.forEach(paintColors);
  }
}).observe(document.documentElement, { childList: true, subtree: true, attributes: true, attributeFilter: ["data-color"] });
