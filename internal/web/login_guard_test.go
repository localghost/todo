package web_test

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"testing"
	"time"

	"todo/internal/web"
)

const lockedMsg = "Too many attempts. Please try again in 15 minutes."

func login(t *testing.T, env *testEnv, user, pass string, h map[string]string) int {
	t.Helper()
	return do(t, env.H, "POST", "/login", url.Values{"username": {user}, "password": {pass}}, h).Code
}

func TestLoginLimitPerUsername(t *testing.T) {
	env := newTestEnv(t)
	env.Auth.SignUp(context.Background(), "alice", pw)
	for i := 0; i < 5; i++ {
		if code := login(t, env, "alice", "wrong password", fromIP("203.0.113."+strconv.Itoa(i+1))); code != http.StatusUnprocessableEntity {
			t.Fatalf("wrong %d: %d, want 422", i+1, code)
		}
	}
	rec := do(t, env.H, "POST", "/login", url.Values{"username": {"alice"}, "password": {pw}}, fromIP("203.0.113.99"))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("6th try with the right password: %d, want 429", rec.Code)
	}
	assertContains(t, rec.Body.String(), lockedMsg)
	if code := login(t, env, "tester-other", "wrong password", fromIP("203.0.113.99")); code != http.StatusUnprocessableEntity {
		t.Fatalf("other username: %d, want 422 (not blocked)", code)
	}
	env.Clock.t = env.Clock.t.Add(15*time.Minute + time.Second)
	if code := login(t, env, "alice", pw, fromIP("203.0.113.99")); code != http.StatusSeeOther {
		t.Fatalf("after 15 minutes: %d, want 303", code)
	}
}

func TestLoginLimitIgnoresUsernameCase(t *testing.T) {
	env := newTestEnv(t)
	env.Auth.SignUp(context.Background(), "alice", pw)
	for i, name := range []string{"alice", "ALICE", "Alice", "aLiCe", "ALIce"} {
		login(t, env, name, "wrong password", fromIP("203.0.113."+strconv.Itoa(i+1)))
	}
	if code := login(t, env, "alicE", pw, fromIP("203.0.113.50")); code != http.StatusTooManyRequests {
		t.Fatalf("after 5 wrong tries in mixed case: %d, want 429", code)
	}
}

func TestLoginLimitPerIP(t *testing.T) {
	env := newTestEnv(t)
	env.Auth.SignUp(context.Background(), "alice", pw)
	ip := fromIP("192.0.2.77")
	for i := 0; i < 20; i++ {
		login(t, env, "nobody"+strconv.Itoa(i), "wrong password", ip)
	}
	if code := login(t, env, "alice", pw, ip); code != http.StatusTooManyRequests {
		t.Fatalf("21st login from one IP: %d, want 429", code)
	}
	if code := login(t, env, "alice", pw, fromIP("192.0.2.78")); code != http.StatusSeeOther {
		t.Fatalf("other IP: %d, want 303", code)
	}
}

func TestSuccessfulLoginDoesNotCount(t *testing.T) {
	env := newTestEnv(t)
	env.Auth.SignUp(context.Background(), "alice", pw)
	ip := fromIP("192.0.2.80")
	for i := 0; i < 8; i++ {
		if code := login(t, env, "alice", pw, ip); code != http.StatusSeeOther {
			t.Fatalf("good login %d: %d, want 303", i+1, code)
		}
	}
}

func TestParallelWrongLoginsAreCapped(t *testing.T) {
	env := newTestEnv(t)
	env.Auth.SignUp(context.Background(), "alice", pw)
	var mu sync.Mutex
	counts := map[int]int{}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			code := login(t, env, "alice", "wrong password", fromIP("198.18.0."+strconv.Itoa(i+1)))
			mu.Lock()
			counts[code]++
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	if counts[http.StatusUnprocessableEntity] != 5 || counts[http.StatusTooManyRequests] != 15 {
		t.Fatalf("parallel wrong logins: %v, want 5×422 and 15×429", counts)
	}
}

func TestForwardedForOnlyWithTrustProxy(t *testing.T) {
	for _, trust := range []bool{false, true} {
		env := newTestEnv(t, web.WithTrustProxy(trust))
		env.Auth.SignUp(context.Background(), "alice", pw)
		proxy := "10.0.0.1"
		for i := 0; i < 20; i++ {
			h := fromIP(proxy)
			h["X-Forwarded-For"] = "1.2.3.4, 198.51.100." + strconv.Itoa(i%2+1) // forged first, real last
			login(t, env, "nobody"+strconv.Itoa(i), "wrong password", h)
		}
		h := fromIP(proxy)
		h["X-Forwarded-For"] = "198.51.100.200"
		code := login(t, env, "alice", pw, h)
		want := http.StatusTooManyRequests // without trust: all 21 requests share the proxy's IP
		if trust {
			want = http.StatusSeeOther // with trust: a new client IP
		}
		if code != want {
			t.Errorf("trust=%v: %d, want %d", trust, code, want)
		}
	}
}
