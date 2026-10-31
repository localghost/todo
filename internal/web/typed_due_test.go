package web_test

import (
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Due date typed in the add field (issue #6, canvas page "10 · Due date",
// board "due date typed in the add field", highlight H2).

// typedDueHarness runs app.js with a stubbed DOM and prints, for each case
// of testdata/typed-due.json, what parseTypedDue found.
const typedDueHarness = `
const fs = require("fs");
const src = fs.readFileSync(process.argv[2], "utf8");
const data = JSON.parse(fs.readFileSync(process.argv[3], "utf8"));
const document = {
  addEventListener() {}, getElementById: () => null, querySelector: () => null, querySelectorAll: () => [],
  activeElement: null, body: { classList: { contains: () => false } },
};
const fns = new Function("document", "window", "navigator", "fetch", "setInterval", "setTimeout", "clearTimeout", "htmx",
  src + "\nreturn { parseTypedDue };")(document, {}, {}, () => new Promise(() => {}), () => 0, () => 0, () => {}, {});
const [d, t] = data.now.split("T");
const [y, mo, da] = d.split("-").map(Number);
const [h, mi] = t.split(":").map(Number);
const now = new Date(y, mo - 1, da, h, mi);
for (const c of data.cases) {
  const m = fns.parseTypedDue(c.text, now);
  console.log(JSON.stringify(m ? { text: c.text, match: c.text.slice(m.start, m.end), date: m.date, time: m.time } : { text: c.text, match: null }));
}
`

type typedDueCase struct {
	Text  string  `json:"text"`
	Match *string `json:"match"`
	Date  string  `json:"date"`
	Time  string  `json:"time"`
}

func TestParseTypedDue(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	raw, err := os.ReadFile("testdata/typed-due.json")
	if err != nil {
		t.Fatal(err)
	}
	var data struct {
		Cases []typedDueCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	harness := filepath.Join(t.TempDir(), "harness.js")
	if err := os.WriteFile(harness, []byte(typedDueHarness), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(node, harness, "static/app.js", "testdata/typed-due.json")
	cmd.Env = append(os.Environ(), "TZ=Europe/Warsaw")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != len(data.Cases) {
		t.Fatalf("got %d lines, want %d:\n%s", len(lines), len(data.Cases), out)
	}
	for i, line := range lines {
		var got typedDueCase
		if err := json.Unmarshal([]byte(line), &got); err != nil {
			t.Fatalf("line %q: %v", line, err)
		}
		want := data.Cases[i]
		show := func(c typedDueCase) string {
			if c.Match == nil {
				return "no date"
			}
			return *c.Match + " → " + c.Date + " " + c.Time
		}
		if show(got) != show(want) {
			t.Errorf("%q: got %s, want %s", want.Text, show(got), show(want))
		}
	}
}

// The add field has the layer behind it that draws the dotted line under the
// date words.
func TestAddFieldHasTypedDueLayer(t *testing.T) {
	env := newTestEnv(t)
	body := do(t, env.H, "GET", "/", nil, nil).Body.String()
	assertContains(t, body,
		`<span class="add-field"><span class="add-mirror" aria-hidden="true"></span><input id="new-item" name="text"`)
	css := do(t, env.H, "GET", "/static/app.css", nil, nil).Body.String()
	assertContains(t, css, ".add-field {", ".add-mirror {", ".add-mirror mark {", "text-decoration: underline dotted")
	js := do(t, env.H, "GET", "/static/app.js", nil, nil).Body.String()
	assertContains(t, js, "function parseTypedDue(text, now)", `"htmx:configRequest"`, `e.detail.elt.id === "add-form"`)
}

// A past date in the add form makes the date button red, as on an overdue
// row; a future date does not.
func TestDraftDuePastIsRed(t *testing.T) {
	env := newTestEnv(t)
	rec := do(t, env.H, "POST", "/new/due", url.Values{"due_date": {"2000-03-04"}}, htmxHeaders)
	assertContains(t, rec.Body.String(), `<span class="due-add-wrap due-set is-over">`)
	rec = do(t, env.H, "POST", "/new/due", url.Values{"due_date": {"2099-03-04"}}, htmxHeaders)
	assertContains(t, rec.Body.String(), `<span class="due-add-wrap due-set">`)
}
