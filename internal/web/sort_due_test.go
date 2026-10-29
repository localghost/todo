package web_test

import (
	"context"
	"net/http"
	"net/url"
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
	assertOrder(t, body, "Pay the rent", "Call the dentist", "Return library books", "Water the plants", "Send the invoice")

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
	assertOrder(t, body, "Pay the rent", "Call the dentist", "Return library books", "Water the plants", "Send the invoice")
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
	for _, text := range []string{"Pay the rent", "Call the dentist", "Return library books", "Water the plants", "Send the invoice"} {
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
