package service

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"metadata-service/internal/model"
)

func newTestLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type fakeBucketRepository struct {
	createFn      func(ctx context.Context, b *model.Bucket) error
	getByNameFn   func(ctx context.Context, name string) (model.Bucket, error)
	listByOwnerFn func(ctx context.Context, ownerID string) ([]model.Bucket, error)
	deleteFn      func(ctx context.Context, name string) error
}

func (f *fakeBucketRepository) Create(ctx context.Context, b *model.Bucket) error {
	if f.createFn != nil {
		return f.createFn(ctx, b)
	}
	return nil
}

func (f *fakeBucketRepository) GetByName(ctx context.Context, name string) (model.Bucket, error) {
	if f.getByNameFn != nil {
		return f.getByNameFn(ctx, name)
	}
	return model.Bucket{}, nil
}

func (f *fakeBucketRepository) ListByOwner(ctx context.Context, ownerID string) ([]model.Bucket, error) {
	if f.listByOwnerFn != nil {
		return f.listByOwnerFn(ctx, ownerID)
	}
	return nil, nil
}

func (f *fakeBucketRepository) Delete(ctx context.Context, name string) error {
	if f.deleteFn != nil {
		return f.deleteFn(ctx, name)
	}
	return nil
}

type fakeObjectRepository struct {
	putFn           func(ctx context.Context, o *model.Object) error
	getFn           func(ctx context.Context, bucketName, key string) (model.Object, error)
	listFn          func(ctx context.Context, bucketName, prefix string, limit int) ([]model.Object, error)
	deleteFn        func(ctx context.Context, bucketName, key string) error
	countByBucketFn func(ctx context.Context, bucketName string) (int, error)
}

func (f *fakeObjectRepository) Put(ctx context.Context, o *model.Object) error {
	if f.putFn != nil {
		return f.putFn(ctx, o)
	}
	return nil
}

func (f *fakeObjectRepository) Get(ctx context.Context, bucketName, key string) (model.Object, error) {
	if f.getFn != nil {
		return f.getFn(ctx, bucketName, key)
	}
	return model.Object{}, nil
}

func (f *fakeObjectRepository) List(ctx context.Context, bucketName, prefix string, limit int) ([]model.Object, error) {
	if f.listFn != nil {
		return f.listFn(ctx, bucketName, prefix, limit)
	}
	return nil, nil
}

func (f *fakeObjectRepository) Delete(ctx context.Context, bucketName, key string) error {
	if f.deleteFn != nil {
		return f.deleteFn(ctx, bucketName, key)
	}
	return nil
}

func (f *fakeObjectRepository) CountByBucketName(ctx context.Context, bucketName string) (int, error) {
	if f.countByBucketFn != nil {
		return f.countByBucketFn(ctx, bucketName)
	}
	return 0, nil
}
