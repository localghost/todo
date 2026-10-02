package web_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// overdueEnv: it is 2 Oct 16:00 in Warsaw. Creation order: library, dentist
// (due today 14:00, overdue), rent (all-day 1 Oct, overdue), plants (tomorrow 14:00).
func overdueEnv(t *testing.T) (*testEnv, map[string]int64) {
	t.Helper()
	env := newTestEnv(t)
	warsaw := mustZone(t, "Europe/Warsaw")
	env.Clock.t = time.Date(2026, 10, 2, 16, 0, 0, 0, warsaw)
	ids := map[string]int64{}
	for _, it := range []struct{ text, date, clock string }{
		{"Return library books", "", ""},
		{"Call the dentist", "2026-10-02", "14:00"},
		{"Pay the rent", "2026-10-01", ""},
		{"Water the plants", "2026-10-03", "14:00"},
	} {
		item, err := env.Svc.AddIn(context.Background(), env.User.ID, it.text, it.date, it.clock, warsaw)
		if err != nil {
			t.Fatal(err)
		}
		ids[it.text] = item.ID
	}
	return env, ids
}

func warsawHTMX() map[string]string {
	return map[string]string{"HX-Request": "true", "Cookie": "todo_tz=Europe/Warsaw"}
}

// assertOrder checks that the texts appear in this order in the body.
func assertOrder(t *testing.T, body string, texts ...string) {
	t.Helper()
	last := -1
	for _, text := range texts {
		i := strings.Index(body, ">"+text+"<")
		if i < 0 {
			t.Fatalf("%q not in the list", text)
		}
		if i < last {
			t.Fatalf("%q is out of order; want %v", text, texts)
		}
		last = i
	}
}

func assertRetarget(t *testing.T, rec *httptest.ResponseRecorder, want bool) {
	t.Helper()
	got := rec.Header().Get("HX-Retarget") == "#list-section" && rec.Header().Get("HX-Reswap") == "outerHTML"
	if got != want {
		t.Fatalf("whole-list response = %v, want %v (headers %v)", got, want, rec.Header())
	}
}

func TestOverdueItemsComeFirst(t *testing.T) {
	env, _ := overdueEnv(t)
	body := do(t, env.H, "GET", "/", nil, tz("Europe/Warsaw")).Body.String()
	assertOrder(t, body, "Pay the rent", "Call the dentist", "Return library books", "Water the plants")
	next := time.Date(2026, 10, 3, 14, 0, 0, 0, mustZone(t, "Europe/Warsaw")).UnixMilli()
	assertContains(t, body, `data-next-overdue="`+strconv.FormatInt(next, 10)+`"`)
}

func TestNoNextOverdueWithoutDueItems(t *testing.T) {
	env := newTestEnv(t)
	mustAdd(t, env.Svc, "Buy milk")
	assertNotContains(t, do(t, env.H, "GET", "/", nil, nil).Body.String(), "data-next-overdue")
}

func TestDoneOverdueItemMovesBack(t *testing.T) {
	env, ids := overdueEnv(t)
	rec := do(t, env.H, "POST", "/items/"+strconv.FormatInt(ids["Call the dentist"], 10)+"/toggle", url.Values{}, warsawHTMX())
	assertRetarget(t, rec, true)
	assertOrder(t, rec.Body.String(), "Pay the rent", "Return library books", "Call the dentist", "Water the plants")

	rec = do(t, env.H, "POST", "/items/"+strconv.FormatInt(ids["Return library books"], 10)+"/toggle", url.Values{}, warsawHTMX())
	assertRetarget(t, rec, false) // not overdue before or after: only the row
}

func TestPostponedItemMovesBack(t *testing.T) {
	env, ids := overdueEnv(t)
	rec := do(t, env.H, "POST", "/items/"+strconv.FormatInt(ids["Call the dentist"], 10)+"/postpone?minutes=10", nil, warsawHTMX())
	if rec.Code != http.StatusOK {
		t.Fatalf("postpone: %d", rec.Code)
	}
	assertRetarget(t, rec, true)
	assertOrder(t, rec.Body.String(), "Pay the rent", "Return library books", "Call the dentist", "Water the plants")
}

func TestEditedItemMoves(t *testing.T) {
	env, ids := overdueEnv(t)
	form := url.Values{"text": {"Call the dentist"}, "due_date": {"2026-10-04"}, "due_time": {"10:00"}, "due_tz": {"Europe/Warsaw"}}
	rec := do(t, env.H, "PUT", "/items/"+strconv.FormatInt(ids["Call the dentist"], 10), form, warsawHTMX())
	assertRetarget(t, rec, true)
	assertOrder(t, rec.Body.String(), "Pay the rent", "Return library books", "Call the dentist", "Water the plants")
}

func TestAddedOverdueItemGoesToTop(t *testing.T) {
	env, _ := overdueEnv(t)
	form := url.Values{"text": {"Old task"}, "due_date": {"2026-10-02"}, "due_time": {"10:00"}}
	rec := do(t, env.H, "POST", "/items", form, warsawHTMX())
	assertRetarget(t, rec, true)
	assertOrder(t, rec.Body.String(), "Pay the rent", "Old task", "Call the dentist", "Return library books")

	rec = do(t, env.H, "POST", "/items", url.Values{"text": {"New task"}}, warsawHTMX())
	assertRetarget(t, rec, false)
}

func TestOverdueOrderWithDoneHidden(t *testing.T) {
	env, ids := overdueEnv(t)
	env.Auth.SetHideDone(context.Background(), env.User.ID, true)
	assertOrder(t, do(t, env.H, "GET", "/", nil, tz("Europe/Warsaw")).Body.String(),
		"Pay the rent", "Call the dentist", "Return library books", "Water the plants")
	rec := do(t, env.H, "POST", "/items/"+strconv.FormatInt(ids["Call the dentist"], 10)+"/toggle", url.Values{}, warsawHTMX())
	assertRetarget(t, rec, true)
	body := rec.Body.String()
	assertNotContains(t, body, ">Call the dentist<")
	assertOrder(t, body, "Pay the rent", "Return library books", "Water the plants")
}

// app.js refreshes the list when the next item becomes overdue, never while an
// edit row is open, and plans at most 6 hours ahead.
func TestAppJSRefreshesWhenOverdue(t *testing.T) {
	env := newTestEnv(t)
	js := do(t, env.H, "GET", "/static/app.js", nil, anon).Body.String()
	assertContains(t, js, "dataset.nextOverdue", `htmx.ajax("GET", "/", { target: "#list-section", swap: "outerHTML" })`,
		`document.querySelector(".item.editing")`, "6 * 60 * 60 * 1000", "clearTimeout(overdueTimer)")
}

// A row-only answer must still tell app.js about a new due moment: the OOB
// toolbar carries data-next-overdue.
func TestRowAnswerCarriesNextOverdue(t *testing.T) {
	env := newTestEnv(t)
	warsaw := mustZone(t, "Europe/Warsaw")
	env.Clock.t = time.Date(2026, 10, 2, 16, 0, 0, 0, warsaw)
	form := url.Values{"text": {"Call Bob"}, "due_date": {"2026-10-02"}, "due_time": {"16:01"}}
	rec := do(t, env.H, "POST", "/items", form, warsawHTMX())
	assertRetarget(t, rec, false)
	next := time.Date(2026, 10, 2, 16, 1, 0, 0, warsaw).UnixMilli()
	assertContains(t, rec.Body.String(), `id="toolbar" class="toolbar" hx-swap-oob="true" data-next-overdue="`+strconv.FormatInt(next, 10)+`"`)
}

// tzHarness-like run of app.js with stubbed DOM, htmx and timers; it prints one
// JSON line per step: the planned delay and how many refreshes ran.
const overdueHarness = `
const src = require("fs").readFileSync(process.argv[2], "utf8");
let now = 1000000000000, timers = [], ajax = 0, editing = false, next = 0;
const handlers = {};
const document = {
  addEventListener: (name, fn) => (handlers[name] = handlers[name] || []).push(fn),
  getElementById: (id) => (id === "toolbar" && next ? { dataset: { nextOverdue: String(next) } } : null),
  querySelector: (sel) => (sel === ".item.editing" && editing ? {} : null),
  activeElement: null, body: {},
};
const fire = (name, detail) => (handlers[name] || []).forEach((fn) => fn({ detail: detail || {} }));
const setTimeout = (fn, ms) => { timers.push({ fn, ms }); return timers.length; };
const clearTimeout = (id) => { if (id) timers[id - 1] = null; };
const live = () => timers.filter(Boolean);
const htmx = { ajax: () => { ajax++; } };
const DateShim = { now: () => now };
new Function("document", "window", "navigator", "fetch", "setInterval", "setTimeout", "clearTimeout", "htmx", "Date",
  src)(document, {}, {}, () => new Promise(() => {}), () => 0, setTimeout, clearTimeout, htmx, DateShim);
const step = (name) => { const t = live(); console.log(JSON.stringify({ name, wait: t.length ? t[t.length - 1].ms : -1, ajax })); };
const runLast = () => { const t = live(); const last = t[t.length - 1]; timers = []; last.fn(); };

next = now + 60000; fire("DOMContentLoaded"); step("plan in 1 min");
now = next + 1000; runLast(); step("fired");
fire("htmx:afterSettle"); step("same moment again");
editing = true; next = now + 1; timers = []; fire("htmx:afterSettle"); now += 2000; runLast(); step("edit row open");
editing = false; next = now + 10 * 24 * 3600 * 1000; timers = []; fire("htmx:afterSettle"); step("10 days ahead");
next = 0; timers = []; fire("htmx:afterSettle"); step("no moment");
`

func TestAppJSOverdueTimer(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	harness := filepath.Join(t.TempDir(), "harness.js")
	if err := os.WriteFile(harness, []byte(overdueHarness), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(node, harness, "static/app.js").CombinedOutput()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, out)
	}
	want := map[string][2]int{ // wait in ms (-1: no timer), refreshes so far
		"plan in 1 min":     {61000, 0},
		"fired":             {-1, 1},
		"same moment again": {30000, 1}, // no 1-second loop when the server still names a past moment
		"edit row open":     {30000, 1}, // no refresh while editing; try again in 30 s
		"10 days ahead":     {6 * 60 * 60 * 1000, 1},
		"no moment":         {-1, 1},
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != len(want) {
		t.Fatalf("got %d steps, want %d:\n%s", len(lines), len(want), out)
	}
	for _, line := range lines {
		var got struct {
			Name string
			Wait int
			Ajax int
		}
		if err := json.Unmarshal([]byte(line), &got); err != nil {
			t.Fatalf("%q: %v", line, err)
		}
		if w := want[got.Name]; got.Wait != w[0] || got.Ajax != w[1] {
			t.Errorf("%s: wait %d, refreshes %d; want %d, %d", got.Name, got.Wait, got.Ajax, w[0], w[1])
		}
	}
}

// Focus after a whole-list answer, and an open edit row survives one.
func TestAppJSKeepsFocusAndEditRow(t *testing.T) {
	env := newTestEnv(t)
	js := do(t, env.H, "GET", "/static/app.js", nil, anon).Body.String()
	assertContains(t, js, `e.detail.elt`, `.closest("li.item")`, `check ? check.id : "new-item"`,
		"keptEdit", "fresh.replaceWith(keptEdit)", `getElementById("toolbar")`)
}

// The overdue order uses the browser's zone, not the server's: at 2 Oct 22:30
// UTC it is already 3 Oct in Warsaw, so the all-day item of 2 Oct is overdue there.
func TestOverdueOrderUsesBrowserZone(t *testing.T) {
	env := newTestEnv(t)
	warsaw := mustZone(t, "Europe/Warsaw")
	env.Clock.t = time.Date(2026, 10, 2, 22, 30, 0, 0, time.UTC) // server zone: UTC
	env.Svc.AddIn(context.Background(), env.User.ID, "Buy milk", "", "", warsaw)
	env.Svc.AddIn(context.Background(), env.User.ID, "Pay the rent", "2026-10-02", "", warsaw)

	body := do(t, env.H, "GET", "/", nil, tz("Europe/Warsaw")).Body.String()
	assertOrder(t, body, "Pay the rent", "Buy milk")
	assertContains(t, body, `class="due overdue"`)

	body = do(t, env.H, "GET", "/", nil, nil).Body.String() // no cookie: UTC, still 2 Oct
	assertOrder(t, body, "Buy milk", "Pay the rent")
	assertNotContains(t, body, `class="due overdue"`)
}

// An edit that keeps the item overdue but moves its due time re-sorts the group.
func TestEditInsideOverdueGroupResorts(t *testing.T) {
	env := newTestEnv(t)
	warsaw := mustZone(t, "Europe/Warsaw")
	env.Clock.t = time.Date(2026, 10, 2, 16, 0, 0, 0, warsaw)
	env.Svc.AddIn(context.Background(), env.User.ID, "Call Anna", "2026-10-02", "10:00", warsaw)
	b, _ := env.Svc.AddIn(context.Background(), env.User.ID, "Call Bob", "2026-10-02", "14:00", warsaw)
	form := url.Values{"text": {"Call Bob"}, "due_date": {"2026-10-02"}, "due_time": {"08:00"}, "due_tz": {"Europe/Warsaw"}}
	rec := do(t, env.H, "PUT", "/items/"+strconv.FormatInt(b.ID, 10), form, warsawHTMX())
	assertRetarget(t, rec, true)
	assertOrder(t, rec.Body.String(), "Call Bob", "Call Anna")

	form.Set("text", "Call Bob today") // text only: the place stays, so only the row comes back
	assertRetarget(t, do(t, env.H, "PUT", "/items/"+strconv.FormatInt(b.ID, 10), form, warsawHTMX()), false)
}
