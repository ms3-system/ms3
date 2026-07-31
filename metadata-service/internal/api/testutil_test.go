package api

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"metadata-service/internal/model"
	"metadata-service/internal/service"
)

func newTestLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type fakeBucketService struct {
	createFn func(ctx context.Context, name, ownerID string) (model.Bucket, error)
	getFn    func(ctx context.Context, name string) (model.Bucket, error)
	listFn   func(ctx context.Context, ownerID string) ([]model.Bucket, error)
	deleteFn func(ctx context.Context, name string) error
}

var _ service.BucketService = (*fakeBucketService)(nil)

func (f *fakeBucketService) CreateBucket(ctx context.Context, name, ownerID string) (model.Bucket, error) {
	if f.createFn != nil {
		return f.createFn(ctx, name, ownerID)
	}
	return model.Bucket{}, nil
}

func (f *fakeBucketService) GetBucket(ctx context.Context, name string) (model.Bucket, error) {
	if f.getFn != nil {
		return f.getFn(ctx, name)
	}
	return model.Bucket{}, nil
}

func (f *fakeBucketService) ListBuckets(ctx context.Context, ownerID string) ([]model.Bucket, error) {
	if f.listFn != nil {
		return f.listFn(ctx, ownerID)
	}
	return nil, nil
}

func (f *fakeBucketService) DeleteBucket(ctx context.Context, name string) error {
	if f.deleteFn != nil {
		return f.deleteFn(ctx, name)
	}
	return nil
}

type fakeObjectService struct {
	putFn    func(ctx context.Context, bucketName, key string, sizeBytes int64, etag, contentType, storageRef string) (model.Object, error)
	getFn    func(ctx context.Context, bucketName, key string) (model.Object, error)
	listFn   func(ctx context.Context, bucketName, prefix string, limit int) ([]model.Object, error)
	deleteFn func(ctx context.Context, bucketName, key string) error
}

var _ service.ObjectService = (*fakeObjectService)(nil)

func (f *fakeObjectService) PutObject(ctx context.Context, bucketName, key string, sizeBytes int64, etag, contentType, storageRef string) (model.Object, error) {
	if f.putFn != nil {
		return f.putFn(ctx, bucketName, key, sizeBytes, etag, contentType, storageRef)
	}
	return model.Object{}, nil
}

func (f *fakeObjectService) GetObject(ctx context.Context, bucketName, key string) (model.Object, error) {
	if f.getFn != nil {
		return f.getFn(ctx, bucketName, key)
	}
	return model.Object{}, nil
}

func (f *fakeObjectService) ListObjects(ctx context.Context, bucketName, prefix string, limit int) ([]model.Object, error) {
	if f.listFn != nil {
		return f.listFn(ctx, bucketName, prefix, limit)
	}
	return nil, nil
}

func (f *fakeObjectService) DeleteObject(ctx context.Context, bucketName, key string) error {
	if f.deleteFn != nil {
		return f.deleteFn(ctx, bucketName, key)
	}
	return nil
}
