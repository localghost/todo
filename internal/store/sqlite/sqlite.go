// Package sqlite stores todo items in a SQLite database file.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // registers the "sqlite" driver

	"todo/internal/todo"
)

var schema = []string{
	`CREATE TABLE IF NOT EXISTS users (
		id   INTEGER PRIMARY KEY,
		name TEXT NOT NULL
	)`,
	`INSERT OR IGNORE INTO users (id, name) VALUES (1, 'default')`,
	`CREATE TABLE IF NOT EXISTS items (
		id         INTEGER PRIMARY KEY,
		user_id    INTEGER NOT NULL REFERENCES users(id),
		text       TEXT    NOT NULL,
		done       INTEGER NOT NULL DEFAULT 0,
		position   INTEGER NOT NULL,
		created_at TEXT    NOT NULL,
		updated_at TEXT    NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS items_user_position ON items (user_id, position)`,
}

const itemCols = `id, user_id, text, done, position, created_at, updated_at`

// Store is a todo.Store backed by SQLite.
type Store struct {
	db *sql.DB
}

var _ todo.Store = (*Store)(nil)

// Open opens (or creates) the database file at path and sets up the schema.
func Open(path string) (*Store, error) {
	// The DSN below uses "?" and "#" as separators.
	if strings.ContainsAny(path, "?#") {
		return nil, fmt.Errorf("open database %q: path must not contain '?' or '#'", path)
	}
	// Without this check, SQLite reports a missing folder as "out of memory".
	if dir := filepath.Dir(path); dir != "" {
		if _, err := os.Stat(dir); err != nil {
			return nil, fmt.Errorf("open database %q: folder %q does not exist", path, dir)
		}
	}
	dsn := "file:" + path + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database %q: %w", path, err)
	}
	// One connection avoids "database is locked" errors between writers.
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("open database %q: %w", path, err)
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			db.Close()
			return nil, fmt.Errorf("set up schema in %q: %w", path, err)
		}
	}
	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) List(ctx context.Context, userID int64, hideDone bool) ([]todo.Item, error) {
	q := `SELECT ` + itemCols + ` FROM items WHERE user_id = ?`
	if hideDone {
		q += ` AND done = 0`
	}
	q += ` ORDER BY position, id`
	rows, err := s.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}
	defer rows.Close()
	items := []todo.Item{}
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}
	return items, nil
}

func (s *Store) Get(ctx context.Context, userID, id int64) (todo.Item, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+itemCols+` FROM items WHERE id = ? AND user_id = ?`, id, userID)
	it, err := scanItem(row)
	if errors.Is(err, sql.ErrNoRows) {
		return todo.Item{}, todo.ErrNotFound
	}
	return it, err
}

func (s *Store) Create(ctx context.Context, userID int64, text string, now time.Time) (todo.Item, error) {
	ts := formatTime(now)
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO items (user_id, text, done, position, created_at, updated_at)
		 VALUES (?, ?, 0, (SELECT COALESCE(MAX(position), 0) + 1 FROM items WHERE user_id = ?), ?, ?)`,
		userID, text, userID, ts, ts)
	if err != nil {
		return todo.Item{}, fmt.Errorf("create item: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return todo.Item{}, fmt.Errorf("create item: %w", err)
	}
	return s.Get(ctx, userID, id)
}

func (s *Store) UpdateText(ctx context.Context, userID, id int64, text string, now time.Time) (todo.Item, error) {
	if err := s.update(ctx, `UPDATE items SET text = ?, updated_at = ? WHERE id = ? AND user_id = ?`,
		text, formatTime(now), id, userID); err != nil {
		return todo.Item{}, err
	}
	return s.Get(ctx, userID, id)
}

func (s *Store) SetDone(ctx context.Context, userID, id int64, done bool, now time.Time) (todo.Item, error) {
	if err := s.update(ctx, `UPDATE items SET done = ?, updated_at = ? WHERE id = ? AND user_id = ?`,
		done, formatTime(now), id, userID); err != nil {
		return todo.Item{}, err
	}
	return s.Get(ctx, userID, id)
}

func (s *Store) Delete(ctx context.Context, userID, id int64) error {
	return s.update(ctx, `DELETE FROM items WHERE id = ? AND user_id = ?`, id, userID)
}

func (s *Store) Counts(ctx context.Context, userID int64) (open, done int, err error) {
	err = s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(done = 0), 0), COALESCE(SUM(done = 1), 0) FROM items WHERE user_id = ?`,
		userID).Scan(&open, &done)
	if err != nil {
		return 0, 0, fmt.Errorf("count items: %w", err)
	}
	return open, done, nil
}

// update runs a statement that must change exactly one row.
func (s *Store) update(ctx context.Context, q string, args ...any) error {
	res, err := s.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("update item: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update item: %w", err)
	}
	if n == 0 {
		return todo.ErrNotFound
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanItem(r scanner) (todo.Item, error) {
	var it todo.Item
	var created, updated string
	if err := r.Scan(&it.ID, &it.UserID, &it.Text, &it.Done, &it.Position, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return todo.Item{}, err
		}
		return todo.Item{}, fmt.Errorf("read item: %w", err)
	}
	var err error
	if it.CreatedAt, err = time.Parse(time.RFC3339Nano, created); err != nil {
		return todo.Item{}, fmt.Errorf("read item %d created_at: %w", it.ID, err)
	}
	if it.UpdatedAt, err = time.Parse(time.RFC3339Nano, updated); err != nil {
		return todo.Item{}, fmt.Errorf("read item %d updated_at: %w", it.ID, err)
	}
	return it, nil
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}
