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
		cfg, found, err := config.Load(write(t, tc.text), false)
		if err != nil || !found {
			t.Fatalf("%s: found %v, %v", tc.name, found, err)
		}
		if cfg.Password.MinLength != tc.want {
			t.Errorf("%s: min_length = %d, want %d", tc.name, cfg.Password.MinLength, tc.want)
		}
	}
}

func TestLoadMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nope.yaml")
	cfg, found, err := config.Load(path, true)
	if err != nil || found || cfg.Password.MinLength != 8 {
		t.Fatalf("allowed missing file: %+v, %v; want defaults", cfg, err)
	}
	if _, _, err := config.Load(path, false); err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("missing file not allowed: %v, want an error naming %s", err, path)
	}
}

func TestLoadRejectsBadFiles(t *testing.T) {
	for _, tc := range []struct{ name, text, want string }{
		{"typo in key", "password:\n  min_lenght: 12\n", "min_lenght"},
		{"zero", "password:\n  min_length: 0\n", "password.min_length"},
		{"above maximum", "password:\n  min_length: 201\n", "password.min_length"},
		{"not a number", "password:\n  min_length: eight\n", "eight"},
		{"second document", "password:\n  min_length: 12\n---\npassword:\n  min_length: 14\n", "one YAML document"},
		{"empty first documents", "---\n---\npassword:\n  min_length: 12\n", "one YAML document"},
		{"decimal number", "password:\n  min_length: 8.5\n", "whole number"},
		{"empty value", "password:\n  min_length:\n", "no value"},
		{"null value", "password:\n  min_length: null\n", "no value"},
		{"null section", "password: null\n", "no value"},
	} {
		path := write(t, tc.text)
		_, _, err := config.Load(path, false)
		if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), path) {
			t.Errorf("%s: error %v, want one naming %q and the path", tc.name, err, tc.want)
		}
	}
}
