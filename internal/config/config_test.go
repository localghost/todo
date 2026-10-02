package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"todo/internal/config"
)

func write(t *testing.T, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDefault(t *testing.T) {
	if n := config.Default().Password.MinLength; n != 8 {
		t.Fatalf("default min_length = %d, want 8", n)
	}
}

func TestLoad(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		want       int
	}{
		{"value", "password:\n  min_length: 12\n", 12},
		{"empty file", "", 8},
		{"comments only", "# nothing set\n", 8},
		{"no password section", "{}\n", 8},
	} {
		cfg, err := config.Load(write(t, tc.text), false)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if cfg.Password.MinLength != tc.want {
			t.Errorf("%s: min_length = %d, want %d", tc.name, cfg.Password.MinLength, tc.want)
		}
	}
}

func TestLoadMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nope.yaml")
	cfg, err := config.Load(path, true)
	if err != nil || cfg.Password.MinLength != 8 {
		t.Fatalf("allowed missing file: %+v, %v; want defaults", cfg, err)
	}
	if _, err := config.Load(path, false); err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("missing file not allowed: %v, want an error naming %s", err, path)
	}
}

func TestLoadRejectsBadFiles(t *testing.T) {
	for _, tc := range []struct{ name, text, want string }{
		{"typo in key", "password:\n  min_lenght: 12\n", "min_lenght"},
		{"zero", "password:\n  min_length: 0\n", "password.min_length"},
		{"above maximum", "password:\n  min_length: 201\n", "password.min_length"},
		{"not a number", "password:\n  min_length: eight\n", "eight"},
	} {
		path := write(t, tc.text)
		_, err := config.Load(path, false)
		if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), path) {
			t.Errorf("%s: error %v, want one naming %q and the path", tc.name, err, tc.want)
		}
	}
}
