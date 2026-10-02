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

const itemCols = `id, user_id, text, done, position, created_at, updated_at, due_at, due_all_day, notified_at`

// schemaV2 is the schema since user accounts (PRAGMA user_version = 2).
var schemaV2 = []string{
	`CREATE TABLE users (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		username      TEXT NOT NULL UNIQUE COLLATE NOCASE,
		password_hash TEXT NOT NULL,
		created_at    TEXT NOT NULL
	)`,
	`CREATE TABLE sessions (
		token_hash TEXT PRIMARY KEY,
		user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		created_at TEXT NOT NULL,
		expires_at TEXT NOT NULL,
		persistent INTEGER NOT NULL
	)`,
	`CREATE INDEX sessions_user ON sessions (user_id)`,
	`CREATE TABLE settings (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	)`,
	`CREATE TABLE items (
		id          INTEGER PRIMARY KEY,
		user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		text        TEXT    NOT NULL,
		done        INTEGER NOT NULL DEFAULT 0,
		position    INTEGER NOT NULL,
		created_at  TEXT    NOT NULL,
		updated_at  TEXT    NOT NULL,
		due_at      TEXT,
		due_all_day INTEGER NOT NULL DEFAULT 0,
		notified_at TEXT
	)`,
	`CREATE INDEX items_user_position ON items (user_id, position)`,
}

// Options change how OpenWith sets up the database.
type Options struct {
	// DeleteOldItems allows the step to schema version 2 to delete items
	// from before user accounts.
	DeleteOldItems bool
	// RequireCurrent opens only an existing database at the current schema
	// version and never migrates (for the admin commands).
	RequireCurrent bool
}

// OldItemsError means the database still has items from before user accounts.
type OldItemsError struct{ Count int }

func (e *OldItemsError) Error() string {
	return fmt.Sprintf("this database has %d items from before user accounts.\n"+
		"Start again with -delete-old-items to delete them and continue.", e.Count)
}

// userTables lists the tables that are not SQLite's own.
func userTables(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// migrate brings the database to schema version 2.
func migrate(db *sql.DB, opts Options) error {
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return err
	}
	if version >= 2 {
		return nil
	}
	tables, err := userTables(db)
	if err != nil {
		return err
	}
	// Before version 2 the app had only the tables users and items. Any other
	// table means -db points to another app's file: change nothing.
	hasItems := false
	var foreign []string
	for _, name := range tables {
		switch name {
		case "items":
			hasItems = true
		case "users":
		default:
			foreign = append(foreign, name)
		}
	}
	if len(foreign) > 0 {
		return fmt.Errorf("the database has tables of another app (%s); use another -db file", strings.Join(foreign, ", "))
	}
	if hasItems {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM items`).Scan(&n); err != nil {
			return err
		}
		if n > 0 && !opts.DeleteOldItems {
			return &OldItemsError{Count: n}
		}
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	steps := append([]string{
		`DROP TABLE IF EXISTS items`,
		`DROP TABLE IF EXISTS sessions`,
		`DROP TABLE IF EXISTS settings`,
		`DROP TABLE IF EXISTS users`,
	}, schemaV2...)
	steps = append(steps, `PRAGMA user_version = 2`)
	for _, stmt := range steps {
		if _, err := tx.Exec(stmt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Store is a todo.Store backed by SQLite.
type Store struct {
	db *sql.DB
}

var _ todo.Store = (*Store)(nil)

// Open opens (or creates) the database file at path with default options.
func Open(path string) (*Store, error) {
	return OpenWith(path, Options{})
}

// OpenWith opens (or creates) the database file at path and brings the
// schema to the newest version.
func OpenWith(path string, opts Options) (*Store, error) {
	// The DSN below uses "?" and "#" as separators.
	if strings.ContainsAny(path, "?#") {
		return nil, fmt.Errorf("open database %q: path must not contain '?' or '#'", path)
	}
	if opts.RequireCurrent {
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("database %q does not exist", path)
		}
		// No journal_mode pragma: the check must not change an old file. A read-write
		// connection also deletes the -wal and -shm files when it closes.
		ro, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(5000)")
		if err != nil {
			return nil, fmt.Errorf("open database %q: %w", path, err)
		}
		var version int
		err = ro.QueryRow(`PRAGMA user_version`).Scan(&version)
		ro.Close()
		if err != nil {
			return nil, fmt.Errorf("open database %q: %w", path, err)
		}
		if version != 2 {
			return nil, fmt.Errorf("database %q is not at the current schema version; start the server once to upgrade it", path)
		}
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
	if opts.RequireCurrent {
		return &Store{db: db}, nil
	}
	if err := migrate(db, opts); err != nil {
		db.Close()
		var old *OldItemsError
		if errors.As(err, &old) {
			return nil, err
		}
		return nil, fmt.Errorf("migrate schema in %q: %w", path, err)
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

func (s *Store) Create(ctx context.Context, userID int64, c todo.Change, now time.Time) (todo.Item, error) {
	ts := formatTime(now)
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO items (user_id, text, done, position, created_at, updated_at, due_at, due_all_day)
		 VALUES (?, ?, 0, (SELECT COALESCE(MAX(position), 0) + 1 FROM items WHERE user_id = ?), ?, ?, ?, ?)`,
		userID, c.Text, userID, ts, ts, formatDue(c.DueAt), c.DueAllDay)
	if err != nil {
		return todo.Item{}, fmt.Errorf("create item: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return todo.Item{}, fmt.Errorf("create item: %w", err)
	}
	return s.Get(ctx, userID, id)
}

func (s *Store) UpdateItem(ctx context.Context, userID, id int64, c todo.Change, now time.Time) (todo.Item, error) {
	if err := s.update(ctx,
		`UPDATE items SET text = ?, due_at = ?, due_all_day = ?, updated_at = ?,
		 notified_at = CASE WHEN ? THEN NULL ELSE notified_at END
		 WHERE id = ? AND user_id = ?`,
		c.Text, formatDue(c.DueAt), c.DueAllDay, formatTime(now), c.ResetNotified, id, userID); err != nil {
		return todo.Item{}, err
	}
	return s.Get(ctx, userID, id)
}

func (s *Store) ClaimDue(ctx context.Context, userID int64, now time.Time) ([]todo.Item, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("claim due items: %w", err)
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx,
		`SELECT `+itemCols+` FROM items
		 WHERE user_id = ? AND done = 0 AND due_at IS NOT NULL AND due_at <= ? AND notified_at IS NULL
		 ORDER BY due_at, id`, userID, formatDue(&now))
	if err != nil {
		return nil, fmt.Errorf("claim due items: %w", err)
	}
	items := []todo.Item{}
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, it)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("claim due items: %w", err)
	}
	ts := formatTime(now)
	claimed := items[:0]
	for _, it := range items {
		// The guard keeps an item from being claimed twice, also if the
		// store ever uses more than one connection.
		res, err := tx.ExecContext(ctx,
			`UPDATE items SET notified_at = ? WHERE id = ? AND notified_at IS NULL`, ts, it.ID)
		if err != nil {
			return nil, fmt.Errorf("claim due items: %w", err)
		}
		if n, err := res.RowsAffected(); err != nil || n != 1 {
			continue
		}
		n := now
		it.NotifiedAt = &n
		claimed = append(claimed, it)
	}
	items = claimed
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("claim due items: %w", err)
	}
	return items, nil
}

// dueLayout has a fixed length, so SQLite can compare due times as text.
const dueLayout = "2006-01-02T15:04:05Z"

// fixedTime has a fixed length, so SQLite can compare such times as text.
func fixedTime(t time.Time) string {
	return t.UTC().Format(dueLayout)
}

func formatDue(t *time.Time) any {
	if t == nil {
		return nil
	}
	return fixedTime(*t)
}

func parseNullTime(s sql.NullString) (*time.Time, error) {
	if !s.Valid {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339Nano, s.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
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
	var dueAt, notifiedAt sql.NullString
	if err := r.Scan(&it.ID, &it.UserID, &it.Text, &it.Done, &it.Position, &created, &updated,
		&dueAt, &it.DueAllDay, &notifiedAt); err != nil {
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
	if it.DueAt, err = parseNullTime(dueAt); err != nil {
		return todo.Item{}, fmt.Errorf("read item %d due_at: %w", it.ID, err)
	}
	if it.NotifiedAt, err = parseNullTime(notifiedAt); err != nil {
		return todo.Item{}, fmt.Errorf("read item %d notified_at: %w", it.ID, err)
	}
	return it, nil
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}
