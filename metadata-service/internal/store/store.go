package store

import (
	"fmt"

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
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 1 * 1000000000})
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
