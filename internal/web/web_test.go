package web_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"todo/internal/auth"
	"todo/internal/guard"
	"todo/internal/store/sqlite"
	"todo/internal/todo"
	"todo/internal/web"
)

// testEnv is an app with one logged-in user "tester" (ID 1).
type testEnv struct {
	H     http.Handler // adds the tester's session cookie unless the request has one or is anonymous
	Raw   http.Handler
	Svc   *todo.Service
	Auth  *auth.Service
	Store *sqlite.Store
	User  auth.User
	Token string
	Clock *testClock
}

func newTestEnv(t *testing.T, opts ...web.Option) *testEnv {
	t.Helper()
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	user, err := store.CreateUser(context.Background(), "tester", "x", time.Now())
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	accounts := auth.NewService(store)
	token, _, err := accounts.StartSession(context.Background(), user.ID, false)
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	svc := todo.NewService(store)
	clock := &testClock{t: time.Now()}
	opts = append([]web.Option{web.WithClock(clock.Now), web.WithPowBits(4)}, opts...)
	raw, err := web.New(svc, accounts, slog.New(slog.NewTextHandler(io.Discard, nil)), opts...)
	if err != nil {
		t.Fatalf("web.New: %v", err)
	}
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(anonHeader) == "" {
			if _, err := r.Cookie("todo_session"); err != nil {
				r.AddCookie(&http.Cookie{Name: "todo_session", Value: token})
			}
		}
		r.Header.Del(anonHeader)
		if ip := r.Header.Get(fromIPHeader); ip != "" {
			r.RemoteAddr = ip + ":40000"
			r.Header.Del(fromIPHeader)
		}
		raw.ServeHTTP(w, r)
	})
	return &testEnv{H: h, Raw: raw, Svc: svc, Auth: accounts, Store: store, User: user, Token: token, Clock: clock}
}

func newTestApp(t *testing.T) (http.Handler, *todo.Service) {
	env := newTestEnv(t)
	return env.H, env.Svc
}

// anonHeader makes a test request without the tester's cookie.
const anonHeader = "X-Test-Anonymous"

var anon = map[string]string{anonHeader: "1"}

// cookie returns headers that send the session cookie of token.
func cookie(token string, extra ...map[string]string) map[string]string {
	h := map[string]string{"Cookie": "todo_session=" + token}
	for _, e := range extra {
		for k, v := range e {
			h[k] = v
		}
	}
	return h
}

// sessionFrom returns the todo_session cookie set by a response.
func sessionFrom(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == "todo_session" {
			return c
		}
	}
	t.Fatalf("no todo_session cookie in response; headers: %v", rec.Header())
	return nil
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

type testClock struct{ t time.Time }

func (c *testClock) Now() time.Time { return c.t }

// fromIPHeader makes a test request come from another client address.
const fromIPHeader = "X-Test-Remote-IP"

func fromIP(ip string) map[string]string { return map[string]string{fromIPHeader: ip, anonHeader: "1"} }

var formTokenRe = regexp.MustCompile(`name="form_token" value="([^"]+)"`)

// signupForm loads the sign-up page (as headers h, default anon), waits past
// the minimum fill time, solves the puzzle, and returns a valid form.
func signupForm(t *testing.T, env *testEnv, username, password string, h ...map[string]string) url.Values {
	t.Helper()
	headers := anon
	if len(h) > 0 {
		headers = h[0]
	}
	page := do(t, env.H, "GET", "/signup", nil, headers).Body.String()
	m := formTokenRe.FindStringSubmatch(page)
	if m == nil {
		t.Fatalf("no form_token in sign-up page:\n%s", page)
	}
	env.Clock.t = env.Clock.t.Add(guard.MinFillTime + time.Second)
	return url.Values{"username": {username}, "password": {password}, "form_token": {m[1]},
		"pow_nonce": {nonceFor(t, m[1], 4)}, "website": {""}}
}

func nonceFor(t *testing.T, token string, bits int) string {
	t.Helper()
	for i := 0; i < 1<<20; i++ {
		if n := strconv.Itoa(i); guard.ProofOK(token, n, bits) {
			return n
		}
	}
	t.Fatal("no nonce")
	return ""
}
