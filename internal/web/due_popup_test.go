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
		`<div class="due-actions"><button type="submit" class="due-save">Save</button></div>`)
	// The × in the date chip removes a date (canvas page 10, option D).
	assertNotContains(t, timed.Body.String(), "Remove date")
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
		`<p class="hint error" id="due-error-1" role="alert">Pick a day for the due date.</p>`)
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

// The gray line is: the date chip, the labels, the "Label" chip (canvas page
// 10, "due date and labels of a row as chips", option D).
func TestRowHasDueChipAndLabelChip(t *testing.T) {
	h, svc := newTestApp(t)
	ctx := context.Background()
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	svc.Add(ctx, 1, "Call the dentist", tomorrow, "14:00")
	svc.AddLabel(ctx, 1, 1, "work", "")
	mustAdd(t, svc, "No date")
	body := do(t, h, "GET", "/", nil, nil).Body.String()
	assertContains(t, body,
		`<span class="meta"><span class="due-add-wrap due-chip is-set"><button type="button" id="due-add-1" class="due-add" aria-label="Due tomorrow, 14:00. Change the due date of “Call the dentist”" title="Change the due date" aria-haspopup="dialog" aria-expanded="false" data-picker="/items/1/due"><svg width="12" height="12"`,
		`</svg>Tomorrow, 14:00</button><button type="button" class="due-x" aria-label="Remove the due date of “Call the dentist”" title="Remove the due date"`,
		`hx-put="/items/1/due" hx-params="none" hx-target="closest li" hx-swap="outerHTML">`,
		`<span class="meta"><span class="due-add-wrap due-chip"><button type="button" id="due-add-2" class="due-add" aria-label="Set the due date of “No date”"`,
		`</svg>Due date</button></span>`,
		`id="label-add-2" class="label-add"`, `</svg>Label</button>`)
	assertNotContains(t, body, `class="due"`, `data-due-for=`)
	if i, j := strings.Index(body, `id="due-add-1"`), strings.Index(body, `class="chip" data-color=`); i < 0 || j < i {
		t.Fatalf("the date chip must come before the labels (date at %d, label at %d)", i, j)
	}
}

func TestCalendarIconCSS(t *testing.T) {
	h, _ := newTestApp(t)
	css := do(t, h, "GET", "/static/app.css", nil, nil).Body.String()
	assertContains(t, css,
		".label-add, .due-add {", ".due-chip.is-set {", ".due-chip.is-over {", ".due-x {",
		".due-picker {", ".due-actions {", ".due-save {", ".due-remove {")
}

// The "Due date" and "Label" chips show on every row, also with no pointer
// on it; the pointer on the row only makes them blue (option D). Only the ×
// waits for the pointer.
func TestRowChipsAlwaysShow(t *testing.T) {
	h, _ := newTestApp(t)
	css := do(t, h, "GET", "/static/app.css", nil, nil).Body.String()
	i := strings.Index(css, ".label-add, .due-add {")
	if i < 0 {
		t.Fatal("no .label-add, .due-add rule")
	}
	if rule := css[i : i+strings.Index(css[i:], "}")]; strings.Contains(rule, "opacity") {
		t.Fatalf("the icons must not be hidden: %s", rule)
	}
	assertContains(t, css, ".chip:hover .chip-x, .chip:focus-within .chip-x {",
		".due-chip:hover .due-x, .due-chip:focus-within .due-x {", ".item:hover .label-add, .item:focus-within .label-add,")
}

// A touch screen has no hover: the chips are blue and taller, and the ×
// always shows.
func TestRowChipsOnTouch(t *testing.T) {
	h, _ := newTestApp(t)
	css := do(t, h, "GET", "/static/app.css", nil, nil).Body.String()
	assertContains(t, css,
		".label-add, .due-add, .due-chip.is-set { height: 28px; }",
		".due-x, .due-chip .due-x { width: 22px; height: 22px; margin: 0 3px 0 -4px; opacity: 1; }")
}

// One pop-up code serves the label list and the due date pop-up.
func TestAppJSOpensTheDuePopup(t *testing.T) {
	h, _ := newTestApp(t)
	js := do(t, h, "GET", "/static/app.js", nil, nil).Body.String()
	assertContains(t, js,
		`closest(".label-add, .due-add, .draft-label, .draft-due")`, `classList.contains("due-picker")`,
		`focusAfterSwap = "due-add-"`, `input:not([type="hidden"])`)
}

// Safari and Firefox on macOS do not focus a button on click: a press on
// Save or Remove date must not take the focus out of the pop-up first, or
// focusout closes it before the click.
func TestAppJSDuePopupButtonsKeepFocus(t *testing.T) {
	h, _ := newTestApp(t)
	js := do(t, h, "GET", "/static/app.js", nil, nil).Body.String()
	assertContains(t, js, `e.target.closest(isDuePicker() ? "input" : ".picker-search")`)
	assertNotContains(t, js, `isDuePicker() ? "input, button"`)
}
