package web_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// wheelHarness runs the math of wheel.js and prints one line per case.
const wheelHarness = `
const src = require("fs").readFileSync(process.argv[2], "utf8");
const fns = new Function(src + "\nreturn { wheelPick, wheelRing };")();
const r = (a) => a.map((x) => Math.round(x * 1000) / 1000).join(" ");
console.log("top " + r(fns.wheelPick(0, -0.5)));
console.log("right " + r(fns.wheelPick(0.5, 0)));
console.log("half down " + r(fns.wheelPick(0, 0.25)));
console.log("outside " + r(fns.wheelPick(0, -2)));
console.log("ring right " + r(fns.wheelRing(90, 1)));
console.log("ring middle " + r(fns.wheelRing(0, 0)));
`

func TestWheelScriptMath(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	harness := filepath.Join(t.TempDir(), "harness.js")
	if err := os.WriteFile(harness, []byte(wheelHarness), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(node, harness, "static/wheel.js").CombinedOutput()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, out)
	}
	want := "top 0 1\nright 90 1\nhalf down 180 0.5\noutside 0 1\nring right 100 50\nring middle 50 50"
	if got := strings.TrimSpace(string(out)); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPageLoadsWheelScript(t *testing.T) {
	env := newTestEnv(t)
	body := do(t, env.H, "GET", "/", nil, nil).Body.String()
	i, j := strings.Index(body, `src="/static/chips.js"`), strings.Index(body, `src="/static/wheel.js"`)
	k := strings.Index(body, `src="/static/app.js"`)
	if i < 0 || j < i || k < j {
		t.Fatal("want chips.js, wheel.js, app.js in this order")
	}
}
