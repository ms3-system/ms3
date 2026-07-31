package service

import (
	"context"
	"metadata-service/internal/model"
)

type BucketService interface {
	CreateBucket(ctx context.Context, name, ownerID string) (model.Bucket, error)
	DeleteBucket(ctx context.Context, name string) error
	GetBucket(ctx context.Context, name string) (model.Bucket, error)
	ListBuckets(ctx context.Context, ownerID string) ([]model.Bucket, error)
}

type ObjectService interface {
	PutObject(ctx context.Context, bucketName, key string, sizeBytes int64, etag, contentType, storageRef string) (model.Object, error)
	DeleteObject(ctx context.Context, bucketName, key string) error
	GetObject(ctx context.Context, bucketName, key string) (model.Object, error)
	ListObjects(ctx context.Context, bucketName, prefix string, limit int) ([]model.Object, error)
}
