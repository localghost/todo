package auth_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"todo/internal/auth"
	"todo/internal/store/sqlite"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newAuth(t *testing.T) (*auth.Service, *clock) {
	t.Helper()
	s, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	c := &clock{t: time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)}
	return auth.NewService(s, auth.WithClock(c.now)), c
}

const pw = "correct horse battery"

func ruleMsg(t *testing.T, err error) string {
	t.Helper()
	var re *auth.RuleError
	if !errors.As(err, &re) {
		t.Fatalf("err = %v, want RuleError", err)
	}
	return re.Msg
}

func TestUsernameRules(t *testing.T) {
	for _, ok := range []string{"abc", "Zbigniew", "a_b-c", strings.Repeat("x", 32)} {
		if err := auth.ValidateUsername(ok); err != nil {
			t.Errorf("ValidateUsername(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"", "ab", strings.Repeat("x", 33), "with space", "zażółć", "a.b", "<script>"} {
		if msg := ruleMsg(t, auth.ValidateUsername(bad)); msg != "Use 3–32 letters, digits, - or _." {
			t.Errorf("ValidateUsername(%q) msg = %q", bad, msg)
		}
	}
}

func TestPasswordRulesCountCharacters(t *testing.T) {
	if err := auth.ValidatePassword("zażółćgę", 8); err != nil { // 8 characters, 13 bytes
		t.Errorf("8 non-ASCII characters: %v, want nil", err)
	}
	if msg := ruleMsg(t, auth.ValidatePassword("zażółćg", 8)); msg != "Use at least 8 characters." { // 7 characters, 11 bytes
		t.Errorf("7 characters msg = %q", msg)
	}
	if msg := ruleMsg(t, auth.ValidatePassword("zażółćgęśl", 12)); msg != "Use at least 12 characters." {
		t.Errorf("10 characters with minimum 12 msg = %q", msg)
	}
	if msg := ruleMsg(t, auth.ValidatePassword(strings.Repeat("a", 201), 8)); msg != "Use at most 200 characters." {
		t.Errorf("201 characters msg = %q", msg)
	}
	if err := auth.ValidatePassword(strings.Repeat("ą", 200), 8); err != nil {
		t.Errorf("200 characters: %v, want nil", err)
	}
}

func TestHashPassword(t *testing.T) {
	h, err := auth.HashPassword(pw)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$m=65536,t=3,p=4$") || strings.Count(h, "$") != 5 {
		t.Fatalf("hash = %q, want argon2id text form", h)
	}
	if !auth.CheckPassword(h, pw) || auth.CheckPassword(h, pw+"x") {
		t.Fatal("CheckPassword gives a wrong answer")
	}
	other, _ := auth.HashPassword(pw)
	if other == h {
		t.Fatal("two hashes of the same password are equal (salt missing)")
	}
	for _, bad := range []string{"", "x", "$argon2i$v=19$m=65536,t=3,p=4$AAAA$AAAA", "$argon2id$v=19$m=x$AAAA$AAAA"} {
		if auth.CheckPassword(bad, pw) {
			t.Errorf("CheckPassword(%q) = true", bad)
		}
	}
}

func TestSignUpAndLogIn(t *testing.T) {
	a, _ := newAuth(t)
	ctx := context.Background()
	u, err := a.SignUp(ctx, "alice", pw)
	if err != nil || u.ID == 0 || u.Username != "alice" {
		t.Fatalf("SignUp = %+v, %v", u, err)
	}
	token, sess, err := a.LogIn(ctx, "alice", pw, false)
	if err != nil || token == "" || sess.Persistent || sess.UserID != u.ID {
		t.Fatalf("LogIn = %q, %+v, %v", token, sess, err)
	}
	if sess.TokenHash != auth.HashToken(token) || strings.Contains(sess.TokenHash, token) {
		t.Fatal("session must store only the hash of the token")
	}
	got, _, err := a.Authenticate(ctx, token)
	if err != nil || got.ID != u.ID {
		t.Fatalf("Authenticate = %+v, %v", got, err)
	}
	if _, _, err := a.LogIn(ctx, "alice", "wrong password!", false); !errors.Is(err, auth.ErrBadLogin) {
		t.Fatalf("wrong password err = %v, want ErrBadLogin", err)
	}
	if _, _, err := a.LogIn(ctx, "nobody", pw, false); !errors.Is(err, auth.ErrBadLogin) {
		t.Fatalf("unknown user err = %v, want ErrBadLogin", err)
	}
	if ruleMsg(t, func() error { _, err := a.SignUp(ctx, "x", pw); return err }()) == "" {
		t.Fatal("short username accepted")
	}
	if _, err := a.SignUp(ctx, "bob", "short"); err == nil {
		t.Fatal("short password accepted")
	}
}

func TestUsernameIgnoresCase(t *testing.T) {
	a, _ := newAuth(t)
	ctx := context.Background()
	a.SignUp(ctx, "alice", pw)
	if _, _, err := a.LogIn(ctx, "ALICE", pw, false); err != nil {
		t.Fatalf("LogIn(ALICE) = %v, want success", err)
	}
	if _, err := a.SignUp(ctx, "Alice", pw); !errors.Is(err, auth.ErrUsernameTaken) {
		t.Fatalf("SignUp(Alice) err = %v, want ErrUsernameTaken", err)
	}
}

func TestSessionExpiry(t *testing.T) {
	a, c := newAuth(t)
	ctx := context.Background()
	u, _ := a.SignUp(ctx, "alice", pw)
	short, s1, _ := a.StartSession(ctx, u.ID, false)
	long, s2, _ := a.StartSession(ctx, u.ID, true)
	if s1.ExpiresAt.Sub(s1.CreatedAt) != auth.ShortSession || s2.ExpiresAt.Sub(s2.CreatedAt) != auth.LongSession || !s2.Persistent {
		t.Fatalf("lifetimes = %v, %v", s1.ExpiresAt.Sub(s1.CreatedAt), s2.ExpiresAt.Sub(s2.CreatedAt))
	}
	c.t = c.t.Add(auth.ShortSession + time.Second)
	if _, _, err := a.Authenticate(ctx, short); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("short session after 12 h: err = %v, want ErrNoSession", err)
	}
	if _, _, err := a.Authenticate(ctx, long); err != nil {
		t.Fatalf("long session after 12 h: %v, want valid", err)
	}
	if n, err := a.CleanUp(ctx); err != nil || n != 1 {
		t.Fatalf("CleanUp = %d, %v; want 1", n, err)
	}
	c.t = c.t.Add(auth.LongSession)
	if _, _, err := a.Authenticate(ctx, long); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("long session after 30 days: err = %v, want ErrNoSession", err)
	}
	if _, _, err := a.Authenticate(ctx, ""); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("empty token: err = %v, want ErrNoSession", err)
	}
}

func TestLogInCreatesNewTokenAndLogOutEndsIt(t *testing.T) {
	a, _ := newAuth(t)
	ctx := context.Background()
	a.SignUp(ctx, "alice", pw)
	t1, _, _ := a.LogIn(ctx, "alice", pw, false)
	t2, _, _ := a.LogIn(ctx, "alice", pw, false)
	if t1 == t2 {
		t.Fatal("two logins gave the same token")
	}
	if err := a.LogOut(ctx, t1); err != nil {
		t.Fatalf("LogOut: %v", err)
	}
	if _, _, err := a.Authenticate(ctx, t1); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("logged-out token: err = %v, want ErrNoSession", err)
	}
	if _, _, err := a.Authenticate(ctx, t2); err != nil {
		t.Fatalf("other session ended by LogOut: %v", err)
	}
}

func TestChangePassword(t *testing.T) {
	a, _ := newAuth(t)
	ctx := context.Background()
	u, _ := a.SignUp(ctx, "alice", pw)
	current, _, _ := a.LogIn(ctx, "alice", pw, false)
	other, _, _ := a.LogIn(ctx, "alice", pw, true)

	if err := a.ChangePassword(ctx, u.ID, current, "wrong one!!", "new password 1"); !errors.Is(err, auth.ErrWrongPassword) {
		t.Fatalf("wrong current err = %v, want ErrWrongPassword", err)
	}
	if err := a.ChangePassword(ctx, u.ID, current, pw, "short"); ruleMsg(t, err) != "Use at least 8 characters." {
		t.Fatalf("short new password err = %v", err)
	}
	if err := a.ChangePassword(ctx, u.ID, current, pw, "new password 1"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if _, _, err := a.Authenticate(ctx, current); err != nil {
		t.Fatalf("current session ended: %v", err)
	}
	if _, _, err := a.Authenticate(ctx, other); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("other session still valid: %v", err)
	}
	if _, _, err := a.LogIn(ctx, "alice", "new password 1", false); err != nil {
		t.Fatalf("login with new password: %v", err)
	}
	if _, _, err := a.LogIn(ctx, "alice", pw, false); !errors.Is(err, auth.ErrBadLogin) {
		t.Fatalf("old password still works: %v", err)
	}
}

func TestDeleteAccount(t *testing.T) {
	a, _ := newAuth(t)
	ctx := context.Background()
	u, _ := a.SignUp(ctx, "alice", pw)
	token, _, _ := a.LogIn(ctx, "alice", pw, true)
	if err := a.DeleteAccount(ctx, u.ID, "bob"); !errors.Is(err, auth.ErrConfirmMismatch) {
		t.Fatalf("mismatch err = %v, want ErrConfirmMismatch", err)
	}
	if err := a.DeleteAccount(ctx, u.ID, " ALICE "); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}
	if _, _, err := a.Authenticate(ctx, token); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("session of deleted user: err = %v, want ErrNoSession", err)
	}
	if _, _, err := a.LogIn(ctx, "alice", pw, false); !errors.Is(err, auth.ErrBadLogin) {
		t.Fatalf("deleted user can log in: %v", err)
	}
}

func TestResetPassword(t *testing.T) {
	a, _ := newAuth(t)
	ctx := context.Background()
	a.SignUp(ctx, "alice", pw)
	token, _, _ := a.LogIn(ctx, "alice", pw, true)
	newPw, err := a.ResetPassword(ctx, "ALICE")
	if err != nil || len([]rune(newPw)) != 16 {
		t.Fatalf("ResetPassword = %q, %v; want 16 characters", newPw, err)
	}
	if _, _, err := a.Authenticate(ctx, token); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("old session still valid: %v", err)
	}
	if _, _, err := a.LogIn(ctx, "alice", newPw, false); err != nil {
		t.Fatalf("login with the new password: %v", err)
	}
	if _, err := a.ResetPassword(ctx, "nobody"); !errors.Is(err, auth.ErrNoUser) {
		t.Fatalf("unknown user: err = %v, want ErrNoUser", err)
	}
}

func TestCheckPasswordRejectsExtremeParameters(t *testing.T) {
	salt := "AAAAAAAAAAAAAAAAAAAAAA"                     // 16 bytes
	key := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" // 32 bytes
	for _, params := range []string{"m=4194304,t=3,p=4", "m=65536,t=11,p=4", "m=65536,t=3,p=17", "m=65536,t=3,p=0", "m=7,t=3,p=4", "m=524288,t=3,p=4", "m=65536,t=7,p=4", "m=131072,t=3,p=4"} {
		h := "$argon2id$v=19$" + params + "$" + salt + "$" + key
		start := time.Now()
		if auth.CheckPassword(h, pw) {
			t.Errorf("%s accepted", params)
		}
		if d := time.Since(start); d > 50*time.Millisecond {
			t.Errorf("%s took %v; must be rejected before hashing", params, d)
		}
	}
	for _, sk := range [][2]string{{"AAAA", key}, {salt, "AAAA"}} {
		if auth.CheckPassword("$argon2id$v=19$m=65536,t=3,p=4$"+sk[0]+"$"+sk[1], pw) {
			t.Errorf("short salt or key accepted: %v", sk)
		}
	}
}

func TestMinPasswordCharsOption(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	a := auth.NewService(store)
	if n := a.MinPasswordChars(); n != 8 {
		t.Fatalf("default MinPasswordChars = %d, want 8", n)
	}
	if _, err := a.SignUp(ctx, "dave", "1234567"); ruleMsg(t, err) != "Use at least 8 characters." {
		t.Fatalf("7 characters: %v", err)
	}
	if _, err := a.SignUp(ctx, "dave", "12345678"); err != nil {
		t.Fatalf("8 characters: %v", err)
	}

	strict := auth.NewService(store, auth.WithMinPasswordChars(12))
	if n := strict.MinPasswordChars(); n != 12 {
		t.Fatalf("MinPasswordChars = %d, want 12", n)
	}
	if _, err := strict.SignUp(ctx, "erin", "12345678901"); ruleMsg(t, err) != "Use at least 12 characters." {
		t.Fatalf("sign-up with 11 characters: %v", err)
	}
	u, err := strict.SignUp(ctx, "erin", "123456789012")
	if err != nil {
		t.Fatalf("sign-up with 12 characters: %v", err)
	}
	token, _, _ := strict.LogIn(ctx, "erin", "123456789012", false)
	if err := strict.ChangePassword(ctx, u.ID, token, "123456789012", "12345678901"); ruleMsg(t, err) != "Use at least 12 characters." {
		t.Fatalf("change to 11 characters: %v", err)
	}
}

// The bounds must still accept every hash the app makes itself.
func TestCheckPasswordAcceptsOwnHashes(t *testing.T) {
	h, err := auth.HashPassword(pw)
	if err != nil {
		t.Fatal(err)
	}
	if !auth.CheckPassword(h, pw) {
		t.Fatalf("own hash %s rejected", h)
	}
}

func TestResetPasswordFollowsMinimum(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	a := auth.NewService(store)
	a.SignUp(ctx, "alice", pw)
	if p, err := a.ResetPassword(ctx, "alice"); err != nil || len(p) != 16 {
		t.Fatalf("default reset = %q, %v; want 16 characters", p, err)
	}
	strict := auth.NewService(store, auth.WithMinPasswordChars(20))
	if p, err := strict.ResetPassword(ctx, "alice"); err != nil || len(p) != 20 {
		t.Fatalf("reset with minimum 20 = %q, %v; want 20 characters", p, err)
	}
}
