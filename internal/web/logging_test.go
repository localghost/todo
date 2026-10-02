package web_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"todo/internal/auth"
	"todo/internal/store/sqlite"
	"todo/internal/todo"
	"todo/internal/web"
)

type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}
func (s *syncBuffer) String() string { s.mu.Lock(); defer s.mu.Unlock(); return s.b.String() }

func TestSecurityEventsAreLogged(t *testing.T) {
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "log.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var logs syncBuffer
	clock := &testClock{t: time.Now()}
	accounts := auth.NewService(store)
	accounts.SignUp(context.Background(), "alice", pw)
	h, _ := web.New(todo.NewService(store), accounts, slog.New(slog.NewTextHandler(&logs, nil)), web.WithClock(clock.Now), web.WithPowBits(4))

	post := func(path string, v url.Values, xff string) {
		req := httptest.NewRequest("POST", path, strings.NewReader(v.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if xff != "" {
			req.Header.Set("X-Forwarded-For", xff)
		}
		h.ServeHTTP(httptest.NewRecorder(), req)
	}
	post("/signup", url.Values{"username": {"bot1"}, "password": {pw}, "website": {"spam"}}, "")
	for i := 0; i < 6; i++ {
		post("/login", url.Values{"username": {"alice"}, "password": {"wrong password"}}, "198.51.100.1")
	}
	out := logs.String()
	for _, want := range []string{"reason=honeypot", "login blocked", "X-Forwarded-For", "if not, ignore this"} {
		if !strings.Contains(out, want) {
			t.Errorf("log does not contain %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "share one limit") {
		t.Errorf("the warning must not push the admin to -trust-proxy without a proxy:\n%s", out)
	}
	if strings.Count(out, "X-Forwarded-For") != 1 {
		t.Errorf("the X-Forwarded-For warning must appear once:\n%s", out)
	}
}

func TestUnreadableForwardedForIsLoggedShort(t *testing.T) {
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "log.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var logs syncBuffer
	h, _ := web.New(todo.NewService(store), auth.NewService(store), slog.New(slog.NewTextHandler(&logs, nil)), web.WithTrustProxy(true))
	v := url.Values{"username": {"alice"}, "password": {"wrong password"}}
	req := httptest.NewRequest("POST", "/login", strings.NewReader(v.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Forwarded-For", strings.Repeat("x", 10000))
	h.ServeHTTP(httptest.NewRecorder(), req)
	out := logs.String()
	if !strings.Contains(out, "cannot read the client address") {
		t.Fatalf("no warning:\n%s", out)
	}
	if len(out) > 1000 {
		t.Fatalf("log has %d bytes; an attacker-controlled header must be cut", len(out))
	}
}
