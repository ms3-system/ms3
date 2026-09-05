package service

import (
	"context"
	"errors"
	"testing"

	"metadata-service/internal/model"
	"metadata-service/internal/repository"
)

func TestObjectService_PutObject_RejectsEmptyKey(t *testing.T) {
	svc := NewObjectService(&fakeObjectRepository{}, &fakeBucketRepository{}, newTestLogger(t))

	_, err := svc.PutObject(context.Background(), "my-bucket", "", 100, "etag", "text/plain", "ref")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("PutObject() error = %v, want ErrInvalidInput", err)
	}
}

func TestObjectService_PutObject_RejectsEmptyBucketName(t *testing.T) {
	svc := NewObjectService(&fakeObjectRepository{}, &fakeBucketRepository{}, newTestLogger(t))

	_, err := svc.PutObject(context.Background(), "", "key.png", 100, "etag", "image/png", "ref")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("PutObject() error = %v, want ErrInvalidInput", err)
	}
}

func TestObjectService_PutObject_RejectsNegativeSize(t *testing.T) {
	svc := NewObjectService(&fakeObjectRepository{}, &fakeBucketRepository{}, newTestLogger(t))

	_, err := svc.PutObject(context.Background(), "my-bucket", "key.png", -1, "etag", "image/png", "ref")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("PutObject() error = %v, want ErrInvalidInput", err)
	}
}

func TestObjectService_PutObject_RejectsEmptyStorageRef(t *testing.T) {
	svc := NewObjectService(&fakeObjectRepository{}, &fakeBucketRepository{}, newTestLogger(t))

	_, err := svc.PutObject(context.Background(), "my-bucket", "key.png", 100, "etag", "image/png", "")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("PutObject() error = %v, want ErrInvalidInput", err)
	}
}

func TestObjectService_PutObject_RejectsMissingBucket(t *testing.T) {
	buckets := &fakeBucketRepository{
		getByNameFn: func(_ context.Context, _ string) (model.Bucket, error) {
			return model.Bucket{}, repository.ErrNotFound
		},
	}
	svc := NewObjectService(&fakeObjectRepository{}, buckets, newTestLogger(t))

	_, err := svc.PutObject(context.Background(), "does-not-exist", "key.png", 100, "etag", "image/png", "ref")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("PutObject() error = %v, want ErrNotFound (bucket does not exist)", err)
	}
}

func TestObjectService_PutObject_GeneratesIDAndDefaults(t *testing.T) {
	buckets := &fakeBucketRepository{
		getByNameFn: func(_ context.Context, name string) (model.Bucket, error) {
			return model.Bucket{Name: name}, nil
		},
	}
	var captured model.Object
	objects := &fakeObjectRepository{
		putFn: func(_ context.Context, o *model.Object) error {
			captured = *o
			return nil
		},
	}
	svc := NewObjectService(objects, buckets, newTestLogger(t))

	got, err := svc.PutObject(context.Background(), "my-bucket", "photo.png", 2048, "etag-1", "image/png", "storage-ref-1")
	if err != nil {
		t.Fatalf("PutObject() error = %v", err)
	}
	if captured.ID == "" {
		t.Error("repository received object with empty ID")
	}
	if !captured.IsLatest {
		t.Error("repository received object with IsLatest = false, want true")
	}
	if captured.VersionID != "null" {
		t.Errorf("captured.VersionID = %q, want %q", captured.VersionID, "null")
	}
	if got.ID != captured.ID {
		t.Errorf("returned object ID %q does not match persisted %q", got.ID, captured.ID)
	}
}

func TestObjectService_GetObject_RejectsEmptyKey(t *testing.T) {
	svc := NewObjectService(&fakeObjectRepository{}, &fakeBucketRepository{}, newTestLogger(t))

	_, err := svc.GetObject(context.Background(), "my-bucket", "")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("GetObject() error = %v, want ErrInvalidInput", err)
	}
}

func TestObjectService_GetObject_RejectsEmptyBucketName(t *testing.T) {
	svc := NewObjectService(&fakeObjectRepository{}, &fakeBucketRepository{}, newTestLogger(t))

	_, err := svc.GetObject(context.Background(), "", "photo.png")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("GetObject() error = %v, want ErrInvalidInput", err)
	}
}

func TestObjectService_ListObjects_RejectsEmptyBucketName(t *testing.T) {
	svc := NewObjectService(&fakeObjectRepository{}, &fakeBucketRepository{}, newTestLogger(t))

	_, err := svc.ListObjects(context.Background(), "", "", 10)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("ListObjects() error = %v, want ErrInvalidInput", err)
	}
}

func TestObjectService_DeleteObject_RejectsEmptyBucketName(t *testing.T) {
	svc := NewObjectService(&fakeObjectRepository{}, &fakeBucketRepository{}, newTestLogger(t))

	err := svc.DeleteObject(context.Background(), "", "photo.png")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("DeleteObject() error = %v, want ErrInvalidInput", err)
	}
}

func TestObjectService_DeleteObject_PassesThroughToRepository(t *testing.T) {
	var calledBucket, calledKey string
	objects := &fakeObjectRepository{
		deleteFn: func(_ context.Context, bucketName, key string) error {
			calledBucket, calledKey = bucketName, key
			return nil
		},
	}
	svc := NewObjectService(objects, &fakeBucketRepository{}, newTestLogger(t))

	if err := svc.DeleteObject(context.Background(), "my-bucket", "photo.png"); err != nil {
		t.Fatalf("DeleteObject() error = %v", err)
	}
	if calledBucket != "my-bucket" || calledKey != "photo.png" {
		t.Errorf("repository.Delete called with (%q, %q), want (%q, %q)", calledBucket, calledKey, "my-bucket", "photo.png")
	}
}
