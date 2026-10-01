// Small page behaviors that htmx attributes cannot express.
// Approved in Claude Design, page "8 · Focus and typing".

// 1. Focus after check or delete. A check keeps focus by itself (the new
// checkbox has the same id). If the focused row disappears, focus moves to
// the next row's checkbox, else the previous row's, else the add input.
let focusAfterSwap = null;

document.addEventListener("htmx:beforeSwap", (e) => {
  const row = e.detail.target;
  const active = document.activeElement;
  if (!row.matches("li.item") || !active || !row.contains(active)) {
    return;
  }
  if (active.matches(".postpone-btn")) {
    // The chips disappear after a postpone; keep focus in the same row.
    focusAfterSwap = "check-" + row.id.slice("item-".length);
    return;
  }
  if (!active.matches(".check, .delete")) {
    return;
  }
  const neighbor = row.nextElementSibling || row.previousElementSibling;
  focusAfterSwap = neighbor ? neighbor.querySelector(".check") : document.getElementById("new-item");
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

// A click inside an edit row (a gap, the hint, "Clear") must not take the
// focus out of the row. Inputs and labels still get focus as usual.
document.addEventListener("mousedown", (e) => {
  const row = e.target.closest && e.target.closest("li.editing");
  if (row && !e.target.closest("input, label")) {
    e.preventDefault();
  }
});

// Enter on "Clear" clears the fields; it must not also save the row.
document.addEventListener("keydown", (e) => {
  if (e.key === "Enter" && e.target.matches && e.target.matches(".clear-due")) {
    e.stopPropagation();
  }
}, true);

// "Clear" in an edit row empties both due fields.
document.addEventListener("click", (e) => {
  if (!e.target.matches(".clear-due")) {
    return;
  }
  e.target.closest(".due-line").querySelectorAll("input").forEach((input) => {
    input.value = "";
  });
});

// A half-typed date or time stops the save (hx-validate). Say why.
document.addEventListener("htmx:validation:halted", (e) => {
  const edit = e.detail.elt;
  if (!edit.matches(".edit")) {
    return;
  }
  const hint = edit.querySelector(".hint");
  hint.textContent = "Please choose a valid date and time.";
  hint.classList.add("error");
  edit.querySelectorAll('.due-line input').forEach((input) => {
    input.setAttribute("aria-invalid", input.validity.valid ? "false" : "true");
  });
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
  if (!canNotify || Notification.permission !== "granted") {
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
