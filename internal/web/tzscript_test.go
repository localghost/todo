package web_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPagesLoadTimeZoneScript(t *testing.T) {
	env := newTestEnv(t)
	tag := `<script src="/static/tz.js" defer></script>`
	assertContains(t, do(t, env.H, "GET", "/", nil, nil).Body.String(), tag)
	assertContains(t, do(t, env.H, "GET", "/login", nil, anon).Body.String(), tag)
	assertContains(t, do(t, env.H, "GET", "/signup", nil, anon).Body.String(), tag)

	js := do(t, env.H, "GET", "/static/tz.js", nil, anon).Body.String()
	assertContains(t, js, "resolvedOptions().timeZone", `"todo_tz="`, "SameSite=Lax", "Secure",
		// Reload only when the cookie was saved and the page used another zone,
		// so blocked cookies or an unknown zone cannot cause a reload loop.
		"if (saved && used && used !== zone) {")
}

// tzHarness runs tz.js with a fake document, location and Intl (zone
// Asia/Tokyo) and prints one JSON line per case.
const tzHarness = `
const src = require("fs").readFileSync(process.argv[2], "utf8");
function run(name, jar, blocked, used) {
  let reloads = 0;
  const document = {
    get cookie() { return jar.join("; "); },
    set cookie(v) {
      if (blocked) return;
      const kv = v.split(";")[0], k = kv.split("=")[0];
      const i = jar.findIndex((c) => c.startsWith(k + "="));
      if (i >= 0) jar[i] = kv; else jar.push(kv);
    },
    body: { dataset: used ? { tz: used } : {} },
  };
  const location = { protocol: "https:", reload: () => reloads++ };
  const Intl = { DateTimeFormat: () => ({ resolvedOptions: () => ({ timeZone: "Asia/Tokyo" }) }) };
  new Function("document", "location", "Intl", src)(document, location, Intl);
  console.log(JSON.stringify({ name, reloads, cookie: jar.includes("todo_tz=Asia/Tokyo") }));
}
run("first visit", ["todo_session=x"], false, "Europe/Warsaw");
run("after reload", ["todo_session=x", "todo_tz=Asia/Tokyo"], false, "Asia/Tokyo");
run("cookies blocked", [], true, "Europe/Warsaw");
run("server used the zone", [], false, "Asia/Tokyo");
run("login page", [], false, "");
`

func TestTimeZoneScriptReloadsAtMostOnce(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	harness := filepath.Join(t.TempDir(), "harness.js")
	if err := os.WriteFile(harness, []byte(tzHarness), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(node, harness, "static/tz.js").CombinedOutput()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, out)
	}
	want := map[string]struct {
		reloads int
		cookie  bool
	}{
		"first visit":          {1, true},
		"after reload":         {0, true},
		"cookies blocked":      {0, false},
		"server used the zone": {0, true},
		"login page":           {0, true},
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != len(want) {
		t.Fatalf("got %d results, want %d:\n%s", len(lines), len(want), out)
	}
	for _, line := range lines {
		var got struct {
			Name    string
			Reloads int
			Cookie  bool
		}
		if err := json.Unmarshal([]byte(line), &got); err != nil {
			t.Fatalf("%q: %v", line, err)
		}
		if w := want[got.Name]; got.Reloads != w.reloads || got.Cookie != w.cookie {
			t.Errorf("%s: reloads %d, cookie %v; want %d, %v", got.Name, got.Reloads, got.Cookie, w.reloads, w.cookie)
		}
	}
}
