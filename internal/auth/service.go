package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

const (
	// ShortSession is the lifetime without "Keep me logged in".
	ShortSession = 12 * time.Hour
	// LongSession is the lifetime with "Keep me logged in".
	LongSession = 30 * 24 * time.Hour
)

// Service holds the account and session rules.
type Service struct {
	store Store
	now   func() time.Time
}

// Option changes a Service.
type Option func(*Service)

// WithClock replaces the clock (for tests).
func WithClock(now func() time.Time) Option {
	return func(s *Service) { s.now = now }
}

// NewService returns a Service that keeps accounts in store.
func NewService(store Store, opts ...Option) *Service {
	s := &Service{store: store, now: time.Now}
	for _, o := range opts {
		o(s)
	}
	return s
}

// HashToken returns the hex SHA-256 of a session token.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// SignUp creates an account. It checks the rules before the slow hash.
func (s *Service) SignUp(ctx context.Context, username, password string) (User, error) {
	if err := ValidateUsername(username); err != nil {
		return User{}, err
	}
	if err := ValidatePassword(password); err != nil {
		return User{}, err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return User{}, err
	}
	return s.store.CreateUser(ctx, username, hash, s.now())
}

// LogIn checks username and password and starts a new session.
func (s *Service) LogIn(ctx context.Context, username, password string, persistent bool) (string, Session, error) {
	if len(password) > 4*maxPasswordChars {
		return "", Session{}, ErrBadLogin // never hash huge input
	}
	u, err := s.store.UserByName(ctx, username)
	if errors.Is(err, ErrNoUser) {
		CheckPassword(dummyHash(), password)
		return "", Session{}, ErrBadLogin
	}
	if err != nil {
		return "", Session{}, err
	}
	if !CheckPassword(u.PasswordHash, password) {
		return "", Session{}, ErrBadLogin
	}
	return s.StartSession(ctx, u.ID, persistent)
}

// StartSession creates a session and returns its token.
func (s *Service) StartSession(ctx context.Context, userID int64, persistent bool) (string, Session, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", Session{}, fmt.Errorf("start session: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	now := s.now()
	life := ShortSession
	if persistent {
		life = LongSession
	}
	sess := Session{TokenHash: HashToken(token), UserID: userID, CreatedAt: now, ExpiresAt: now.Add(life), Persistent: persistent}
	if err := s.store.CreateSession(ctx, sess); err != nil {
		return "", Session{}, err
	}
	return token, sess, nil
}

// Authenticate returns the user of a valid session token.
func (s *Service) Authenticate(ctx context.Context, token string) (User, Session, error) {
	if token == "" {
		return User{}, Session{}, ErrNoSession
	}
	sess, u, err := s.store.SessionUser(ctx, HashToken(token), s.now())
	return u, sess, err
}

// LogOut ends the session of token.
func (s *Service) LogOut(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.store.DeleteSession(ctx, HashToken(token))
}

// ChangePassword sets a new password and ends all other sessions of the user.
func (s *Service) ChangePassword(ctx context.Context, userID int64, token, current, next string) error {
	if len(current) > 4*maxPasswordChars {
		return ErrWrongPassword
	}
	u, err := s.store.UserByID(ctx, userID)
	if err != nil {
		return err
	}
	if !CheckPassword(u.PasswordHash, current) {
		return ErrWrongPassword
	}
	if err := ValidatePassword(next); err != nil {
		return err
	}
	hash, err := HashPassword(next)
	if err != nil {
		return err
	}
	if err := s.store.SetPasswordHash(ctx, userID, hash); err != nil {
		return err
	}
	return s.store.DeleteOtherSessions(ctx, userID, HashToken(token))
}

// DeleteAccount deletes the user, their items, and their sessions, if confirm
// matches the username (case does not matter).
func (s *Service) DeleteAccount(ctx context.Context, userID int64, confirm string) error {
	u, err := s.store.UserByID(ctx, userID)
	if err != nil {
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(confirm), u.Username) {
		return ErrConfirmMismatch
	}
	return s.store.DeleteUser(ctx, userID)
}

// CleanUp deletes expired sessions.
func (s *Service) CleanUp(ctx context.Context) (int64, error) {
	return s.store.DeleteExpiredSessions(ctx, s.now())
}

const resetAlphabet = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// ResetPassword sets a random 16-character password and ends all sessions of the user.
func (s *Service) ResetPassword(ctx context.Context, username string) (string, error) {
	u, err := s.store.UserByName(ctx, username)
	if err != nil {
		return "", err
	}
	pw := make([]byte, 16)
	for i := range pw {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(resetAlphabet))))
		if err != nil {
			return "", fmt.Errorf("reset password: %w", err)
		}
		pw[i] = resetAlphabet[n.Int64()]
	}
	hash, err := HashPassword(string(pw))
	if err != nil {
		return "", err
	}
	if err := s.store.SetPasswordHash(ctx, u.ID, hash); err != nil {
		return "", err
	}
	if err := s.store.DeleteSessions(ctx, u.ID); err != nil {
		return "", err
	}
	return string(pw), nil
}
