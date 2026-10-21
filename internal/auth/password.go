package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"golang.org/x/crypto/argon2"
)

const (
	argonMemory  = 64 * 1024 // KiB
	argonTime    = 3
	argonThreads = 4
	argonSalt    = 16
	argonKey     = 32
)

var b64 = base64.RawStdEncoding

// hashSlotCount limits parallel argon2 hashes: each needs 64 MiB, so many
// parallel logins could otherwise use all memory. Two fit in a 256 MB machine.
const hashSlotCount = 2

var hashSlots = make(chan struct{}, hashSlotCount)

// maxHashWaiters limits the requests that wait for a hash slot. When more
// wait, new work fails at once with ErrBusy, so a flood of password checks
// cannot make every login wait until it times out (code review 2).
const maxHashWaiters = 8

var hashWaiters atomic.Int32

// ErrBusy means too many password checks wait already; try again later.
var ErrBusy = errors.New("auth: too many password checks at once")

// withHashSlot runs f when one of the hash slots is free. It returns ErrBusy
// if too many requests wait already, and ctx's error if ctx ends first.
func withHashSlot(ctx context.Context, f func()) error {
	select {
	case hashSlots <- struct{}{}:
	default:
		if hashWaiters.Add(1) > maxHashWaiters {
			hashWaiters.Add(-1)
			return ErrBusy
		}
		select {
		case hashSlots <- struct{}{}:
			hashWaiters.Add(-1)
		case <-ctx.Done():
			hashWaiters.Add(-1)
			return ctx.Err()
		}
	}
	defer func() { <-hashSlots }()
	f()
	return nil
}

// HashPassword returns the argon2id hash of password in its standard text form.
func HashPassword(ctx context.Context, password string) (string, error) {
	var hash string
	var err error
	if slotErr := withHashSlot(ctx, func() { hash, err = hashPassword(password) }); slotErr != nil {
		return "", slotErr
	}
	return hash, err
}

// hashPassword is HashPassword without a hash slot.
func hashPassword(password string) (string, error) {
	salt := make([]byte, argonSalt)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKey)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// CheckPassword reports whether password matches the stored hash. It reads
// the parameters from the hash, so they can change later. The error is
// ErrBusy or ctx's error when the check could not run.
func CheckPassword(ctx context.Context, encoded, password string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return false, nil
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, nil
	}
	var memory, time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false, nil
	}
	// Bounds stop a broken or hostile stored hash from using huge memory or time:
	// at most the app's own memory (so hashSlotCount checks fit in the machine) and
	// 2× its time. If argonMemory is ever lowered, raise this bound for old hashes first.
	if memory < 8*1024 || memory > argonMemory || time == 0 || time > 2*argonTime || threads == 0 || threads > 16 {
		return false, nil
	}
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return false, nil
	}
	key, err := b64.DecodeString(parts[5])
	if err != nil {
		return false, nil
	}
	if len(salt) < 8 || len(salt) > 64 || len(key) < 16 || len(key) > 64 {
		return false, nil
	}
	var got []byte
	if err := withHashSlot(ctx, func() { got = argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(key))) }); err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare(got, key) == 1, nil
}

// dummyHash is checked for unknown usernames, so the answer takes as long
// as for a real user.
var dummyHash = sync.OnceValue(func() string {
	h, _ := hashPassword("not a real password, only for timing") // once, outside the slots
	return h
})
