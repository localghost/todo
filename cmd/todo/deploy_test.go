package main

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"todo/internal/config"
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
	// On Fly the rightmost X-Forwarded-For entry is Fly's address, not the client's.
	if !strings.Contains(docker, `"-client-ip-header", "Fly-Client-IP"`) || strings.Contains(docker, "-trust-proxy") {
		t.Error("Dockerfile must take the client IP from Fly-Client-IP, not X-Forwarded-For")
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

	// Two password checks (128 MiB live) need a GC limit to stay inside 256 MB.
	if !regexp.MustCompile(`(?m)^\s*GOMEMLIMIT = "\d+MiB"`).MatchString(fly) {
		t.Error("fly.toml sets no GOMEMLIMIT")
	}
	// Due dates use the server's local time.
	if !strings.Contains(fly, `TZ = "Europe/Warsaw"`) {
		t.Error("fly.toml does not set TZ")
	}
	// The docs show kill_timeout only as a number of seconds.
	find(fly, `(?m)^kill_timeout = (\d+)$`, "fly.toml kill_timeout in seconds")
	// SSH does not wake a stopped machine, so every ssh task wakes it first.
	for _, block := range strings.Split(mise, "[tasks.")[1:] {
		if strings.Contains(block, "fly ssh") && !strings.Contains(block, `depends = ["fly:wake"]`) {
			t.Errorf("mise task %s uses fly ssh without depends = [\"fly:wake\"]", strings.SplitN(block, "]", 2)[0])
		}
	}
	// During a restore there are two volumes; list the snapshots of the attached one.
	if !strings.Contains(mise, `select(.name == \"todo_data\" and .attached_machine_id != null)`) {
		t.Error("fly:snapshots does not select the attached todo_data volume")
	}
	// The region must exist (EU regions from https://docs.fly.io/reference/regions),
	// and mise.toml and the guide must use the same one.
	region := find(fly, `(?m)^primary_region = "([^"]+)"`, "fly.toml primary_region")
	if !strings.Contains(" ams arn cdg fra lhr ", " "+region+" ") {
		t.Errorf("fly.toml primary_region = %s, not a European Fly region", region)
	}
	guide := read("docs/deploy.md")
	for name, text := range map[string]string{"mise.toml": mise, "docs/deploy.md": guide} {
		for _, m := range regexp.MustCompile(`--region (\w+)`).FindAllStringSubmatch(text, -1) {
			if m[1] != region {
				t.Errorf("%s uses --region %s, fly.toml uses %s", name, m[1], region)
			}
		}
	}
	// The image contains config.yaml at the path that -config names.
	copied := find(docker, `(?m)^COPY config\.yaml (\S+)$`, "Dockerfile COPY config.yaml")
	if p := find(docker, `"-config", "([^"]+)"`, "Dockerfile -config path"); p != copied {
		t.Errorf("Dockerfile copies config.yaml to %s but starts the app with -config %s", copied, p)
	}
	// The admin commands on Fly read the same settings file as the server.
	if m := find(mise, `TODO_CONFIG = "([^"]+)"`, "mise.toml TODO_CONFIG"); m != copied {
		t.Errorf("mise.toml TODO_CONFIG = %s, Dockerfile copies config.yaml to %s", m, copied)
	}
	for _, block := range strings.Split(mise, "[tasks.")[1:] {
		if strings.Contains(block, "fly ssh") && !strings.Contains(block, "-config $TODO_CONFIG") {
			t.Errorf("mise task %s runs a users command without -config $TODO_CONFIG", strings.SplitN(block, "]", 2)[0])
		}
	}
	cfg, _, err := config.Load("../../config.yaml", false)
	if err != nil {
		t.Fatalf("config.yaml: %v", err)
	}
	if cfg.Password.MinLength != 8 {
		t.Errorf("config.yaml min_length = %d, want 8", cfg.Password.MinLength)
	}
}

// fly comes from mise, so the guide must not tell the user to run plain fly.
func TestDeployGuideUsesMise(t *testing.T) {
	guide, err := os.ReadFile("../../docs/deploy.md")
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range strings.Split(string(guide), "\n") {
		if strings.Contains(line, "`fly ") {
			t.Errorf("docs/deploy.md:%d runs plain fly; use `mise exec -- fly …` or a task: %s", i+1, line)
		}
	}
	mise, err := os.ReadFile("../../mise.toml")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"-- --help", "fixed 10"} {
		if !strings.Contains(string(guide), want) {
			t.Errorf("docs/deploy.md does not contain %q", want)
		}
	}
	if !strings.Contains(string(mise), `[tasks."fly:login"]`) {
		t.Error(`mise.toml has no fly:login task`)
	}
}
