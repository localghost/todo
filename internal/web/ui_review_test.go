package web_test

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"testing"
)

// The UI review (canvas page "2 · Main list", board "UI review of the web
// app"): points 4, 6, 7 and 9. Points 1 to 3 are browser behavior (app.js,
// app.css).

// 6. A list without items has no counter and no buttons. With a label
// filter, the toolbar stays, because it shows the filter.
func TestToolbarHiddenOnEmptyList(t *testing.T) {
	env := newTestEnv(t)
	body := do(t, env.H, "GET", "/", nil, nil).Body.String()
	assertContains(t, body, `<div id="toolbar" class="toolbar" hidden`, "No items yet")

	// The first item brings the toolbar back (OOB answer).
	rec := do(t, env.H, "POST", "/items", url.Values{"text": {"Buy milk"}}, htmxHeaders)
	assertContains(t, rec.Body.String(), `<div id="toolbar" class="toolbar" hx-swap-oob="true"`)
	assertNotContains(t, rec.Body.String(), `class="toolbar" hidden`, `hx-swap-oob="true" hidden`)

	// Deleting the last item hides it again.
	rec = do(t, env.H, "DELETE", "/items/1", nil, htmxHeaders)
	assertContains(t, rec.Body.String(), `<div id="toolbar" class="toolbar" hx-swap-oob="true" hidden`)

	// A label filter with no items keeps the toolbar.
	l, err := env.Svc.CreateLabel(context.Background(), env.User.ID, "work", "#2555da")
	if err != nil {
		t.Fatal(err)
	}
	body = do(t, env.H, "GET", "/?label="+strconv.FormatInt(l.ID, 10), nil, nil).Body.String()
	assertContains(t, body, `<div id="toolbar" class="toolbar"`, "No items with this label")
	assertNotContains(t, body, `class="toolbar" hidden`)
}

// 4. On a phone the header has no "Signed in as" and no "Log out" (CSS hides
// these two parts; "Log out" is on the Account page).
func TestHeaderPartsForPhone(t *testing.T) {
	env := newTestEnv(t)
	body := do(t, env.H, "GET", "/", nil, nil).Body.String()
	assertContains(t, body,
		`<span class="nav-who">Signed in as <strong>tester</strong> ·</span>`,
		`<span class="nav-logout">· <form method="post" action="/logout" class="inline-form">`)
	css := do(t, env.H, "GET", "/static/app.css", nil, anon).Body.String()
	assertContains(t, css, ".nav-who, .nav-logout { display: none; }")
	account := do(t, env.H, "GET", "/account", nil, nil).Body.String()
	assertContains(t, account, `<button type="submit" class="link-button">Log out</button>`)
}

// 7. An address that does not exist shows a page in the app style with a
// way back. htmx requests and the API keep their short answers.
func TestNotFoundPage(t *testing.T) {
	env := newTestEnv(t)
	for _, path := range []string{"/no-such-page", "/items/5/nothing"} {
		rec := do(t, env.H, "GET", path, nil, anon)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("GET %s: %d, want 404", path, rec.Code)
		}
		assertContains(t, rec.Body.String(), `<link rel="stylesheet" href="/static/app.css">`,
			"<title>Page not found – TooDoo</title>", "<h1>Page not found</h1>",
			"This address does not exist.", `<a href="/">← Back to my list</a>`)
	}
	rec := do(t, env.H, "GET", "/no-such-page", nil, htmxHeaders)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("htmx GET: %d, want 404", rec.Code)
	}
	assertNotContains(t, rec.Body.String(), "<h1>Page not found</h1>")
	rec = do(t, env.H, "GET", "/api/v1/nothing", nil, anon)
	assertNotContains(t, rec.Body.String(), "<h1>Page not found</h1>")
}

// 9. Every password field has the Show button: log in, sign up, account.
func TestShowButtonOnEveryPasswordField(t *testing.T) {
	env := newTestEnv(t)
	assertContains(t, do(t, env.H, "GET", "/login", nil, anon).Body.String(),
		`<button type="button" class="show-password" data-target="login-password">Show</button>`)
	assertContains(t, do(t, env.H, "GET", "/account", nil, nil).Body.String(),
		`<button type="button" class="show-password" data-target="current-password">Show</button>`,
		`<button type="button" class="show-password" data-target="new-password">Show</button>`)
}
