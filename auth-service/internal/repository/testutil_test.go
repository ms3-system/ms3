package repository

import (
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"go.etcd.io/bbolt"

	"auth-service/internal/store"
)

func newTestDB(t *testing.T) *bbolt.DB {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("close test store: %v", err)
		}
	})

	return s.DB()
}

func newTestLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
