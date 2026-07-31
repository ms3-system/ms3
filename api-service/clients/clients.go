package clients

import (
	"context"
	"fmt"
	"io"
	"time"
)

type BucketInfo struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type ObjectInfo struct {
	Key         string    `json:"object_key"`
	Bucket      string    `json:"bucket_name"`
	SizeBytes   int64     `json:"size_bytes"`
	ETag        string    `json:"etag"`
	ContentType string    `json:"content_type"`
	StorageRef  string    `json:"storage_ref"`
	CreatedAt   time.Time `json:"created_at"`
}

type MetadataClient interface {
	CreateBucket(ctx context.Context, name string) (*BucketInfo, error)
	DeleteBucket(ctx context.Context, name string) error
	ListObjects(ctx context.Context, bucket string) ([]ObjectInfo, error)
	PutObjectMeta(ctx context.Context, obj ObjectInfo) (*ObjectInfo, error)
	GetObjectMeta(ctx context.Context, bucket, key string) (*ObjectInfo, error)
	DeleteObjectMeta(ctx context.Context, bucket, key string) error
}

type DataClient interface {
	Write(ctx context.Context, namespace string, r io.Reader) (hash string, size int64, err error)
	Read(ctx context.Context, namespace, hash string) (io.ReadCloser, error)
	Delete(ctx context.Context, namespace, hash string) error
}

type NotFoundError struct {
	Resource string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s not found", e.Resource)
}
