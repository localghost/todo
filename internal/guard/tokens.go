package guard

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"math/bits"
	"strings"
	"sync"
	"time"
)

const (
	// MinFillTime is the shortest time between showing and sending the form.
	MinFillTime = 3 * time.Second
	// MaxTokenAge is the longest time a form token stays valid.
	MaxTokenAge = time.Hour
	// DefaultPowBits is the proof-of-work difficulty (about one second in a browser).
	DefaultPowBits = 16
)

var (
	ErrBadToken  = errors.New("guard: bad form token")
	ErrTooFast   = errors.New("guard: form sent too fast")
	ErrTooOld    = errors.New("guard: form token too old")
	ErrUsedToken = errors.New("guard: form token already used")
	ErrWeakProof = errors.New("guard: proof of work too weak")
)

var b64 = base64.RawURLEncoding

// Tokens makes and checks signed sign-up form tokens.
type Tokens struct {
	key []byte
	now func() time.Time

	mu   sync.Mutex
	used map[string]time.Time
}

// NewTokens signs tokens with key.
func NewTokens(key []byte, now func() time.Time) *Tokens {
	return &Tokens{key: key, now: now, used: map[string]time.Time{}}
}

// New returns a token: base64url(8-byte Unix seconds + 16 random bytes) "." base64url(HMAC).
func (t *Tokens) New() string {
	payload := make([]byte, 24)
	binary.BigEndian.PutUint64(payload, uint64(t.now().Unix()))
	rand.Read(payload[8:])
	p := b64.EncodeToString(payload)
	return p + "." + b64.EncodeToString(t.sign(p))
}

func (t *Tokens) sign(p string) []byte {
	m := hmac.New(sha256.New, t.key)
	m.Write([]byte(p))
	return m.Sum(nil)
}

// Check verifies the signature, the age, the proof of work, and one use.
func (t *Tokens) Check(token, nonce string, bits int) error {
	p, sig, ok := strings.Cut(token, ".")
	if !ok {
		return ErrBadToken
	}
	got, err := b64.DecodeString(sig)
	if err != nil || !hmac.Equal(got, t.sign(p)) {
		return ErrBadToken
	}
	payload, err := b64.DecodeString(p)
	if err != nil || len(payload) != 24 {
		return ErrBadToken
	}
	age := t.now().Sub(time.Unix(int64(binary.BigEndian.Uint64(payload)), 0))
	switch {
	case age < MinFillTime:
		return ErrTooFast
	case age > MaxTokenAge:
		return ErrTooOld
	}
	if !ProofOK(token, nonce, bits) {
		return ErrWeakProof
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, seen := t.used[token]; seen {
		return ErrUsedToken
	}
	if len(t.used) > 1000 {
		for k, at := range t.used {
			if t.now().Sub(at) > MaxTokenAge {
				delete(t.used, k)
			}
		}
	}
	t.used[token] = t.now()
	return nil
}

// ProofOK reports whether SHA-256(token + ":" + nonce) starts with at least
// bits zero bits. Nonces longer than 32 characters never pass.
func ProofOK(token, nonce string, want int) bool {
	if len(nonce) > 32 {
		return false
	}
	sum := sha256.Sum256([]byte(token + ":" + nonce))
	zeros := 0
	for _, b := range sum {
		if b == 0 {
			zeros += 8
			continue
		}
		zeros += bits.LeadingZeros8(b)
		break
	}
	return zeros >= want
}
