package web_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
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
