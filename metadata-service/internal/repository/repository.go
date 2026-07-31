package repository

import (
	"context"
	"errors"
	"metadata-service/internal/model"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrAlreadyExists  = errors.New("already exists")
	ErrBucketNotEmpty = errors.New("bucket is not empty")
)

type BucketRepository interface {
	Create(ctx context.Context, b *model.Bucket) error
	GetByName(ctx context.Context, name string) (model.Bucket, error)
	ListByOwner(ctx context.Context, ownerID string) ([]model.Bucket, error)
	Delete(ctx context.Context, name string) error
}

type ObjectRepository interface {
	Put(ctx context.Context, o *model.Object) error
	Get(ctx context.Context, bucketName, key string) (model.Object, error)
	List(ctx context.Context, bucketName, prefix string, limit int) ([]model.Object, error)
	Delete(ctx context.Context, bucketName, key string) error
	CountByBucketName(ctx context.Context, bucketName string) (int, error)
}
