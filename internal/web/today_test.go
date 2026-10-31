package web_test

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"todo/internal/todo"
)

// The Today view (issue #7, canvas page "22 · Today", option V1): a switch
// "All · Today" above the buttons; /?due=today shows the open items that
// are overdue or due today, and the done items due today.

// todayEnv is an app at Sat 1 Nov 2026 10:00 in Warsaw with these items:
// 1 overdue, 2 today 18:00, 3 no date, 4 in two days, 5 today all day,
// 6 done today, 7 done yesterday.
func todayEnv(t *testing.T) *testEnv {
	t.Helper()
	env := newTestEnv(t)
	loc := warsawZone(t)
	env.Clock.t = time.Date(2026, 11, 1, 10, 0, 0, 0, loc)
	ctx := context.Background()
	for _, in := range []todo.Input{
		{Text: "Call the dentist", DueDate: "2026-10-31", DueTime: "09:00"},
		{Text: "Send the invoice", DueDate: "2026-11-01", DueTime: "18:00", Labels: "work"},
		{Text: "Water the plants"},
		{Text: "Plan the team meeting", DueDate: "2026-11-03", Labels: "work"},
		{Text: "Pay the rent", DueDate: "2026-11-01"},
		{Text: "Buy milk", DueDate: "2026-11-01"},
		{Text: "Return the books", DueDate: "2026-10-31"},
	} {
		if _, err := env.Svc.AddItem(ctx, env.User.ID, in, loc); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []int64{6, 7} {
		if _, err := env.Svc.SetDone(ctx, env.User.ID, id, true); err != nil {
			t.Fatal(err)
		}
	}
	return env
}

var warsawPage = map[string]string{"Cookie": "todo_tz=Europe/Warsaw"}

// todayHTMX is an htmx request from the page /?due=today (more: the query).
func todayHTMX(more string) map[string]string {
	h := warsawHTMX()
	h["HX-Current-URL"] = "http://example.com/?due=today" + more
	return h
}

func TestTodayViewList(t *testing.T) {
	env := todayEnv(t)
	body := do(t, env.H, "GET", "/?due=today", nil, warsawPage).Body.String()
	assertContains(t, body, "Call the dentist", "Send the invoice", "Pay the rent", "Buy milk",
		`<span class="counter">3 open · 1 done</span>`,
		`<nav class="view-switch" aria-label="Views"><a href="/">All</a><a href="/?due=today" aria-current="page">Today</a></nav>`)
	assertNotContains(t, body, "Water the plants", "Plan the team meeting", "Return the books")
	// A new item in the view gets today, all day.
	assertContains(t, body, `<input type="hidden" form="add-form" name="due_date" value="2026-11-01">`,
		`<input type="hidden" form="add-form" name="due_time" value="">`, `aria-label="Due date: Today"`)

	body = do(t, env.H, "GET", "/", nil, warsawPage).Body.String()
	assertContains(t, body, "Water the plants", "Return the books", `<span class="counter">5 open · 2 done</span>`,
		`<nav class="view-switch" aria-label="Views"><a href="/" aria-current="page">All</a><a href="/?due=today">Today</a></nav>`,
		`<input type="hidden" form="add-form" name="due_date" value="">`)
}

// The view and a label filter work together; each link keeps the other.
func TestTodayViewWithLabel(t *testing.T) {
	env := todayEnv(t)
	body := do(t, env.H, "GET", "/?label=1&due=today", nil, warsawPage).Body.String()
	assertContains(t, body, "Send the invoice", `<span class="counter">1 open · 0 done</span>`,
		`<a href="/?label=1">All</a><a href="/?label=1&amp;due=today" aria-current="page">Today</a>`,
		`<a class="clear-filter" href="/?due=today"`)
	assertNotContains(t, body, "Plan the team meeting", "Call the dentist")
}

// An empty view keeps the switch and the buttons, and says why it is empty.
// The page loads the list again at midnight, when the view changes.
func TestTodayViewEmpty(t *testing.T) {
	env := newTestEnv(t)
	env.Clock.t = time.Date(2026, 11, 1, 10, 0, 0, 0, warsawZone(t))
	mustAdd(t, env.Svc, "Water the plants")
	body := do(t, env.H, "GET", "/?due=today", nil, warsawPage).Body.String()
	assertContains(t, body, `<p class="empty-title">Nothing due today</p>`,
		`<p class="empty-hint">Items that are overdue or due today show here. Click “All” to see all items.</p>`,
		`data-next-overdue="`+strconv.FormatInt(time.Date(2026, 11, 2, 0, 0, 0, 0, warsawZone(t)).UnixMilli(), 10)+`"`)
	assertNotContains(t, body, `id="toolbar" class="toolbar" hidden`, "Water the plants")
}

// Requests from the page in the view answer for the view.
func TestTodayViewRequests(t *testing.T) {
	env := todayEnv(t)
	// The list again (as after midnight): the view, from HX-Current-URL.
	body := do(t, env.H, "GET", "/", nil, todayHTMX("")).Body.String()
	assertContains(t, body, "Call the dentist", `<span class="counter">3 open · 1 done</span>`)
	assertNotContains(t, body, "Water the plants")

	// A new item due today goes in as a row, and the next draft has today.
	rec := do(t, env.H, "POST", "/items", url.Values{"text": {"Buy flowers"}, "due_date": {"2026-11-01"}}, todayHTMX(""))
	assertContains(t, rec.Body.String(), "Buy flowers", `name="due_date" value="2026-11-01"`)
	if rec.Header().Get("HX-Retarget") != "" {
		t.Fatalf("a new item of the view: HX-Retarget = %q, want none", rec.Header().Get("HX-Retarget"))
	}
	// A new item without a date is not in the view: the whole list comes back.
	rec = do(t, env.H, "POST", "/items", url.Values{"text": {"Read a book"}}, todayHTMX(""))
	if rec.Header().Get("HX-Retarget") != "#list-section" {
		t.Fatalf("a new item outside the view: HX-Retarget = %q, want #list-section", rec.Header().Get("HX-Retarget"))
	}
	assertNotContains(t, rec.Body.String(), "Read a book")

	// An overdue item that is checked off leaves the view.
	rec = do(t, env.H, "POST", "/items/1/toggle", url.Values{"done": {"1"}}, todayHTMX(""))
	if rec.Header().Get("HX-Retarget") != "#list-section" {
		t.Fatalf("check off an overdue item: HX-Retarget = %q, want #list-section", rec.Header().Get("HX-Retarget"))
	}
	assertNotContains(t, rec.Body.String(), "Call the dentist")
	// A date moved to another day takes the item out too.
	rec = do(t, env.H, "PUT", "/items/2/due", url.Values{"due_date": {"2026-11-05"}, "due_time": {""}, "due_tz": {"Europe/Warsaw"}}, todayHTMX(""))
	if rec.Header().Get("HX-Retarget") != "#list-section" || strings.Contains(rec.Body.String(), "Send the invoice") {
		t.Fatalf("move the date out of the view: HX-Retarget = %q", rec.Header().Get("HX-Retarget"))
	}
}

// "Clear done" in the view deletes only the done items of the view.
func TestTodayViewClearDone(t *testing.T) {
	env := todayEnv(t)
	body := do(t, env.H, "GET", "/items/done/clear", nil, todayHTMX("")).Body.String()
	assertContains(t, body, "Delete 1 done item due today? You cannot undo this.")
	rec := do(t, env.H, "POST", "/items/done/delete", url.Values{"count": {"1"}}, todayHTMX(""))
	assertNotContains(t, rec.Body.String(), "Buy milk")
	if _, err := env.Svc.Get(context.Background(), env.User.ID, 6); err == nil {
		t.Fatal("the done item of the view is still there")
	}
	if _, err := env.Svc.Get(context.Background(), env.User.ID, 7); err != nil {
		t.Fatalf("the done item outside the view is gone: %v", err)
	}
}

// The switch looks as on the canvas (the chosen part blue), and a click on a
// label chip of a row keeps the view (app.js section 22).
func TestTodayViewAssets(t *testing.T) {
	h, _ := newTestApp(t)
	css := do(t, h, "GET", "/static/app.css", nil, nil).Body.String()
	assertContains(t, css, ".view-switch {", `.view-switch a[aria-current="page"] { background: var(--accent);`)
	js := do(t, h, "GET", "/static/app.js", nil, nil).Body.String()
	assertContains(t, js, `e.target.closest('.chip a[href^="/?label="]')`, `a.setAttribute("href", a.getAttribute("href") + "&due=today");`)
}
