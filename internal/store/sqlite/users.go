package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"todo/internal/auth"
)

var _ auth.Store = (*Store)(nil)

// sqliteConstraintUnique is SQLITE_CONSTRAINT_UNIQUE.
const sqliteConstraintUnique = 2067

func isUniqueViolation(err error) bool {
	var coded interface{ Code() int }
	return errors.As(err, &coded) && coded.Code() == sqliteConstraintUnique
}

const userCols = `id, username, password_hash, created_at`

func scanUser(r scanner) (auth.User, error) {
	var u auth.User
	var created string
	if err := r.Scan(&u.ID, &u.Username, &u.PasswordHash, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return auth.User{}, auth.ErrNoUser
		}
		return auth.User{}, fmt.Errorf("read user: %w", err)
	}
	var err error
	if u.CreatedAt, err = time.Parse(time.RFC3339Nano, created); err != nil {
		return auth.User{}, fmt.Errorf("read user %d created_at: %w", u.ID, err)
	}
	return u, nil
}

func (s *Store) CreateUser(ctx context.Context, username, passwordHash string, now time.Time) (auth.User, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO users (username, password_hash, created_at) VALUES (?, ?, ?)`,
		username, passwordHash, formatTime(now))
	if isUniqueViolation(err) {
		return auth.User{}, auth.ErrUsernameTaken
	}
	if err != nil {
		return auth.User{}, fmt.Errorf("create user: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return auth.User{}, fmt.Errorf("create user: %w", err)
	}
	return s.UserByID(ctx, id)
}

func (s *Store) UserByName(ctx context.Context, username string) (auth.User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE username = ?`, username))
}

func (s *Store) UserByID(ctx context.Context, id int64) (auth.User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE id = ?`, id))
}

// changeUser runs a statement that must change exactly one user row.
func (s *Store) changeUser(ctx context.Context, q string, args ...any) error {
	res, err := s.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("change user: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("change user: %w", err)
	}
	if n == 0 {
		return auth.ErrNoUser
	}
	return nil
}

// SetHideDone saves whether the user's list hides done items.
func (s *Store) SetHideDone(ctx context.Context, id int64, hide bool) error {
	return s.changeUser(ctx, `UPDATE users SET hide_done = ? WHERE id = ?`, hide, id)
}

func (s *Store) DeleteUser(ctx context.Context, id int64) error {
	return s.changeUser(ctx, `DELETE FROM users WHERE id = ?`, id)
}

func (s *Store) CreateSession(ctx context.Context, sess auth.Session) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (token_hash, user_id, created_at, expires_at, persistent) VALUES (?, ?, ?, ?, ?)`,
		sess.TokenHash, sess.UserID, formatTime(sess.CreatedAt), fixedTime(sess.ExpiresAt), sess.Persistent)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (s *Store) SessionUser(ctx context.Context, tokenHash string, now time.Time) (auth.Session, auth.User, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT s.token_hash, s.user_id, s.created_at, s.expires_at, s.persistent,
		        u.id, u.username, u.password_hash, u.created_at, u.hide_done
		 FROM sessions s JOIN users u ON u.id = s.user_id
		 WHERE s.token_hash = ? AND s.expires_at > ?`, tokenHash, fixedTime(now))
	var sess auth.Session
	var u auth.User
	var sCreated, sExpires, uCreated string
	err := row.Scan(&sess.TokenHash, &sess.UserID, &sCreated, &sExpires, &sess.Persistent,
		&u.ID, &u.Username, &u.PasswordHash, &uCreated, &u.HideDone)
	if errors.Is(err, sql.ErrNoRows) {
		return auth.Session{}, auth.User{}, auth.ErrNoSession
	}
	if err != nil {
		return auth.Session{}, auth.User{}, fmt.Errorf("read session: %w", err)
	}
	for _, f := range []struct {
		dst *time.Time
		src string
	}{{&sess.CreatedAt, sCreated}, {&sess.ExpiresAt, sExpires}, {&u.CreatedAt, uCreated}} {
		if *f.dst, err = time.Parse(time.RFC3339Nano, f.src); err != nil {
			return auth.Session{}, auth.User{}, fmt.Errorf("read session time: %w", err)
		}
	}
	return sess, u, nil
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (s *Store) DeleteExpiredSessions(ctx context.Context, now time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, fixedTime(now))
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}
	return res.RowsAffected()
}

// UserSummary is one line of the admin user list.
type UserSummary struct {
	Username  string
	CreatedAt time.Time
	Items     int
	Sessions  int // not expired at now
}

// ListUsers returns all users sorted by username (case does not matter).
func (s *Store) ListUsers(ctx context.Context, now time.Time) ([]UserSummary, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT u.username, u.created_at,
		        (SELECT COUNT(*) FROM items i WHERE i.user_id = u.id),
		        (SELECT COUNT(*) FROM sessions se WHERE se.user_id = u.id AND se.expires_at > ?)
		 FROM users u ORDER BY u.username COLLATE NOCASE`, fixedTime(now))
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	var list []UserSummary
	for rows.Next() {
		var u UserSummary
		var created string
		if err := rows.Scan(&u.Username, &created, &u.Items, &u.Sessions); err != nil {
			return nil, fmt.Errorf("list users: %w", err)
		}
		if u.CreatedAt, err = time.Parse(time.RFC3339Nano, created); err != nil {
			return nil, fmt.Errorf("list users: %w", err)
		}
		list = append(list, u)
	}
	return list, rows.Err()
}

// SetPasswordAndEndSessions sets a new password hash and deletes the user's
// sessions except keepTokenHash ("" deletes all), in one transaction.
func (s *Store) SetPasswordAndEndSessions(ctx context.Context, userID int64, passwordHash, keepTokenHash string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("set password: %w", err)
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, passwordHash, userID)
	if err != nil {
		return fmt.Errorf("set password: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return auth.ErrNoUser
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ? AND token_hash != ?`, userID, keepTokenHash); err != nil {
		return fmt.Errorf("set password: %w", err)
	}
	return tx.Commit()
}
