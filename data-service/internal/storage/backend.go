package storage

import (
	"context"
	"io"
)

type Backend interface {
	Write(ctx context.Context, namespace string, r io.Reader) (hash string, size int64, err error)

	Read(ctx context.Context, namespace string, hash string) (io.ReadCloser, error)

	Delete(ctx context.Context, namespace string, hash string) error
}
