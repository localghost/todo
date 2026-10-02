package web_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"
)

// The hide state is saved per user and applies on every page load, in every
// session of that user, and never for another user.
func TestHideDoneIsSavedPerUser(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	mustAdd(t, env.Svc, "Buy milk")
	b := mustAdd(t, env.Svc, "Call the dentist")
	env.Svc.Toggle(ctx, env.User.ID, b.ID)

	rec := do(t, env.H, "POST", "/settings/hide-done", url.Values{"hide_done": {"1"}}, htmxHeaders)
	if rec.Code != http.StatusOK {
		t.Fatalf("save: %d", rec.Code)
	}
	assertContains(t, rec.Body.String(), `id="list-section"`, "Show done (1)", `hx-vals='{"hide_done":"0"}'`)
	assertNotContains(t, rec.Body.String(), "Call the dentist")

	body := do(t, env.H, "GET", "/", nil, nil).Body.String() // no query
	assertContains(t, body, "Buy milk", "Show done (1)", `aria-pressed="true"`)
	assertNotContains(t, body, "Call the dentist")

	other, _, err := env.Auth.StartSession(ctx, env.User.ID, false) // another device
	if err != nil {
		t.Fatal(err)
	}
	assertNotContains(t, do(t, env.H, "GET", "/", nil, cookie(other)).Body.String(), "Call the dentist")

	bob, _ := env.Auth.SignUp(ctx, "bob", pw)
	bobToken, _, _ := env.Auth.LogIn(ctx, "bob", pw, false)
	done, _ := env.Svc.Add(ctx, bob.ID, "Bobs done item", "", "")
	env.Svc.Toggle(ctx, bob.ID, done.ID)
	assertContains(t, do(t, env.H, "GET", "/", nil, cookie(bobToken)).Body.String(), "Bobs done item", "Hide done (1)")

	rec = do(t, env.H, "POST", "/settings/hide-done", url.Values{"hide_done": {"0"}}, htmxHeaders)
	assertContains(t, rec.Body.String(), "Call the dentist", "Hide done (1)", `hx-vals='{"hide_done":"1"}'`)
}

func TestHideDoneWithoutHTMXRedirects(t *testing.T) {
	env := newTestEnv(t)
	rec := do(t, env.H, "POST", "/settings/hide-done", url.Values{"hide_done": {"1"}}, nil)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("got %d %q, want 303 /", rec.Code, rec.Header().Get("Location"))
	}
}

// No page or fragment carries the hide state any more.
func TestNoHideStateInPages(t *testing.T) {
	env := newTestEnv(t)
	mustAdd(t, env.Svc, "Buy milk")
	env.Auth.SetHideDone(context.Background(), env.User.ID, true)
	bodies := map[string]string{
		"page":     do(t, env.H, "GET", "/", nil, nil).Body.String(),
		"fragment": do(t, env.H, "GET", "/", nil, htmxHeaders).Body.String(),
		"add":      do(t, env.H, "POST", "/items", url.Values{"text": {"Tea"}}, htmxHeaders).Body.String(),
		"edit":     do(t, env.H, "GET", "/items/1/edit", nil, htmxHeaders).Body.String(),
	}
	for name, body := range bodies {
		assertNotContains(t, body, `id="hide-done"`, "#hide-done", "?hide_done", `name="hide_done"`)
		if t.Failed() {
			t.Fatalf("%s still carries the hide state", name)
		}
	}
}
