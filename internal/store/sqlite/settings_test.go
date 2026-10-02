package sqlite_test

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"todo/internal/store/sqlite"
)

func TestSigningKeyIsCreatedOnceAndKept(t *testing.T) {
	path := filepath.Join(t.TempDir(), "k.db")
	s, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	k1, err := s.SigningKey(context.Background())
	if err != nil || len(k1) != 32 {
		t.Fatalf("SigningKey = %x, %v; want 32 bytes", k1, err)
	}
	k2, _ := s.SigningKey(context.Background())
	s.Close()
	s, _ = sqlite.Open(path)
	defer s.Close()
	k3, _ := s.SigningKey(context.Background())
	if !bytes.Equal(k1, k2) || !bytes.Equal(k1, k3) {
		t.Fatal("signing key changed")
	}
}
