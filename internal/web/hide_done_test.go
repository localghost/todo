package web_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	rec = do(t, env.H, "GET", "/", nil, cookie(other))
	if rec.Code != http.StatusOK {
		t.Fatalf("second session: %d, want 200", rec.Code)
	}
	assertContains(t, rec.Body.String(), "Buy milk", "Show done (1)")
	assertNotContains(t, rec.Body.String(), "Call the dentist")

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
	recs := map[string]*httptest.ResponseRecorder{
		"page":     do(t, env.H, "GET", "/", nil, nil),
		"fragment": do(t, env.H, "GET", "/", nil, htmxHeaders),
		"add":      do(t, env.H, "POST", "/items", url.Values{"text": {"Tea"}}, htmxHeaders),
		"edit":     do(t, env.H, "GET", "/items/1/edit", nil, htmxHeaders),
	}
	for name, rec := range recs {
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status %d, want 200", name, rec.Code)
		}
		body := rec.Body.String()
		assertNotContains(t, body, `id="hide-done"`, "#hide-done", "?hide_done", `name="hide_done"`)
		if t.Failed() {
			t.Fatalf("%s still carries the hide state", name)
		}
	}
}

// The list section and the toolbar name the hide state they were made with, so
// app.js can see when another tab changed it.
func TestHideStateMarkersInListAndToolbar(t *testing.T) {
	env := newTestEnv(t)
	mustAdd(t, env.Svc, "Buy milk")
	assertContains(t, do(t, env.H, "GET", "/", nil, nil).Body.String(),
		`<section id="list-section" data-hide-done="false">`, `id="toolbar" class="toolbar" data-hide-done="false"`)
	env.Auth.SetHideDone(context.Background(), env.User.ID, true) // another tab hides done items
	rec := do(t, env.H, "POST", "/items/1/toggle", url.Values{}, htmxHeaders)
	assertContains(t, rec.Body.String(), `id="toolbar" class="toolbar" hx-swap-oob="true" data-hide-done="true"`)
}

const hideStateHarness = `
const src = require("fs").readFileSync(process.argv[2], "utf8");
let ajax = 0, toolbarState = "false", sectionState = "false";
const handlers = {};
const document = {
  addEventListener: (name, fn) => (handlers[name] = handlers[name] || []).push(fn),
  getElementById: (id) => id === "toolbar" ? { dataset: { hideDone: toolbarState } }
    : id === "list-section" ? { dataset: { hideDone: sectionState } } : null,
  querySelector: () => null, activeElement: null, body: {},
};
const fire = (name) => (handlers[name] || []).forEach((fn) => fn({ detail: {} }));
new Function("document", "window", "navigator", "fetch", "setInterval", "setTimeout", "clearTimeout", "htmx",
  src)(document, {}, {}, () => new Promise(() => {}), () => 0, () => 0, () => {}, { ajax: () => { ajax++; } });
fire("htmx:oobAfterSwap"); console.log(JSON.stringify({ name: "same state", ajax }));
toolbarState = "true"; fire("htmx:oobAfterSwap"); console.log(JSON.stringify({ name: "other tab hid done", ajax }));
`

func TestAppJSReloadsListWhenHideStateDiffers(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	harness := filepath.Join(t.TempDir(), "harness.js")
	if err := os.WriteFile(harness, []byte(hideStateHarness), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(node, harness, "static/app.js").CombinedOutput()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, out)
	}
	want := `{"name":"same state","ajax":0}` + "\n" + `{"name":"other tab hid done","ajax":1}`
	if got := strings.TrimSpace(string(out)); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}
