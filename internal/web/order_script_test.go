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
