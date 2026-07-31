package localfs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"

	"github.com/naseyro/ms3/data-service/internal/storage"
)

// Ensure LocalStore implements storage.Backend at compile time
var _ storage.Backend = (*LocalStore)(nil)

type LocalStore struct {
	baseDir string
}

func NewLocalStore(baseDir string) *LocalStore {
	return &LocalStore{baseDir: baseDir}
}

func (s *LocalStore) Write(ctx context.Context, namespace string, r io.Reader) (string, int64, error) {
	tmpFile, err := os.CreateTemp(filepath.Join(s.baseDir, "tmp"), "upload-*.tmp")
	if err != nil {
		return "", 0, err
	}
	defer tmpFile.Close()

	hasher := sha256.New()
	multiWriter := io.MultiWriter(tmpFile, hasher)

	size, err := io.Copy(multiWriter, r)
	if err != nil {
		// Remove the file if the write operation is not complete.
		os.Remove(tmpFile.Name())
		return "", 0, err
	}

	hash := hex.EncodeToString(hasher.Sum(nil))
	finalPath := s.getShardedPath(namespace, hash)

	os.MkdirAll(filepath.Dir(finalPath), 0755)

	err = os.Rename(tmpFile.Name(), finalPath)
	return hash, size, err
}

func (s *LocalStore) getShardedPath(namespace, hash string) string {
	return filepath.Join(s.baseDir, namespace, "objects", hash[0:2], hash[2:4], hash)
}

// Read opens a stream to the physical data by its hash.
func (s *LocalStore) Read(ctx context.Context, namespace string, hash string) (io.ReadCloser, error) {
	// Reconstruct the physical path on disk
	path := s.getShardedPath(namespace, hash)

	file, err := os.Open(path)
	if err != nil {
		// File does not exist
		return nil, err
	}

	return file, nil
}

// Delete removes the physical data and checks directory cleanup.
func (s *LocalStore) Delete(ctx context.Context, namespace string, hash string) error {
	path := s.getShardedPath(namespace, hash)

	err := os.Remove(path)

	// If the file doesn't exist, we treat the deletion as successful operation.
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// Check to remove empty dirs if exist.
	s.cleanUpEmptyDirs(filepath.Dir(path))

	return nil
}

// cleanUpEmptyDirs removes only empty Dirs, stops when we hit
// the parent data directory or a filled dir
func (s *LocalStore) cleanUpEmptyDirs(dir string) {
	objectsDir := filepath.Join(s.baseDir, "objects")

	for dir != objectsDir && dir != "." && dir != "/" {
		err := os.Remove(dir)
		if err != nil {
			break
		}
		// == We deleted the directory, so we check the parent directory.
		dir = filepath.Dir(dir)
	}
}
