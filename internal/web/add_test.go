package web_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"
)

func TestAddReturnsRowAndOOB(t *testing.T) {
	h, _ := newTestApp(t)
	rec := do(t, h, "POST", "/items", url.Values{"text": {"  Buy milk "}, "hide_done": {""}}, htmxHeaders)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	assertContains(t, body,
		`<li id="item-1" class="item">`, ">Buy milk</button>",
		`id="toolbar" class="toolbar" hx-swap-oob="true"`,
		`id="empty-state" hx-swap-oob="true"`, "1 open · 0 done")
	assertNotContains(t, body, "Please type some text first.", "No items yet")
}

// The success answer must not replace the add form: text typed meanwhile stays.
func TestAddKeepsFormInPage(t *testing.T) {
	h, _ := newTestApp(t)
	body := do(t, h, "POST", "/items", url.Values{"text": {"Buy milk"}}, htmxHeaders).Body.String()
	assertNotContains(t, body, `id="add-form"`)
}

func TestAddEmptyText(t *testing.T) {
	h, svc := newTestApp(t)
	rec := do(t, h, "POST", "/items", url.Values{"text": {"   "}}, htmxHeaders)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	if got := rec.Header().Get("HX-Retarget"); got != "#add-form" {
		t.Errorf("HX-Retarget = %q, want #add-form", got)
	}
	if got := rec.Header().Get("HX-Reswap"); got != "outerHTML" {
		t.Errorf("HX-Reswap = %q, want outerHTML", got)
	}
	assertContains(t, rec.Body.String(), "Please type some text first.", `aria-invalid="true"`)
	open, done, _ := svc.Counts(context.Background(), 1)
	if open+done != 0 {
		t.Fatalf("an item was stored: open=%d done=%d", open, done)
	}
}
