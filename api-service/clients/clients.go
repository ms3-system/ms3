package clients

import (
	"context"
	"fmt"
	"io"
	"time"
)

type BucketInfo struct {
	Name      string    `json:"name"`
	OwnerID   string    `json:"owner_id"`
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
	CreateBucket(ctx context.Context, name, ownerID string) (*BucketInfo, error)
	GetBucket(ctx context.Context, name string) (*BucketInfo, error)
	ListBucketsByOwner(ctx context.Context, ownerID string) ([]BucketInfo, error)
	DeleteBucket(ctx context.Context, name string) error
	ListObjects(ctx context.Context, bucket, prefix string) ([]ObjectInfo, error)
	PutObjectMeta(ctx context.Context, obj ObjectInfo) (*ObjectInfo, error)
	GetObjectMeta(ctx context.Context, bucket, key string) (*ObjectInfo, error)
	DeleteObjectMeta(ctx context.Context, bucket, key string) error

	Ping(ctx context.Context) error
}

type DataClient interface {
	Write(ctx context.Context, namespace string, r io.Reader) (hash string, size int64, err error)
	Read(ctx context.Context, namespace, hash string) (io.ReadCloser, error)
	Delete(ctx context.Context, namespace, hash string) error

	Ping(ctx context.Context) error
}

// Credential is what auth-service returns for a looked-up access key: the
// owning user and the secret key needed to verify a SigV4 signature.
type Credential struct {
	UserID    string
	SecretKey string
}

// AuthClient looks up the secret key and owning user for an access key, so
// api-service can verify SigV4 signatures without ever seeing secret keys
// at rest itself.
type AuthClient interface {
	LookupCredential(ctx context.Context, accessKey string) (*Credential, error)

	Ping(ctx context.Context) error
}

type NotFoundError struct {
	Resource string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s not found", e.Resource)
}

// ConflictError means the upstream service rejected the request because it
// conflicts with existing state (duplicate bucket name, non-empty bucket).
type ConflictError struct {
	Message string
}

func (e *ConflictError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "conflict"
}

// InvalidInputError means the upstream service rejected the request body
// as malformed or missing required fields.
type InvalidInputError struct {
	Message string
}

func (e *InvalidInputError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "invalid input"
}

// ForbiddenError means the request was authenticated (a valid SigV4
// signature) but the caller isn't allowed to perform the action — e.g.
// they aren't the bucket's owner.
type ForbiddenError struct {
	Message string
}

func (e *ForbiddenError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "forbidden"
}
