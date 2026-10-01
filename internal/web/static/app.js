// Small page behaviors that htmx attributes cannot express.
// Approved in Claude Design, page "8 · Focus and typing".

// 1. Focus after check or delete. A check keeps focus by itself (the new
// checkbox has the same id). If the focused row disappears, focus moves to
// the next row's checkbox, else the previous row's, else the add input.
let focusAfterSwap = null;

document.addEventListener("htmx:beforeSwap", (e) => {
  const row = e.detail.target;
  const active = document.activeElement;
  if (!row.matches("li.item") || !active || !active.matches(".check, .delete") || !row.contains(active)) {
    return;
  }
  const neighbor = row.nextElementSibling || row.previousElementSibling;
  focusAfterSwap = neighbor ? neighbor.querySelector(".check") : document.getElementById("new-item");
});

document.addEventListener("htmx:afterSettle", () => {
  const lost = !document.activeElement || document.activeElement === document.body;
  if (focusAfterSwap && lost && document.body.contains(focusAfterSwap)) {
    focusAfterSwap.focus();
  }
  focusAfterSwap = null;
});

// 2. When focus leaves an edit row, save it. Moving between the fields of
// the row does not save. Empty text cancels the edit instead.
document.addEventListener("focusout", (e) => {
  const row = e.target.closest && e.target.closest("li.editing");
  if (!row || (e.relatedTarget && row.contains(e.relatedTarget))) {
    return;
  }
  const text = row.querySelector('input[name="text"]');
  if (text.value.trim() === "") {
    htmx.trigger(row, "cancel-edit");
  } else {
    htmx.trigger(row.querySelector(".edit"), "save-edit");
  }
});

// "Clear" in an edit row empties both due fields.
document.addEventListener("click", (e) => {
  if (!e.target.matches(".clear-due")) {
    return;
  }
  e.target.closest(".due-line").querySelectorAll("input").forEach((input) => {
    input.value = "";
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
  try {
    const res = await fetch("/notifications/claim", { method: "POST", headers: { "HX-Request": "true" } });
    if (!res.ok) {
      return;
    }
    if (errorLine && errorLine.textContent === NETWORK_ERROR) {
      errorLine.textContent = "";
    }
    for (const item of await res.json()) {
      const note = new Notification("Todo: due now", { body: `${item.text} — ${item.due}`, tag: `todo-${item.id}` });
      note.onclick = () => {
        window.focus();
        note.close();
      };
    }
  } catch (err) {
    if (errorLine) {
      errorLine.textContent = NETWORK_ERROR;
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
renderNotifyBar();
claimDue();
setInterval(claimDue, 30000);
