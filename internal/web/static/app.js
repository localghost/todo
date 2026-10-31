// Small page behaviors that htmx attributes cannot express.
// Approved in Claude Design, page "8 · Focus and typing".

// 1. Focus after check or delete. A check keeps focus by itself (the new
// checkbox has the same id). If the focused row disappears, focus moves to
// the next row's checkbox, else the previous row's, else the add input.
let focusAfterSwap = null;

document.addEventListener("htmx:beforeSwap", (e) => {
  // The row of the element that sent the request; the target can be the whole
  // list section when an item moves to another place.
  const source = e.detail.elt;
  const row = source && source.closest ? source.closest("li.item") : null;
  const active = document.activeElement;
  if (e.detail.target.id === "add-extras" && active && active.closest && active.closest("#add-extras")) {
    // The buttons come back new: go on at the one that was used.
    focusAfterSwap = active.closest(".due-picker, .pill-x") ? "due-add-0" : "label-add-0";
    return;
  }
  if (!row || !active || !row.contains(active)) {
    return;
  }
  if (active.matches(".postpone-btn")) {
    // The chips disappear after a postpone; keep focus in the same row.
    focusAfterSwap = "check-" + row.id.slice("item-".length);
    return;
  }
  if (active.matches(".chip-x") && e.detail.serverResponse.includes('id="' + row.id + '"')) {
    // The row stays: go on at its +.
    focusAfterSwap = "label-add-" + row.id.slice("item-".length);
    return;
  }
  if (active.closest(".due-picker")) {
    // The row comes back without the pop-up: go on at its calendar icon.
    focusAfterSwap = "due-add-" + row.id.slice("item-".length);
    return;
  }
  if (active.closest(".notes-picker")) {
    // The same for the notes pop-up: go on at its notes button.
    focusAfterSwap = "notes-add-" + row.id.slice("item-".length);
    return;
  }
  if (active.matches(".due-x")) {
    // The date is gone, the row stays: go on at its "Due date" chip.
    focusAfterSwap = "due-add-" + row.id.slice("item-".length);
    return;
  }
  if (active.matches(".notes-x")) {
    // The same for the notes: go on at the gray notes icon.
    focusAfterSwap = "notes-add-" + row.id.slice("item-".length);
    return;
  }
  if (!active.matches(".check, .delete, .chip-x")) {
    return;
  }
  // Keep the id, not the node: a whole-list answer replaces the node.
  const neighbor = row.nextElementSibling || row.previousElementSibling;
  const check = neighbor && neighbor.querySelector(".check");
  focusAfterSwap = check ? check.id : "new-item";
});

document.addEventListener("htmx:afterSettle", () => {
  const lost = !document.activeElement || document.activeElement === document.body;
  const target = typeof focusAfterSwap === "string" ? document.getElementById(focusAfterSwap) : focusAfterSwap;
  if (target && lost && document.body.contains(target)) {
    target.focus();
  }
  focusAfterSwap = null;
});

// 2. When focus leaves an edit row, save it. Moving between the fields of
// the row does not save. Empty text cancels the edit instead.
// The decision waits one tick: some browsers move focus to <body> first,
// and the new focus is known only afterwards (for example a native picker).
document.addEventListener("focusout", (e) => {
  const row = e.target.closest && e.target.closest("li.editing");
  if (!row) {
    return;
  }
  setTimeout(() => {
    if (!row.isConnected || row.contains(document.activeElement)) {
      return;
    }
    const text = row.querySelector('input[name="text"]');
    if (text.value.trim() === "") {
      htmx.trigger(row, "cancel-edit");
    } else {
      htmx.trigger(row.querySelector(".edit"), "save-edit");
    }
  }, 0);
});

// A click inside an edit row (a gap, the hint) must not take the focus out
// of the row. Inputs and labels still get focus as usual.
document.addEventListener("mousedown", (e) => {
  const row = e.target.closest && e.target.closest("li.editing");
  if (row && !e.target.closest("input, label")) {
    e.preventDefault();
  }
});

// 3. After an item is added, clear each add-form field only if it still
// holds the value that was sent. Text typed while the request ran stays.
const ADD_FIELDS = ["text"];

document.addEventListener("htmx:beforeRequest", (e) => {
  const form = e.detail.elt;
  if (form.id === "add-form") {
    form._sent = Object.fromEntries(ADD_FIELDS.map((name) => [name, form.elements[name].value]));
  }
});

document.addEventListener("htmx:afterRequest", (e) => {
  const form = e.detail.elt;
  if (form.id !== "add-form" || !e.detail.successful) {
    return;
  }
  for (const name of ADD_FIELDS) {
    const input = form.elements[name];
    if (input.value === form._sent[name]) {
      input.value = "";
    }
    input.removeAttribute("aria-invalid");
    input.removeAttribute("aria-describedby");
  }
  form.querySelectorAll(".error").forEach((p) => p.remove());
});

// 4. When an item opens for editing, put the cursor at the end of the text.
// htmx has already focused the field (autofocus) when it fires htmx:load.
document.addEventListener("htmx:load", (e) => {
  const input = e.detail.elt.querySelector(".edit input");
  if (input) {
    const end = input.value.length;
    input.setSelectionRange(end, end);
  }
});

// 5. Due notifications. The page asks the server which items are due; the
// server gives each item only once, also with several tabs open.
const canNotify = "Notification" in window;
const NETWORK_ERROR = "Cannot reach the server. Please try again.";
let claimStopped = false;

function renderNotifyBar() {
  const bar = document.getElementById("notify-bar");
  if (!bar) {
    return;
  }
  bar.replaceChildren();
  bar.classList.remove("blocked");
  if (!canNotify) {
    return;
  }
  if (Notification.permission === "denied") {
    bar.classList.add("blocked");
    bar.textContent = "Notifications are blocked in your browser settings.";
  } else if (Notification.permission === "default" && bar.dataset.hasDue === "true") {
    const text = document.createElement("span");
    text.textContent = "Get a notification when an item is due.";
    const button = document.createElement("button");
    button.type = "button";
    button.textContent = "Turn on notifications";
    button.addEventListener("click", async () => {
      await Notification.requestPermission();
      renderNotifyBar();
      claimDue();
    });
    bar.append(text, button);
  }
}

async function claimDue() {
  if (claimStopped || !canNotify || Notification.permission !== "granted") {
    return;
  }
  const errorLine = document.getElementById("error");
  let res;
  try {
    res = await fetch("/notifications/claim", { method: "POST", headers: { "HX-Request": "true" } });
  } catch (err) {
    if (errorLine) {
      errorLine.textContent = NETWORK_ERROR;
    }
    return;
  }
  if (errorLine && errorLine.textContent === NETWORK_ERROR) {
    errorLine.textContent = "";
  }
  if (res.status === 401) {
    // Not logged in any more: stop asking until the page reloads.
    claimStopped = true;
    return;
  }
  if (!res.ok) {
    return;
  }
  let items;
  try {
    items = await res.json();
  } catch (err) {
    console.warn("Bad claim response", err);
    return;
  }
  for (const item of items) {
    try {
      const note = new Notification(item.title, { body: `${item.text} — ${item.due}`, tag: `todo-${item.id}` });
      note.onclick = () => {
        window.focus();
        note.close();
      };
    } catch (err) {
      // For example Android Chrome, which allows notifications only from a service worker.
      console.warn("Cannot show a notification", err);
    }
    // Reload the row, so its label shows the new state (for example red "Overdue").
    const row = document.getElementById(`item-${item.id}`);
    if (row && !row.classList.contains("editing")) {
      htmx.ajax("GET", "/items/" + item.id, { target: row, swap: "outerHTML" });
    }
  }
}

document.addEventListener("htmx:afterSettle", renderNotifyBar);
document.addEventListener("htmx:oobAfterSwap", renderNotifyBar);
document.addEventListener("visibilitychange", () => {
  if (document.visibilityState === "visible") {
    claimDue();
  }
});
// A permission change in the browser settings updates the bar at once.
if (canNotify && navigator.permissions) {
  navigator.permissions.query({ name: "notifications" }).then((status) => {
    status.onchange = () => {
      renderNotifyBar();
      claimDue();
    };
  }).catch(() => {});
}
renderNotifyBar();
claimDue();
setInterval(claimDue, 30000);

// 6. Start typing anywhere to add an item: if nothing has focus, a printable
// key goes into the add field.
document.addEventListener("keydown", (e) => {
  const pageHasFocus = !document.activeElement || document.activeElement === document.body;
  if (!pageHasFocus || e.defaultPrevented || e.isComposing || e.ctrlKey || e.altKey || e.metaKey) {
    return;
  }
  if (e.key.length !== 1 || e.key === " " || (e.key === "/" && searchAvailable())) {
    return; // "/" opens the search (section 18)
  }
  const input = document.getElementById("new-item");
  if (!input) {
    return;
  }
  e.preventDefault();
  input.focus();
  input.value += e.key;
  const end = input.value.length;
  input.setSelectionRange(end, end);
});

// 7. Enter saves and Escape cancels an edit row. (These were htmx trigger
// filters, which need eval; the CSP does not allow eval.)
document.addEventListener("keydown", (e) => {
  if (e.isComposing || e.keyCode === 229) {
    return; // the key ends an IME composition; it is not a real Enter
  }
  const row = e.target.closest && e.target.closest("li.editing");
  if (!row) {
    return;
  }
  if (e.key === "Enter") {
    e.preventDefault();
    htmx.trigger(row.querySelector(".edit"), "save-edit");
  } else if (e.key === "Escape") {
    e.preventDefault();
    htmx.trigger(row, "cancel-edit");
  }
});

// 8. The error line under the title: show network errors, clear it after a
// successful request. (These were hx-on attributes.)
document.addEventListener("htmx:sendError", () => {
  const line = document.getElementById("error");
  if (line) {
    line.textContent = NETWORK_ERROR;
  }
});

document.addEventListener("htmx:afterRequest", (e) => {
  const line = document.getElementById("error");
  if (line && e.detail.successful) {
    line.textContent = "";
  }
});

// 9. If the session ended and the user presses Back, htmx cannot load the
// old page (401). Reload, so the server sends the log-in page.
document.addEventListener("htmx:historyCacheMissLoadError", () => {
  location.reload();
});

// 10. An overdue row turns red and gets the postpone buttons (in "Sort by due
// date" it also moves to the top). The toolbar (swapped with every list and
// row answer) names the moment when the next item becomes overdue
// (data-next-overdue, Unix ms); refresh the list then. Never while an edit row
// is open: try again later. Plan at most 6 hours ahead, so a sleeping laptop
// does not miss the moment for long. If the server still names a moment we
// already refreshed for (an error, or this clock is ahead), wait 30 s, so the
// page cannot ask every second. Never during a drag (order.js sets the body
// class "dragging").
let overdueTimer = null;
let refreshedFor = 0;
function planOverdueRefresh() {
  clearTimeout(overdueTimer);
  const toolbar = document.getElementById("toolbar");
  const next = toolbar ? Number(toolbar.dataset.nextOverdue) : 0;
  if (!next) {
    return;
  }
  let wait = next - Date.now() + 1000;
  if (next <= refreshedFor) {
    wait = Math.max(wait, 30000);
  }
  wait = Math.min(Math.max(wait, 1000), 6 * 60 * 60 * 1000);
  overdueTimer = setTimeout(() => refreshForOverdue(next), wait);
}
function refreshForOverdue(next) {
  if (document.querySelector(".item.editing") || document.body.classList.contains("dragging")) {
    overdueTimer = setTimeout(() => refreshForOverdue(next), 30000);
    return;
  }
  refreshedFor = next;
  htmx.ajax("GET", "/", { target: "#list-section", swap: "outerHTML" });
}
document.addEventListener("DOMContentLoaded", planOverdueRefresh);
document.addEventListener("htmx:afterSettle", planOverdueRefresh);

// 11. A whole-list answer (an item moved to another place, or the overdue
// refresh) must not replace a row that is being edited elsewhere:
// put the open edit row back after the swap, with its focus.
let keptEdit = null;
let keptFocus = false;
document.addEventListener("htmx:beforeSwap", (e) => {
  keptEdit = null;
  if (!e.detail.target || e.detail.target.id !== "list-section") {
    return;
  }
  const editing = document.querySelector(".item.editing");
  if (!editing || (e.detail.elt && editing.contains(e.detail.elt))) {
    return;
  }
  keptEdit = editing;
  keptFocus = editing.contains(document.activeElement);
});
document.addEventListener("htmx:afterSwap", () => {
  if (!keptEdit) {
    return;
  }
  const fresh = document.getElementById(keptEdit.id);
  if (fresh) {
    fresh.replaceWith(keptEdit);
    const input = keptEdit.querySelector('input[name="text"]');
    if (keptFocus && input) {
      input.focus();
    }
  }
  keptEdit = null;
});

// 12. Another tab may have changed "hide done" or the order. A row answer
// brings the current toolbar; if its state differs from the list on this
// page, load the list again, so the page does not show a mix of both states.
document.addEventListener("htmx:oobAfterSwap", () => {
  const toolbar = document.getElementById("toolbar");
  const section = document.getElementById("list-section");
  if (toolbar && section && (toolbar.dataset.hideDone !== section.dataset.hideDone || toolbar.dataset.sortDue !== section.dataset.sortDue)) {
    htmx.ajax("GET", "/", { target: "#list-section", swap: "outerHTML" });
  }
});

// After "Sort by due date" or "Sort by hand" the list comes back new. Each
// row slides from its old place to its new one, the checkbox and the text
// slide into the place of the grip column, and the grips and the gray line
// fade in (canvas page "14 · Overdue at the top", "transition between the
// two orders", option A, 0.7 s). With "reduce motion" the list changes at once.
// The focus goes to the new button, so a keyboard user stays in the toolbar.
const SLIDE_MS = 700;
const SLIDE_EASE = "cubic-bezier(0.2, 0.7, 0.3, 1)";
let sortBefore = null; // row id -> { top, left } before the swap

function isSortRequest(e) {
  const elt = e.detail.requestConfig && e.detail.requestConfig.elt;
  return !!(elt && elt.classList && elt.classList.contains("sort-due"));
}

function reduceMotion() {
  return !!(window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches);
}

// rowPlaces returns the top of each row and the left of its checkbox.
function rowPlaces() {
  const places = new Map();
  document.querySelectorAll("#items > li.item").forEach((row) => {
    const check = row.querySelector(".check");
    places.set(row.id, { top: row.getBoundingClientRect().top, left: check ? check.getBoundingClientRect().left : 0 });
  });
  return places;
}

// slideRows plays the move from the places in before to the current ones.
// A moving row gets the background of the list, so it hides a row that it
// passes over. (A class on #items would not stay: htmx puts the attributes
// of the new list back after the swap.)
function slideRows(before) {
  const list = document.getElementById("items");
  const card = list && window.getComputedStyle ? window.getComputedStyle(list).backgroundColor : "";
  const options = { duration: SLIDE_MS, easing: SLIDE_EASE };
  const slide = (el, from) => el.animate([{ transform: from }, { transform: "none" }], options);
  document.querySelectorAll("#items > li.item").forEach((row) => {
    const old = before.get(row.id);
    if (!old) {
      return;
    }
    const dy = Math.round(old.top - row.getBoundingClientRect().top);
    const check = row.querySelector(".check");
    const dx = check ? Math.round(old.left - check.getBoundingClientRect().left) : 0;
    if (dy !== 0) {
      const from = `translateY(${dy}px)`;
      row.animate([{ transform: from, backgroundColor: card }, { transform: "none", backgroundColor: card }], options);
    }
    if (dx !== 0) {
      row.querySelectorAll(".check, .text-col").forEach((el) => slide(el, `translateX(${dx}px)`));
    }
    const grip = row.querySelector(".grip svg");
    if (grip) {
      grip.animate([{ opacity: 0 }, { opacity: 0 }, { opacity: 1 }], { duration: SLIDE_MS });
    }
  });
  const hint = document.querySelector(".sort-hint");
  if (hint) {
    hint.animate([{ opacity: 0 }, { opacity: 1 }], { duration: SLIDE_MS });
  }
}

document.addEventListener("htmx:beforeSwap", (e) => {
  sortBefore = isSortRequest(e) && !reduceMotion() ? rowPlaces() : null;
});
document.addEventListener("htmx:afterSwap", (e) => {
  if (!isSortRequest(e)) {
    return;
  }
  // htmx sends this event before the browser paints the new list, so the
  // rows never show at their new places first.
  if (sortBefore) {
    slideRows(sortBefore);
    sortBefore = null;
  }
  const button = document.querySelector(".sort-due");
  if (button) button.focus();
});

// 13. Pop-ups on a row: the label list under the + (canvas page "16 ·
// Labels", board "label list – several labels, color wheel, × deletes"; spec
// 2026-10-20-label-list-manage-design.md) and the due date pop-up under the
// calendar icon (canvas page "10 · Due date"). Only one is open at a time.
// Esc, a click outside, or focus that leaves it closes it.
// The label list shows all labels. A click on a name adds or takes off that
// label; the server answers with the row, and the list moves into the new
// row and loads again, so it stays open. Typing narrows the list; a name
// that matches no label offers "Create …", whose circle chooses the new
// label's color with the wheel. Colors and deletes of labels are on the
// Labels page ("Manage labels…" at the end of the list; board "Labels page
// again", option P2).
// The due date pop-up is a form: Enter saves and Tab moves inside it. The
// due text opens it too.
// The notes pop-up (canvas page "21 · Notes", option N2) is a form too. It
// shows the notes with "Edit", or the field if the item has no notes. Enter
// in the field makes a new line; Ctrl+Enter (⌘+Enter on a Mac) saves.
let picker = null; // the open pop-up
let pickerButton = null; // its + or calendar icon
let reopenFor = null; // the item whose label list stays open after the next swap
let pickerBusy = false; // a label request is on its way; focus can leave for a moment
let carrySearch = null; // the search field of a list that loads again
let carryColor = null; // the new label's color that a list keeps when it loads again
let focusIfGone = null; // the id to focus if the row of the open list goes away
let wheel = null; // the wheel of the open label list (wheel.js)
let colorEdit = null; // { swatch, old } while the wheel view is open

function isDuePicker() {
  return picker !== null && picker.classList.contains("due-picker");
}

function isNotesPicker() {
  return picker !== null && picker.classList.contains("notes-picker");
}

// isFormPicker: the pop-up is a form with fields (due date or notes).
function isFormPicker() {
  return isDuePicker() || isNotesPicker();
}

function isLabelPicker() {
  return picker !== null && !isFormPicker();
}

// focusPicker puts the cursor in the first field of the pop-up: the search
// of the label list, the date of the due date pop-up, or the end of the
// notes field. Notes that show (not the field) give the focus to "Edit".
function focusPicker() {
  if (isNotesPicker()) {
    const field = picker.querySelector(".notes-form:not([hidden]) textarea");
    const el = field || picker.querySelector(".notes-edit");
    if (el) {
      el.focus();
      if (field) {
        field.setSelectionRange(field.value.length, field.value.length);
      }
    }
    return field;
  }
  const input = picker.querySelector('input:not([type="hidden"])');
  if (input) {
    input.focus();
  }
  return input;
}

// editNotes changes the notes pop-up from the notes to the field.
function editNotes() {
  picker.querySelector(".notes-view").hidden = true;
  picker.querySelector(".notes-form").hidden = false;
  focusPicker();
  placePicker();
}

// popupButton is the date chip or the "Label" chip that a press on target
// belongs to, on a row or under the add field.
function popupButton(target) {
  return target.closest ? target.closest(".label-add, .due-add, .notes-add, .draft-label, .draft-due") : null;
}

// resetPicker forgets the open pop-up. A color that the wheel shows but did
// not save goes back.
function resetPicker() {
  if (colorEdit) {
    colorEdit.swatch.dataset.color = colorEdit.old;
    colorEdit = null;
  }
  picker = null;
  pickerButton = null;
  reopenFor = null;
  pickerBusy = false;
  carrySearch = null;
  carryColor = null;
  focusIfGone = null;
  wheel = null;
}

function closePicker(refocus) {
  if (!picker) {
    // A pop-up that still loads, or that failed to load: forget it, so the
    // next click on its button opens it again (code review 5).
    if (pickerButton) {
      pickerButton.setAttribute("aria-expanded", "false");
      pickerButton = null;
    }
    return;
  }
  picker.remove();
  pickerButton.setAttribute("aria-expanded", "false");
  if (refocus && pickerButton.isConnected) {
    pickerButton.focus();
  }
  resetPicker();
}

// installPicker gets a pop-up that just came from the server ready.
function installPicker() {
  placePicker();
  filterPicker();
  const el = picker.querySelector(".wheel");
  wheel = el ? makeWheel(el, previewColor) : null;
  const input = focusPicker();
  // Near the bottom of the screen, the page scrolls so that the whole pop-up
  // shows (canvas page "2 · Main list", board "UI review", point 3).
  picker.scrollIntoView({ block: "nearest" });
  return input;
}

function openPicker(button) {
  closePicker(false);
  pickerButton = button;
  button.setAttribute("aria-expanded", "true");
  const wrap = button.parentElement;
  // No pop-up came (a network or server error): the button works again.
  const failed = () => {
    if (pickerButton === button) {
      button.setAttribute("aria-expanded", "false");
      pickerButton = null;
    }
  };
  htmx.ajax("GET", button.dataset.picker, { target: wrap, swap: "beforeend" }).then(() => {
    const box = wrap.querySelector(".picker");
    if (!box || pickerButton !== button) {
      if (box) box.remove();
      else failed();
      return;
    }
    picker = box;
    installPicker();
  }).catch(failed);
}

// Open to the left of the + when the list does not fit on the right.
// If it fits neither way (a narrow screen), move it in from the left edge.
function placePicker() {
  const width = document.documentElement.clientWidth;
  picker.classList.remove("flip");
  picker.style.left = "";
  picker.style.removeProperty("--caret");
  if (picker.getBoundingClientRect().right <= width - 8) {
    return;
  }
  picker.classList.add("flip");
  if (picker.getBoundingClientRect().left >= 8) {
    return;
  }
  // The pointer at the top still points at the +.
  const plus = picker.parentElement.getBoundingClientRect();
  picker.classList.remove("flip");
  picker.style.left = (8 - plus.left) + "px";
  picker.style.setProperty("--caret", (plus.left + plus.width / 2 - 8 - 6) + "px");
}

// labelLines are the lines of the label list that the search shows.
function labelLines() {
  return Array.from(picker.querySelectorAll(".picker-opt")).filter((o) => !o.hidden);
}

function cleanName(v) {
  return v.trim().replace(/\s+/g, " ");
}

function filterPicker() {
  const input = picker && picker.querySelector(".picker-search");
  if (!input) {
    return;
  }
  const typed = cleanName(input.value);
  const q = typed.toLowerCase();
  const full = picker.dataset.full === "true";
  let exact = false;
  picker.querySelectorAll(".picker-opt:not(.picker-create)").forEach((o) => {
    const name = o.dataset.name.toLowerCase();
    o.hidden = q !== "" && !name.includes(q);
    exact = exact || name === q;
  });
  const create = picker.querySelector(".picker-create");
  create.hidden = q === "" || exact || full;
  create.dataset.name = typed;
  create.querySelector(".picker-create-text").textContent = "Create “" + typed + "”";
  create.querySelector(".sw").setAttribute("aria-label", "Color of the new label “" + typed + "”");
  const empty = picker.querySelector(".picker-empty");
  empty.hidden = labelLines().length > 0;
  empty.textContent = full ? "No label has this name." : "Type a name to create a label.";
}

// sendForList sends a label change. The answer replaces the row or the list
// section; the htmx:afterSwap handler below keeps the list open. The search
// field gets empty at once, so what the user types next is a new search.
// keep (a delete) keeps the search and the color chosen for a new label.
// If the row leaves the list (a filter), the focus goes on at the next row,
// as after a delete (section 1).
function sendForList(method, url, options, keep) {
  reopenFor = picker.dataset.item;
  pickerBusy = true;
  carryColor = keep ? picker.dataset.newColor : null;
  const row = document.getElementById("item-" + reopenFor);
  const neighbor = row && (row.nextElementSibling || row.previousElementSibling);
  const check = neighbor && neighbor.querySelector(".check");
  focusIfGone = check ? check.id : "new-item";
  const input = picker.querySelector(".picker-search");
  if (input && !keep) {
    input.value = "";
    filterPicker();
  }
  // A network error rejects the promise; htmx:afterRequest below handles it.
  htmx.ajax(method, url, options).catch(() => {});
}

// toggleLabel adds the line's label to the item, or takes it off. The Create
// line makes the label with the color of its circle.
function toggleLabel(line) {
  const id = picker.dataset.item;
  const name = line.querySelector(".picker-name");
  if (name.getAttribute("aria-disabled") === "true") {
    return;
  }
  if (picker.dataset.draft) {
    // The list of the add form: the server draws #add-extras again with
    // the label picked or not (draft.go).
    const extras = document.getElementById("add-extras");
    const values = {};
    extras.querySelectorAll('input[type="hidden"]').forEach((i) => { values[i.name] = i.value; });
    const to = { target: "#add-extras", swap: "outerHTML" };
    if (line.classList.contains("picker-create")) {
      values.name = line.dataset.name;
      values.color = picker.dataset.newColor;
    } else if (name.getAttribute("aria-pressed") === "true") {
      values.remove = line.dataset.label;
    } else {
      values.name = line.dataset.name;
    }
    sendForList("POST", "/new/labels", { ...to, values });
    return;
  }
  const row = { target: "#item-" + id, swap: "outerHTML" };
  if (line.classList.contains("picker-create")) {
    sendForList("POST", "/items/" + id + "/labels", { ...row, values: { name: line.dataset.name, color: picker.dataset.newColor } });
  } else if (name.getAttribute("aria-pressed") === "true") {
    sendForList("DELETE", "/items/" + id + "/labels/" + line.dataset.label, row);
  } else {
    sendForList("POST", "/items/" + id + "/labels", { ...row, values: { name: line.dataset.name } });
  }
}

// The circle of the Create line chooses the new label's color with the wheel,
// in place of the list. Colors of the other labels change on the Labels page.
function previewColor(color) {
  if (!colorEdit) {
    return;
  }
  picker.querySelector(".picker-color .chip").dataset.color = color;
  colorEdit.swatch.dataset.color = color;
}

// openColor shows the wheel in place of the list.
function openColor(swatch, line) {
  colorEdit = { swatch, old: swatch.dataset.color };
  const view = picker.querySelector(".picker-color");
  view.querySelector(".picker-color-title").textContent = "Color of the new label “" + line.dataset.name + "”";
  const chip = view.querySelector(".chip");
  chip.textContent = line.dataset.name;
  chip.dataset.color = colorEdit.old;
  picker.querySelector(".picker-main").hidden = true;
  view.hidden = false;
  wheel.set(colorEdit.old);
  view.querySelector(".wheel").focus();
}

// endColor goes back to the list. keep keeps the color for the new label,
// which gets it when it is created.
function endColor(keep) {
  const edit = colorEdit;
  colorEdit = null;
  const color = keep ? wheel.color() : edit.old;
  edit.swatch.dataset.color = color;
  picker.dataset.newColor = color;
  picker.querySelector(".picker-color").hidden = true;
  picker.querySelector(".picker-main").hidden = false;
  edit.swatch.focus();
}

document.addEventListener("click", (e) => {
  const button = popupButton(e.target);
  if (button) {
    if (pickerButton === button) {
      closePicker(true);
    } else {
      openPicker(button);
    }
    return;
  }
  if (isNotesPicker() && e.target.closest(".notes-edit")) {
    editNotes();
    return;
  }
  if (!isLabelPicker() || !picker.contains(e.target) || pickerBusy) {
    return;
  }
  const line = e.target.closest(".picker-opt");
  if (e.target.closest("[data-color-done]")) endColor(true);
  else if (e.target.closest("[data-color-cancel]")) endColor(false);
  else if (!line) return;
  else if (e.target.closest("button.sw")) openColor(e.target.closest("button.sw"), line);
  else if (e.target.closest(".picker-name")) toggleLabel(line);
});

// A press in a pop-up must not take the focus out of its field; a press on
// another field of the due date or notes pop-up (also the Repeat list,
// which opens only if its press goes through) still moves the focus there. Its
// buttons must not take the focus first: Safari and Firefox on macOS do not
// focus a pressed button, so the field would blur and focusout would close
// the pop-up before the click. A press on the pop-up's own button keeps the
// focus, so its click can close the pop-up.
// A press on any pop-up button keeps the focus where it is, too: a focused
// date chip shows its ×, and if the focus left it, the chip would get
// narrower and move the next button away under the pointer before the
// click. The pop-up then puts the focus into its own field.
document.addEventListener("mousedown", (e) => {
  const button = popupButton(e.target);
  if (!picker) {
    if (button) e.preventDefault();
    return;
  }
  if (picker.contains(e.target)) {
    if (!e.target.closest(isFormPicker() ? "input, textarea" : ".picker-search")) {
      e.preventDefault();
    }
  } else if (button === pickerButton) {
    e.preventDefault();
  } else {
    if (button) e.preventDefault();
    closePicker(false);
  }
});

document.addEventListener("input", (e) => {
  if (picker && e.target.matches(".picker-search")) {
    filterPicker();
  }
});

// Keys in the label list: ↓ goes from the search field to the names, ↑ ↓
// move between names, Enter in the search field picks the first line. In
// the wheel view Enter is Done and Esc is Cancel; elsewhere Esc closes the
// question or the list.
document.addEventListener("keydown", (e) => {
  if (!isLabelPicker() || !picker.contains(e.target)) {
    return;
  }
  if (colorEdit) {
    if (e.key === "Escape" || (e.key === "Enter" && e.target.matches(".wheel"))) {
      e.preventDefault();
      endColor(e.key === "Enter");
    }
    return;
  }
  if (e.key === "Escape") {
    e.preventDefault();
    closePicker(true);
    return;
  }
  const names = labelLines().map((l) => l.querySelector(".picker-name"));
  if ((e.key === "ArrowDown" || e.key === "ArrowUp") && e.target.matches(".picker-search, .picker-name")) {
    e.preventDefault();
    const at = names.indexOf(e.target) + (e.key === "ArrowDown" ? 1 : -1);
    if (at < 0) picker.querySelector(".picker-search").focus();
    else if (names.length) names[Math.min(at, names.length - 1)].focus();
  } else if (e.key === "Enter" && e.target.matches(".picker-search")) {
    e.preventDefault();
    if (names.length && !e.isComposing && !pickerBusy) toggleLabel(names[0].closest(".picker-opt"));
  }
});

// Esc closes the due date or notes pop-up without saving. Enter submits
// the due date form; in the notes field Ctrl+Enter or ⌘+Enter does.
document.addEventListener("keydown", (e) => {
  if (!isFormPicker() || !picker.contains(e.target)) {
    return;
  }
  if (e.key === "Escape") {
    e.preventDefault();
    // An open Repeat list closes first; the next Esc closes the pop-up.
    if (!closeRepeatMenu(true)) closePicker(true);
  } else if (e.key === "Enter" && (e.ctrlKey || e.metaKey) && e.target.matches("textarea") && !e.isComposing) {
    e.preventDefault();
    picker.requestSubmit();
  }
});

// Focus that leaves a pop-up closes it (the due date pop-up without
// saving). The decision waits one tick, as for an edit row (section 2).
// While a label request is on its way, the list is moved to a new row and
// the focus is lost for a moment: that does not close it.
document.addEventListener("focusout", (e) => {
  if (!picker || !picker.contains(e.target)) {
    return;
  }
  const box = picker;
  setTimeout(() => {
    const now = document.activeElement;
    if (picker === box && !pickerBusy && !box.contains(now) && now !== pickerButton) {
      closePicker(false);
    }
  }, 0);
});

// A request of the label list that failed (also a network error, which has
// no answer): the list is not waiting any more, and the focus goes back to
// its search field.
document.addEventListener("htmx:afterRequest", (e) => {
  if (pickerBusy && !e.detail.successful) {
    pickerBusy = false;
    reopenFor = null;
    carrySearch = null;
    carryColor = null;
    focusIfGone = null;
    if (picker && picker.isConnected) {
      focusPicker();
    }
  }
});

// The pop-up belongs to the row: a swap of the row closes it. An error
// answer replaces only the pop-up (HX-Retarget): go on in the new one. After
// a label change the row (or the list) comes back: the old list moves into
// the new row and loads again, so it stays open.
document.addEventListener("htmx:afterSwap", () => {
  if (!picker || picker.isConnected) {
    return;
  }
  const fresh = pickerButton.isConnected && pickerButton.parentElement.querySelector(".picker");
  const button = !fresh && reopenFor && document.getElementById("label-add-" + reopenFor);
  if (button) {
    reopenFor = null;
    pickerButton = button;
    button.setAttribute("aria-expanded", "true");
    button.parentElement.append(picker);
    placePicker();
    // Keys typed while the list loads again go to its search field, and
    // from there to the new list.
    const search = focusPicker();
    carrySearch = search;
    htmx.ajax("GET", button.dataset.picker, { target: picker, swap: "outerHTML" }).catch(() => {});
    return;
  }
  if (fresh) {
    const typed = carrySearch ? carrySearch.value : "";
    const color = carryColor;
    reopenFor = null;
    pickerBusy = false;
    carrySearch = null;
    carryColor = null;
    focusIfGone = null;
    picker = fresh;
    if (color) {
      picker.dataset.newColor = color;
      picker.querySelector(".picker-create .sw").dataset.color = color;
    }
    const input = installPicker();
    if (input && input.matches(".picker-search")) {
      if (typed && !input.value) {
        input.value = typed;
        filterPicker();
      }
      input.setSelectionRange(input.value.length, input.value.length);
    }
    return;
  }
  // The list is gone with its row, or the whole list was drawn again.
  if (pickerButton.isConnected) {
    pickerButton.setAttribute("aria-expanded", "false");
  }
  const next = !pickerButton.isConnected && focusIfGone && document.getElementById(focusIfGone);
  resetPicker();
  if (next && (!document.activeElement || document.activeElement === document.body)) {
    next.focus();
  }
});

// 14. The due time field (canvas page "12 · 24-hour time", "type digits
// only"). A phone's number keyboard has no ":", so the field adds it after
// the hour: 1 4 3 0 gives 14:30, and 9 gives 09:. Only typed or pasted text
// at the end of the field is changed, so Backspace and edits in the middle
// work as usual. When the field loses focus, and before a send, a short time
// gets its full form: 14 and 14: give 14:00, 930 gives 09:30. The server
// reads the same short forms (normalizeClock in internal/todo/due.go).
function formatTyping(value) {
  const clean = value.replace(/[^0-9:]/g, "");
  const at = clean.indexOf(":");
  if (at >= 0) {
    let h = clean.slice(0, at);
    const m = clean.slice(at + 1).replace(/:/g, "").slice(0, 2);
    if (h.length === 1) {
      h = "0" + h;
    }
    return h + ":" + m;
  }
  if (clean.length === 1) {
    return clean >= "3" ? "0" + clean + ":" : clean;
  }
  if (clean.length === 2) {
    return clean <= "23" ? clean + ":" : clean;
  }
  if (clean.length === 3 && clean[0] >= "3") {
    return "0" + clean[0] + ":" + clean.slice(1);
  }
  if (clean.length <= 4 && clean.slice(0, 2) <= "23") {
    return clean.slice(0, 2) + ":" + clean.slice(2);
  }
  return clean;
}

function completeTime(value) {
  const s = value.trim();
  let h = s, m = "";
  if (s.includes(":")) {
    [h, m] = [s.slice(0, s.indexOf(":")), s.slice(s.indexOf(":") + 1)];
  } else if (s.length === 3 || s.length === 4) {
    [h, m] = [s.slice(0, -2), s.slice(-2)];
  } else if (s.length > 4) {
    return value;
  }
  m = m || "00";
  if (!/^[0-9]{1,2}$/.test(h) || !/^[0-9]{2}$/.test(m)) {
    return value;
  }
  h = h.padStart(2, "0");
  return h <= "23" && m <= "59" ? h + ":" + m : value;
}

document.addEventListener("input", (e) => {
  if (!e.target.matches(".due-time") || !e.inputType || !e.inputType.startsWith("insert")) {
    return;
  }
  const field = e.target;
  if (field.selectionStart === field.value.length) {
    field.value = formatTyping(field.value);
  }
});

document.addEventListener("change", (e) => {
  if (e.target.matches(".due-time")) {
    e.target.value = completeTime(e.target.value);
  }
});

// Capture runs before htmx reads the form, so Enter right after 14: sends
// 14:00.
document.addEventListener("submit", (e) => {
  e.target.querySelectorAll(".due-time").forEach((field) => {
    field.value = completeTime(field.value);
  });
}, true);

// 15. "Clear done" asks first (canvas page "3 · Done hidden"). The question
// comes from the server into #clear-confirm, with the focus on Delete.
// Cancel, Esc, or a second click on the button closes it and gives the
// focus back to the button.
function closeClearConfirm() {
  const slot = document.getElementById("clear-confirm");
  if (slot) {
    slot.replaceChildren();
  }
  const button = document.querySelector(".clear-done");
  if (button) {
    button.setAttribute("aria-expanded", "false");
    button.focus();
  }
}

// Capture runs before htmx: a click on the open button only closes.
document.addEventListener("click", (e) => {
  const button = e.target.closest(".clear-done");
  if (button && button.getAttribute("aria-expanded") === "true") {
    e.preventDefault();
    e.stopPropagation();
    closeClearConfirm();
  } else if (e.target.closest(".clear-cancel")) {
    closeClearConfirm();
  }
}, true);

document.addEventListener("htmx:afterSwap", (e) => {
  if (e.detail.target.id === "clear-confirm") {
    const button = document.querySelector(".clear-done");
    if (button) {
      button.setAttribute("aria-expanded", e.detail.target.childElementCount ? "true" : "false");
    }
  }
});

document.addEventListener("keydown", (e) => {
  const open = document.querySelector("#clear-confirm .clear-confirm");
  if (e.key === "Escape" && open && (open.contains(e.target) || e.target.closest(".clear-done"))) {
    e.preventDefault();
    closeClearConfirm();
  }
});

// 16. Undo after deleting an item (canvas page "2 · Main list", board "UI
// review", point 1). A click on × does not send the delete at once: the row
// becomes a gray line with Undo, in its place. As in the Android app, Undo
// needs no request. The delete goes to the server after UNDO_MS, before the
// next change (a request that is not a GET waits for it), before a link or a
// plain form leaves the page, and when the page closes. Ctrl+Z also undoes.
const UNDO_MS = 5000;
let pendingDelete = null; // { id, url, html, text, timer }

function undoLine(id, text) {
  const line = document.createElement("li");
  line.id = id;
  line.className = "item-undo";
  const msg = document.createElement("span");
  msg.id = id + "-undo-text";
  msg.textContent = "Deleted “" + text + "”.";
  const button = document.createElement("button");
  button.type = "button";
  button.className = "undo-btn";
  button.textContent = "Undo";
  button.setAttribute("aria-describedby", msg.id);
  line.append(msg, button);
  return line;
}

// commitDelete sends the waiting delete. The server answers with an empty
// row and the new toolbar, as for a delete without Undo.
function commitDelete() {
  const p = pendingDelete;
  if (!p) {
    return Promise.resolve();
  }
  pendingDelete = null;
  clearTimeout(p.timer);
  const line = document.getElementById(p.id);
  if (!line) {
    return fetch(p.url, { method: "DELETE", headers: { "HX-Request": "true" } }).catch(() => {});
  }
  // If the line has focus, go on at the next row's checkbox, else the
  // previous row's, else the add field.
  let near = null;
  if (line.contains(document.activeElement)) {
    const row = line.nextElementSibling || line.previousElementSibling;
    near = (row && row.querySelector(".check")) || document.getElementById("new-item");
  }
  return htmx.ajax("DELETE", p.url, { source: line, target: line, swap: "outerHTML" }).then(() => {
    if (near && document.body.contains(near) && (!document.activeElement || document.activeElement === document.body)) {
      near.focus();
    }
    if (document.getElementById(p.id)) {
      // A list answer brought the row back while the delete was on its way.
      htmx.ajax("GET", "/", { target: "#list-section", swap: "outerHTML" });
    }
  }).catch(() => {});
}

function undoDelete() {
  const p = pendingDelete;
  if (!p) {
    return;
  }
  pendingDelete = null;
  clearTimeout(p.timer);
  const line = document.getElementById(p.id);
  if (!line) {
    return;
  }
  const t = document.createElement("template");
  t.innerHTML = p.html;
  const row = t.content.firstElementChild;
  line.replaceWith(row);
  htmx.process(row);
  const button = row.querySelector(".delete");
  if (button) {
    button.focus();
  }
}

document.addEventListener("htmx:confirm", (e) => {
  const elt = e.detail.elt;
  const verb = String(e.detail.verb || "").toLowerCase();
  const row = verb === "delete" && elt.matches && elt.matches(".delete") ? elt.closest("li.item") : null;
  if (row) {
    e.preventDefault();
    commitDelete(); // one Undo at a time
    const text = row.querySelector(".text");
    pendingDelete = {
      id: row.id, url: e.detail.path, html: row.outerHTML,
      text: text ? text.textContent.trim() : "", timer: setTimeout(commitDelete, UNDO_MS),
    };
    const line = undoLine(row.id, pendingDelete.text);
    row.replaceWith(line);
    line.querySelector(".undo-btn").focus();
    return;
  }
  if (!pendingDelete || verb === "get") {
    return;
  }
  // The next change: first the delete, then this request.
  e.preventDefault();
  commitDelete().then(() => e.detail.issueRequest(true));
});

// A list answer (a GET) can bring the row back while Undo shows: keep the line.
document.addEventListener("htmx:afterSettle", () => {
  const p = pendingDelete;
  const row = p && document.getElementById(p.id);
  if (row && row.classList.contains("item")) {
    p.html = row.outerHTML;
    row.replaceWith(undoLine(p.id, p.text));
  }
});

document.addEventListener("click", (e) => {
  // Only the Undo of a deleted item: the other Undo buttons have their own.
  if (e.target.closest && e.target.closest(".item-undo .undo-btn")) {
    undoDelete();
  }
});

document.addEventListener("keydown", (e) => {
  if (!pendingDelete || !(e.ctrlKey || e.metaKey) || e.shiftKey || e.altKey || e.key.toLowerCase() !== "z") {
    return;
  }
  const el = document.activeElement;
  if (el && (el.matches("input, textarea, select") || el.isContentEditable)) {
    return; // Ctrl+Z in a field undoes the typing
  }
  e.preventDefault();
  undoDelete();
});

// A link or a plain form (Log out) leaves the page: delete first.
document.addEventListener("click", (e) => {
  if (!pendingDelete || e.button !== 0 || e.ctrlKey || e.metaKey || e.shiftKey || e.altKey) {
    return;
  }
  const link = e.target.closest && e.target.closest("a[href]");
  if (!link || link.target || link.hasAttribute("download") || link.matches("[hx-get], [hx-boost]")) {
    return;
  }
  e.preventDefault();
  commitDelete().finally(() => { location.href = link.href; });
}, true);

document.addEventListener("submit", (e) => {
  const form = e.target;
  if (!pendingDelete || form.matches("[hx-post], [hx-put], [hx-get], [hx-delete]")) {
    return;
  }
  e.preventDefault();
  commitDelete().finally(() => form.submit());
}, true);

// The page closes or reloads: the browser still sends the delete.
if (window.addEventListener) {
  window.addEventListener("pagehide", () => {
    const p = pendingDelete;
    if (!p) {
      return;
    }
    pendingDelete = null;
    clearTimeout(p.timer);
    fetch(p.url, { method: "DELETE", headers: { "HX-Request": "true" }, keepalive: true }).catch(() => {});
  });
}

// 17. A repeating item that is checked off moves to its next date (canvas
// page "20 · Repeat", option R1). The server sends the header
// X-Todo-Repeated (handlers.go, announceRepeat); a gray line under the row
// says when the item comes again, with Undo for UNDO_MS. Undo sends the old
// date back. One answer can settle more than one part (the row and the
// toolbar): the line comes once per answer.
const repeatShown = new WeakSet();
document.addEventListener("htmx:afterSettle", (e) => {
  const xhr = e.detail && e.detail.xhr;
  const header = xhr && !repeatShown.has(xhr) && xhr.getResponseHeader("X-Todo-Repeated");
  if (!header) {
    return;
  }
  repeatShown.add(xhr);
  let d;
  try {
    d = JSON.parse(header);
  } catch {
    return;
  }
  const row = document.getElementById("item-" + d.id);
  const col = row && row.querySelector(".text-col");
  if (!col) {
    return;
  }
  const old = col.querySelector(".repeat-msg");
  if (old) {
    old.remove();
  }
  const line = document.createElement("div");
  line.className = "repeat-msg";
  line.setAttribute("role", "status");
  const msg = document.createElement("span");
  msg.textContent = "Done. Next time: " + d.next + ".";
  const button = document.createElement("button");
  button.type = "button";
  button.className = "undo-btn";
  button.textContent = "Undo";
  button.addEventListener("click", () => {
    htmx.ajax("PUT", "/items/" + d.id + "/due", {
      target: row, swap: "outerHTML",
      values: { due_date: d.date, due_time: d.time, due_tz: d.tz, due_repeat: d.repeat },
    }).catch(() => {});
  });
  line.append(msg, button);
  col.append(line);
  setTimeout(() => line.remove(), UNDO_MS);
});

// 18. Search the items (canvas page "19 · Search", option A, match M2). The
// ⌕ left of the buttons, or "/", opens a field between the buttons and the
// list. While you type, the list shows only the rows that match, and the
// matching letters are yellow. Fuzzy (M2): the typed letters must come in
// this order in the text or the label names, with other letters between
// them allowed; case and Polish letters do not matter. The notes count too;
// a match in them gives the notes chip a yellow edge (canvas page "21 ·
// Notes"). Esc or × closes it.
// A row that comes after the search started (a new item) always shows. The
// search needs no request: it works on the rows of the page.
const search = { open: false, query: "", known: null };

function searchAvailable() {
  const toolbar = document.getElementById("toolbar");
  return !!(toolbar && !toolbar.hidden && document.getElementById("search-row"));
}

// foldChars gives one lower-case letter without its accent for each
// character of s ("Żółw" → z, o, l, w).
function foldChars(s) {
  return Array.from(s).map((c) => (c === "ł" || c === "Ł" ? "l" : c.normalize("NFD")[0].toLowerCase()));
}

// findMatch returns the places in hay of the letters of q: one run of them
// if there is one, else the first letters in order; null if q is not there.
function findMatch(q, hay) {
  const at = hay.join("").indexOf(q.join(""));
  if (at >= 0) {
    return q.map((_, i) => at + i);
  }
  const places = [];
  for (let k = 0; k < hay.length && places.length < q.length; k++) {
    if (hay[k] === q[places.length]) {
      places.push(k);
    }
  }
  return places.length === q.length ? places : null;
}

// paint writes chars into el, with the letters at places in <mark>.
function paint(el, chars, places) {
  if (!places.length) {
    if (el.querySelector("mark")) {
      el.textContent = chars.join("");
    }
    return;
  }
  const marked = new Set(places);
  el.textContent = "";
  let run = "";
  let inMark = marked.has(0);
  const flush = () => {
    if (!run) {
      return;
    }
    if (inMark) {
      const m = document.createElement("mark");
      m.textContent = run;
      el.append(m);
    } else {
      el.append(run);
    }
    run = "";
  };
  chars.forEach((c, i) => {
    if (marked.has(i) !== inMark) {
      flush();
      inMark = marked.has(i);
    }
    run += c;
  });
  flush();
}

function applySearch() {
  const items = document.getElementById("items");
  if (!items) {
    return;
  }
  const q = search.open ? foldChars(search.query).filter((c) => !/\s/.test(c)) : [];
  let shown = 0;
  let total = 0;
  for (const row of items.querySelectorAll("li.item")) {
    total++;
    const textEl = row.querySelector(".text");
    if (!textEl) {
      row.classList.remove("search-miss"); // an edit row stays
      shown++;
      continue;
    }
    const chars = Array.from(textEl.textContent);
    const labels = Array.from(row.querySelectorAll(".chip a"), (a) => a.textContent).join(" ");
    const notesEl = row.querySelector(".notes-add[data-notes]");
    const head = textEl.textContent + " " + labels;
    const places = q.length ? findMatch(q, foldChars(head + (notesEl ? " " + notesEl.dataset.notes : ""))) : [];
    const keep = !q.length || !!places || (search.known && !search.known.has(row.id));
    row.classList.toggle("search-miss", !keep);
    if (notesEl) {
      const headLength = Array.from(head).length;
      notesEl.parentElement.classList.toggle("search-hit", !!places && places.some((p) => p > headLength));
    }
    paint(textEl, chars, places ? places.filter((p) => p < chars.length) : []);
    if (keep) {
      shown++;
    }
  }
  const counter = document.querySelector("#toolbar .counter");
  if (counter) {
    if (!counter.dataset.base) {
      counter.dataset.base = counter.textContent;
    }
    counter.textContent = q.length ? (shown ? shown + " of " + total + " items match" : "No match") : counter.dataset.base;
  }
  const status = document.getElementById("search-status");
  if (status) {
    status.textContent = q.length ? (shown ? shown + " of " + total + " items match" : "No items match") : "";
  }
  let empty = document.getElementById("search-empty");
  if (q.length && !shown) {
    if (!empty) {
      empty = document.createElement("div");
      empty.id = "search-empty";
      empty.className = "search-empty";
      items.before(empty);
    }
    const title = document.createElement("b");
    title.textContent = "No items match “" + search.query.trim() + "”";
    empty.replaceChildren(title, "Check the spelling, or press Esc to see all items.");
  } else if (empty) {
    empty.remove();
  }
  document.body.classList.toggle("searching", q.length > 0);
}

// syncSearch makes the page show the search state; list answers replace the
// field and the rows, so it runs again after each of them.
function syncSearch() {
  const row = document.getElementById("search-row");
  const input = document.getElementById("search");
  const toggle = document.querySelector(".search-toggle");
  if (row) {
    row.hidden = !search.open;
  }
  if (input && input.value !== search.query) {
    input.value = search.query;
  }
  if (toggle) {
    toggle.setAttribute("aria-expanded", String(search.open));
  }
  applySearch();
}

function openSearch() {
  if (!search.open) {
    search.open = true;
    search.known = new Set(Array.from(document.querySelectorAll("#items li.item"), (r) => r.id));
  }
  syncSearch();
  const input = document.getElementById("search");
  if (input) {
    input.focus();
  }
}

function closeSearch() {
  search.open = false;
  search.query = "";
  search.known = null;
  syncSearch();
  const toggle = document.querySelector(".search-toggle");
  if (toggle) {
    toggle.focus();
  }
}

document.addEventListener("click", (e) => {
  if (!e.target.closest) {
    return;
  }
  if (e.target.closest(".search-toggle")) {
    if (search.open) {
      closeSearch();
    } else {
      openSearch();
    }
  } else if (e.target.closest(".search-close")) {
    closeSearch();
  }
});

document.addEventListener("input", (e) => {
  if (e.target.id === "search") {
    search.query = e.target.value;
    applySearch();
  }
});

document.addEventListener("keydown", (e) => {
  if (e.target.id === "search" && e.key === "Escape") {
    e.preventDefault();
    closeSearch();
    return;
  }
  if (e.key !== "/" || e.ctrlKey || e.metaKey || e.altKey || e.isComposing || !searchAvailable()) {
    return;
  }
  const el = document.activeElement;
  if (el && (el.matches("input, textarea, select") || el.isContentEditable)) {
    return;
  }
  e.preventDefault();
  openSearch();
});

// A list answer replaces the field: if it had the focus, the new field gets
// it. Other answers (a pop-up that opens) keep the focus where they put it.
let searchFocused = null;
document.addEventListener("htmx:beforeSwap", () => {
  const el = document.activeElement;
  searchFocused = el && el.id === "search" ? el : null;
});
document.addEventListener("htmx:afterSettle", () => {
  if (!search.open) {
    return;
  }
  syncSearch();
  const input = document.getElementById("search");
  if (searchFocused && !searchFocused.isConnected && input) {
    input.focus();
    input.setSelectionRange(input.value.length, input.value.length);
  }
  searchFocused = null;
});

// 19. The × on the notes chip removes the notes at once, with Undo (canvas
// page "21 · Notes", board "Remove the notes", option X1). The row comes back
// with the gray notes icon and "Notes removed. Undo" at the end of its chips.
// Undo, or Ctrl+Z, sends the old notes back. The line goes away after
// UNDO_MS, or when the row changes again.
const notesRemoves = new WeakMap(); // xhr → { id, notes }
let removedNotes = null; // { id, notes, line, timer }

document.addEventListener("htmx:beforeRequest", (e) => {
  const elt = e.detail.elt;
  if (!elt || !elt.matches || !elt.matches(".notes-x") || !e.detail.xhr) {
    return;
  }
  const button = elt.parentElement.querySelector(".notes-add[data-notes]");
  if (button) {
    notesRemoves.set(e.detail.xhr, { id: button.id.slice("notes-add-".length), notes: button.dataset.notes });
  }
});

document.addEventListener("htmx:afterSettle", (e) => {
  const xhr = e.detail && e.detail.xhr;
  const removed = xhr && notesRemoves.get(xhr);
  if (!removed) {
    return;
  }
  notesRemoves.delete(xhr);
  const row = document.getElementById("item-" + removed.id);
  const meta = row && row.querySelector(".meta");
  if (xhr.status !== 200 || !meta) {
    return;
  }
  clearNotesUndo();
  const line = document.createElement("span");
  line.className = "notes-msg";
  line.setAttribute("role", "status");
  const msg = document.createElement("span");
  msg.textContent = "Notes removed.";
  const button = document.createElement("button");
  button.type = "button";
  button.className = "undo-btn notes-undo";
  button.textContent = "Undo";
  button.addEventListener("click", undoNotes);
  line.append(msg, button);
  meta.append(line);
  removedNotes = { id: removed.id, notes: removed.notes, line, timer: setTimeout(clearNotesUndo, UNDO_MS) };
});

function clearNotesUndo() {
  const r = removedNotes;
  if (!r) {
    return;
  }
  removedNotes = null;
  clearTimeout(r.timer);
  r.line.remove();
}

function undoNotes() {
  const r = removedNotes;
  if (!r || !document.body.contains(r.line)) {
    clearNotesUndo(); // the row came back new: its notes may be new too
    return;
  }
  const row = r.line.closest("li.item");
  clearNotesUndo();
  htmx.ajax("PUT", "/items/" + r.id + "/notes", { target: row, swap: "outerHTML", values: { notes: r.notes } }).then(() => {
    const chip = document.getElementById("notes-add-" + r.id);
    if (chip && (!document.activeElement || document.activeElement === document.body || !document.body.contains(document.activeElement))) {
      chip.focus();
    }
  }).catch(() => {});
}

document.addEventListener("keydown", (e) => {
  if (!removedNotes || pendingDelete || !(e.ctrlKey || e.metaKey) || e.shiftKey || e.altKey || e.key.toLowerCase() !== "z") {
    return;
  }
  const el = document.activeElement;
  if (el && (el.matches("input, textarea, select") || el.isContentEditable)) {
    return; // Ctrl+Z in a field undoes the typing
  }
  e.preventDefault();
  undoNotes();
});

// 20. A due date typed at the end of the add field (issue #6, canvas page
// "10 · Due date", board "due date typed in the add field", option H2). A
// date counts only after the word "due" or the sign "@", and only at the end
// of the text. While the user types, a dotted line marks the date words, and
// the date button under the field shows the date. Enter adds the item
// without these words. The × on the date button, or a date picked in the
// calendar, says no: the words stay text, and the app does not take them
// again. testdata/typed-due.json has the cases: a wrong date is worse than no
// date.
const TYPED_DAYS = new Map(Object.entries({
  sun: 0, sunday: 0, mon: 1, monday: 1, tue: 2, tuesday: 2, wed: 3, wednesday: 3,
  thu: 4, thursday: 4, fri: 5, friday: 5, sat: 6, saturday: 6,
}));
const TYPED_MONTHS = new Map(Object.entries({
  jan: 1, january: 1, feb: 2, february: 2, mar: 3, march: 3, apr: 4, april: 4, may: 5,
  jun: 6, june: 6, jul: 7, july: 7, aug: 8, august: 8, sep: 9, sept: 9, september: 9,
  oct: 10, october: 10, nov: 11, november: 11, dec: 12, december: 12,
}));

// parseTypedDue finds a due date at the end of text: after "due" or "@" at
// the start of a word, with some text before it. It returns the place of the
// date words (start, end) and the date as the date fields want it, or null.
function parseTypedDue(text, now) {
  const marker = /(?<=^|\s)(?:due\s+|@\s*)/gi;
  let m;
  while ((m = marker.exec(text))) {
    if (!text.slice(0, m.index).trim()) {
      continue;
    }
    const when = typedWhen(text.slice(m.index + m[0].length), now);
    if (when) {
      return { start: m.index, end: text.trimEnd().length, date: when.date, time: when.time };
    }
  }
  return null;
}

// typedWhen reads the words after "due" or "@": a day word, a weekday or a
// date, then a time; or only a time (today). Anything else gives null.
function typedWhen(rest, now) {
  const words = rest.trim().split(/\s+/).map((w) => w.toLowerCase());
  if (!words[0]) {
    return null;
  }
  let time = "";
  const t = /^([01]?\d|2[0-3]):([0-5]\d)$/.exec(words[words.length - 1]);
  if (t) {
    time = t[1].padStart(2, "0") + ":" + t[2];
    words.pop();
  }
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const later = (days) => new Date(today.getFullYear(), today.getMonth(), today.getDate() + days);
  let day = null;
  if (!words.length) {
    day = time ? today : null;
  } else if (words.length === 1 && words[0] === "today") {
    day = today;
  } else if (words.length === 1 && words[0] === "tomorrow") {
    day = later(1);
  } else if (words.length === 1 && TYPED_DAYS.has(words[0])) {
    // The next such day. Today only with a time later than now.
    let ahead = (TYPED_DAYS.get(words[0]) - today.getDay() + 7) % 7;
    if (ahead === 0 && !(time && Number(time.slice(0, 2)) * 60 + Number(time.slice(3)) > now.getHours() * 60 + now.getMinutes())) {
      ahead = 7;
    }
    day = later(ahead);
  } else {
    day = typedDate(words, today.getFullYear());
  }
  if (!day) {
    return null;
  }
  const pad = (n) => String(n).padStart(2, "0");
  return { date: day.getFullYear() + "-" + pad(day.getMonth() + 1) + "-" + pad(day.getDate()), time };
}

// typedDate reads "12.11", "12.11.2027", "12 Nov", "Nov 12", each with an
// optional year. A day that does not exist (31 Feb) gives null.
function typedDate(words, year) {
  let d, mo, y = year;
  const dots = words.length === 1 && /^(\d{1,2})\.(\d{1,2})(?:\.(\d{4}))?$/.exec(words[0]);
  if (dots) {
    [d, mo] = [Number(dots[1]), Number(dots[2])];
    y = dots[3] ? Number(dots[3]) : year;
  } else if (words.length === 2 || words.length === 3) {
    const num = (w) => (/^\d{1,2}$/.test(w) ? Number(w) : 0);
    if (num(words[0]) && TYPED_MONTHS.has(words[1])) {
      [d, mo] = [num(words[0]), TYPED_MONTHS.get(words[1])];
    } else if (TYPED_MONTHS.has(words[0]) && num(words[1])) {
      [d, mo] = [num(words[1]), TYPED_MONTHS.get(words[0])];
    } else {
      return null;
    }
    if (words.length === 3) {
      if (!/^\d{4}$/.test(words[2])) {
        return null;
      }
      y = Number(words[2]);
    }
  } else {
    return null;
  }
  const day = new Date(y, mo - 1, d);
  return mo >= 1 && mo <= 12 && day.getMonth() === mo - 1 && day.getDate() === d ? day : null;
}

// The state of the add field: the date that the text put in the date button
// ("date time"), and the date words that the user said no to.
const typed = { applied: null, refused: null, timer: 0 };

// typedMatch returns the date at the end of the add field, unless the user
// said no to these words.
function typedMatch() {
  const input = document.getElementById("new-item");
  const m = input && parseTypedDue(input.value, new Date());
  if (!m) {
    return null;
  }
  m.words = input.value.slice(m.start, m.end).toLowerCase().replace(/\s+/g, " ");
  return m.words === typed.refused ? null : m;
}

// drawTypedLine repeats the text of the add field in its layer, with the
// date words in <mark>, at the same place as in the field.
function drawTypedLine() {
  const input = document.getElementById("new-item");
  const layer = input && input.parentElement.querySelector(".add-mirror");
  if (!layer) {
    return;
  }
  const m = typedMatch();
  layer.textContent = "";
  if (!m) {
    return;
  }
  const cs = window.getComputedStyle(input);
  for (const p of ["fontFamily", "fontSize", "fontWeight", "fontStyle", "lineHeight", "letterSpacing", "wordSpacing", "textIndent",
    "paddingTop", "paddingRight", "paddingBottom", "paddingLeft",
    "borderTopWidth", "borderRightWidth", "borderBottomWidth", "borderLeftWidth"]) {
    layer.style[p] = cs[p];
  }
  const mark = document.createElement("mark");
  mark.textContent = input.value.slice(m.start, m.end);
  layer.append(input.value.slice(0, m.start), mark, input.value.slice(m.end));
  layer.scrollLeft = input.scrollLeft;
}

// syncTypedDue puts the typed date in the date button, or takes it out when
// the text has no date any more. The server draws the button (draft.go).
function syncTypedDue() {
  const extras = document.getElementById("add-extras");
  if (!extras) {
    return;
  }
  const m = typedMatch();
  const key = m ? m.date + " " + m.time : null;
  if (key === typed.applied || (!m && !typed.applied)) {
    return;
  }
  const values = {};
  extras.querySelectorAll('input[type="hidden"]').forEach((i) => { values[i.name] = i.value; });
  if (m) {
    Object.assign(values, { due_date: m.date, due_time: m.time, due_repeat: "" });
  } else {
    values.clear = "1";
  }
  typed.applied = key;
  htmx.ajax("POST", "/new/due", { target: "#add-extras", swap: "outerHTML", values });
}

// refuseTyped keeps the words of the field as text from now on.
function refuseTyped() {
  const m = typedMatch();
  if (typed.applied && m) {
    typed.refused = m.words;
  }
  typed.applied = null;
  drawTypedLine();
}

document.addEventListener("input", (e) => {
  if (e.target.id !== "new-item") {
    return;
  }
  drawTypedLine();
  clearTimeout(typed.timer);
  typed.timer = setTimeout(syncTypedDue, 150);
});

// The field scrolls a long text; the layer follows it.
for (const name of ["keyup", "click", "focusin", "select"]) {
  document.addEventListener(name, (e) => {
    if (e.target.id === "new-item") {
      drawTypedLine();
    }
  });
}
if (window.addEventListener) {
  window.addEventListener("resize", drawTypedLine);
}

// The × of the date button, or a date from the calendar: no to the words.
document.addEventListener("click", (e) => {
  if (e.target.closest && e.target.closest("#add-extras .pill-x")) {
    refuseTyped();
  }
}, true);
document.addEventListener("htmx:beforeRequest", (e) => {
  if (e.detail.elt && e.detail.elt.closest && e.detail.elt.closest("#due-picker-0")) {
    refuseTyped();
  }
});

// Enter or Add: the item gets the date, and its text loses the date words.
// The date goes with the request too, in case the button is not drawn yet.
document.addEventListener("htmx:configRequest", (e) => {
  if (!e.detail.elt || e.detail.elt.id !== "add-form") {
    return;
  }
  const m = typedMatch();
  if (!m) {
    return;
  }
  clearTimeout(typed.timer);
  const f = e.detail.formData;
  f.set("text", document.getElementById("new-item").value.slice(0, m.start).trimEnd());
  f.set("due_date", m.date);
  f.set("due_time", m.time);
  f.set("due_repeat", "");
});

document.addEventListener("htmx:afterRequest", (e) => {
  if (e.detail.elt && e.detail.elt.id === "add-form" && e.detail.successful) {
    typed.applied = null;
    typed.refused = null;
  }
});
document.addEventListener("htmx:afterSettle", drawTypedLine);

// 21. The Repeat list of the due date pop-up (canvas page "20 · Repeat",
// frame "The Repeat list open"). The page draws it, not a <select>: Firefox
// draws the open list of a <select> with the system look, and CSS cannot
// change it. A click on the button opens the list; ↑ ↓ Home End move, Enter
// or Space or a click chooses; Esc, Tab or a click outside closes it. The
// choice goes into the hidden field due_repeat, which the form sends.

// repeatStep is the line that key moves to, from line at in a list of n
// lines, or -1 if the key does not move.
function repeatStep(at, key, n) {
  switch (key) {
    case "ArrowDown": return Math.min(at + 1, n - 1);
    case "ArrowUp": return Math.max(at - 1, 0);
    case "Home": return 0;
    case "End": return n - 1;
  }
  return -1;
}

// openRepeatMenu shows the list of the button, with the focus on the chosen
// line. The focus ring shows only when the keys are in use (byKey).
function openRepeatMenu(button, byKey) {
  const menu = button.nextElementSibling;
  menu.classList.toggle("by-key", byKey);
  menu.hidden = false;
  button.setAttribute("aria-expanded", "true");
  (menu.querySelector('[aria-selected="true"]') || menu.firstElementChild).focus();
}

// closeRepeatMenu hides the open Repeat list, if there is one, and says if
// it did. With refocus, the focus goes back to its button.
function closeRepeatMenu(refocus) {
  const menu = document.querySelector(".repeat-menu:not([hidden])");
  if (!menu) {
    return false;
  }
  const button = menu.previousElementSibling;
  menu.hidden = true;
  button.setAttribute("aria-expanded", "false");
  if (refocus) {
    button.focus();
  }
  return true;
}

// chooseRepeat puts the rule of a line into the form and the button.
function chooseRepeat(line) {
  const menu = line.parentElement;
  menu.querySelectorAll('[role="option"]').forEach((o) => o.setAttribute("aria-selected", String(o === line)));
  menu.closest("form").elements.due_repeat.value = line.dataset.value;
  menu.previousElementSibling.querySelector(".due-repeat-text").textContent = line.textContent;
  closeRepeatMenu(true);
}

document.addEventListener("click", (e) => {
  const button = e.target.closest && e.target.closest(".due-repeat");
  if (button) {
    if (button.getAttribute("aria-expanded") === "true") closeRepeatMenu(true);
    else openRepeatMenu(button, e.detail === 0); // detail 0: Enter or Space
    return;
  }
  const line = e.target.closest && e.target.closest('.repeat-menu [role="option"]');
  if (line) {
    chooseRepeat(line);
  }
});

// A press outside the open list (and its button) closes it.
document.addEventListener("mousedown", (e) => {
  if (!e.target.closest || !e.target.closest(".repeat-field")) {
    closeRepeatMenu(false);
  }
});

// The focus that goes out of the list (Tab, a click on a field) closes it.
document.addEventListener("focusin", (e) => {
  if (!e.target.closest || !e.target.closest(".repeat-field")) {
    closeRepeatMenu(false);
  }
});

document.addEventListener("keydown", (e) => {
  const line = e.target.closest && e.target.closest('.repeat-menu [role="option"]');
  if (!line) {
    if (e.target.matches && e.target.matches(".due-repeat") && (e.key === "ArrowDown" || e.key === "ArrowUp")) {
      e.preventDefault();
      openRepeatMenu(e.target, true);
    }
    return;
  }
  line.parentElement.classList.add("by-key");
  const lines = Array.from(line.parentElement.children);
  const at = repeatStep(lines.indexOf(line), e.key, lines.length);
  if (at >= 0) {
    e.preventDefault();
    lines[at].focus();
  } else if (e.key === "Enter" || e.key === " ") {
    e.preventDefault();
    chooseRepeat(line);
  } else if (e.key === "Tab") {
    // From the button, Tab goes on to the next field as usual.
    closeRepeatMenu(true);
  }
});
