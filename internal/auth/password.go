package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"

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
// parallel logins could otherwise use all memory.
const hashSlotCount = 4

var hashSlots = make(chan struct{}, hashSlotCount)

// withHashSlot runs f when one of the hash slots is free.
func withHashSlot(f func()) {
	hashSlots <- struct{}{}
	defer func() { <-hashSlots }()
	f()
}

// HashPassword returns the argon2id hash of password in its standard text form.
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSalt)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	var key []byte
	withHashSlot(func() { key = argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKey) })
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// CheckPassword reports whether password matches the stored hash. It reads
// the parameters from the hash, so they can change later.
func CheckPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return false
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false
	}
	var memory, time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil || threads == 0 || time == 0 {
		return false
	}
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return false
	}
	key, err := b64.DecodeString(parts[5])
	if err != nil || len(key) == 0 {
		return false
	}
	var got []byte
	withHashSlot(func() { got = argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(key))) })
	return subtle.ConstantTimeCompare(got, key) == 1
}

// dummyHash is checked for unknown usernames, so the answer takes as long
// as for a real user.
var dummyHash = sync.OnceValue(func() string {
	h, _ := HashPassword("not a real password, only for timing")
	return h
})
