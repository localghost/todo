package web_test

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPageLoadsOrderScripts(t *testing.T) {
	env := newTestEnv(t)
	body := do(t, env.H, "GET", "/", nil, nil).Body.String()
	tags := []string{
		`<script src="/static/Sortable.min.js" defer></script>`,
		`<script src="/static/app.js" defer></script>`,
		`<script src="/static/order.js" defer></script>`,
	}
	last := -1
	for _, tag := range tags {
		i := strings.Index(body, tag)
		if i < 0 || i < last {
			t.Fatalf("want the script tags in this order: %v\nbody:\n%s", tags, body)
		}
		last = i
	}
	for _, path := range []string{"/static/Sortable.min.js", "/static/order.js"} {
		if rec := do(t, env.H, "GET", path, nil, nil); rec.Code != http.StatusOK {
			t.Errorf("GET %s status = %d, want 200", path, rec.Code)
		}
	}
	assertContains(t, do(t, env.H, "GET", "/static/app.js", nil, nil).Body.String(),
		`document.body.classList.contains("dragging")`)
}

// orderHarness runs order.js with a fake document and prints one JSON line
// per case. A list is a string like "o1 o2 3 4": "o" marks an overdue row.
const orderHarness = `
const src = require("fs").readFileSync(process.argv[2], "utf8");
const document = { addEventListener() {} };
const fns = new Function("document", src + "\nreturn { afterFor, stepTarget };")(document);
function rows(spec) {
  const out = spec.split(" ").map((s) => ({
    id: "item-" + s.replace("o", ""),
    classList: { contains: (c) => c === "item" || (c === "overdue-row" && s.startsWith("o")) },
  }));
  out.forEach((r, i) => {
    r.previousElementSibling = out[i - 1] || null;
    r.nextElementSibling = out[i + 1] || null;
  });
  return out;
}
function show(node) {
  if (node === undefined) return "none";
  if (node === null) return "end";
  return node.id;
}
const r = rows("o1 o2 3 4 5");
console.log(JSON.stringify({ name: "after: first normal row", got: fns.afterFor(r[2]) }));
console.log(JSON.stringify({ name: "after: second normal row", got: fns.afterFor(r[3]) }));
console.log(JSON.stringify({ name: "after: first row, no overdue", got: fns.afterFor(rows("7 8")[0]) }));
console.log(JSON.stringify({ name: "up: below overdue", got: show(fns.stepTarget(r[2], -1)) }));
console.log(JSON.stringify({ name: "up: middle", got: show(fns.stepTarget(r[3], -1)) }));
console.log(JSON.stringify({ name: "down: middle", got: show(fns.stepTarget(r[2], 1)) }));
console.log(JSON.stringify({ name: "down: second last", got: show(fns.stepTarget(r[3], 1)) }));
console.log(JSON.stringify({ name: "down: last", got: show(fns.stepTarget(r[4], 1)) }));
`

func TestOrderScriptHelpers(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	harness := filepath.Join(t.TempDir(), "harness.js")
	if err := os.WriteFile(harness, []byte(orderHarness), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(node, harness, "static/order.js").CombinedOutput()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, out)
	}
	want := map[string]string{
		"after: first normal row":      "",
		"after: second normal row":     "3",
		"after: first row, no overdue": "",
		"up: below overdue":            "none",
		"up: middle":                   "item-3",
		"down: middle":                 "item-5",
		"down: second last":            "end",
		"down: last":                   "none",
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != len(want) {
		t.Fatalf("got %d results, want %d:\n%s", len(lines), len(want), out)
	}
	for _, line := range lines {
		var got struct{ Name, Got string }
		if err := json.Unmarshal([]byte(line), &got); err != nil {
			t.Fatalf("%q: %v", line, err)
		}
		if w, ok := want[got.Name]; !ok || got.Got != w {
			t.Errorf("%s: got %q, want %q", got.Name, got.Got, w)
		}
	}
}

// After a drag, SortableJS leaves a style attribute on the rows. htmx must not
// copy it to the new rows during a swap: the CSP blocks inline styles, and the
// browser would log a CSP error.
func TestHTMXDoesNotSettleStyle(t *testing.T) {
	env := newTestEnv(t)
	body := do(t, env.H, "GET", "/", nil, nil).Body.String()
	assertContains(t, body, `"attributesToSettle":["class","width","height"]`)
}

// queueHarness runs order.js with a small fake DOM (rows A B C D) and a
// fake fetch that answers from a script. It prints one JSON line per case.
const queueHarness = `
const src = require("fs").readFileSync(process.argv[2], "utf8");
class El {
  constructor(id) {
    this.id = id; this.kids = []; this.parentElement = null; this.textContent = "";
    const cls = new Set(id.startsWith("item-") ? ["item"] : []);
    this.classList = { contains: (c) => cls.has(c), add: (c) => cls.add(c), remove: (c) => cls.delete(c) };
  }
  get children() { return this.kids; }
  get previousElementSibling() { const k = this.parentElement ? this.parentElement.kids : []; const i = k.indexOf(this); return i > 0 ? k[i - 1] : null; }
  get nextElementSibling() { const k = this.parentElement ? this.parentElement.kids : []; const i = k.indexOf(this); return i >= 0 && i < k.length - 1 ? k[i + 1] : null; }
  remove() { if (this.parentElement) { const k = this.parentElement.kids; k.splice(k.indexOf(this), 1); this.parentElement = null; } }
  insertBefore(n, ref) {
    if (ref && ref.parentElement !== this) throw new Error("NotFoundError");
    n.remove();
    this.kids.splice(ref ? this.kids.indexOf(ref) : this.kids.length, 0, n);
    n.parentElement = this;
    return n;
  }
  appendChild(n) { return this.insertBefore(n, null); }
  replaceWith(n) { const p = this.parentElement; p.kids[p.kids.indexOf(this)] = n; n.parentElement = p; this.parentElement = null; }
  querySelector(sel) { return sel === ".grip" ? { focus() {} } : null; }
}
async function run(name, answers, steps) {
  const list = new El("items"), error = new El("error"), status = new El("move-status");
  for (const t of "ABCD") list.appendChild(new El("item-" + t));
  const document = {
    addEventListener() {}, body: new El("body"),
    getElementById: (id) => [list, error, status, ...list.kids].find((e) => e.id === id) || null,
  };
  let sent = 0;
  const fetch = async () => {
    const a = answers[sent++];
    if (a === "offline") throw new TypeError("Failed to fetch");
    return { status: a };
  };
  const htmx = { ajax() {} }, location = { reload() {} };
  const fns = new Function("document", "fetch", "htmx", "location", "console",
    'const NETWORK_ERROR = "Cannot reach the server. Please try again.";\n' + src +
    "\nreturn { moveByKey, idle: () => moveQueue };")(document, fetch, htmx, location, { warn() {} });
  const row = (t) => document.getElementById("item-" + t);
  for (const step of steps) {
    if (step.replace) row(step.replace).replaceWith(new El("item-" + step.replace));
    else fns.moveByKey(row(step.move), step.dir);
    if (step.wait) await fns.idle();
  }
  await fns.idle();
  console.log(JSON.stringify({ name, order: list.kids.map((r) => r.id.slice(5)).join(""), sent, error: error.textContent }));
}
(async () => {
  await run("two moves, both fail", ["offline", "offline"], [{ move: "A", dir: 1 }, { move: "A", dir: 1 }]);
  await run("first works, second fails", [204, 500], [{ move: "A", dir: 1 }, { move: "A", dir: 1 }]);
  await run("next move after a failure", [500, 204], [{ move: "A", dir: 1, wait: true }, { move: "D", dir: -1 }]);
  await run("failure after a row was replaced", [500, 204], [{ move: "A", dir: 1 }, { replace: "C", wait: true }, { move: "D", dir: -1 }]);
})();
`

func TestOrderScriptQueue(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	harness := filepath.Join(t.TempDir(), "harness.js")
	if err := os.WriteFile(harness, []byte(queueHarness), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(node, harness, "static/order.js")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("node: %v\n%s%s", err, out, stderr.String())
	}
	type result struct {
		Order string
		Sent  int
		Error string
	}
	const network, server = "Cannot reach the server. Please try again.", "Something went wrong. Please try again."
	want := map[string]result{
		// Both moves fail: the page is back at the order the server has.
		"two moves, both fail": {"ABCD", 1, network},
		// The server has the first move only, so the page shows that order.
		"first works, second fails": {"BACD", 2, server},
		// A failure must not stop the moves after it.
		"next move after a failure": {"ABDC", 2, ""},
		// A put-back must not break when htmx replaced a row in the meantime.
		"failure after a row was replaced": {"ABDC", 2, ""},
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != len(want) {
		t.Fatalf("got %d results, want %d:\n%s%s", len(lines), len(want), out, stderr.String())
	}
	for _, line := range lines {
		var got struct {
			Name string
			result
		}
		if err := json.Unmarshal([]byte(line), &got); err != nil {
			t.Fatalf("%q: %v", line, err)
		}
		if w, ok := want[got.Name]; !ok || got.result != w {
			t.Errorf("%s: got %+v, want %+v", got.Name, got.result, w)
		}
	}
}
