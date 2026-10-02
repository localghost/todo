// Package auth holds user accounts, password rules, and login sessions.
package auth

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrUsernameTaken means another user has this username (case does not matter).
	ErrUsernameTaken = errors.New("auth: username taken")
	// ErrNoUser means there is no user with this name or ID.
	ErrNoUser = errors.New("auth: no such user")
	// ErrNoSession means the token is unknown or its session has expired.
	ErrNoSession = errors.New("auth: no valid session")
	// ErrBadLogin means the username or the password is wrong.
	ErrBadLogin = errors.New("auth: wrong username or password")
	// ErrWrongPassword means the current password given for a change is wrong.
	ErrWrongPassword = errors.New("auth: current password is wrong")
	// ErrConfirmMismatch means the typed username does not match the account.
	ErrConfirmMismatch = errors.New("auth: confirmation does not match")
)

// RuleError is a broken username or password rule, with a message for the user.
type RuleError struct {
	Field string // "username" or "password"
	Msg   string
}

func (e *RuleError) Error() string { return "auth: " + e.Field + ": " + e.Msg }

// User is one account.
type User struct {
	ID           int64
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}

// Session is one login. Only the hash of the token is stored.
type Session struct {
	TokenHash  string
	UserID     int64
	CreatedAt  time.Time
	ExpiresAt  time.Time
	Persistent bool
}

// Store keeps users and sessions.
type Store interface {
	CreateUser(ctx context.Context, username, passwordHash string, now time.Time) (User, error)            // ErrUsernameTaken
	UserByName(ctx context.Context, username string) (User, error)                                         // ErrNoUser; case does not matter
	UserByID(ctx context.Context, id int64) (User, error)                                                  // ErrNoUser
	SetPasswordAndEndSessions(ctx context.Context, userID int64, passwordHash, keepTokenHash string) error // ErrNoUser; keep "" ends all
	DeleteUser(ctx context.Context, id int64) error                                                        // ErrNoUser; deletes items and sessions too
	CreateSession(ctx context.Context, s Session) error
	SessionUser(ctx context.Context, tokenHash string, now time.Time) (Session, User, error) // ErrNoSession if unknown or expired
	DeleteSession(ctx context.Context, tokenHash string) error
	DeleteExpiredSessions(ctx context.Context, now time.Time) (int64, error)
}
