package web_test

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"testing"

	"todo/internal/todo"
)

func TestToggleMarksDone(t *testing.T) {
	h, s := newTestApp(t)
	mustAdd(t, s, "Buy milk")
	rec := do(t, h, "POST", "/items/1/toggle", url.Values{"hide_done": {""}}, htmxHeaders)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	assertContains(t, rec.Body.String(),
		`<li id="item-1" class="item done">`, "Mark not done: Buy milk",
		"0 open · 1 done", "Hide done (1)", `hx-swap-oob="true"`)
}

func TestToggleTwiceMarksNotDone(t *testing.T) {
	h, s := newTestApp(t)
	mustAdd(t, s, "Buy milk")
	do(t, h, "POST", "/items/1/toggle", url.Values{}, htmxHeaders)
	body := do(t, h, "POST", "/items/1/toggle", url.Values{}, htmxHeaders).Body.String()
	assertContains(t, body, `<li id="item-1" class="item">`, "1 open · 0 done")
}

func TestToggleInHideModeRemovesRow(t *testing.T) {
	h, s := newTestApp(t)
	mustAdd(t, s, "Buy milk")
	rec := do(t, h, "POST", "/items/1/toggle", url.Values{"hide_done": {"1"}}, htmxHeaders)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	assertNotContains(t, body, `<li id="item-1"`)
	assertContains(t, body, "Show done (1)", "All done")
}

func TestToggleUnknownItem(t *testing.T) {
	h, s := newTestApp(t)
	mustAdd(t, s, "Buy milk")
	rec := do(t, h, "POST", "/items/99/toggle", url.Values{}, htmxHeaders)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	body := rec.Body.String()
	assertNotContains(t, body, "<li")
	assertContains(t, body, "1 open · 0 done", `hx-swap-oob="true"`)
}

func TestToggleBadID(t *testing.T) {
	h, _ := newTestApp(t)
	for _, p := range []string{"/items/abc/toggle", "/items/0/toggle", "/items/-1/toggle"} {
		if rec := do(t, h, "POST", p, url.Values{}, htmxHeaders); rec.Code != http.StatusNotFound {
			t.Errorf("POST %s status = %d, want 404", p, rec.Code)
		}
	}
}

func TestDeleteRemovesItem(t *testing.T) {
	h, s := newTestApp(t)
	mustAdd(t, s, "Buy milk")
	rec := do(t, h, "DELETE", "/items/1?hide_done=", nil, htmxHeaders)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	assertNotContains(t, body, "<li")
	assertContains(t, body, "0 open · 0 done", "No items yet")
	if _, err := s.Get(context.Background(), 1, 1); !errors.Is(err, todo.ErrNotFound) {
		t.Fatalf("Get after delete err = %v, want ErrNotFound", err)
	}
}

func TestDeleteUnknownItem(t *testing.T) {
	h, _ := newTestApp(t)
	if rec := do(t, h, "DELETE", "/items/99", nil, htmxHeaders); rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}
