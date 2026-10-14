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

func TestAddWithLabelsShowsChips(t *testing.T) {
	env := newTestEnv(t)
	rec := do(t, env.H, "POST", "/items", url.Values{"text": {"Invoice"}, "labels": {"work, money"}}, htmxHeaders)
	if rec.Code != http.StatusOK {
		t.Fatalf("add: %d %s", rec.Code, rec.Body)
	}
	items, _ := env.Svc.List(context.Background(), env.User.ID, false)
	work := labelID(t, items[0], "work")
	assertContains(t, rec.Body.String(),
		fmt.Sprintf(`<a class="chip c-%s" href="/?label=%d"`, items[0].Labels[1].Color, work), ">work</a>", ">money</a>",
		`id="add-suggest"`, `data-label="work"`)
}

func TestAddWithTooManyLabelsShowsError(t *testing.T) {
	env := newTestEnv(t)
	rec := do(t, env.H, "POST", "/items", url.Values{"text": {"Trip"}, "labels": {"a, b, c, d, e, f"}}, htmxHeaders)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got %d, want 422", rec.Code)
	}
	assertContains(t, rec.Body.String(), "An item can have at most 5 labels.", `value="a, b, c, d, e, f"`, `aria-invalid="true"`)
	if open, _, _ := env.Svc.Counts(context.Background(), env.User.ID); open != 0 {
		t.Fatalf("item saved despite the error")
	}
}

func TestFilterByLabel(t *testing.T) {
	env := newTestEnv(t)
	a := addLabeled(t, env.Svc, env.User.ID, "Invoice", "work")
	addLabeled(t, env.Svc, env.User.ID, "Water the plants", "home")
	work := labelID(t, a, "work")

	body := do(t, env.H, "GET", fmt.Sprintf("/?label=%d", work), nil, nil).Body.String()
	assertContains(t, body, "Invoice", `class="active-filter"`, `href="/" aria-label="Show all labels"`,
		"1 open · 0 done", `name="labels" value="work" data-start="work"`)
	assertNotContains(t, body, "Water the plants")

	// An unknown or foreign label shows the full list.
	body = do(t, env.H, "GET", "/?label=999", nil, nil).Body.String()
	assertContains(t, body, "Invoice", "Water the plants")
	assertNotContains(t, body, `class="active-filter"`)

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

func TestAddWhileFilteredWithoutTheLabelHidesTheRow(t *testing.T) {
	env := newTestEnv(t)
	a := addLabeled(t, env.Svc, env.User.ID, "Invoice", "work")
	work := labelID(t, a, "work")
	rec := do(t, env.H, "POST", "/items", url.Values{"text": {"Other thing"}, "labels": {"home"}}, filtered(work))
	if rec.Code != http.StatusOK {
		t.Fatalf("add: %d", rec.Code)
	}
	assertNotContains(t, rec.Body.String(), `class="item`)
	assertContains(t, rec.Body.String(), "1 open · 0 done", `id="toolbar"`)

	rec = do(t, env.H, "POST", "/items", url.Values{"text": {"Report"}, "labels": {"work"}}, filtered(work))
	assertContains(t, rec.Body.String(), "Report", "2 open · 0 done")
}

func TestEditLabels(t *testing.T) {
	env := newTestEnv(t)
	a := addLabeled(t, env.Svc, env.User.ID, "Invoice", "work, money")
	addLabeled(t, env.Svc, env.User.ID, "Other", "home")
	path := fmt.Sprintf("/items/%d", a.ID)

	body := do(t, env.H, "GET", path+"/edit", nil, htmxHeaders).Body.String()
	assertContains(t, body, `name="labels" value="money, work"`, `data-label="home"`)

	rec := do(t, env.H, "PUT", path, url.Values{"text": {"Invoice"}, "labels": {"home"}}, htmxHeaders)
	if rec.Code != http.StatusOK {
		t.Fatalf("edit: %d %s", rec.Code, rec.Body)
	}
	assertContains(t, rec.Body.String(), ">home</a>")
	assertNotContains(t, rec.Body.String(), ">work</a>")

	rec = do(t, env.H, "PUT", path, url.Values{"text": {"Invoice"}, "labels": {strings.Repeat("x", 31)}}, htmxHeaders)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("long label: %d, want 422", rec.Code)
	}
	assertContains(t, rec.Body.String(), "A label can have at most 30 characters.", `class="item editing`)
}

func TestEditWhileFilteredRemovingTheLabelRemovesTheRow(t *testing.T) {
	env := newTestEnv(t)
	a := addLabeled(t, env.Svc, env.User.ID, "Invoice", "work")
	work := labelID(t, a, "work")
	rec := do(t, env.H, "PUT", fmt.Sprintf("/items/%d", a.ID), url.Values{"text": {"Invoice"}, "labels": {""}}, filtered(work))
	if rec.Code != http.StatusOK {
		t.Fatalf("edit: %d", rec.Code)
	}
	assertNotContains(t, rec.Body.String(), `id="item-`)
	assertContains(t, rec.Body.String(), "0 open · 0 done")
}

func TestLabelsPage(t *testing.T) {
	env := newTestEnv(t)
	a := addLabeled(t, env.Svc, env.User.ID, "Invoice", "work, home")
	work, home := labelID(t, a, "work"), labelID(t, a, "home")

	body := do(t, env.H, "GET", "/labels", nil, nil).Body.String()
	assertContains(t, body, "<h1>Labels</h1>", ">work</span>", fmt.Sprintf(`href="/?label=%d">1 item</a>`, work),
		fmt.Sprintf(`action="/labels/%d"`, work), `name="color" value="purple"`)
	assertContains(t, do(t, env.H, "GET", "/", nil, nil).Body.String(), `<a href="/labels">Labels</a>`)

	// Rename and recolor.
	rec := do(t, env.H, "POST", fmt.Sprintf("/labels/%d", work), url.Values{"name": {"deep work"}, "color": {"purple"}}, nil)
	if rec.Code != http.StatusSeeOther || !strings.HasPrefix(rec.Header().Get("Location"), fmt.Sprintf("/labels?saved=%d", work)) {
		t.Fatalf("save: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	l, _ := env.Svc.Label(context.Background(), env.User.ID, work)
	if l.Name != "deep work" || l.Color != "purple" {
		t.Fatalf("label = %+v", l)
	}
	assertContains(t, do(t, env.H, "GET", fmt.Sprintf("/labels?saved=%d", work), nil, nil).Body.String(), "Saved.")

	// A name that is taken.
	rec = do(t, env.H, "POST", fmt.Sprintf("/labels/%d", work), url.Values{"name": {"HOME"}, "color": {"red"}}, nil)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("clash: %d, want 422", rec.Code)
	}
	assertContains(t, rec.Body.String(), "You already have a label “HOME”.", `value="HOME"`)

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

	do(t, env.H, "POST", fmt.Sprintf("/labels/%d", id), url.Values{"name": {"mine"}, "color": {"red"}}, nil)
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
