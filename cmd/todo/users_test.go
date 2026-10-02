package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"todo/internal/auth"
	"todo/internal/store/sqlite"
	"todo/internal/todo"
)

func usersDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "todo.db")
	s, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()
	a := auth.NewService(s)
	ctx := context.Background()
	alice, _ := a.SignUp(ctx, "alice", "correct horse battery")
	a.SignUp(ctx, "bob", "correct horse battery")
	todo.NewService(s).Add(ctx, alice.ID, "x", "", "")
	a.StartSession(ctx, alice.ID, false)
	return path
}

func TestUsersList(t *testing.T) {
	path := usersDB(t)
	var out bytes.Buffer
	if err := runUsers([]string{"list", "-db", path}, nil, &out); err != nil {
		t.Fatalf("list: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 || strings.Join(strings.Fields(lines[0]), " ") != "USERNAME CREATED ITEMS SESSIONS" {
		t.Fatalf("output:\n%s", out.String())
	}
	if f := strings.Fields(lines[1]); f[0] != "alice" || f[2] != "1" || f[3] != "1" {
		t.Fatalf("alice line = %q", lines[1])
	}
	if f := strings.Fields(lines[2]); f[0] != "bob" || f[2] != "0" || f[3] != "0" {
		t.Fatalf("bob line = %q", lines[2])
	}
}

func TestUsersResetPassword(t *testing.T) {
	path := usersDB(t)
	var out bytes.Buffer
	if err := runUsers([]string{"reset-password", "-db", path, "alice"}, nil, &out); err != nil {
		t.Fatalf("reset-password: %v", err)
	}
	line := strings.TrimSpace(out.String())
	if !strings.HasPrefix(line, "New password for alice: ") || len(strings.TrimPrefix(line, "New password for alice: ")) != 16 {
		t.Fatalf("output = %q", line)
	}
	err := runUsers([]string{"reset-password", "-db", path, "nobody"}, nil, &out)
	if err == nil || err.Error() != `no user "nobody"` {
		t.Fatalf("unknown user err = %v", err)
	}
}

func TestUsersDelete(t *testing.T) {
	path := usersDB(t)
	var out bytes.Buffer
	err := runUsers([]string{"delete", "-db", path, "bob"}, strings.NewReader("alice\n"), &out)
	if err == nil || !strings.Contains(out.String(), "Type the username to delete bob and all items: ") {
		t.Fatalf("wrong confirmation: err = %v, output %q", err, out.String())
	}
	out.Reset()
	if err := runUsers([]string{"delete", "-db", path, "bob"}, strings.NewReader("bob\n"), &out); err != nil || !strings.Contains(out.String(), "Deleted bob.") {
		t.Fatalf("delete: %v, output %q", err, out.String())
	}
	out.Reset()
	if err := runUsers([]string{"delete", "-db", path, "alice", "-yes"}, nil, &out); err != nil || strings.TrimSpace(out.String()) != "Deleted alice." {
		t.Fatalf("delete -yes: %v, output %q", err, out.String())
	}
	out.Reset()
	runUsers([]string{"list", "-db", path}, nil, &out)
	if strings.Count(strings.TrimSpace(out.String()), "\n") != 0 {
		t.Fatalf("users left:\n%s", out.String())
	}
}

func TestUsersCommandNeedsCurrentDatabase(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "none.db")
	var out bytes.Buffer
	if err := runUsers([]string{"list", "-db", missing}, nil, &out); err == nil {
		t.Fatal("list on a missing database: err = nil")
	}
	if err := runUsers([]string{"frobnicate"}, nil, &out); err == nil || !strings.Contains(err.Error(), "usage") {
		t.Fatalf("unknown command err = %v, want usage", err)
	}
}

func TestUsersPolish(t *testing.T) {
	path := usersDB(t)
	var out bytes.Buffer
	if err := runUsers([]string{"reset-password", "-db", path, "ALICE"}, nil, &out); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if !strings.HasPrefix(out.String(), "New password for alice: ") {
		t.Fatalf("output %q, want the stored username", out.String())
	}
	err := runUsers([]string{"frobnicate"}, nil, &out)
	if err == nil || !strings.Contains(err.Error(), "--") {
		t.Fatalf("usage %v must mention --", err)
	}
}

func TestUsersResetFollowsConfig(t *testing.T) {
	path := usersDB(t)
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfg, []byte("password:\n  min_length: 20\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runUsers([]string{"reset-password", "-db", path, "-config", cfg, "alice"}, nil, &out); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if pw := strings.TrimSpace(strings.TrimPrefix(out.String(), "New password for alice: ")); len(pw) != 20 {
		t.Fatalf("new password %q has %d characters, want 20", pw, len(pw))
	}
	if err := runUsers([]string{"list", "-db", path, "-config", filepath.Join(t.TempDir(), "missing.yaml")}, nil, &out); err == nil {
		t.Fatal("an explicit -config path that does not exist must be an error")
	}
}
