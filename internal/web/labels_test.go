package web_test

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"todo/internal/todo"
)

func addLabeled(t *testing.T, svc *todo.Service, userID int64, text, labels string) todo.Item {
	t.Helper()
	it, err := svc.AddItem(context.Background(), userID, todo.Input{Text: text, Labels: labels}, nil)
	if err != nil {
		t.Fatalf("AddItem(%q): %v", text, err)
	}
	return it
}

func labelID(t *testing.T, it todo.Item, name string) int64 {
	t.Helper()
	for _, l := range it.Labels {
		if l.Name == name {
			return l.ID
		}
	}
	t.Fatalf("item %q has no label %q", it.Text, name)
	return 0
}

// filtered returns htmx headers for a request from the page filtered by label id.
func filtered(id int64) map[string]string {
	return map[string]string{"HX-Request": "true", "HX-Current-URL": "http://example.com/?label=" + strconv.FormatInt(id, 10)}
}

// The "Label" chip is gray until the pointer is on its row (canvas page 10,
// option D). It shows on every row, also on a row without labels.
func TestLabelChipIsQuiet(t *testing.T) {
	env := newTestEnv(t)
	addLabeled(t, env.Svc, env.User.ID, "Invoice", "")
	css := do(t, env.H, "GET", "/static/app.css", nil, nil).Body.String()
	assertContains(t, css, "color: #8C8A84", "left: var(--caret, 8px)")
	assertNotContains(t, css, ".label-add.bare", "border: 1px dashed #B9B6AD")
	body := do(t, env.H, "GET", "/", nil, nil).Body.String()
	assertContains(t, body, `<svg width="10" height="10" viewBox="0 0 12 12"`, `</svg>Label</button>`)
}

func TestPlusAddsOneLabel(t *testing.T) {
	env := newTestEnv(t)
	a := addLabeled(t, env.Svc, env.User.ID, "Invoice", "")
	addLabeled(t, env.Svc, env.User.ID, "Other", "home, work")
	path := fmt.Sprintf("/items/%d/labels", a.ID)

	body := do(t, env.H, "GET", "/", nil, nil).Body.String()
	assertContains(t, body, fmt.Sprintf(`id="label-add-%d" class="label-add"`, a.ID), fmt.Sprintf(`data-picker="%s"`, path))
	assertNotContains(t, body, "suggest-btn")

	// The list offers all the user's labels; the item's labels are pressed.
	body = do(t, env.H, "GET", path, nil, htmxHeaders).Body.String()
	assertContains(t, body, `data-name="home"`, `data-name="work"`, `class="picker-search"`,
		`aria-pressed="false">home</button>`, `<a class="picker-manage" href="/labels">Manage labels…</a>`,
		`class="wheel" role="slider"`, `data-full="false"`,
		fmt.Sprintf(`data-new-color="%s"`, todo.LabelColors[2]))
	// Colors and deletes are on the Labels page (option P2): a line has a
	// color dot, not a button, and no ×; only the Create line has a circle.
	assertNotContains(t, body, `role="listbox"`, `data-taken`, `class="picker-x"`)
	if n := strings.Count(body, `<button type="button" class="sw"`); n != 1 {
		t.Fatalf("%d color circle buttons, want 1 (the Create line)", n)
	}
	assertContains(t, body, `<span class="sw" data-color=`, `aria-hidden="true"></span><button type="button" class="picker-name"`)

	rec := do(t, env.H, "POST", path, url.Values{"name": {"work"}}, htmxHeaders)
	if rec.Code != http.StatusOK {
		t.Fatalf("add label: %d %s", rec.Code, rec.Body)
	}
	got, _ := env.Svc.Get(context.Background(), env.User.ID, a.ID)
	work := labelID(t, got, "work")
	assertContains(t, rec.Body.String(), fmt.Sprintf(`<span class="chip" data-color="%s" data-label="%d"><a href="/?label=%d"`, got.Labels[0].Color, work, work),
		fmt.Sprintf(`hx-delete="/items/%d/labels/%d"`, a.ID, work), `id="toolbar"`)

	// The item's label is pressed and checked; a new name makes a label.
	body = do(t, env.H, "GET", path, nil, htmxHeaders).Body.String()
	assertContains(t, body, `aria-pressed="true">work</button><span class="picker-check"`, `aria-pressed="false">home</button>`)
	do(t, env.H, "POST", path, url.Values{"name": {"  Deep   work "}}, htmxHeaders)
	got, _ = env.Svc.Get(context.Background(), env.User.ID, a.ID)
	if todo.LabelNames(got.Labels) != "Deep work, work" {
		t.Fatalf("labels = %q", todo.LabelNames(got.Labels))
	}
}

func TestPlusShowsLabelErrorsInTheList(t *testing.T) {
	env := newTestEnv(t)
	a := addLabeled(t, env.Svc, env.User.ID, "Trip", "a, b, c, d")
	path := fmt.Sprintf("/items/%d/labels", a.ID)

	rec := do(t, env.H, "POST", path, url.Values{"name": {strings.Repeat("x", 31)}}, htmxHeaders)
	if rec.Code != http.StatusUnprocessableEntity || rec.Header().Get("HX-Retarget") != fmt.Sprintf("#picker-%d", a.ID) {
		t.Fatalf("long name: %d %q", rec.Code, rec.Header().Get("HX-Retarget"))
	}
	assertContains(t, rec.Body.String(), "A label can have at most 30 characters.", `aria-invalid="true"`)

	if rec := do(t, env.H, "POST", path, url.Values{"name": {"e"}}, htmxHeaders); rec.Code != http.StatusOK {
		t.Fatalf("fifth label: %d", rec.Code)
	}
	rec = do(t, env.H, "POST", path, url.Values{"name": {"f"}}, htmxHeaders)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("sixth label: %d, want 422", rec.Code)
	}
	assertContains(t, rec.Body.String(), "An item can have at most 5 labels.")
	full := do(t, env.H, "GET", path, nil, htmxHeaders).Body.String()
	assertContains(t, full, "An item can have at most 5 labels.", `data-full="true"`)
	assertNotContains(t, full, "Remove one first.")
}

func TestRemoveLabelFromItem(t *testing.T) {
	env := newTestEnv(t)
	a := addLabeled(t, env.Svc, env.User.ID, "Invoice", "work, money")
	work := labelID(t, a, "work")
	rec := do(t, env.H, "DELETE", fmt.Sprintf("/items/%d/labels/%d", a.ID, work), nil, htmxHeaders)
	if rec.Code != http.StatusOK {
		t.Fatalf("remove: %d", rec.Code)
	}
	assertContains(t, rec.Body.String(), ">money</a>", fmt.Sprintf(`id="item-%d"`, a.ID))
	assertNotContains(t, rec.Body.String(), ">work</a>")
	// The label itself stays.
	if _, err := env.Svc.Label(context.Background(), env.User.ID, work); err != nil {
		t.Fatalf("label gone: %v", err)
	}
}

func TestFilterByLabel(t *testing.T) {
	env := newTestEnv(t)
	a := addLabeled(t, env.Svc, env.User.ID, "Invoice", "work")
	addLabeled(t, env.Svc, env.User.ID, "Water the plants", "home")
	work := labelID(t, a, "work")

	body := do(t, env.H, "GET", fmt.Sprintf("/?label=%d", work), nil, nil).Body.String()
	assertContains(t, body, "Invoice", `class="active-filter"`, `href="/" aria-label="Show all labels"`,
		"1 open · 0 done", fmt.Sprintf(`name="labels" value="%d"`, work), `<span class="chip-name">work</span>`)
	assertNotContains(t, body, "Water the plants")

	// An unknown or foreign label shows the full list.
	body = do(t, env.H, "GET", "/?label=999", nil, nil).Body.String()
	assertContains(t, body, "Invoice", "Water the plants")
	assertContains(t, body, `name="labels" value=""`)
	assertNotContains(t, body, `class="active-filter"`, `class="chip-name"`)

	// htmx requests from the filtered page are filtered too.
	rec := do(t, env.H, "POST", "/settings/hide-done", url.Values{"hide_done": {"1"}}, filtered(work))
	assertContains(t, rec.Body.String(), "Invoice", `class="active-filter"`)
	assertNotContains(t, rec.Body.String(), "Water the plants")
}

func TestFilterEmptyState(t *testing.T) {
	env := newTestEnv(t)
	a := addLabeled(t, env.Svc, env.User.ID, "Invoice", "work")
	work := labelID(t, a, "work")
	if err := env.Svc.Delete(context.Background(), env.User.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	body := do(t, env.H, "GET", fmt.Sprintf("/?label=%d", work), nil, nil).Body.String()
	assertContains(t, body, "No items with this label")
}

func TestAddWhileFilteredGetsTheLabel(t *testing.T) {
	env := newTestEnv(t)
	a := addLabeled(t, env.Svc, env.User.ID, "Invoice", "work")
	work := labelID(t, a, "work")
	// The filtered page picks the filter label in #add-extras.
	rec := do(t, env.H, "POST", "/items", url.Values{"text": {"Report"}, "labels": {fmt.Sprint(work)}}, filtered(work))
	if rec.Code != http.StatusOK {
		t.Fatalf("add: %d", rec.Code)
	}
	assertContains(t, rec.Body.String(), "Report", ">work</a>", "2 open · 0 done", fmt.Sprintf(`name="labels" value="%d"`, work))
	// Without the label the item is not in the filtered list: the list comes back without it.
	rec = do(t, env.H, "POST", "/items", url.Values{"text": {"Elsewhere"}, "labels": {""}}, filtered(work))
	if rec.Code != http.StatusOK || rec.Header().Get("HX-Retarget") != "#list-section" {
		t.Fatalf("add without the label: %d %q", rec.Code, rec.Header().Get("HX-Retarget"))
	}
	assertNotContains(t, rec.Body.String(), "Elsewhere")
	// Unknown label ids are left out.
	do(t, env.H, "POST", "/items", url.Values{"text": {"Plain"}, "labels": {"x,999"}}, htmxHeaders)
	items, _ := env.Svc.List(context.Background(), env.User.ID, false)
	if got := items[len(items)-1]; got.Text != "Plain" || len(got.Labels) != 0 {
		t.Fatalf("plain item = %+v", got)
	}
}

func TestEditKeepsLabels(t *testing.T) {
	env := newTestEnv(t)
	a := addLabeled(t, env.Svc, env.User.ID, "Invoice", "work, money")
	path := fmt.Sprintf("/items/%d", a.ID)
	assertNotContains(t, do(t, env.H, "GET", path+"/edit", nil, htmxHeaders).Body.String(), `name="labels"`)
	rec := do(t, env.H, "PUT", path, url.Values{"text": {"Invoice 2"}, "labels": {""}}, htmxHeaders)
	if rec.Code != http.StatusOK {
		t.Fatalf("edit: %d %s", rec.Code, rec.Body)
	}
	assertContains(t, rec.Body.String(), "Invoice 2", ">work</a>", ">money</a>")
}

func TestRemoveWhileFilteredRemovesTheRow(t *testing.T) {
	env := newTestEnv(t)
	a := addLabeled(t, env.Svc, env.User.ID, "Invoice", "work")
	work := labelID(t, a, "work")
	rec := do(t, env.H, "DELETE", fmt.Sprintf("/items/%d/labels/%d", a.ID, work), nil, filtered(work))
	if rec.Code != http.StatusOK {
		t.Fatalf("remove: %d", rec.Code)
	}
	assertNotContains(t, rec.Body.String(), `id="item-`)
	assertContains(t, rec.Body.String(), "0 open · 0 done")
}

func TestLabelsOfOtherUsersItems(t *testing.T) {
	env := newTestEnv(t)
	bob, err := env.Auth.SignUp(context.Background(), "bob2", pw)
	if err != nil {
		t.Fatal(err)
	}
	b := addLabeled(t, env.Svc, bob.ID, "Bob's", "secret")
	path := fmt.Sprintf("/items/%d/labels", b.ID)
	if rec := do(t, env.H, "GET", path, nil, htmxHeaders); rec.Code != http.StatusNotFound {
		t.Fatalf("picker: %d, want 404", rec.Code)
	}
	if rec := do(t, env.H, "POST", path, url.Values{"name": {"mine"}}, htmxHeaders); rec.Code != http.StatusNotFound {
		t.Fatalf("add: %d, want 404", rec.Code)
	}
	do(t, env.H, "DELETE", fmt.Sprintf("%s/%d", path, labelID(t, b, "secret")), nil, htmxHeaders)
	got, _ := env.Svc.Get(context.Background(), bob.ID, b.ID)
	if todo.LabelNames(got.Labels) != "secret" {
		t.Fatalf("bob's labels = %q", todo.LabelNames(got.Labels))
	}
}

func TestLabelsOfAnotherUserCannotBeChanged(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	bob, err := env.Auth.SignUp(ctx, "bob", pw)
	if err != nil {
		t.Fatal(err)
	}
	b := addLabeled(t, env.Svc, bob.ID, "Bob's", "secret")
	id := labelID(t, b, "secret")

	if rec := do(t, env.H, "PUT", fmt.Sprintf("/labels/%d/color", id), url.Values{"color": {"#d1382e"}}, htmxHeaders); rec.Code != http.StatusNotFound {
		t.Fatalf("color of bob's label: %d, want 404", rec.Code)
	}
	if rec := do(t, env.H, "DELETE", fmt.Sprintf("/labels/%d", id), nil, htmxHeaders); rec.Code != http.StatusNotFound {
		t.Fatalf("delete bob's label: %d, want 404", rec.Code)
	}
	l, err := env.Svc.Label(ctx, bob.ID, id)
	if err != nil || l.Color == "#d1382e" {
		t.Fatalf("bob's label = %+v, %v", l, err)
	}
	body := do(t, env.H, "GET", fmt.Sprintf("/?label=%d", id), nil, nil).Body.String()
	assertNotContains(t, body, "secret", "Bob's")
}

func TestLabelNamesAreEscaped(t *testing.T) {
	env := newTestEnv(t)
	it := addLabeled(t, env.Svc, env.User.ID, "X", `<b>bold</b>`)
	for _, path := range []string{"/", fmt.Sprintf("/items/%d/labels", it.ID)} {
		body := do(t, env.H, "GET", path, nil, htmxHeaders).Body.String()
		assertContains(t, body, "&lt;b&gt;bold&lt;/b&gt;")
		assertNotContains(t, body, "<b>bold</b>")
	}
}

func TestCreateLabelWithColor(t *testing.T) {
	env := newTestEnv(t)
	a := addLabeled(t, env.Svc, env.User.ID, "Invoice", "")
	path := fmt.Sprintf("/items/%d/labels", a.ID)
	rec := do(t, env.H, "POST", path, url.Values{"name": {"gym"}, "color": {"#12ab34"}}, htmxHeaders)
	if rec.Code != http.StatusOK {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	got, _ := env.Svc.Get(context.Background(), env.User.ID, a.ID)
	if len(got.Labels) != 1 || got.Labels[0].Color != "#12ab34" {
		t.Fatalf("labels = %+v", got.Labels)
	}
	// After an error the list keeps the chosen color of the new label.
	rec = do(t, env.H, "POST", path, url.Values{"name": {strings.Repeat("x", 31)}, "color": {"#abcdef"}}, htmxHeaders)
	assertContains(t, rec.Body.String(), `data-new-color="#abcdef"`)
}

func TestSetLabelColorRoute(t *testing.T) {
	env := newTestEnv(t)
	a := addLabeled(t, env.Svc, env.User.ID, "Invoice", "work")
	work := labelID(t, a, "work")
	path := fmt.Sprintf("/labels/%d/color", work)
	if rec := do(t, env.H, "PUT", path, url.Values{"color": {"#6B44BB"}}, htmxHeaders); rec.Code != http.StatusNoContent {
		t.Fatalf("set: %d %s", rec.Code, rec.Body)
	}
	if l, _ := env.Svc.Label(context.Background(), env.User.ID, work); l.Color != "#6b44bb" {
		t.Fatalf("color = %q", l.Color)
	}
	if rec := do(t, env.H, "PUT", path, url.Values{"color": {"red"}}, htmxHeaders); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad color: %d, want 422", rec.Code)
	}
	if rec := do(t, env.H, "PUT", "/labels/999/color", url.Values{"color": {"#000000"}}, htmxHeaders); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown: %d, want 404", rec.Code)
	}
}

func TestFiftyLabelsMessageNamesTheLabelsPage(t *testing.T) {
	env := newTestEnv(t)
	for i := 0; i < todo.MaxLabelsPerUser; i += 5 {
		addLabeled(t, env.Svc, env.User.ID, "Item", fmt.Sprintf("l%d, l%d, l%d, l%d, l%d", i, i+1, i+2, i+3, i+4))
	}
	a := addLabeled(t, env.Svc, env.User.ID, "One more", "")
	rec := do(t, env.H, "POST", fmt.Sprintf("/items/%d/labels", a.ID), url.Values{"name": {"new"}}, htmxHeaders)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("51st label: %d, want 422", rec.Code)
	}
	assertContains(t, rec.Body.String(), "You have 50 labels. Delete one on the Labels page first.")
}

func TestLabelListScriptAndStyle(t *testing.T) {
	env := newTestEnv(t)
	js := do(t, env.H, "GET", "/static/app.js", nil, nil).Body.String()
	assertContains(t, js, "reopenFor", "makeWheel(", `closest("button.sw")`)
	assertNotContains(t, js, "aria-activedescendant", "data-taken", "askDelete(", `"/labels/" + edit.label`)
	css := do(t, env.H, "GET", "/static/app.css", nil, nil).Body.String()
	assertContains(t, css, ".picker-color[hidden]", ".picker-manage")
	assertNotContains(t, css, ".picker-x", ".picker-opt.asking")
}

// The wheel view is a group named by its title ("Color of “work”").
func TestWheelViewIsLabelledByItsTitle(t *testing.T) {
	env := newTestEnv(t)
	a := addLabeled(t, env.Svc, env.User.ID, "Invoice", "work")
	body := do(t, env.H, "GET", fmt.Sprintf("/items/%d/labels", a.ID), nil, htmxHeaders).Body.String()
	assertContains(t, body,
		fmt.Sprintf(`class="picker-color" role="group" aria-labelledby="picker-color-title-%d"`, a.ID),
		fmt.Sprintf(`<span class="picker-color-title" id="picker-color-title-%d">`, a.ID))
}
