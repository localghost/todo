package web_test

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// sortDueEnv is overdueEnv plus "Send the invoice", due today at 18:00.
// The hand-made order: library, dentist, rent, plants, invoice.
func sortDueEnv(t *testing.T) (*testEnv, map[string]int64) {
	t.Helper()
	env, ids := overdueEnv(t)
	item, err := env.Svc.AddIn(context.Background(), env.User.ID, "Send the invoice", "2026-10-02", "18:00", mustZone(t, "Europe/Warsaw"))
	if err != nil {
		t.Fatal(err)
	}
	ids["Send the invoice"] = item.ID
	return env, ids
}

const sortHint = "Sorted by due date. To drag items, click “Sort by hand”."

// The switch "Sort by due date" / "Sort by hand" (canvas page "14 · Overdue
// at the top", board "sort the list by due date", option A). The server
// keeps the choice, and the hand-made order comes back.
func TestSortByDueSwitch(t *testing.T) {
	env, _ := sortDueEnv(t)
	body := do(t, env.H, "GET", "/", nil, warsawHTMX()).Body.String()
	assertContains(t, body, `data-sort-due="false"`, ">Sort by due date</button>", `hx-vals='{"sort_due":"1"}'`)
	assertNotContains(t, body, sortHint)
	assertOrder(t, body, "Return library books", "Call the dentist", "Pay the rent", "Water the plants", "Send the invoice")

	rec := do(t, env.H, "POST", "/settings/sort-due", url.Values{"sort_due": {"1"}}, warsawHTMX())
	if rec.Code != http.StatusOK {
		t.Fatalf("sort by due date: %d", rec.Code)
	}
	body = rec.Body.String()
	assertContains(t, body, `id="list-section"`, `data-sort-due="true"`, ">Sort by hand</button>", `hx-vals='{"sort_due":"0"}'`, sortHint)
	assertNotContains(t, body, `aria-pressed="true"`)
	assertOrder(t, body, "Pay the rent", "Call the dentist", "Send the invoice", "Water the plants", "Return library books")

	body = do(t, env.H, "GET", "/", nil, map[string]string{"Cookie": "todo_tz=Europe/Warsaw"}).Body.String() // a new page load
	assertContains(t, body, `data-sort-due="true"`, ">Sort by hand</button>", sortHint)
	assertOrder(t, body, "Pay the rent", "Call the dentist", "Send the invoice", "Water the plants", "Return library books")

	body = do(t, env.H, "POST", "/settings/sort-due", url.Values{"sort_due": {"0"}}, warsawHTMX()).Body.String()
	assertContains(t, body, `data-sort-due="false"`, ">Sort by due date</button>")
	assertNotContains(t, body, sortHint)
	assertOrder(t, body, "Return library books", "Call the dentist", "Pay the rent", "Water the plants", "Send the invoice")
}

func TestSortByDueWithoutHTMXRedirects(t *testing.T) {
	env := newTestEnv(t)
	rec := do(t, env.H, "POST", "/settings/sort-due", url.Values{"sort_due": {"1"}}, nil)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("got %d %q, want 303 /", rec.Code, rec.Header().Get("Location"))
	}
	assertContains(t, do(t, env.H, "GET", "/", nil, nil).Body.String(), `data-sort-due="true"`)
}

// In due date order a move is refused: the page gets the true list, and the
// hand-made order stays.
func TestMoveIsRefusedWhenSortedByDue(t *testing.T) {
	env, ids := sortDueEnv(t)
	env.Auth.SetSortDue(context.Background(), env.User.ID, true)
	rec := do(t, env.H, "POST", moveURL(ids["Send the invoice"]), url.Values{"after": {""}}, warsawHTMX())
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("move: %d, want 422", rec.Code)
	}
	assertContains(t, rec.Body.String(), `id="list-section"`, `data-sort-due="true"`)
	env.Auth.SetSortDue(context.Background(), env.User.ID, false)
	body := do(t, env.H, "GET", "/", nil, warsawHTMX()).Body.String()
	assertOrder(t, body, "Return library books", "Water the plants", "Send the invoice")
}

// In due date order a new due date moves the row, so the answer is the
// whole list. In the hand-made order it is only the row, as before.
func TestDueChangeGivesWholeListWhenSortedByDue(t *testing.T) {
	env, ids := sortDueEnv(t)
	set := func(text, date, clock string) *http.Response {
		rec := do(t, env.H, "PUT", "/items/"+strconv.FormatInt(ids[text], 10)+"/due",
			url.Values{"due_date": {date}, "due_time": {clock}}, warsawHTMX())
		if rec.Code != http.StatusOK {
			t.Fatalf("set due of %q: %d", text, rec.Code)
		}
		return rec.Result()
	}
	if res := set("Return library books", "2026-10-05", "10:00"); res.Header.Get("HX-Retarget") != "" {
		t.Fatalf("hand-made order: HX-Retarget %q, want only the row", res.Header.Get("HX-Retarget"))
	}
	env.Auth.SetSortDue(context.Background(), env.User.ID, true)
	res := set("Return library books", "2026-10-02", "17:00")
	if res.Header.Get("HX-Retarget") != "#list-section" {
		t.Fatalf("due date order: HX-Retarget %q, want #list-section", res.Header.Get("HX-Retarget"))
	}
	body := do(t, env.H, "GET", "/", nil, warsawHTMX()).Body.String()
	assertOrder(t, body, "Call the dentist", "Return library books", "Send the invoice", "Water the plants")
}

// In due date order a new item with a due date goes to its place: the
// answer is the whole list. A new item without a due date goes at the end,
// as in the hand-made order.
func TestAddInSortDueOrder(t *testing.T) {
	env, _ := sortDueEnv(t)
	env.Auth.SetSortDue(context.Background(), env.User.ID, true)
	rec := do(t, env.H, "POST", "/items", url.Values{"text": {"Book the flight"}, "due_date": {"2026-10-02"}, "due_time": {"17:30"}}, warsawHTMX())
	if rec.Code != http.StatusOK || rec.Header().Get("HX-Retarget") != "#list-section" {
		t.Fatalf("add with a due date: %d, HX-Retarget %q", rec.Code, rec.Header().Get("HX-Retarget"))
	}
	assertOrder(t, rec.Body.String(), "Call the dentist", "Book the flight", "Send the invoice", "Water the plants")

	rec = do(t, env.H, "POST", "/items", url.Values{"text": {"Fix the bike"}}, warsawHTMX())
	if rec.Code != http.StatusOK || rec.Header().Get("HX-Retarget") != "" {
		t.Fatalf("add without a due date: %d, HX-Retarget %q", rec.Code, rec.Header().Get("HX-Retarget"))
	}
	body := do(t, env.H, "GET", "/", nil, warsawHTMX()).Body.String()
	assertOrder(t, body, "Water the plants", "Return library books", "Fix the bike")
}

// The API keeps the hand-made order, because the Android app drags items.
func TestAPIItemsKeepHandOrder(t *testing.T) {
	env, _ := sortDueEnv(t)
	env.Auth.SetSortDue(context.Background(), env.User.ID, true)
	rec := apiDo(t, env, "GET", "/api/v1/items", env.Token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("api items: %d", rec.Code)
	}
	body, last := rec.Body.String(), -1
	for _, text := range []string{"Return library books", "Call the dentist", "Pay the rent", "Water the plants", "Send the invoice"} {
		i := strings.Index(body, `"text":"`+text+`"`)
		if i < last {
			t.Fatalf("%q is out of the hand-made order: %s", text, body)
		}
		last = i
	}
}

// In due date order the grips and their column go away: no drag, no arrow keys, and the
// checkbox moves to the left. The toolbar buttons wrap on a narrow screen.
func TestSortDueCSS(t *testing.T) {
	env := newTestEnv(t)
	css := do(t, env.H, "GET", "/static/app.css", nil, nil).Body.String()
	assertContains(t, css, `#list-section[data-sort-due="true"] :is(.grip, .grip-space) { display: none; }`,
		".sort-hint { margin: 2px 0 8px; font-size: 13px; color: var(--muted); }",
		".toolbar-right { display: flex; flex-wrap: wrap; gap: 8px; }")
}

// sortSlideHarness runs app.js with fake rows. A "Sort by due date" answer
// moves row 2 above row 1 and the checkboxes 28 px to the left; the rows
// slide from their old places in 0.7 s (canvas page "14 · Overdue at the
// top", "transition between the two orders", option A).
const sortSlideHarness = `
const src = require("fs").readFileSync(process.argv[2], "utf8");
const handlers = {};
let reduce = false;
const anims = [];
const rect = (o) => ({ getBoundingClientRect: () => ({ top: o.top, left: o.left }) });
function makeRow(id, top, left) {
  const check = Object.assign(rect({ top, left }), { animate: (k) => { anims.push(id + " check " + JSON.stringify(k[0])); return { finished: Promise.resolve() }; } });
  const col = { animate: (k) => { anims.push(id + " text " + JSON.stringify(k[0])); return { finished: Promise.resolve() }; } };
  const row = { id, top, check, col,
    getBoundingClientRect: () => ({ top: row.top, left: 0 }),
    querySelector: (s) => s === ".check" ? check : null,
    querySelectorAll: (s) => s === ".check, .text-col" ? [check, col] : [],
    animate: (k, o) => { anims.push(id + " row " + JSON.stringify(k[0]) + " " + o.duration + "ms"); return { finished: Promise.resolve() }; } };
  return row;
}
let rows = [makeRow("item-1", 100, 40), makeRow("item-2", 160, 40)];
const items = {};
const button = { classList: { contains: (c) => c === "sort-due" }, focus: () => {} };
const document = {
  addEventListener: (name, fn) => (handlers[name] = handlers[name] || []).push(fn),
  getElementById: (id) => id === "items" ? items : null,
  querySelector: (s) => s === ".sort-due" ? button : null,
  querySelectorAll: (s) => s === "#items > li.item" ? rows : [],
  activeElement: null, body: {},
};
const window = { matchMedia: () => ({ matches: reduce }), getComputedStyle: (el) => ({ backgroundColor: el === items ? "rgb(255, 255, 255)" : "" }) };
const fire = (name, elt) => (handlers[name] || []).forEach((fn) => fn({ detail: { requestConfig: { elt }, target: { id: "list-section" }, elt } }));
new Function("document", "window", "navigator", "fetch", "setInterval", "setTimeout", "clearTimeout", "htmx",
  src)(document, window, {}, () => new Promise(() => {}), () => 0, () => 0, () => {}, { ajax: () => {} });
function sortClick(elt) {
  anims.length = 0;
  fire("htmx:beforeSwap", elt);
  rows = [makeRow("item-2", 100, 12), makeRow("item-1", 160, 12)];
  fire("htmx:afterSwap", elt);
}
(async () => {
  sortClick(button);
  console.log(JSON.stringify({ name: "sort", anims: anims.slice() }));
  sortClick({ classList: { contains: () => false } });
  console.log(JSON.stringify({ name: "other request", anims: anims.length }));
  reduce = true;
  sortClick(button);
  console.log(JSON.stringify({ name: "reduced motion", anims: anims.length }));
})();
`

func TestAppJSSlidesRowsAfterSort(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	harness := filepath.Join(t.TempDir(), "harness.js")
	if err := os.WriteFile(harness, []byte(sortSlideHarness), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(node, harness, "static/app.js").CombinedOutput()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, out)
	}
	want := `{"name":"sort","anims":[` +
		`"item-2 row {\"transform\":\"translateY(60px)\",\"backgroundColor\":\"rgb(255, 255, 255)\"} 700ms",` +
		`"item-2 check {\"transform\":\"translateX(28px)\"}","item-2 text {\"transform\":\"translateX(28px)\"}",` +
		`"item-1 row {\"transform\":\"translateY(-60px)\",\"backgroundColor\":\"rgb(255, 255, 255)\"} 700ms",` +
		`"item-1 check {\"transform\":\"translateX(28px)\"}","item-1 text {\"transform\":\"translateX(28px)\"}"]}` + "\n" +
		`{"name":"other request","anims":0}` + "\n" +
		`{"name":"reduced motion","anims":0}`
	if got := strings.TrimSpace(string(out)); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}
