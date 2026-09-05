package service

import (
	"context"
	"errors"
	"testing"

	"metadata-service/internal/model"
	"metadata-service/internal/repository"
)

func TestBucketService_CreateBucket_RejectsInvalidName(t *testing.T) {
	svc := NewBucketService(&fakeBucketRepository{}, newTestLogger(t))

	_, err := svc.CreateBucket(context.Background(), "AB", "owner-1") // too short, uppercase
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("CreateBucket() error = %v, want ErrInvalidInput", err)
	}
}

func TestBucketService_CreateBucket_RejectsMissingOwner(t *testing.T) {
	svc := NewBucketService(&fakeBucketRepository{}, newTestLogger(t))

	_, err := svc.CreateBucket(context.Background(), "my-bucket", "")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("CreateBucket() error = %v, want ErrInvalidInput", err)
	}
}

func TestBucketService_CreateBucket_GeneratesID(t *testing.T) {
	var captured model.Bucket
	repo := &fakeBucketRepository{
		createFn: func(_ context.Context, b *model.Bucket) error {
			captured = *b
			return nil
		},
	}
	svc := NewBucketService(repo, newTestLogger(t))

	got, err := svc.CreateBucket(context.Background(), "my-bucket", "owner-1")
	if err != nil {
		t.Fatalf("CreateBucket() error = %v", err)
	}
	if captured.ID == "" {
		t.Error("repository received bucket with empty ID")
	}
	if got.ID != captured.ID {
		t.Errorf("returned bucket ID %q does not match what was persisted %q", got.ID, captured.ID)
	}
}

func TestBucketService_CreateBucket_PropagatesRepositoryError(t *testing.T) {
	repo := &fakeBucketRepository{
		createFn: func(_ context.Context, _ *model.Bucket) error {
			return repository.ErrAlreadyExists
		},
	}
	svc := NewBucketService(repo, newTestLogger(t))

	_, err := svc.CreateBucket(context.Background(), "my-bucket", "owner-1")
	if !errors.Is(err, repository.ErrAlreadyExists) {
		t.Fatalf("CreateBucket() error = %v, want ErrAlreadyExists", err)
	}
}

func TestBucketService_GetBucket_RejectsEmptyName(t *testing.T) {
	svc := NewBucketService(&fakeBucketRepository{}, newTestLogger(t))

	_, err := svc.GetBucket(context.Background(), "")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("GetBucket() error = %v, want ErrInvalidInput", err)
	}
}

func TestBucketService_ListBuckets_RejectsEmptyOwnerID(t *testing.T) {
	svc := NewBucketService(&fakeBucketRepository{}, newTestLogger(t))

	_, err := svc.ListBuckets(context.Background(), "")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("ListBuckets() error = %v, want ErrInvalidInput", err)
	}
}

func TestBucketService_DeleteBucket_PassesThroughToRepository(t *testing.T) {
	var calledWith string
	repo := &fakeBucketRepository{
		deleteFn: func(_ context.Context, name string) error {
			calledWith = name
			return nil
		},
	}
	svc := NewBucketService(repo, newTestLogger(t))

	if err := svc.DeleteBucket(context.Background(), "my-bucket"); err != nil {
		t.Fatalf("DeleteBucket() error = %v", err)
	}
	if calledWith != "my-bucket" {
		t.Errorf("repository.Delete called with %q, want %q", calledWith, "my-bucket")
	}
}
