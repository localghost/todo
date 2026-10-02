package main

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	twelve := filepath.Join(dir, "twelve.yaml")
	if err := os.WriteFile(twelve, []byte("password:\n  min_length: 12\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	parse := func(args ...string) (*flag.FlagSet, string) {
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		path := fs.String("config", "config.yaml", "")
		if err := fs.Parse(args); err != nil {
			t.Fatal(err)
		}
		return fs, *path
	}

	if _, _, err := loadConfig(parse("-config", filepath.Join(dir, "missing.yaml"))); err == nil {
		t.Fatal("an explicit -config path that does not exist must be an error")
	}
	cfg, label, err := loadConfig(parse("-config", twelve))
	if err != nil || cfg.Password.MinLength != 12 || label != twelve {
		t.Fatalf("-config file: %+v, %q, %v; want 12 and the path", cfg, label, err)
	}
	t.Chdir(t.TempDir()) // no config.yaml here
	cfg, label, err = loadConfig(parse())
	if err != nil || cfg.Password.MinLength != 8 || label != "none (defaults)" {
		t.Fatalf("no flag, no file: %+v, %q, %v; want 8 and none (defaults)", cfg, label, err)
	}
}
