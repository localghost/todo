package auth

import (
	"regexp"
	"unicode/utf8"
)

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{3,32}$`)

const (
	minPasswordChars = 10
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

// ValidatePassword checks the length of a password in characters.
func ValidatePassword(password string) error {
	n := utf8.RuneCountInString(password)
	if n < minPasswordChars {
		return &RuleError{Field: "password", Msg: "Use at least 10 characters."}
	}
	if n > MaxPasswordChars {
		return &RuleError{Field: "password", Msg: "Use at most 200 characters."}
	}
	return nil
}
