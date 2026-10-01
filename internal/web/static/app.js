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

// 2. A click outside an edit field with empty text cancels the edit.
// (The field's own blur trigger only saves non-empty text.)
document.addEventListener("focusout", (e) => {
  const input = e.target;
  if (input.matches(".edit input") && input.value.trim() === "") {
    htmx.trigger(input.closest("li"), "cancel-edit");
  }
});

// 3. After an item is added, clear the add input only if it still holds the
// text that was sent. Text typed while the request ran stays.
document.addEventListener("htmx:beforeRequest", (e) => {
  const form = e.detail.elt;
  if (form.id === "add-form") {
    form.dataset.sent = form.elements.text.value;
  }
});

document.addEventListener("htmx:afterRequest", (e) => {
  const form = e.detail.elt;
  if (form.id !== "add-form" || !e.detail.successful) {
    return;
  }
  const input = form.elements.text;
  if (input.value === form.dataset.sent) {
    input.value = "";
  }
  input.removeAttribute("aria-invalid");
  input.removeAttribute("aria-describedby");
  form.querySelector("#add-error")?.remove();
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
