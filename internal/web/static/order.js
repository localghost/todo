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
// reach the server in the same order as on the page. Each move keeps the row
// order from before it. If a move fails, the server still has that order: the
// page puts it back, and the moves after it are dropped (a new round starts).
let moveQueue = Promise.resolve();
let moveRound = 0;

// A move request that gets no answer in this time fails like a network error,
// so it cannot stop the moves after it.
const MOVE_TIMEOUT_MS = 10000;

// rowIds returns the ids of the rows in list, in page order.
function rowIds(list) {
  return Array.from(list.children, (row) => row.id);
}

// restoreOrder puts the rows of #items back in the order of ids. It finds the
// rows by id, because htmx may have replaced some of them. Rows that are gone
// are skipped; rows that are not in ids (for example a new item) go last.
function restoreOrder(ids) {
  const list = document.getElementById("items");
  if (!list) {
    return;
  }
  const known = new Set(ids);
  const rest = Array.from(list.children).filter((row) => !known.has(row.id));
  for (const id of ids) {
    const row = document.getElementById(id);
    if (row && row.parentElement === list) {
      list.appendChild(row);
    }
  }
  rest.forEach((row) => list.appendChild(row));
}

// sendMove saves the new place of row. before is the row order before the move.
function sendMove(row, before) {
  const id = row.id.slice("item-".length);
  const body = new URLSearchParams({ after: afterFor(row) });
  const round = moveRound;
  moveQueue = moveQueue
    .then(() => (round === moveRound ? postMove(id, body, before) : undefined))
    // An error must not stop the queue: the moves after it must still go out.
    .catch((err) => console.warn("Cannot save the new order", err));
}

async function postMove(id, body, before) {
  const errorLine = document.getElementById("error");
  let res;
  try {
    res = await fetch(`/items/${id}/move`, {
      method: "POST",
      headers: { "HX-Request": "true" },
      body,
      signal: AbortSignal.timeout(MOVE_TIMEOUT_MS),
    });
  } catch (err) {
    moveRound++;
    restoreOrder(before);
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
    moveRound++;
    htmx.ajax("GET", "/", { target: "#list-section", swap: "outerHTML" });
    return;
  }
  moveRound++;
  restoreOrder(before);
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

// The row order when the current drag started. null means that there is no
// drag to save.
let dragBefore = null;

// Start SortableJS on the list. htmx replaces the list after some answers,
// so this runs again for each new list.
function startSortable(list) {
  if (!list || typeof Sortable === "undefined" || Sortable.get(list)) {
    return;
  }
  Sortable.create(list, {
    handle: ".grip",
    // The edit row has no grip, so the handle option already stops a drag of
    // it. Do not add :not(.editing) here: SortableJS counts oldIndex and
    // newIndex only among draggable rows, so a move past the edit row would
    // keep the same index, and onEnd would not send it.
    draggable: "li.item:not(.overdue-row)",
    ghostClass: "drag-ghost",
    chosenClass: "drag-chosen",
    animation: 150,
    onMove: (e) => !e.related.classList.contains("overdue-row"),
    onStart: () => {
      document.body.classList.add("dragging");
      dragBefore = rowIds(list);
    },
    onEnd: (e) => {
      document.body.classList.remove("dragging");
      const before = dragBefore;
      dragBefore = null;
      if (before === null || e.oldIndex === e.newIndex) {
        return; // the list is gone, or the row is at the start place
      }
      sendMove(e.item, before);
    },
  });
}

// When htmx removes the list (for example to load it again), stop its
// SortableJS. A drag on it must not stay open: "dragging" would stop the
// overdue refresh in app.js. htmx sends this event before it removes the
// list, and destroy calls onEnd, so dragBefore is cleared first.
document.addEventListener("htmx:beforeCleanupElement", (e) => {
  const elt = e.detail.elt;
  if (elt.id !== "items" || typeof Sortable === "undefined") {
    return;
  }
  dragBefore = null;
  const sortable = Sortable.get(elt);
  if (sortable) {
    sortable.destroy();
  }
  document.body.classList.remove("dragging");
});

document.addEventListener("htmx:load", (e) => {
  const elt = e.detail.elt;
  startSortable(elt.id === "items" ? elt : elt.querySelector && elt.querySelector("#items"));
});
document.addEventListener("DOMContentLoaded", () => startSortable(document.getElementById("items")));

// moveByKey moves row one place up (-1) or down (+1), if it can move.
function moveByKey(row, dir) {
  const ref = stepTarget(row, dir);
  if (ref === undefined) {
    return;
  }
  const list = row.parentElement;
  const before = rowIds(list);
  list.insertBefore(row, ref);
  row.querySelector(".grip").focus(); // a moved node can lose the focus
  announce(row);
  sendMove(row, before);
}

// Arrow Up and Arrow Down on a focused grip move the row one place.
document.addEventListener("keydown", (e) => {
  if (!e.target.matches || !e.target.matches(".grip") || (e.key !== "ArrowUp" && e.key !== "ArrowDown")) {
    return;
  }
  e.preventDefault();
  moveByKey(e.target.closest("li.item"), e.key === "ArrowUp" ? -1 : 1);
});
