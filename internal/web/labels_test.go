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

// The + is small and gray, with no circle (canvas page 16, option C). It shows
// only when the pointer is on the row or focus is in it, also on a row
// without labels; an invisible area keeps it easy to click.
func TestLabelPlusIsSmallAndGray(t *testing.T) {
	env := newTestEnv(t)
	addLabeled(t, env.Svc, env.User.ID, "Invoice", "")
	css := do(t, env.H, "GET", "/static/app.css", nil, nil).Body.String()
	assertContains(t, css, "color: #8C8A84", ".label-add::after", "left: var(--caret, 8px)")
	assertNotContains(t, css, ".label-add.bare", "border: 1px dashed #B9B6AD")
	body := do(t, env.H, "GET", "/", nil, nil).Body.String()
	assertContains(t, body, `<svg width="9" height="9" viewBox="0 0 12 12"`)
}

func TestPlusAddsOneLabel(t *testing.T) {
	env := newTestEnv(t)
	a := addLabeled(t, env.Svc, env.User.ID, "Invoice", "")
	addLabeled(t, env.Svc, env.User.ID, "Other", "home, work")
	path := fmt.Sprintf("/items/%d/labels", a.ID)

	body := do(t, env.H, "GET", "/", nil, nil).Body.String()
	assertContains(t, body, fmt.Sprintf(`id="label-add-%d" class="label-add"`, a.ID), fmt.Sprintf(`data-picker="%s"`, path))
	assertNotContains(t, body, `name="labels"`, "suggest-btn")

	// The list offers the user's labels, by name.
	body = do(t, env.H, "GET", path, nil, htmxHeaders).Body.String()
	assertContains(t, body, `role="listbox"`, `data-name="home"`, `data-name="work"`, `class="picker-search"`)

	rec := do(t, env.H, "POST", path, url.Values{"name": {"work"}}, htmxHeaders)
	if rec.Code != http.StatusOK {
		t.Fatalf("add label: %d %s", rec.Code, rec.Body)
	}
	got, _ := env.Svc.Get(context.Background(), env.User.ID, a.ID)
	work := labelID(t, got, "work")
	assertContains(t, rec.Body.String(), fmt.Sprintf(`<span class="chip" data-color="%s"><a href="/?label=%d"`, got.Labels[0].Color, work),
		fmt.Sprintf(`hx-delete="/items/%d/labels/%d"`, a.ID, work), `id="toolbar"`)

	// The list leaves out the item's labels; a new name makes a label.
	body = do(t, env.H, "GET", path, nil, htmxHeaders).Body.String()
	assertContains(t, body, `data-name="home"`, `data-taken="work"`)
	assertNotContains(t, body, `data-name="work"`)
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
	assertContains(t, do(t, env.H, "GET", path, nil, htmxHeaders).Body.String(), "An item can have at most 5 labels. Remove one first.")
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
		"1 open · 0 done", `New items get the label <span class="chip" data-color=`)
	assertNotContains(t, body, "Water the plants")

	// An unknown or foreign label shows the full list.
	body = do(t, env.H, "GET", "/?label=999", nil, nil).Body.String()
	assertContains(t, body, "Invoice", "Water the plants")
	assertNotContains(t, body, `class="active-filter"`, "New items get the label")

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
	rec := do(t, env.H, "POST", "/items", url.Values{"text": {"Report"}, "labels": {"ignored"}}, filtered(work))
	if rec.Code != http.StatusOK {
		t.Fatalf("add: %d", rec.Code)
	}
	assertContains(t, rec.Body.String(), "Report", ">work</a>", "2 open · 0 done")
	// Without a filter a new item has no labels.
	do(t, env.H, "POST", "/items", url.Values{"text": {"Plain"}, "labels": {"x"}}, htmxHeaders)
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

func TestLabelsPage(t *testing.T) {
	env := newTestEnv(t)
	a := addLabeled(t, env.Svc, env.User.ID, "Invoice", "work, home")
	work, home := labelID(t, a, "work"), labelID(t, a, "home")

	body := do(t, env.H, "GET", "/labels", nil, nil).Body.String()
	assertContains(t, body, "<h1>Labels</h1>", ">work</span>", fmt.Sprintf(`href="/?label=%d">1 item</a>`, work),
		fmt.Sprintf(`action="/labels/%d"`, work))
	// No fixed colors: a color button after the name opens the color wheel.
	wl, _ := env.Svc.Label(context.Background(), env.User.ID, work)
	assertContains(t, body,
		fmt.Sprintf(`<span class="chip" data-color="%s">work</span>`, wl.Color),
		fmt.Sprintf(`<button type="button" class="color-btn" data-color="%s" data-name="work" aria-label="Color of “work”"`, wl.Color),
		fmt.Sprintf(`<input type="hidden" name="color" value="%s">`, wl.Color),
		`id="color-picker"`, `class="wheel" role="slider"`, `/static/chips.js`, `/static/labels.js`)
	if strings.Contains(body, "swatches") || strings.Contains(body, `type="radio"`) {
		t.Fatal("the labels page still shows fixed colors")
	}
	if i := strings.Index(body, fmt.Sprintf(`id="label-name-%d"`, work)); i < 0 || !strings.Contains(body[i:strings.Index(body[i:], "</div>")+i], `class="color-btn"`) {
		t.Fatal("the color button must come right after the name field")
	}
	assertContains(t, do(t, env.H, "GET", "/", nil, nil).Body.String(), `<a href="/labels">Labels</a>`)
	assertContains(t, body, "click + on an item")

	// Rename and recolor.
	rec := do(t, env.H, "POST", fmt.Sprintf("/labels/%d", work), url.Values{"name": {"deep work"}, "color": {"#6b44bb"}}, nil)
	if rec.Code != http.StatusSeeOther || !strings.HasPrefix(rec.Header().Get("Location"), fmt.Sprintf("/labels?saved=%d", work)) {
		t.Fatalf("save: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	l, _ := env.Svc.Label(context.Background(), env.User.ID, work)
	if l.Name != "deep work" || l.Color != "#6b44bb" {
		t.Fatalf("label = %+v", l)
	}
	assertContains(t, do(t, env.H, "GET", fmt.Sprintf("/labels?saved=%d", work), nil, nil).Body.String(), "Saved.")

	// A name that is taken.
	rec = do(t, env.H, "POST", fmt.Sprintf("/labels/%d", work), url.Values{"name": {"HOME"}, "color": {"#d1382e"}}, nil)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("clash: %d, want 422", rec.Code)
	}
	assertContains(t, rec.Body.String(), "You already have a label “HOME”.", `value="HOME"`,
		`<input type="hidden" name="color" value="#d1382e">`)

	// A color that is not #rrggbb.
	rec = do(t, env.H, "POST", fmt.Sprintf("/labels/%d", work), url.Values{"name": {"deep work"}, "color": {"red"}}, nil)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad color: %d, want 422", rec.Code)
	}
	assertContains(t, rec.Body.String(), "Pick a color.", `<input type="hidden" name="color" value="#6b44bb">`)

	// Delete asks first, then removes the label but keeps the item.
	body = do(t, env.H, "GET", fmt.Sprintf("/labels?delete=%d", home), nil, nil).Body.String()
	assertContains(t, body, "Delete “home”? It comes off 1 item. The items stay.", fmt.Sprintf(`action="/labels/%d/delete"`, home))
	rec = do(t, env.H, "POST", fmt.Sprintf("/labels/%d/delete", home), url.Values{}, nil)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("delete: %d", rec.Code)
	}
	got, err := env.Svc.Get(context.Background(), env.User.ID, a.ID)
	if err != nil || todo.LabelNames(got.Labels) != "deep work" {
		t.Fatalf("item after delete = %q, %v", todo.LabelNames(got.Labels), err)
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

	do(t, env.H, "POST", fmt.Sprintf("/labels/%d", id), url.Values{"name": {"mine"}, "color": {"#d1382e"}}, nil)
	do(t, env.H, "POST", fmt.Sprintf("/labels/%d/delete", id), url.Values{}, nil)
	l, err := env.Svc.Label(ctx, bob.ID, id)
	if err != nil || l.Name != "secret" {
		t.Fatalf("bob's label = %+v, %v", l, err)
	}
	body := do(t, env.H, "GET", fmt.Sprintf("/?label=%d", id), nil, nil).Body.String()
	assertNotContains(t, body, "secret", "Bob's")
}

func TestLabelNamesAreEscaped(t *testing.T) {
	env := newTestEnv(t)
	addLabeled(t, env.Svc, env.User.ID, "X", `<b>bold</b>`)
	for _, path := range []string{"/", "/labels"} {
		body := do(t, env.H, "GET", path, nil, nil).Body.String()
		assertContains(t, body, "&lt;b&gt;bold&lt;/b&gt;")
		assertNotContains(t, body, "<b>bold</b>")
	}
}
