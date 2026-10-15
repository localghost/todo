// Small page behaviors that htmx attributes cannot express.
// Approved in Claude Design, page "8 · Focus and typing".

// 1. Focus after check or delete. A check keeps focus by itself (the new
// checkbox has the same id). If the focused row disappears, focus moves to
// the next row's checkbox, else the previous row's, else the add input.
let focusAfterSwap = null;

document.addEventListener("htmx:beforeSwap", (e) => {
  // The row of the element that sent the request; the target can be the whole
  // list section when an item moves into or out of the overdue group.
  const source = e.detail.elt;
  const row = source && source.closest ? source.closest("li.item") : null;
  const active = document.activeElement;
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
const ADD_FIELDS = ["text", "due_date", "due_time"];

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
  if (e.key.length !== 1 || e.key === " ") {
    return;
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

// 10. Overdue items move to the top. The toolbar (swapped with every list and
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

// 11. A whole-list answer (an item moved into or out of the overdue group, or
// the overdue refresh) must not replace a row that is being edited elsewhere:
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

// 12. Another tab may have changed "hide done". A row answer brings the
// current toolbar; if its state differs from the list on this page, load the
// list again, so the page does not show a mix of both states.
document.addEventListener("htmx:oobAfterSwap", () => {
  const toolbar = document.getElementById("toolbar");
  const section = document.getElementById("list-section");
  if (toolbar && section && toolbar.dataset.hideDone !== section.dataset.hideDone) {
    htmx.ajax("GET", "/", { target: "#list-section", swap: "outerHTML" });
  }
});

// 13. Pop-ups on a row: the label list under the + (canvas page "16 ·
// Labels") and the due date pop-up under the calendar icon (canvas page
// "10 · Due date"). Only one is open at a time. Esc or a click outside
// closes it.
// The label list has a search field. Typing narrows the list; a name that
// matches no label offers "Create …". Choosing one adds that one label and
// closes the list. Tab closes it.
// The due date pop-up is a form: Enter saves, Tab moves inside it, and focus
// that leaves it closes it without saving. The due text opens it too.
let picker = null; // the open pop-up
let pickerButton = null; // its + or calendar icon

function isDuePicker() {
  return picker !== null && picker.classList.contains("due-picker");
}

// focusPicker puts the cursor in the first field of the pop-up: the search
// of the label list, or the date of the due date pop-up.
function focusPicker() {
  const input = picker.querySelector('input:not([type="hidden"])');
  if (input) {
    input.focus();
  }
  return input;
}

// popupButton is the + or calendar icon that a press on target belongs to.
// A due text belongs to the calendar icon of its row.
function popupButton(target) {
  if (!target.closest) {
    return null;
  }
  const due = target.closest("button.due");
  return due ? due.parentElement.querySelector(".due-add") : target.closest(".label-add, .due-add");
}

function closePicker(refocus) {
  if (!picker) {
    return;
  }
  picker.remove();
  pickerButton.setAttribute("aria-expanded", "false");
  if (refocus && pickerButton.isConnected) {
    pickerButton.focus();
  }
  picker = null;
  pickerButton = null;
}

function openPicker(button) {
  closePicker(false);
  pickerButton = button;
  button.setAttribute("aria-expanded", "true");
  const wrap = button.parentElement;
  htmx.ajax("GET", button.dataset.picker, { target: wrap, swap: "beforeend" }).then(() => {
    const box = wrap.querySelector(".picker");
    if (!box || pickerButton !== button) {
      if (box) box.remove();
      return;
    }
    picker = box;
    placePicker();
    filterPicker();
    focusPicker();
  });
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

function pickerOptions() {
  return Array.from(picker.querySelectorAll(".picker-opt")).filter((o) => !o.hidden);
}

function setActive(option) {
  const input = picker.querySelector(".picker-search");
  picker.querySelectorAll(".picker-opt").forEach((o) => o.setAttribute("aria-selected", o === option ? "true" : "false"));
  if (option) {
    input.setAttribute("aria-activedescendant", option.id);
    option.scrollIntoView({ block: "nearest" });
  } else {
    input.removeAttribute("aria-activedescendant");
  }
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
  const taken = (picker.dataset.taken || "").split(",").map((n) => n.trim().toLowerCase()).filter(Boolean);
  let exact = false;
  picker.querySelectorAll(".picker-opt:not(.picker-create)").forEach((o) => {
    const name = o.dataset.name.toLowerCase();
    o.hidden = q !== "" && !name.includes(q);
    exact = exact || name === q;
  });
  const create = picker.querySelector(".picker-create");
  create.hidden = q === "" || exact || taken.includes(q);
  create.dataset.name = typed;
  create.querySelector(".picker-create-text").textContent = "Create “" + typed + "”";
  const shown = pickerOptions();
  const empty = picker.querySelector(".picker-empty");
  empty.hidden = shown.length > 0;
  empty.textContent = taken.includes(q) ? "“" + typed + "” is already on this item." : "Type a name to create a label.";
  setActive(shown[0] || null);
}

function choose(option) {
  const id = picker.dataset.item;
  const name = option.dataset.name;
  // The row comes back without the list; focus goes on at its + (section 1).
  focusAfterSwap = "label-add-" + id;
  htmx.ajax("POST", "/items/" + id + "/labels", { target: "#item-" + id, swap: "outerHTML", values: { name: name } });
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
  const option = e.target.closest && e.target.closest(".picker-opt");
  if (option && picker && picker.contains(option)) {
    choose(option);
  }
});

// A press in a pop-up must not take the focus out of its field; the fields
// and buttons of the due date pop-up still take focus. A press on the
// pop-up's own button keeps the focus, so its click can close the pop-up.
document.addEventListener("mousedown", (e) => {
  if (!picker) {
    return;
  }
  if (picker.contains(e.target)) {
    if (!e.target.closest(isDuePicker() ? "input, button" : ".picker-search")) {
      e.preventDefault();
    }
  } else if (popupButton(e.target) === pickerButton) {
    e.preventDefault();
  } else {
    closePicker(false);
  }
});

document.addEventListener("input", (e) => {
  if (picker && e.target.matches(".picker-search")) {
    filterPicker();
  }
});

document.addEventListener("keydown", (e) => {
  if (!picker || isDuePicker() || !picker.contains(e.target)) {
    return;
  }
  const shown = pickerOptions();
  const active = shown.find((o) => o.getAttribute("aria-selected") === "true");
  const at = shown.indexOf(active);
  if (e.key === "ArrowDown" || e.key === "ArrowUp") {
    e.preventDefault();
    if (shown.length) {
      const step = e.key === "ArrowDown" ? 1 : -1;
      setActive(shown[(at + step + shown.length) % shown.length]);
    }
  } else if (e.key === "Enter") {
    e.preventDefault();
    if (active && !e.isComposing) {
      choose(active);
    }
  } else if (e.key === "Escape") {
    e.preventDefault();
    closePicker(true);
  } else if (e.key === "Tab") {
    closePicker(false);
  }
});

// Esc closes the due date pop-up without saving. Enter submits its form.
document.addEventListener("keydown", (e) => {
  if (e.key === "Escape" && isDuePicker() && picker.contains(e.target)) {
    e.preventDefault();
    closePicker(true);
  }
});

// Focus that leaves the due date pop-up closes it without saving. The
// decision waits one tick, as for an edit row (section 2).
document.addEventListener("focusout", (e) => {
  if (!isDuePicker() || !picker.contains(e.target)) {
    return;
  }
  const box = picker;
  setTimeout(() => {
    if (picker === box && !box.contains(document.activeElement)) {
      closePicker(false);
    }
  }, 0);
});

// The pop-up belongs to the row: a swap of the row closes it. An error
// answer replaces only the pop-up (HX-Retarget): go on in the new one.
document.addEventListener("htmx:afterSwap", () => {
  if (!picker || picker.isConnected) {
    return;
  }
  const fresh = pickerButton.isConnected && pickerButton.parentElement.querySelector(".picker");
  if (fresh) {
    picker = fresh;
    placePicker();
    filterPicker();
    const input = focusPicker();
    if (input && input.matches(".picker-search")) {
      input.setSelectionRange(input.value.length, input.value.length);
    }
    return;
  }
  if (pickerButton.isConnected) {
    pickerButton.setAttribute("aria-expanded", "false");
  }
  picker = null;
  pickerButton = null;
});
