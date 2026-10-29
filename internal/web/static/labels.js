// The Labels page (canvas page "16 · Labels", board "Labels page again").
// Typing in the field shows only the labels whose name has the text. A color
// circle opens the color wheel under its line; Done saves the color at once
// (the circle of the new label keeps it for Create). × asks before a delete.
// The server answers a create or a delete with the whole section, so the
// page starts again from the new section. Needs chips.js and wheel.js.
"use strict";

let colorEdit = null; // { sw, line, label, old } while the wheel is open

function section() {
  return document.getElementById("labels-section");
}

function cleanName(v) {
  return v.trim().replace(/\s+/g, " ");
}

// filterLines shows the lines whose name has the typed text.
function filterLines() {
  const s = section();
  const input = s && s.querySelector("#label-search");
  if (!input) {
    return;
  }
  const q = cleanName(input.value).toLowerCase();
  let shown = 0;
  s.querySelectorAll(".lp-line").forEach((line) => {
    line.hidden = q !== "" && !line.dataset.name.toLowerCase().includes(q);
    if (!line.hidden) shown++;
  });
  const none = s.querySelector(".lp-none");
  if (none) none.hidden = shown > 0;
}

function wheelOf(view) {
  const el = view.querySelector(".wheel");
  if (!el._wheel) {
    el._wheel = makeWheel(el, (color) => {
      if (colorEdit) paintLabel(colorEdit, color);
    });
  }
  return el._wheel;
}

// paintLabel shows color on the circle of the edited label (or of the new
// label) and on the preview chip.
function paintLabel(edit, color) {
  edit.sw.dataset.color = color;
  section().querySelector(".lp-color .chip").dataset.color = color;
}

function openColor(sw) {
  if (colorEdit) closeColor(false);
  const s = section();
  const line = sw.closest(".lp-line");
  const view = s.querySelector(".lp-color");
  const name = line ? line.dataset.name : cleanName(s.querySelector("#label-search").value) || "new label";
  colorEdit = { sw, line, label: line ? line.dataset.label : "", old: sw.dataset.color };
  view.querySelector(".lp-color-title").textContent = line ? "Color of “" + name + "”" : "Color of the new label";
  const chip = view.querySelector(".chip");
  chip.textContent = name;
  chip.dataset.color = colorEdit.old;
  // In a line the wheel goes inside it, under the name (the list holds
  // only lines); for the new label it goes under the field.
  if (line) line.append(view);
  else s.querySelector(".label-create").after(view);
  view.hidden = false;
  sw.setAttribute("aria-expanded", "true");
  wheelOf(view).set(colorEdit.old);
  view.querySelector(".wheel").focus();
}

// closeColor hides the wheel. keep saves the color: a label's color at
// once, the new label's color when the label is created.
function closeColor(keep) {
  const edit = colorEdit;
  if (!edit) {
    return;
  }
  colorEdit = null;
  const view = section().querySelector(".lp-color");
  const color = keep ? wheelOf(view).color() : edit.old;
  edit.sw.dataset.color = color;
  if (!edit.label) {
    section().querySelector('.label-create input[name="color"]').value = color;
  } else if (keep && color !== edit.old) {
    saveColor(edit, color);
  }
  view.hidden = true;
  edit.sw.setAttribute("aria-expanded", "false");
  edit.sw.focus();
}

function saveColor(edit, color) {
  // HX-Request makes an ended session answer 401, not a redirect to the
  // login page, which fetch would follow and see as a success.
  fetch("/labels/" + edit.label + "/color", { method: "PUT", headers: { "HX-Request": "true" }, body: new URLSearchParams({ color }) })
    .then((res) => {
      if (res.status !== 204) throw new Error("status " + res.status);
    })
    .catch(() => {
      edit.sw.dataset.color = edit.old;
      showError("The color was not saved. Try again.");
    });
}

function showError(text) {
  const s = section();
  let p = s.querySelector("#label-error");
  if (!p) {
    p = document.createElement("p");
    p.id = "label-error";
    p.className = "error";
    p.setAttribute("role", "alert");
    s.querySelector(".label-create").after(p);
  }
  p.textContent = text;
}

// askDelete shows the question under the line of the label.
function askDelete(line) {
  closeAsk();
  const n = Number(line.dataset.items);
  const ask = document.createElement("li");
  ask.className = "lp-ask";
  ask.setAttribute("role", "alert");
  const p = document.createElement("p");
  p.textContent = "Delete “" + line.dataset.name + "”? " +
    (n === 0 ? "No items use it." : "It comes off " + n + (n === 1 ? " item. The item stays." : " items. The items stay."));
  const yes = document.createElement("button");
  yes.type = "button";
  yes.className = "btn-small danger-solid";
  yes.dataset.askDelete = line.dataset.label;
  yes.textContent = "Delete";
  const no = document.createElement("button");
  no.type = "button";
  no.className = "btn-small";
  no.dataset.askCancel = "";
  no.textContent = "Cancel";
  const buttons = document.createElement("div");
  buttons.className = "confirm-buttons";
  buttons.append(yes, no);
  ask.append(p, buttons);
  line.classList.add("asking");
  line.after(ask);
  yes.focus();
}

function closeAsk(refocus) {
  const ask = section() && section().querySelector(".lp-ask");
  if (!ask) {
    return;
  }
  const line = ask.previousElementSibling;
  ask.remove();
  if (line) {
    line.classList.remove("asking");
    if (refocus) line.querySelector(".lp-x").focus();
  }
}

document.addEventListener("click", (e) => {
  const t = e.target;
  if (!t.closest || !t.closest("#labels-section")) {
    return;
  }
  const sw = t.closest("button.sw");
  if (sw) {
    if (colorEdit && colorEdit.sw === sw) closeColor(false);
    else openColor(sw);
  } else if (t.closest("[data-color-done]")) {
    closeColor(true);
  } else if (t.closest("[data-color-cancel]")) {
    closeColor(false);
  } else if (t.closest(".lp-x")) {
    askDelete(t.closest(".lp-line"));
  } else if (t.closest("[data-ask-cancel]")) {
    closeAsk(true);
  } else if (t.closest("[data-ask-delete]")) {
    const id = t.closest("[data-ask-delete]").dataset.askDelete;
    htmx.ajax("DELETE", "/labels/" + id, { target: "#labels-section", swap: "outerHTML" }).catch(() => {});
  }
});

document.addEventListener("input", (e) => {
  if (e.target.id === "label-search") {
    filterLines();
  }
});

// Enter in the wheel is Done, Esc is Cancel; Esc also closes the question.
document.addEventListener("keydown", (e) => {
  if (colorEdit && e.target.closest && e.target.closest(".lp-color")) {
    if (e.key === "Escape" || (e.key === "Enter" && e.target.matches(".wheel"))) {
      e.preventDefault();
      closeColor(e.key === "Enter");
    }
  } else if (e.key === "Escape" && e.target.closest && e.target.closest(".lp-ask")) {
    e.preventDefault();
    closeAsk(true);
  }
});

// A new section (after Create or a delete): no wheel is open, the field
// gets the focus, and a typed name filters the lines.
document.addEventListener("htmx:afterSwap", (e) => {
  if (e.detail.target.id !== "labels-section" && !document.getElementById("labels-section").contains(e.detail.target)) {
    return;
  }
  colorEdit = null;
  const input = document.getElementById("label-search");
  if (input) {
    input.focus();
    input.setSelectionRange(input.value.length, input.value.length);
  }
  filterLines();
});
