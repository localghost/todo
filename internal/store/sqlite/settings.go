package sqlite

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// SigningKey returns the key that signs sign-up form tokens. It is created
// at the first call and then kept in the settings table.
func (s *Store) SigningKey(ctx context.Context) ([]byte, error) {
	fresh := make([]byte, 32)
	if _, err := rand.Read(fresh); err != nil {
		return nil, fmt.Errorf("signing key: %w", err)
	}
	if _, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO settings (key, value) VALUES ('signing_key', ?)`, hex.EncodeToString(fresh)); err != nil {
		return nil, fmt.Errorf("signing key: %w", err)
	}
	var v string
	if err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = 'signing_key'`).Scan(&v); err != nil {
		return nil, fmt.Errorf("signing key: %w", err)
	}
	key, err := hex.DecodeString(v)
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("signing key: stored value is broken")
	}
	return key, nil
}
