package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

const DefaultDBPath = "data/metadata.db"

type Store struct {
	db *bolt.DB
}

func (s *Store) DB() *bolt.DB {
	return s.db
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir for %q: %w", path, err)
	}

	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: time.Second})
	if err != nil {
		return nil, fmt.Errorf("open bbolt file %q: %w", path, err)
	}

	s := &Store{db: db}
	if err := s.bootstrap(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("bootstrap: %w", err)
	}

	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

// Ready reports whether the underlying bbolt database can still service a
// transaction, so it can back a k8s readiness probe.
func (s *Store) Ready(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.db.View(func(tx *bolt.Tx) error { return nil })
}

func (s *Store) bootstrap() error {
	return s.db.Update(func(tx *bolt.Tx) error {
		for _, name := range allBoltBuckets {
			if _, err := tx.CreateBucketIfNotExists([]byte(name)); err != nil {
				return fmt.Errorf("create bolt-bucket %q: %w", name, err)
			}
		}
		return checkSchemaVersion(tx)
	})
}
