package web_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"todo/internal/todo"
)

// Notes on an item (canvas page "21 · Notes", option N2).

// A row without notes has a gray notes icon; a row with notes has a
// "notes" chip that holds the notes for the search.
func TestRowNotesButton(t *testing.T) {
	env := newTestEnv(t)
	mustAdd(t, env.Svc, "Pay the rent")
	mustAdd(t, env.Svc, "Visit grandma")
	if _, err := env.Svc.SetNotes(context.Background(), env.User.ID, 2, "ul. Lipowa 12/4\ntel. 600 100 200"); err != nil {
		t.Fatal(err)
	}
	body := do(t, env.H, "GET", "/", nil, nil).Body.String()
	assertContains(t, body,
		`<span class="notes-add-wrap"><button type="button" id="notes-add-1" class="notes-add" aria-label="Add notes to “Pay the rent”" title="Add notes" aria-haspopup="dialog" aria-expanded="false" data-picker="/items/1/notes">`,
		`<button type="button" id="notes-add-2" class="notes-add is-set" aria-label="Show the notes of “Visit grandma”" title="Show the notes" aria-haspopup="dialog" aria-expanded="false" data-picker="/items/2/notes" data-notes="ul. Lipowa 12/4
tel. 600 100 200">`,
		`</svg>notes</button></span>`)
	// The notes button comes after the date chip and before the labels.
	row := body[strings.Index(body, `id="item-2"`):]
	if !strings.Contains(row, "notes-add-wrap") || strings.Index(row, "due-add-wrap") > strings.Index(row, "notes-add-wrap") || strings.Index(row, "notes-add-wrap") > strings.Index(row, "label-add-wrap") {
		t.Fatalf("order of the row's buttons:\n%s", row[:strings.Index(row, "</li>")])
	}
}

// An item without notes opens the pop-up with the field; an item with notes
// opens it with the notes and "Edit", and the field is hidden.
func TestNotesPicker(t *testing.T) {
	env := newTestEnv(t)
	mustAdd(t, env.Svc, "Visit grandma")
	body := do(t, env.H, "GET", "/items/1/notes", nil, htmxHeaders).Body.String()
	assertContains(t, body,
		`<form class="picker notes-picker" id="notes-picker-1" role="dialog" aria-label="Notes of “Visit grandma”"`,
		`hx-put="/items/1/notes" hx-target="#item-1" hx-swap="outerHTML">`,
		`<div class="notes-form">`, `<textarea name="notes" id="notes-1" rows="6"`, `</textarea>`,
		`<p class="hint">Ctrl+Enter saves · Esc closes</p>`, `<button type="submit" class="due-save">Save</button>`)
	assertNotContains(t, body, `class="notes-view"`, `class="notes-edit"`)

	env.Svc.SetNotes(context.Background(), env.User.ID, 1, "Code: 4512\nMap: https://maps.example.com/abc?x=1&y=2.")
	body = do(t, env.H, "GET", "/items/1/notes", nil, htmxHeaders).Body.String()
	assertContains(t, body,
		`<div class="notes-view">`,
		`<div class="notes-text">Code: 4512
Map: <a href="https://maps.example.com/abc?x=1&amp;y=2" target="_blank" rel="noopener noreferrer nofollow">https://maps.example.com/abc?x=1&amp;y=2</a>.</div>`,
		`<button type="button" class="notes-edit">Edit</button>`,
		`<div class="notes-form" hidden>`,
		">Code: 4512\nMap: https://maps.example.com/abc?x=1&amp;y=2.</textarea>")
}

// Only http and https links become links; all other text is escaped.
func TestNotesLinksAreSafe(t *testing.T) {
	env := newTestEnv(t)
	mustAdd(t, env.Svc, "Read")
	env.Svc.SetNotes(context.Background(), env.User.ID, 1,
		`<script>alert(1)</script> javascript:alert(1) (see https://en.example.org/wiki/Kettle_(device)) "https://a.example/"><b>x</b>`)
	body := do(t, env.H, "GET", "/items/1/notes", nil, htmxHeaders).Body.String()
	at := strings.Index(body, `<div class="notes-text">`)
	if at < 0 {
		t.Fatalf("no notes in the pop-up:\n%s", body)
	}
	view := body[at:]
	view = view[:strings.Index(view, "</div>")]
	if strings.Count(view, "<a ") != 2 {
		t.Fatalf("want 2 links:\n%s", view)
	}
	assertContains(t, view,
		`&lt;script&gt;alert(1)&lt;/script&gt; javascript:alert(1) (see `,
		`<a href="https://en.example.org/wiki/Kettle_(device)" target="_blank" rel="noopener noreferrer nofollow">https://en.example.org/wiki/Kettle_(device)</a>)`,
		`&#34;<a href="https://a.example/" target="_blank" rel="noopener noreferrer nofollow">https://a.example/</a>&#34;&gt;&lt;b&gt;x&lt;/b&gt;`)
	assertNotContains(t, view, "<script", "<b>")
}

// Save answers with the row; the notes chip shows. Empty notes remove it.
func TestSaveNotes(t *testing.T) {
	env := newTestEnv(t)
	mustAdd(t, env.Svc, "Visit grandma")
	rec := do(t, env.H, "PUT", "/items/1/notes", url.Values{"notes": {"  ul. Lipowa 12/4\r\ntel. 600 100 200  "}}, htmxHeaders)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT notes: %d\n%s", rec.Code, rec.Body.String())
	}
	assertContains(t, rec.Body.String(), `<li id="item-1" class="item">`, `class="notes-add is-set"`, `data-notes="ul. Lipowa 12/4
tel. 600 100 200"`)
	if it, _ := env.Svc.Get(context.Background(), env.User.ID, 1); it.Notes != "ul. Lipowa 12/4\ntel. 600 100 200" {
		t.Fatalf("notes = %q", it.Notes)
	}
	rec = do(t, env.H, "PUT", "/items/1/notes", url.Values{"notes": {" "}}, htmxHeaders)
	assertContains(t, rec.Body.String(), `class="notes-add"`)
	assertNotContains(t, rec.Body.String(), "is-set", "data-notes")
}

// Notes that are too long come back in the pop-up with a message; the item
// keeps its old notes.
func TestSaveNotesTooLong(t *testing.T) {
	env := newTestEnv(t)
	mustAdd(t, env.Svc, "Visit grandma")
	env.Svc.SetNotes(context.Background(), env.User.ID, 1, "old")
	long := strings.Repeat("a", todo.MaxNotesChars+1)
	rec := do(t, env.H, "PUT", "/items/1/notes", url.Values{"notes": {long}}, htmxHeaders)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("PUT long notes: %d, want 422", rec.Code)
	}
	if rec.Header().Get("HX-Retarget") != "#notes-picker-1" || rec.Header().Get("HX-Reswap") != "outerHTML" {
		t.Fatalf("retarget = %q %q", rec.Header().Get("HX-Retarget"), rec.Header().Get("HX-Reswap"))
	}
	assertContains(t, rec.Body.String(), `<div class="notes-form">`, `aria-invalid="true" aria-describedby="notes-error-1"`,
		`<p class="hint error" id="notes-error-1" role="alert">The notes are too long: at most 2000 characters.</p>`, ">"+long+"</textarea>")
	if it, _ := env.Svc.Get(context.Background(), env.User.ID, 1); it.Notes != "old" {
		t.Fatalf("notes = %q, want old", it.Notes)
	}
	if rec := do(t, env.H, "PUT", "/items/9/notes", url.Values{"notes": {"x"}}, htmxHeaders); rec.Code != http.StatusNotFound {
		t.Fatalf("PUT notes of a missing item: %d, want 404", rec.Code)
	}
	if rec := do(t, env.H, "GET", "/items/9/notes", nil, htmxHeaders); rec.Code != http.StatusNotFound {
		t.Fatalf("GET notes of a missing item: %d, want 404", rec.Code)
	}
}

// The API has "notes" on the item; PATCH changes them, and without "notes"
// the item keeps them.
func TestAPINotes(t *testing.T) {
	env := newTestEnv(t)
	mustAdd(t, env.Svc, "Visit grandma")
	body := apiDo(t, env, "GET", "/api/v1/items/1", env.Token, "").Body.String()
	assertContains(t, body, `"notes":""`)
	body = apiDo(t, env, "PATCH", "/api/v1/items/1", env.Token, `{"notes":"line 1\nline 2"}`).Body.String()
	var got struct {
		Notes string `json:"notes"`
	}
	if err := json.Unmarshal([]byte(body), &got); err != nil || got.Notes != "line 1\nline 2" {
		t.Fatalf("PATCH notes: %s (%v)", body, err)
	}
	body = apiDo(t, env, "PATCH", "/api/v1/items/1", env.Token, `{"text":"Visit grandma on Sunday"}`).Body.String()
	assertContains(t, body, `"notes":"line 1\nline 2"`)
	rec := apiDo(t, env, "PATCH", "/api/v1/items/1", env.Token, `{"notes":"`+strings.Repeat("a", todo.MaxNotesChars+1)+`"}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("PATCH long notes: %d, want 422", rec.Code)
	}
	assertContains(t, rec.Body.String(), `"error":"bad_notes"`, `"message":"The notes are too long: at most 2000 characters."`)
}
