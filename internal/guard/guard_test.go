package guard_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"todo/internal/guard"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func TestLimiterReserveAndRelease(t *testing.T) {
	c := &clock{t: time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)}
	l := guard.NewLimiter(3, 15*time.Minute, c.now)
	for i := 0; i < 3; i++ {
		if !l.Reserve("a") {
			t.Fatalf("reserve %d refused", i+1)
		}
	}
	if l.Reserve("a") {
		t.Fatal("4th reserve allowed, want refused")
	}
	if !l.Reserve("b") {
		t.Fatal("other key refused")
	}
	l.Release("a")
	if !l.Reserve("a") {
		t.Fatal("reserve after release refused")
	}
	c.t = c.t.Add(15*time.Minute + time.Second)
	if !l.Reserve("a") {
		t.Fatal("reserve after the window refused")
	}
	l.Release("never-used") // must not panic
}

func solve(t *testing.T, token string, bits int) string {
	t.Helper()
	for i := 0; i < 1<<22; i++ {
		if n := strconv.Itoa(i); guard.ProofOK(token, n, bits) {
			return n
		}
	}
	t.Fatal("no nonce found")
	return ""
}

func TestTokens(t *testing.T) {
	c := &clock{t: time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)}
	tk := guard.NewTokens([]byte("0123456789abcdef0123456789abcdef"), c.now)
	tok := tk.New()
	nonce := solve(t, tok, 8)

	if err := tk.Check(tok, nonce, 8); !errors.Is(err, guard.ErrTooFast) {
		t.Fatalf("at once: err = %v, want ErrTooFast", err)
	}
	c.t = c.t.Add(guard.MinFillTime)
	if err := tk.Check(tok, solveWeak(t, tok, 8), 8); !errors.Is(err, guard.ErrWeakProof) {
		t.Fatalf("weak nonce: err = %v, want ErrWeakProof", err)
	}
	if err := tk.Check(tok, nonce, 8); err != nil {
		t.Fatalf("valid token: err = %v", err)
	}
	if err := tk.Check(tok, nonce, 8); !errors.Is(err, guard.ErrUsedToken) {
		t.Fatalf("second use: err = %v, want ErrUsedToken", err)
	}

	old := tk.New()
	c.t = c.t.Add(guard.MaxTokenAge + time.Second)
	if err := tk.Check(old, solve(t, old, 8), 8); !errors.Is(err, guard.ErrTooOld) {
		t.Fatalf("old token: err = %v, want ErrTooOld", err)
	}

	other := guard.NewTokens([]byte("another key, 32 bytes long......"), c.now)
	forged := other.New()
	c.t = c.t.Add(guard.MinFillTime)
	last := "A"
	if strings.HasSuffix(tok, "A") {
		last = "B"
	}
	for _, bad := range []string{"", "x", forged, tok[:len(tok)-1] + last, strings.Replace(tok, ".", "", 1)} {
		if err := tk.Check(bad, "0", 0); !errors.Is(err, guard.ErrBadToken) {
			t.Errorf("Check(%q) err = %v, want ErrBadToken", bad, err)
		}
	}
}

// solveWeak finds a nonce that does NOT meet the difficulty.
func solveWeak(t *testing.T, token string, bits int) string {
	t.Helper()
	for i := 0; ; i++ {
		if n := strconv.Itoa(i); !guard.ProofOK(token, n, bits) {
			return n
		}
	}
}

func TestProofOK(t *testing.T) {
	if !guard.ProofOK("anything", "0", 0) {
		t.Fatal("0 bits must always pass")
	}
	if guard.ProofOK("anything", strings.Repeat("9", 100), 0) {
		t.Fatal("a nonce longer than 32 characters must fail")
	}
}

// A token stays used in every spelling the base64 decoder accepts.
func TestUsedTokenCannotBeReusedInAnotherSpelling(t *testing.T) {
	c := &clock{t: time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)}
	tk := guard.NewTokens([]byte("0123456789abcdef0123456789abcdef"), c.now)
	tok := tk.New()
	c.t = c.t.Add(guard.MinFillTime)
	if err := tk.Check(tok, "0", 0); err != nil {
		t.Fatalf("first use: %v", err)
	}
	p, sig, _ := strings.Cut(tok, ".")
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	last := strings.IndexByte(alphabet, sig[len(sig)-1])
	flipped := sig[:len(sig)-1] + string(alphabet[last^1]) // same decoded bytes (unused low bits)
	for name, v := range map[string]string{
		"newline at end":      tok + "\n",
		"newline in sig":      p + ".\n" + sig,
		"unused bits changed": p + "." + flipped,
	} {
		if err := tk.Check(v, "0", 0); err == nil {
			t.Errorf("%s: reused token accepted", name)
		}
	}
}
