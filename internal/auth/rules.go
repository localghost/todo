package auth

import (
	"fmt"
	"regexp"
	"unicode/utf8"
)

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{3,32}$`)

const (
	// MaxPasswordChars is the longest allowed password, in characters.
	MaxPasswordChars = 200
	// DefaultMinPasswordChars is the shortest allowed password when config.yaml sets none.
	DefaultMinPasswordChars = 8
)

// ValidateUsername checks the characters and the length of a username.
func ValidateUsername(name string) error {
	if !usernamePattern.MatchString(name) {
		return &RuleError{Field: "username", Msg: "Use 3–32 letters, digits, - or _."}
	}
	return nil
}

// ValidatePassword checks the length of a password in characters: at least
// minChars and at most MaxPasswordChars.
func ValidatePassword(password string, minChars int) error {
	n := utf8.RuneCountInString(password)
	if n < minChars {
		return &RuleError{Field: "password", Msg: fmt.Sprintf("Use at least %d characters.", minChars)}
	}
	if n > MaxPasswordChars {
		return &RuleError{Field: "password", Msg: "Use at most 200 characters."}
	}
	return nil
}
