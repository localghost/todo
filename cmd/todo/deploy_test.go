package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The Dockerfile, fly.toml and mise.toml must name the same port, paths and app.
func TestDeployConfigMatches(t *testing.T) {
	read := func(name string) string {
		b, err := os.ReadFile("../../" + name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		return string(b)
	}
	find := func(text, pattern, what string) string {
		m := regexp.MustCompile(pattern).FindStringSubmatch(text)
		if m == nil {
			t.Fatalf("%s not found (pattern %s)", what, pattern)
		}
		return m[1]
	}
	docker, fly, mise := read("Dockerfile"), read("fly.toml"), read("mise.toml")

	port := find(docker, `"-addr", ":(\d+)"`, "Dockerfile -addr port")
	db := find(docker, `"-db", "([^"]+)"`, "Dockerfile -db path")
	if !strings.Contains(docker, `"-trust-proxy"`) {
		t.Error("Dockerfile does not start the app with -trust-proxy")
	}
	if p := find(fly, `internal_port = (\d+)`, "fly.toml internal_port"); p != port {
		t.Errorf("fly.toml internal_port = %s, Dockerfile -addr port = %s", p, port)
	}
	if mount := find(fly, `destination = "([^"]+)"`, "fly.toml mount destination"); !strings.HasPrefix(db, mount+"/") {
		t.Errorf("database %s is not on the volume at %s", db, mount)
	}
	if m := find(mise, `TODO_DB = "([^"]+)"`, "mise.toml TODO_DB"); m != db {
		t.Errorf("mise.toml TODO_DB = %s, Dockerfile -db = %s", m, db)
	}
	if a, b := find(fly, `(?m)^app = "([^"]+)"`, "fly.toml app"), find(mise, `FLY_APP = "([^"]+)"`, "mise.toml FLY_APP"); a != b {
		t.Errorf("fly.toml app = %s, mise.toml FLY_APP = %s", a, b)
	}
}
