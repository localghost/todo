package web_test

import (
	"io/fs"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"todo/docs"
	"todo/internal/web"
)

// The user guide (canvas page 2, "user guide at /guide") is public: it
// opens with no session, with a contents list of its level-2 headings.
func TestGuideIsPublic(t *testing.T) {
	env := newTestEnv(t, web.WithGuide(docs.Guide))
	rec := do(t, env.H, "GET", "/guide", nil, anon)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	assertContains(t, body, "<title>User guide – TooDoo</title>", `<h1 id="user-guide">User guide</h1>`,
		`<a href="/">Open the app →</a>`, `<p id="guide-toc-title">Contents</p>`)
	for _, s := range []struct{ id, title string }{
		{"get-started", "Get started"}, {"items", "Items"}, {"order-of-the-items", "Order of the items"},
		{"due-dates", "Due dates"}, {"reminders", "Reminders"}, {"labels", "Labels"},
		{"on-a-phone", "On a phone"}, {"keys", "Keys"},
	} {
		assertContains(t, body, `<li><a href="#`+s.id+`">`+s.title+`</a></li>`, `<h2 id="`+s.id+`">`+s.title+`</h2>`)
	}
	if strings.Index(body, `id="guide-toc-title"`) > strings.Index(body, `<h2 id="get-started">`) {
		t.Errorf("the contents list must come before the first section")
	}
	assertContains(t, body, "<table>", `<a href="https://toodoo.fly.dev">https://toodoo.fly.dev</a>`)
	assertNotContains(t, body, "<script")
}

// Each image the guide links is served, as a PNG, with no session.
func TestGuideImages(t *testing.T) {
	env := newTestEnv(t, web.WithGuide(docs.Guide))
	md, err := fs.ReadFile(docs.Guide, "user-guide.md")
	if err != nil {
		t.Fatal(err)
	}
	links := regexp.MustCompile(`\]\((guide-images/[^)\s]+\.png)`).FindAllStringSubmatch(string(md), -1)
	if len(links) < 8 {
		t.Fatalf("the guide links %d images, want at least 8", len(links))
	}
	for _, l := range links {
		rec := do(t, env.H, "GET", "/"+l[1], nil, anon)
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" {
			t.Errorf("%s: status %d, type %q", l[1], rec.Code, rec.Header().Get("Content-Type"))
		}
	}
	for _, path := range []string{"/guide-images/", "/guide-images/user-guide.md", "/guide-images/nope.png"} {
		if rec := do(t, env.H, "GET", path, nil, anon); rec.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", path, rec.Code)
		}
	}
}

// Raw HTML in the Markdown is not rendered, so the guide cannot add scripts.
func TestGuideDropsRawHTML(t *testing.T) {
	guide := fstest.MapFS{"user-guide.md": {Data: []byte("# User guide\n\n## A\n\n<script>alert(1)</script>\n\nText <b>bold</b>.\n")}}
	env := newTestEnv(t, web.WithGuide(guide))
	body := do(t, env.H, "GET", "/guide", nil, anon).Body.String()
	assertNotContains(t, body, "<script>alert", "<b>bold</b>")
	assertContains(t, body, `<h2 id="a">A</h2>`)
}

// An image alone in a paragraph, with a title, becomes a figure: the title is
// the caption above the image (canvas page 2, "centered screenshots",
// option B). An image with no title stays as it is.
func TestGuideFigures(t *testing.T) {
	md := "# User guide\n\n## A\n\n![The list](guide-images/list.png \"the list with two items\")\n\n" +
		"![Plain](guide-images/plain.png)\n\n![Bold](guide-images/bold.png \"a <b>bold</b> word\")\n"
	env := newTestEnv(t, web.WithGuide(fstest.MapFS{"user-guide.md": {Data: []byte(md)}}))
	body := do(t, env.H, "GET", "/guide", nil, anon).Body.String()
	assertContains(t, body,
		"<figure>\n<figcaption><b>Screenshot:</b> the list with two items</figcaption>\n"+
			`<img src="guide-images/list.png" alt="The list">`+"\n</figure>",
		`<p><img src="guide-images/plain.png" alt="Plain"></p>`,
		"<figcaption><b>Screenshot:</b> a &lt;b&gt;bold&lt;/b&gt; word</figcaption>")
	assertNotContains(t, body, `title="the list`)
}

// Each screenshot in the guide has a caption (the image title).
func TestGuideImagesHaveCaptions(t *testing.T) {
	md, err := fs.ReadFile(docs.Guide, "user-guide.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`).FindAllString(string(md), -1) {
		if !regexp.MustCompile(`\.png "[^"]+"\)$`).MatchString(l) {
			t.Errorf("no caption: %s", l)
		}
	}
}

// The guide is public, so the browser may keep it for an hour. The CSP is
// the same as on the other pages.
func TestGuideCacheAndCSP(t *testing.T) {
	env := newTestEnv(t, web.WithGuide(docs.Guide))
	for _, path := range []string{"/guide", "/guide-images/list.png"} {
		rec := do(t, env.H, "GET", path, nil, anon)
		if got := rec.Header().Get("Cache-Control"); got != "public, max-age=3600" {
			t.Errorf("%s Cache-Control = %q", path, got)
		}
		if got := rec.Header().Get("Content-Security-Policy"); got != wantCSP {
			t.Errorf("%s CSP = %q", path, got)
		}
	}
	if got := do(t, env.H, "GET", "/", nil, nil).Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("/ Cache-Control = %q, want no-store", got)
	}
}

// With no guide given, its routes answer 404.
func TestGuideMissing(t *testing.T) {
	env := newTestEnv(t)
	for _, path := range []string{"/guide", "/guide-images/list.png"} {
		if rec := do(t, env.H, "GET", path, nil, anon); rec.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", path, rec.Code)
		}
	}
}

// The list, the login and sign-up pages and the Account page link the guide.
func TestGuideLinks(t *testing.T) {
	env := newTestEnv(t, web.WithGuide(docs.Guide))
	assertContains(t, do(t, env.H, "GET", "/", nil, nil).Body.String(), ` · <a href="/guide">Help</a> · <a href="/account">Account</a>`)
	for _, path := range []string{"/login", "/signup"} {
		assertContains(t, do(t, env.H, "GET", path, nil, anon).Body.String(), `<p class="auth-foot">New here? <a href="/guide">Read the user guide</a> · `)
	}
	assertContains(t, do(t, env.H, "GET", "/account", nil, nil).Body.String(), `<a href="/">← Back to my list</a> · <a href="/guide">User guide</a>`)
}

// The guide covers each feature of the README list "What it does".
func TestGuideCoversTheFeatures(t *testing.T) {
	md, err := fs.ReadFile(docs.Guide, "user-guide.md")
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, string(md), "**Hide done**", "**Clear done**", "calendar icon", "`1430`", "+5 min",
		"notification", "grip (⋮⋮)", "arrow", "Overdue items", "color wheel", "**Delete**",
		"Show only the items of one label", "time zone")
}

// Every Undo (a deleted item, a repeating item checked off, removed notes)
// shows for 5 seconds, and the guide says the same.
func TestUndoShowsFiveSeconds(t *testing.T) {
	h, _ := newTestApp(t)
	js := do(t, h, "GET", "/static/app.js", nil, nil).Body.String()
	assertContains(t, js, "const UNDO_MS = 5000;")
	md, err := fs.ReadFile(docs.Guide, "user-guide.md")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(strings.Fields(string(md)), " ")
	if n := strings.Count(text, "5 seconds"); n != 3 || strings.Contains(text, "8 seconds") {
		t.Fatalf("the guide names \"5 seconds\" %d times (want 3) or still names \"8 seconds\"", n)
	}
}
