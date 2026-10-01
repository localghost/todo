package web_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"
)

func TestEditShowsInput(t *testing.T) {
	h, s := newTestApp(t)
	mustAdd(t, s, "Buy milk")
	rec := do(t, h, "GET", "/items/1/edit", nil, htmxHeaders)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	assertContains(t, rec.Body.String(),
		`class="item editing"`, `name="text" value="Buy milk"`, `hx-put="/items/1"`,
		`hx-trigger="keydown[key=='Enter'], blur[target.value.trim() != '']"`, `hx-sync="closest li:drop"`,
		"Enter to save · Esc to cancel")
}

func TestUpdateSavesText(t *testing.T) {
	h, s := newTestApp(t)
	mustAdd(t, s, "Buy milk")
	rec := do(t, h, "PUT", "/items/1", url.Values{"text": {" Buy  oat milk "}}, htmxHeaders)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	assertContains(t, body, `<li id="item-1" class="item">`, ">Buy oat milk</button>")
	assertNotContains(t, body, `name="text"`)
	got, _ := s.Get(context.Background(), 1, 1)
	if got.Text != "Buy oat milk" {
		t.Fatalf("stored text = %q, want %q", got.Text, "Buy oat milk")
	}
}

func TestUpdateEmptyText(t *testing.T) {
	h, s := newTestApp(t)
	mustAdd(t, s, "Buy milk")
	rec := do(t, h, "PUT", "/items/1", url.Values{"text": {"  "}}, htmxHeaders)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	assertContains(t, rec.Body.String(), `class="item editing"`, "Text cannot be empty.", `aria-invalid="true"`)
	got, _ := s.Get(context.Background(), 1, 1)
	if got.Text != "Buy milk" {
		t.Fatalf("stored text = %q, want unchanged", got.Text)
	}
}

func TestShowItemCancelsEdit(t *testing.T) {
	h, s := newTestApp(t)
	mustAdd(t, s, "Buy milk")
	rec := do(t, h, "GET", "/items/1", nil, htmxHeaders)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	assertContains(t, rec.Body.String(), `<li id="item-1" class="item">`, ">Buy milk</button>")
}

func TestEditRoutesUnknownItem(t *testing.T) {
	h, _ := newTestApp(t)
	cases := []struct {
		method, path string
		form         url.Values
	}{
		{"GET", "/items/99/edit", nil},
		{"GET", "/items/99", nil},
		{"PUT", "/items/99", url.Values{"text": {"x"}}},
		{"GET", "/items/abc/edit", nil},
	}
	for _, c := range cases {
		rec := do(t, h, c.method, c.path, c.form, htmxHeaders)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s %s status = %d, want 404", c.method, c.path, rec.Code)
		}
		assertNotContains(t, rec.Body.String(), "<li")
	}
}
