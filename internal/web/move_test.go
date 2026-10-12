package web_test

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func moveForm(after int64) url.Values {
	if after == 0 {
		return url.Values{"after": {""}}
	}
	return url.Values{"after": {strconv.FormatInt(after, 10)}}
}

func moveURL(id int64) string { return "/items/" + strconv.FormatInt(id, 10) + "/move" }

// svcOrder returns the texts of the tester's items in stored order.
func svcOrder(t *testing.T, env *testEnv) string {
	t.Helper()
	items, err := env.Svc.List(context.Background(), env.User.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, it := range items {
		out = append(out, it.Text)
	}
	return strings.Join(out, ",")
}

func TestMoveSavesOrder(t *testing.T) {
	env := newTestEnv(t)
	a := mustAdd(t, env.Svc, "a")
	mustAdd(t, env.Svc, "b")
	c := mustAdd(t, env.Svc, "c")

	rec := do(t, env.H, "POST", moveURL(c.ID), moveForm(0), htmxHeaders)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("status = %d, body %q; want 204 and no body", rec.Code, rec.Body.String())
	}
	if got := svcOrder(t, env); got != "c,a,b" {
		t.Fatalf("order = %s, want c,a,b", got)
	}
	do(t, env.H, "POST", moveURL(c.ID), moveForm(a.ID), htmxHeaders)
	if got := svcOrder(t, env); got != "a,c,b" {
		t.Fatalf("order = %s, want a,c,b", got)
	}
}

func TestMoveNotFoundAnswersWithList(t *testing.T) {
	env := newTestEnv(t)
	a := mustAdd(t, env.Svc, "a")
	mustAdd(t, env.Svc, "b")
	other, err := env.Store.CreateUser(context.Background(), "other", "x", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	theirs, err := env.Svc.Add(context.Background(), other.ID, "theirs", "", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name, target string
		form         url.Values
	}{
		{"unknown id", moveURL(999), moveForm(0)},
		{"bad id", "/items/x/move", moveForm(0)},
		{"bad after", moveURL(a.ID), url.Values{"after": {"x"}}},
		{"unknown after", moveURL(a.ID), moveForm(999)},
		{"other user's id", moveURL(theirs.ID), moveForm(0)},
		{"other user's after", moveURL(a.ID), moveForm(theirs.ID)},
	} {
		rec := do(t, env.H, "POST", c.target, c.form, htmxHeaders)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", c.name, rec.Code)
			continue
		}
		if rec.Header().Get("HX-Retarget") != "#list-section" || rec.Header().Get("HX-Reswap") != "outerHTML" {
			t.Errorf("%s: headers = %v, want the list retarget", c.name, rec.Header())
		}
		assertContains(t, rec.Body.String(), `id="list-section"`)
	}
	if got := svcOrder(t, env); got != "a,b" {
		t.Fatalf("order = %s, want a,b unchanged", got)
	}
}

func TestMoveOverdueAnswers422WithList(t *testing.T) {
	env, ids := overdueEnv(t)
	rec := do(t, env.H, "POST", moveURL(ids["Call the dentist"]), moveForm(0), warsawHTMX())
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	if rec.Header().Get("HX-Retarget") != "#list-section" {
		t.Fatalf("HX-Retarget = %q, want #list-section", rec.Header().Get("HX-Retarget"))
	}
	assertOrder(t, rec.Body.String(), "Pay the rent", "Call the dentist", "Return library books", "Water the plants")
}

func TestMoveNeedsLogin(t *testing.T) {
	env := newTestEnv(t)
	a := mustAdd(t, env.Svc, "a")
	rec := do(t, env.H, "POST", moveURL(a.ID), moveForm(0), map[string]string{"HX-Request": "true", anonHeader: "1"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestMoveWithHideDoneKeepsHiddenItemsInPlace(t *testing.T) {
	env := newTestEnv(t)
	// Long texts, so assertOrder (">TEXT<") cannot match other page text.
	a := mustAdd(t, env.Svc, "Alpha task")
	done := mustAdd(t, env.Svc, "Finished task")
	mustAdd(t, env.Svc, "Beta task")
	c := mustAdd(t, env.Svc, "Gamma task")
	do(t, env.H, "POST", "/items/"+strconv.FormatInt(done.ID, 10)+"/toggle", nil, htmxHeaders)
	do(t, env.H, "POST", "/settings/hide-done", url.Values{"hide_done": {"1"}}, htmxHeaders)

	// The page shows Alpha, Beta, Gamma. The user drags Gamma after Alpha.
	if rec := do(t, env.H, "POST", moveURL(c.ID), moveForm(a.ID), htmxHeaders); rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	do(t, env.H, "POST", "/settings/hide-done", url.Values{"hide_done": {"0"}}, htmxHeaders)
	body := do(t, env.H, "GET", "/", nil, nil).Body.String()
	assertOrder(t, body, "Alpha task", "Gamma task", "Finished task", "Beta task")
}

func TestGripOnNormalRowsOnly(t *testing.T) {
	env, ids := overdueEnv(t)
	do(t, env.H, "POST", "/items/"+strconv.FormatInt(ids["Return library books"], 10)+"/toggle", nil, warsawHTMX())
	body := do(t, env.H, "GET", "/", nil, map[string]string{"Cookie": "todo_tz=Europe/Warsaw"}).Body.String()

	// A done row and an open row that is not overdue have a grip.
	assertContains(t, body, `aria-label="Move: Return library books"`, `aria-label="Move: Water the plants"`,
		`<p id="move-status" class="visually-hidden" aria-live="polite"></p>`)
	// Overdue rows have no grip, and they carry the class overdue-row.
	assertNotContains(t, body, `aria-label="Move: Call the dentist"`, `aria-label="Move: Pay the rent"`)
	assertContains(t, body, `id="item-`+strconv.FormatInt(ids["Call the dentist"], 10)+`" class="item overdue-row"`)

	edit := do(t, env.H, "GET", "/items/"+strconv.FormatInt(ids["Water the plants"], 10)+"/edit", nil, warsawHTMX()).Body.String()
	assertNotContains(t, edit, `class="grip"`)
	assertContains(t, edit, `class="grip-space"`)
}

// The edit row of an overdue item stays in the overdue group: the page must
// not treat it as a normal row (no drop next to it, not counted in "N of M").
func TestOverdueEditRowIsMarked(t *testing.T) {
	env, ids := overdueEnv(t)
	edit := do(t, env.H, "GET", "/items/"+strconv.FormatInt(ids["Call the dentist"], 10)+"/edit", nil, warsawHTMX()).Body.String()
	assertContains(t, edit, `class="item editing overdue-row"`)
	normal := do(t, env.H, "GET", "/items/"+strconv.FormatInt(ids["Water the plants"], 10)+"/edit", nil, warsawHTMX()).Body.String()
	assertNotContains(t, normal, "overdue-row")
}
