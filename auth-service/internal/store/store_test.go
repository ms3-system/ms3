package store

import (
	"path/filepath"
	"testing"

	bolt "go.etcd.io/bbolt"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	return s
}

func TestOpen_CreatesAllBoltBuckets(t *testing.T) {
	s := openTestStore(t)

	err := s.DB().View(func(tx *bolt.Tx) error {
		for _, name := range allBoltBuckets {
			if tx.Bucket([]byte(name)) == nil {
				t.Errorf("bolt-bucket %q was not created", name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("View() error = %v", err)
	}
}

func TestOpen_StampsSchemaVersion(t *testing.T) {
	s := openTestStore(t)

	err := s.DB().View(func(tx *bolt.Tx) error {
		raw := tx.Bucket([]byte(BoltBucketMeta)).Get(schemaVersionKey)
		if raw == nil {
			t.Fatal("schema_version was not stamped")
		}
		if got := decodeVersion(raw); got != currentSchemaVersion {
			t.Errorf("schema_version = %d, want %d", got, currentSchemaVersion)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("View() error = %v", err)
	}
}

func TestOpen_ReopenExistingFileSucceeds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	s1, err := Open(path)
	if err != nil {
		t.Fatalf("first Open() error = %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	s2, err := Open(path)
	if err != nil {
		t.Fatalf("second Open() error = %v", err)
	}
	if err := s2.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestOpen_CreatesParentDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "test.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}
