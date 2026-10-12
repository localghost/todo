// Drag-and-drop and keyboard order (spec 2026-10-12-drag-order-design.md).
// Only the grip starts a drag. Overdue rows (.overdue-row) stay at the top:
// they cannot move, and nothing can be dropped between them. This file loads
// after app.js and uses its NETWORK_ERROR.

// isNormal reports whether row is a list row that is not overdue.
function isNormal(row) {
  return !!row && row.classList.contains("item") && !row.classList.contains("overdue-row");
}

// afterFor returns the "after" value for row at its current place: the id of
// the nearest normal row above it, or "" for the first place.
function afterFor(row) {
  for (let prev = row.previousElementSibling; prev; prev = prev.previousElementSibling) {
    if (isNormal(prev)) {
      return prev.id.slice("item-".length);
    }
  }
  return "";
}

// stepTarget returns the node to insert row before, for one step up (-1) or
// down (+1). null means the end of the list; undefined means no move.
function stepTarget(row, dir) {
  if (dir < 0) {
    const prev = row.previousElementSibling;
    return isNormal(prev) ? prev : undefined;
  }
  const next = row.nextElementSibling;
  return next ? next.nextElementSibling : undefined;
}

// Move requests go out one at a time and in order, so fast key presses
// reach the server in the same order as on the page.
let moveQueue = Promise.resolve();

function sendMove(row, putBack) {
  const id = row.id.slice("item-".length);
  const body = new URLSearchParams({ after: afterFor(row) });
  moveQueue = moveQueue.then(() => postMove(id, body, putBack));
}

async function postMove(id, body, putBack) {
  const errorLine = document.getElementById("error");
  let res;
  try {
    res = await fetch(`/items/${id}/move`, { method: "POST", headers: { "HX-Request": "true" }, body });
  } catch (err) {
    putBack();
    if (errorLine) {
      errorLine.textContent = NETWORK_ERROR;
    }
    return;
  }
  if (res.status === 204) {
    if (errorLine) {
      errorLine.textContent = "";
    }
    return;
  }
  if (res.status === 401) {
    // Not logged in any more: the reload shows the log-in page.
    location.reload();
    return;
  }
  if (res.status === 404 || res.status === 422) {
    // The page shows an old state (another tab, or the item became overdue).
    // Load the true order. app.js section 11 keeps an open edit row.
    htmx.ajax("GET", "/", { target: "#list-section", swap: "outerHTML" });
    return;
  }
  putBack();
  if (errorLine) {
    errorLine.textContent = "Something went wrong. Please try again.";
  }
}

// Say the new place of row to a screen reader, for example "Moved to 3 of 7".
function announce(row) {
  const status = document.getElementById("move-status");
  const rows = Array.from(row.parentElement.children).filter(isNormal);
  if (status) {
    status.textContent = `Moved to ${rows.indexOf(row) + 1} of ${rows.length}`;
  }
}

// Start SortableJS on the list. htmx replaces the list after some answers,
// so this runs again for each new list.
function startSortable(list) {
  if (!list || typeof Sortable === "undefined" || Sortable.get(list)) {
    return;
  }
  Sortable.create(list, {
    handle: ".grip",
    draggable: "li.item:not(.overdue-row)",
    ghostClass: "drag-ghost",
    chosenClass: "drag-chosen",
    animation: 150,
    onMove: (e) => !e.related.classList.contains("overdue-row"),
    onStart: () => document.body.classList.add("dragging"),
    onEnd: (e) => {
      document.body.classList.remove("dragging");
      if (e.oldIndex === e.newIndex) {
        return; // dropped at the start place: nothing changed
      }
      const row = e.item;
      const oldIndex = e.oldIndex;
      sendMove(row, () => {
        row.remove();
        list.insertBefore(row, list.children[oldIndex] || null);
      });
    },
  });
}

document.addEventListener("htmx:load", (e) => {
  const elt = e.detail.elt;
  startSortable(elt.id === "items" ? elt : elt.querySelector && elt.querySelector("#items"));
});
document.addEventListener("DOMContentLoaded", () => startSortable(document.getElementById("items")));

// Arrow Up and Arrow Down on a focused grip move the row one place.
document.addEventListener("keydown", (e) => {
  if (!e.target.matches || !e.target.matches(".grip") || (e.key !== "ArrowUp" && e.key !== "ArrowDown")) {
    return;
  }
  e.preventDefault();
  const row = e.target.closest("li.item");
  const before = stepTarget(row, e.key === "ArrowUp" ? -1 : 1);
  if (before === undefined) {
    return;
  }
  const list = row.parentElement;
  const oldNext = row.nextElementSibling;
  list.insertBefore(row, before);
  row.querySelector(".grip").focus(); // a moved node can lose the focus
  announce(row);
  sendMove(row, () => list.insertBefore(row, oldNext));
});
