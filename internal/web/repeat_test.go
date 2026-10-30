package web_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"todo/internal/todo"
)

// Repeating items (canvas page "20 · Repeat", option R1).

func warsawZone(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Warsaw")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

// The due date pop-up has a Repeat list with the item's rule selected.
func TestDuePickerHasRepeat(t *testing.T) {
	env := newTestEnv(t)
	it, err := env.Svc.AddItem(context.Background(), env.User.ID,
		todo.Input{Text: "Water the plants", DueDate: "2099-10-05", DueTime: "18:00", Repeat: todo.RepeatWeekly}, warsawZone(t))
	if err != nil {
		t.Fatal(err)
	}
	body := do(t, env.H, "GET", "/items/1/due", nil, warsawHTMX()).Body.String()
	if it.ID != 1 {
		t.Fatalf("id = %d, want 1", it.ID)
	}
	assertContains(t, body, `<select name="due_repeat" id="due-repeat-1" class="due-repeat">`,
		`<option value="">Never</option>`, `<option value="day">Every day</option>`,
		`<option value="week" selected>Every week</option>`, `<option value="month">Every month</option>`,
		`<option value="year">Every year</option>`, `<label class="due-picker-title" for="due-repeat-1">Repeat</label>`)
}

// Saving the pop-up with a rule makes the item repeat; the date chip shows ↻.
func TestSetDueWithRepeat(t *testing.T) {
	env := newTestEnv(t)
	mustAdd(t, env.Svc, "Water the plants")
	form := url.Values{"due_date": {"2099-10-05"}, "due_time": {"18:00"}, "due_tz": {"Europe/Warsaw"}, "due_repeat": {"week"}}
	rec := do(t, env.H, "PUT", "/items/1/due", form, warsawHTMX())
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT due: %d\n%s", rec.Code, rec.Body.String())
	}
	assertContains(t, rec.Body.String(), `class="repeat-icon" title="Every week"`, "Repeats every week. Change the due date")
	it, _ := env.Svc.Get(context.Background(), env.User.ID, 1)
	if it.Repeat != todo.RepeatWeekly {
		t.Fatalf("repeat = %q, want week", it.Repeat)
	}

	// No date with a rule: the pop-up comes back with a message.
	form = url.Values{"due_date": {""}, "due_time": {""}, "due_repeat": {"week"}}
	rec = do(t, env.H, "PUT", "/items/1/due", form, warsawHTMX())
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("no date: %d, want 422", rec.Code)
	}
	assertContains(t, rec.Body.String(), "A repeating item needs a due date.", `<option value="week" selected>`)

	// The × of the date chip removes the date and the repeat.
	rec = do(t, env.H, "PUT", "/items/1/due", nil, warsawHTMX())
	assertNotContains(t, rec.Body.String(), "repeat-icon")
	it, _ = env.Svc.Get(context.Background(), env.User.ID, 1)
	if it.DueAt != nil || it.Repeat != "" {
		t.Fatalf("after ×: %+v, want no date, no repeat", it)
	}
}

// The checkbox of a repeating item moves its date and keeps it open. The
// answer (header X-Todo-Repeated) tells the page what to show ("Done. Next time: …") and how to undo.
func TestToggleRepeatingItem(t *testing.T) {
	env := newTestEnv(t)
	_, err := env.Svc.AddItem(context.Background(), env.User.ID,
		todo.Input{Text: "Water the plants", DueDate: "2099-10-05", DueTime: "18:00", Repeat: todo.RepeatWeekly}, warsawZone(t))
	if err != nil {
		t.Fatal(err)
	}
	rec := do(t, env.H, "POST", "/items/1/toggle", url.Values{"done": {"1"}}, warsawHTMX())
	if rec.Code != http.StatusOK {
		t.Fatalf("toggle: %d", rec.Code)
	}
	assertContains(t, rec.Body.String(), `<li id="item-1" class="item">`, "Mark done: Water the plants", "12 Oct 2099, 18:00")
	var got struct {
		ID     int64  `json:"id"`
		Next   string `json:"next"`
		Date   string `json:"date"`
		Time   string `json:"time"`
		TZ     string `json:"tz"`
		Repeat string `json:"repeat"`
	}
	if err := json.Unmarshal([]byte(rec.Header().Get("X-Todo-Repeated")), &got); err != nil {
		t.Fatalf("X-Todo-Repeated %q: %v", rec.Header().Get("X-Todo-Repeated"), err)
	}
	if got.ID != 1 || got.Next != "12 Oct 2099, 18:00" || got.Date != "2099-10-05" || got.Time != "18:00" || got.TZ != "Europe/Warsaw" || got.Repeat != "week" {
		t.Fatalf("event = %+v", got)
	}

	// Undo puts the old date back.
	form := url.Values{"due_date": {got.Date}, "due_time": {got.Time}, "due_tz": {got.TZ}, "due_repeat": {got.Repeat}}
	rec = do(t, env.H, "PUT", "/items/1/due", form, warsawHTMX())
	assertContains(t, rec.Body.String(), "5 Oct 2099, 18:00", `class="repeat-icon"`)

	// A plain item: no event.
	mustAdd(t, env.Svc, "Pay the rent")
	rec = do(t, env.H, "POST", "/items/2/toggle", url.Values{"done": {"1"}}, warsawHTMX())
	if h := rec.Header().Get("X-Todo-Repeated"); h != "" {
		t.Fatalf("plain item: event %q, want none", h)
	}
}

// The add form keeps the rule of its due date pop-up and gives it to the new item.
func TestNewItemWithRepeat(t *testing.T) {
	env := newTestEnv(t)
	form := url.Values{"due_date": {"2099-10-05"}, "due_time": {""}, "due_repeat": {"month"}}
	rec := do(t, env.H, "POST", "/new/due", form, warsawHTMX())
	assertContains(t, rec.Body.String(), `<input type="hidden" form="add-form" name="due_repeat" value="month">`,
		`class="repeat-icon" title="Every month"`, "due_repeat=month")
	rec = do(t, env.H, "POST", "/items", url.Values{"text": {"Pay the rent"}, "due_date": {"2099-10-05"}, "due_repeat": {"month"}}, warsawHTMX())
	if rec.Code != http.StatusOK {
		t.Fatalf("add: %d\n%s", rec.Code, rec.Body.String())
	}
	it, _ := env.Svc.Get(context.Background(), env.User.ID, 1)
	if it.Repeat != todo.RepeatMonthly || it.RepeatDay != 5 {
		t.Fatalf("new item: repeat %q day %d, want month 5", it.Repeat, it.RepeatDay)
	}
}

// The API shows the rule, PATCH done moves the date, and PUT due keeps the
// rule unless it sends one.
func TestAPIRepeat(t *testing.T) {
	env := newTestEnv(t)
	_, err := env.Svc.AddItem(context.Background(), env.User.ID,
		todo.Input{Text: "Water the plants", DueDate: "2099-10-05", DueTime: "18:00", Repeat: todo.RepeatWeekly}, warsawZone(t))
	if err != nil {
		t.Fatal(err)
	}
	h := map[string]string{"Time-Zone": "Europe/Warsaw"}
	body := apiDo(t, env, "GET", "/api/v1/items/1", env.Token, "", h).Body.String()
	assertContains(t, body, `"repeat":"week"`)

	body = apiDo(t, env, "PATCH", "/api/v1/items/1", env.Token, `{"done":true}`, h).Body.String()
	assertContains(t, body, `"done":false`, `"repeat":"week"`, `"due_at":"2099-10-12T16:00:00Z"`)

	body = apiDo(t, env, "PUT", "/api/v1/items/1/due", env.Token, `{"date":"2099-11-02","time":"07:00"}`, h).Body.String()
	assertContains(t, body, `"repeat":"week"`)
	body = apiDo(t, env, "PUT", "/api/v1/items/1/due", env.Token, `{"date":"2099-11-02","time":"07:00","repeat":""}`, h).Body.String()
	assertContains(t, body, `"repeat":""`)
	if !strings.Contains(body, `"due_at":"2099-11-02T06:00:00Z"`) {
		t.Fatalf("due kept? %s", body)
	}
}

// Search (canvas page "19 · Search", option A): the ⌕ is the first button of
// the toolbar, and the closed search row sits between the toolbar and the list.
func TestSearchMarkup(t *testing.T) {
	env := newTestEnv(t)
	mustAdd(t, env.Svc, "Fix the bike with Julek")
	body := do(t, env.H, "GET", "/", nil, nil).Body.String()
	assertContains(t, body,
		`<div class="toolbar-right"><button class="filter search-toggle" type="button" aria-label="Search the items" title="Search the items (/)" aria-expanded="false" aria-controls="search-row">`,
		`<div id="search-row" class="search-row" role="search" hidden>`,
		`<input id="search" type="text" placeholder="Search the items" aria-label="Search the items"`)
	if a, b, c := strings.Index(body, `id="toolbar"`), strings.Index(body, `id="search-row"`), strings.Index(body, `id="items"`); a < 0 || !(a < b && b < c) {
		t.Fatalf("want toolbar, search row, list in this order: %d %d %d", a, b, c)
	}
}
