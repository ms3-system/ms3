package storage

import (
	"context"
	"io"
)

// Backend defines the behavior for any physical storage medium
type Backend interface {
	// Write consumes a stream, calculates the hash, stores data on disk,
	// and returns the final hash and file size.
	Write(ctx context.Context, namespace string, r io.Reader) (hash string, size int64, err error)

	// Read opens a stream to an existing stored data by its hash.
	Read(ctx context.Context, namespace string, hash string) (io.ReadCloser, error)

	// Delete removes the physical payload (used by GC).
	Delete(ctx context.Context, namespace string, hash string) error
}
