package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"metadata-service/internal/model"
)

func newTestObject(bucketName, key string) model.Object {
	return model.Object{
		ID:          "object-id-" + key,
		BucketName:  bucketName,
		ObjectKey:   key,
		SizeBytes:   1024,
		ETag:        "etag-" + key,
		ContentType: "application/octet-stream",
		StorageRef:  bucketName + "/" + key,
		VersionID:   "null",
		IsLatest:    true,
		CreatedAt:   time.Now().UTC(),
	}
}

func TestBoltObjectRepository_Put_and_Get(t *testing.T) {
	repo := NewBoltObjectRepository(newTestDB(t), newTestLogger(t))
	ctx := context.Background()

	obj := newTestObject("my-bucket", "photo.png")
	if err := repo.Put(ctx, obj); err != nil {
		t.Fatalf("Put() error = %v, want nil", err)
	}

	got, err := repo.Get(ctx, "my-bucket", "photo.png")
	if err != nil {
		t.Fatalf("Get() error = %v, want nil", err)
	}
	if got.ETag != obj.ETag {
		t.Errorf("got.ETag = %q, want %q", got.ETag, obj.ETag)
	}
	if got.SizeBytes != obj.SizeBytes {
		t.Errorf("got.SizeBytes = %d, want %d", got.SizeBytes, obj.SizeBytes)
	}
}

func TestBoltObjectRepository_Put_OverwritesExisting(t *testing.T) {
	repo := NewBoltObjectRepository(newTestDB(t), newTestLogger(t))
	ctx := context.Background()

	first := newTestObject("my-bucket", "photo.png")
	first.ETag = "etag-v1"
	if err := repo.Put(ctx, first); err != nil {
		t.Fatalf("first Put() error = %v", err)
	}

	second := newTestObject("my-bucket", "photo.png")
	second.ETag = "etag-v2"
	if err := repo.Put(ctx, second); err != nil {
		t.Fatalf("second Put() error = %v", err)
	}

	got, err := repo.Get(ctx, "my-bucket", "photo.png")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.ETag != "etag-v2" {
		t.Errorf("got.ETag = %q, want %q (re-upload should overwrite)", got.ETag, "etag-v2")
	}
}

func TestBoltObjectRepository_Get_NotFound(t *testing.T) {
	repo := NewBoltObjectRepository(newTestDB(t), newTestLogger(t))

	_, err := repo.Get(context.Background(), "my-bucket", "does-not-exist.png")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() error = %v, want ErrNotFound", err)
	}
}

func TestBoltObjectRepository_Get_ExcludesSoftDeleted(t *testing.T) {
	repo := NewBoltObjectRepository(newTestDB(t), newTestLogger(t))
	ctx := context.Background()

	if err := repo.Put(ctx, newTestObject("my-bucket", "photo.png")); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if err := repo.Delete(ctx, "my-bucket", "photo.png"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err := repo.Get(ctx, "my-bucket", "photo.png")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() on soft-deleted object error = %v, want ErrNotFound", err)
	}
}

func TestBoltObjectRepository_List(t *testing.T) {
	repo := NewBoltObjectRepository(newTestDB(t), newTestLogger(t))
	ctx := context.Background()

	keys := []string{"photos/a.png", "photos/b.png", "docs/readme.txt"}
	for _, k := range keys {
		if err := repo.Put(ctx, newTestObject("my-bucket", k)); err != nil {
			t.Fatalf("Put(%q) error = %v", k, err)
		}
	}

	got, err := repo.List(ctx, "my-bucket", "photos/", 100)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("List() returned %d objects, want 2", len(got))
	}
}

func TestBoltObjectRepository_List_EmptyPrefixListsWholeBucket(t *testing.T) {
	repo := NewBoltObjectRepository(newTestDB(t), newTestLogger(t))
	ctx := context.Background()

	for _, k := range []string{"a.png", "b.png", "c.png"} {
		if err := repo.Put(ctx, newTestObject("my-bucket", k)); err != nil {
			t.Fatalf("Put(%q) error = %v", k, err)
		}
	}

	got, err := repo.List(ctx, "my-bucket", "", 100)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("List() returned %d objects, want 3", len(got))
	}
}

func TestBoltObjectRepository_List_ScopedToBucket(t *testing.T) {
	repo := NewBoltObjectRepository(newTestDB(t), newTestLogger(t))
	ctx := context.Background()

	if err := repo.Put(ctx, newTestObject("bucket-a", "shared-name.png")); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if err := repo.Put(ctx, newTestObject("bucket-b", "shared-name.png")); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	got, err := repo.List(ctx, "bucket-a", "", 100)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("List() returned %d objects, want 1 (should not see bucket-b's object)", len(got))
	}
}

func TestBoltObjectRepository_List_RespectsLimit(t *testing.T) {
	repo := NewBoltObjectRepository(newTestDB(t), newTestLogger(t))
	ctx := context.Background()

	for _, k := range []string{"a.png", "b.png", "c.png", "d.png"} {
		if err := repo.Put(ctx, newTestObject("my-bucket", k)); err != nil {
			t.Fatalf("Put(%q) error = %v", k, err)
		}
	}

	got, err := repo.List(ctx, "my-bucket", "", 2)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("List() returned %d objects, want 2 (limit)", len(got))
	}
}

func TestBoltObjectRepository_List_ExcludesSoftDeleted(t *testing.T) {
	repo := NewBoltObjectRepository(newTestDB(t), newTestLogger(t))
	ctx := context.Background()

	if err := repo.Put(ctx, newTestObject("my-bucket", "a.png")); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if err := repo.Put(ctx, newTestObject("my-bucket", "b.png")); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if err := repo.Delete(ctx, "my-bucket", "a.png"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	got, err := repo.List(ctx, "my-bucket", "", 100)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("List() returned %d objects, want 1", len(got))
	}
	if got[0].ObjectKey != "b.png" {
		t.Errorf("got[0].ObjectKey = %q, want %q", got[0].ObjectKey, "b.png")
	}
}

func TestBoltObjectRepository_Delete_NotFound(t *testing.T) {
	repo := NewBoltObjectRepository(newTestDB(t), newTestLogger(t))

	err := repo.Delete(context.Background(), "my-bucket", "does-not-exist.png")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete() error = %v, want ErrNotFound", err)
	}
}

func TestBoltObjectRepository_Delete_PreservesOtherFields(t *testing.T) {
	repo := NewBoltObjectRepository(newTestDB(t), newTestLogger(t))
	ctx := context.Background()

	obj := newTestObject("my-bucket", "photo.png")
	if err := repo.Put(ctx, obj); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	if err := repo.Delete(ctx, "my-bucket", "photo.png"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	count, err := repo.CountByBucketName(ctx, "my-bucket")
	if err != nil {
		t.Fatalf("CountByBucketName() error = %v", err)
	}
	if count != 0 {
		t.Fatalf("CountByBucketName() = %d, want 0 after delete", count)
	}
}

func TestBoltObjectRepository_CountByBucketName(t *testing.T) {
	repo := NewBoltObjectRepository(newTestDB(t), newTestLogger(t))
	ctx := context.Background()

	for _, k := range []string{"a.png", "b.png", "c.png"} {
		if err := repo.Put(ctx, newTestObject("my-bucket", k)); err != nil {
			t.Fatalf("Put(%q) error = %v", k, err)
		}
	}
	if err := repo.Delete(ctx, "my-bucket", "a.png"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	count, err := repo.CountByBucketName(ctx, "my-bucket")
	if err != nil {
		t.Fatalf("CountByBucketName() error = %v", err)
	}
	if count != 2 {
		t.Fatalf("CountByBucketName() = %d, want 2", count)
	}
}

func TestBoltObjectRepository_CountByBucketName_EmptyBucket(t *testing.T) {
	repo := NewBoltObjectRepository(newTestDB(t), newTestLogger(t))

	count, err := repo.CountByBucketName(context.Background(), "empty-bucket")
	if err != nil {
		t.Fatalf("CountByBucketName() error = %v", err)
	}
	if count != 0 {
		t.Fatalf("CountByBucketName() = %d, want 0", count)
	}
}
