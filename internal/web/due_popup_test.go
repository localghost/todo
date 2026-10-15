package web_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestDuePopupShowsFields(t *testing.T) {
	h, svc := newTestApp(t)
	ctx := context.Background()
	svc.Add(ctx, 1, "Timed", "2026-10-03", "14:00")
	mustAdd(t, svc, "No date")

	timed := do(t, h, "GET", "/items/1/due", nil, htmxHeaders)
	if timed.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", timed.Code)
	}
	assertContains(t, timed.Body.String(),
		`<form class="picker due-picker" id="due-picker-1" role="dialog"`,
		`hx-put="/items/1/due" hx-target="#item-1" hx-swap="outerHTML"`,
		`<p class="due-picker-title">Due date</p>`,
		`name="due_date" value="2026-10-03"`, `name="due_time" value="14:00"`,
		`<input type="hidden" name="due_tz" value="`,
		`<p class="hint">Leave the time empty for “all day”</p>`,
		`<button type="submit" class="due-save">Save</button>`,
		`<button type="button" class="due-remove" hx-put="/items/1/due" hx-params="none">Remove date</button>`)
	empty := do(t, h, "GET", "/items/2/due", nil, htmxHeaders).Body.String()
	assertContains(t, empty, `name="due_date" value=""`, `name="due_time" value=""`)
	assertNotContains(t, empty, "Remove date", "novalidate")
}

func TestDuePopupSaves(t *testing.T) {
	h, svc := newTestApp(t)
	mustAdd(t, svc, "Call the dentist")
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	rec := do(t, h, "PUT", "/items/1/due", url.Values{"due_date": {tomorrow}, "due_time": {"14:00"}}, htmxHeaders)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	assertContains(t, rec.Body.String(), `<li id="item-1" class="item">`, "Due tomorrow, 14:00",
		`<div id="notify-bar" class="notify-bar" data-has-due="true" hx-swap-oob="true"></div>`)
	assertNotContains(t, rec.Body.String(), "due-picker-1")
}

func TestDuePopupRemoves(t *testing.T) {
	h, svc := newTestApp(t)
	svc.Add(context.Background(), 1, "Call the dentist", "2099-10-03", "14:00")
	rec := do(t, h, "PUT", "/items/1/due", url.Values{}, htmxHeaders)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got, _ := svc.Get(context.Background(), 1, 1); got.DueAt != nil {
		t.Fatalf("due = %v, want none", got.DueAt)
	}
}

func TestDuePopupBadValue(t *testing.T) {
	h, svc := newTestApp(t)
	svc.Add(context.Background(), 1, "Call the dentist", "2099-10-03", "14:00")
	rec := do(t, h, "PUT", "/items/1/due", url.Values{"due_date": {""}, "due_time": {"10:00"}}, htmxHeaders)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	if rec.Header().Get("HX-Retarget") != "#due-picker-1" || rec.Header().Get("HX-Reswap") != "outerHTML" {
		t.Fatalf("headers = %v, want HX-Retarget #due-picker-1 and outerHTML", rec.Header())
	}
	body := rec.Body.String()
	assertContains(t, body, `id="due-picker-1"`,
		`name="due_date" value="" aria-label="Due date" aria-invalid="true" aria-describedby="due-error-1"`,
		`name="due_time" value="10:00"`,
		`<p class="hint error" id="due-error-1" role="alert">Pick a day for the due date.</p>`,
		"Remove date")
	assertNotContains(t, body, "<li")
	if got, _ := svc.Get(context.Background(), 1, 1); got.DueAt == nil {
		t.Fatal("the due date was removed after a bad value")
	}
}

func TestDuePopupUnknownItem(t *testing.T) {
	h, _ := newTestApp(t)
	for _, c := range []struct{ method, path string }{
		{"GET", "/items/99/due"}, {"PUT", "/items/99/due"}, {"PUT", "/items/abc/due"},
	} {
		rec := do(t, h, c.method, c.path, url.Values{"due_date": {"2099-01-01"}}, htmxHeaders)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s %s status = %d, want 404", c.method, c.path, rec.Code)
		}
		if strings.Contains(rec.Body.String(), "<form") {
			t.Errorf("%s %s answered a pop-up", c.method, c.path)
		}
	}
}
