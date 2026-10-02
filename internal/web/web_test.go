package web_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"todo/internal/store/sqlite"
	"todo/internal/todo"
	"todo/internal/web"
)

func newTestApp(t *testing.T) (http.Handler, *todo.Service) {
	t.Helper()
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	if _, err := store.CreateUser(context.Background(), "tester", "x", time.Now()); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	svc := todo.NewService(store)
	h, err := web.New(svc, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("web.New: %v", err)
	}
	return h, svc
}

func do(t *testing.T, h http.Handler, method, target string, form url.Values, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req := httptest.NewRequest(method, target, body)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func mustAdd(t *testing.T, svc *todo.Service, text string) todo.Item {
	t.Helper()
	it, err := svc.Add(context.Background(), 1, text, "", "")
	if err != nil {
		t.Fatalf("Add(%q): %v", text, err)
	}
	return it
}

var htmxHeaders = map[string]string{"HX-Request": "true"}

func assertContains(t *testing.T, body string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(body, w) {
			t.Errorf("body does not contain %q\nbody:\n%s", w, body)
		}
	}
}

func assertNotContains(t *testing.T, body string, nots ...string) {
	t.Helper()
	for _, n := range nots {
		if strings.Contains(body, n) {
			t.Errorf("body contains %q but should not\nbody:\n%s", n, body)
		}
	}
}
