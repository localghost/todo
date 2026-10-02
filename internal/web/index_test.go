package web_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestIndexEmpty(t *testing.T) {
	h, _ := newTestApp(t)
	rec := do(t, h, "GET", "/", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	assertContains(t, rec.Body.String(),
		"<!doctype html>", `id="add-form"`, "What needs to be done?",
		"0 open · 0 done", "Hide done (0)", "No items yet", "Type a task above and press Enter.",
		`/static/htmx.min.js`, `name="htmx-config"`)
}

func TestIndexListsItemsInOrder(t *testing.T) {
	h, svc := newTestApp(t)
	mustAdd(t, svc, "Buy milk")
	b := mustAdd(t, svc, "Call the dentist")
	svc.Toggle(context.Background(), 1, b.ID)

	body := do(t, h, "GET", "/", nil, nil).Body.String()
	assertContains(t, body, `id="item-1"`, `id="item-2" class="item done"`,
		"1 open · 1 done", "Hide done (1)")
	assertNotContains(t, body, "No items yet")
	if i1, i2 := strings.Index(body, "Buy milk"), strings.Index(body, "Call the dentist"); i1 > i2 {
		t.Fatalf("items out of order: Buy milk at %d, Call the dentist at %d", i1, i2)
	}
}

func TestIndexHideDone(t *testing.T) {
	h, svc := newTestApp(t)
	mustAdd(t, svc, "Buy milk")
	b := mustAdd(t, svc, "Call the dentist")
	svc.Toggle(context.Background(), 1, b.ID)

	body := do(t, h, "GET", "/?hide_done=1", nil, nil).Body.String()
	assertContains(t, body, "Buy milk", "Show done (1)", `aria-pressed="true"`,
		`id="hide-done" name="hide_done" value="1"`)
	assertNotContains(t, body, "Call the dentist")
}

func TestIndexAllDoneHidden(t *testing.T) {
	h, svc := newTestApp(t)
	a := mustAdd(t, svc, "Buy milk")
	svc.Toggle(context.Background(), 1, a.ID)

	body := do(t, h, "GET", "/?hide_done=1", nil, nil).Body.String()
	assertContains(t, body, "All done", "Done items are hidden.")
	assertNotContains(t, body, "No items yet")
}

func TestIndexHTMXReturnsFragment(t *testing.T) {
	h, _ := newTestApp(t)
	rec := do(t, h, "GET", "/?hide_done=1", nil, htmxHeaders)
	body := rec.Body.String()
	assertContains(t, body, `id="list-section"`, `id="toolbar"`)
	assertNotContains(t, body, "<!doctype html>", `id="add-form"`)
	if v := rec.Header().Values("Vary"); len(v) == 0 {
		t.Errorf("missing Vary header")
	}
}

func TestIndexHistoryRestoreReturnsFullPage(t *testing.T) {
	h, _ := newTestApp(t)
	body := do(t, h, "GET", "/", nil, map[string]string{
		"HX-Request": "true", "HX-History-Restore-Request": "true",
	}).Body.String()
	assertContains(t, body, "<!doctype html>")
}

func TestIndexEscapesText(t *testing.T) {
	h, svc := newTestApp(t)
	mustAdd(t, svc, "<script>alert(1)</script>")
	body := do(t, h, "GET", "/", nil, nil).Body.String()
	assertContains(t, body, "&lt;script&gt;alert(1)&lt;/script&gt;")
	assertNotContains(t, body, "<script>alert(1)</script>")
}

func TestStaticFiles(t *testing.T) {
	h, _ := newTestApp(t)
	for _, p := range []string{"/static/htmx.min.js", "/static/app.css"} {
		if rec := do(t, h, "GET", p, nil, nil); rec.Code != http.StatusOK {
			t.Errorf("GET %s status = %d, want 200", p, rec.Code)
		}
	}
}

func TestUnknownPathIs404(t *testing.T) {
	h, _ := newTestApp(t)
	if rec := do(t, h, "GET", "/nope", nil, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestStaticDirectoryListingIs404(t *testing.T) {
	h, _ := newTestApp(t)
	if rec := do(t, h, "GET", "/static/", nil, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("GET /static/ status = %d, want 404", rec.Code)
	}
}

// The content column is 40% wider than the first design (560 px → 784 px).
func TestContentColumnWidth(t *testing.T) {
	h, _ := newTestApp(t)
	body := do(t, h, "GET", "/static/app.css", nil, nil).Body.String()
	// The list column (main) is 784 px; other pages, like the account page, have their own width.
	assertContains(t, body, "main {\n  max-width: 784px;")
}

// Each item shows when it was added, also when done; edit mode hides it.
func TestItemShowsAddedDate(t *testing.T) {
	h, svc := newTestApp(t)
	mustAdd(t, svc, "Buy milk")
	b := mustAdd(t, svc, "Call the dentist")
	svc.Toggle(context.Background(), 1, b.ID)

	body := do(t, h, "GET", "/", nil, nil).Body.String()
	if n := strings.Count(body, `<span class="added">Added today, `); n != 2 {
		t.Fatalf("found %d added-date labels, want 2 (open and done item)\nbody:\n%s", n, body)
	}
	edit := do(t, h, "GET", "/items/1/edit", nil, htmxHeaders).Body.String()
	assertNotContains(t, edit, `class="added"`)
}

// The due label follows the added date; overdue open items are marked.
func TestItemShowsDueLabel(t *testing.T) {
	h, svc := newTestApp(t)
	ctx := context.Background()
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	svc.Add(ctx, 1, "Water the plants", tomorrow, "14:00")
	svc.Add(ctx, 1, "Call the dentist", "2020-01-01", "08:00")
	done, _ := svc.Add(ctx, 1, "Buy milk", "2020-01-01", "")
	svc.Toggle(ctx, 1, done.ID)

	body := do(t, h, "GET", "/", nil, nil).Body.String()
	assertContains(t, body,
		` · <span class="due">Due tomorrow, 14:00</span>`,
		` · <span class="due overdue">Overdue since 1 Jan 2020, 08:00</span>`,
		` · <span class="due">Due 1 Jan 2020</span>`)
}

// Only the text field of the add form stretches; the due fields keep their natural width.
func TestAddFormOnlyTextFieldStretches(t *testing.T) {
	h, _ := newTestApp(t)
	css := do(t, h, "GET", "/static/app.css", nil, nil).Body.String()
	assertContains(t, css, ".add-row input {")
	assertNotContains(t, css, ".add input {")
}
