package repository

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"go.etcd.io/bbolt"

	"metadata-service/internal/model"
	"metadata-service/internal/repository/bolt"
	"metadata-service/internal/store"
)

func newTestBucket(name, ownerID string) model.Bucket {
	return model.Bucket{
		ID:        "bucket-id-" + name,
		Name:      name,
		OwnerID:   ownerID,
		CreatedAt: time.Now().UTC(),
	}
}

func TestBoltBucketRepository_Create(t *testing.T) {
	repo := NewBoltBucketRepository(newTestDB(t), newTestLogger(t))
	ctx := context.Background()

	if err := repo.Create(ctx, newTestBucket("my-bucket", "owner-1")); err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	got, err := repo.GetByName(ctx, "my-bucket")
	if err != nil {
		t.Fatalf("GetByName() error = %v, want nil", err)
	}
	if got.OwnerID != "owner-1" {
		t.Errorf("got.OwnerID = %q, want %q", got.OwnerID, "owner-1")
	}
}

func TestBoltBucketRepository_Create_DuplicateName(t *testing.T) {
	repo := NewBoltBucketRepository(newTestDB(t), newTestLogger(t))
	ctx := context.Background()

	if err := repo.Create(ctx, newTestBucket("my-bucket", "owner-1")); err != nil {
		t.Fatalf("first Create() error = %v", err)
	}

	err := repo.Create(ctx, newTestBucket("my-bucket", "owner-2"))
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("Create() error = %v, want ErrAlreadyExists", err)
	}
}

func TestBoltBucketRepository_Create_StampsCreatedAtWhenUnset(t *testing.T) {
	repo := NewBoltBucketRepository(newTestDB(t), newTestLogger(t))
	ctx := context.Background()

	before := time.Now().UTC()
	err := repo.Create(ctx, model.Bucket{ID: "b-1", Name: "my-bucket", OwnerID: "owner-1"}) // CreatedAt intentionally omitted
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	after := time.Now().UTC()

	got, err := repo.GetByName(ctx, "my-bucket")
	if err != nil {
		t.Fatalf("GetByName() error = %v", err)
	}
	if got.CreatedAt.IsZero() {
		t.Fatal("got.CreatedAt is zero, want it stamped to roughly now")
	}
	if got.CreatedAt.Before(before) || got.CreatedAt.After(after) {
		t.Errorf("got.CreatedAt = %v, want between %v and %v", got.CreatedAt, before, after)
	}
}

func TestBoltBucketRepository_Create_PreservesExplicitCreatedAt(t *testing.T) {
	repo := NewBoltBucketRepository(newTestDB(t), newTestLogger(t))
	ctx := context.Background()

	historical := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	err := repo.Create(ctx, model.Bucket{ID: "b-1", Name: "my-bucket", OwnerID: "owner-1", CreatedAt: historical})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := repo.GetByName(ctx, "my-bucket")
	if err != nil {
		t.Fatalf("GetByName() error = %v", err)
	}
	if !got.CreatedAt.Equal(historical) {
		t.Errorf("got.CreatedAt = %v, want preserved value %v", got.CreatedAt, historical)
	}
}

func TestBoltBucketRepository_Create_ResurrectsSoftDeletedName(t *testing.T) {
	repo := NewBoltBucketRepository(newTestDB(t), newTestLogger(t))
	ctx := context.Background()

	if err := repo.Create(ctx, newTestBucket("my-bucket", "owner-1")); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := repo.Delete(ctx, "my-bucket"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if err := repo.Create(ctx, newTestBucket("my-bucket", "owner-2")); err != nil {
		t.Fatalf("Create() over soft-deleted name error = %v, want nil", err)
	}

	got, err := repo.GetByName(ctx, "my-bucket")
	if err != nil {
		t.Fatalf("GetByName() error = %v", err)
	}
	if got.OwnerID != "owner-2" {
		t.Errorf("got.OwnerID = %q, want %q (resurrect should overwrite owner)", got.OwnerID, "owner-2")
	}
}

func TestBoltBucketRepository_GetByName_NotFound(t *testing.T) {
	repo := NewBoltBucketRepository(newTestDB(t), newTestLogger(t))

	_, err := repo.GetByName(context.Background(), "does-not-exist")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetByName() error = %v, want ErrNotFound", err)
	}
}

func TestBoltBucketRepository_GetByName_ExcludesSoftDeleted(t *testing.T) {
	repo := NewBoltBucketRepository(newTestDB(t), newTestLogger(t))
	ctx := context.Background()

	if err := repo.Create(ctx, newTestBucket("my-bucket", "owner-1")); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := repo.Delete(ctx, "my-bucket"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err := repo.GetByName(ctx, "my-bucket")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetByName() on soft-deleted bucket error = %v, want ErrNotFound", err)
	}
}

func TestBoltBucketRepository_ListByOwner(t *testing.T) {
	repo := NewBoltBucketRepository(newTestDB(t), newTestLogger(t))
	ctx := context.Background()

	for _, name := range []string{"bucket-a", "bucket-b"} {
		if err := repo.Create(ctx, newTestBucket(name, "owner-1")); err != nil {
			t.Fatalf("Create(%q) error = %v", name, err)
		}
	}
	if err := repo.Create(ctx, newTestBucket("bucket-c", "owner-2")); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := repo.ListByOwner(ctx, "owner-1")
	if err != nil {
		t.Fatalf("ListByOwner() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListByOwner() returned %d buckets, want 2", len(got))
	}
}

func TestBoltBucketRepository_ListByOwner_ExcludesSoftDeleted(t *testing.T) {
	repo := NewBoltBucketRepository(newTestDB(t), newTestLogger(t))
	ctx := context.Background()

	if err := repo.Create(ctx, newTestBucket("bucket-a", "owner-1")); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := repo.Delete(ctx, "bucket-a"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	got, err := repo.ListByOwner(ctx, "owner-1")
	if err != nil {
		t.Fatalf("ListByOwner() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ListByOwner() returned %d buckets, want 0", len(got))
	}
}

func TestBoltBucketRepository_Delete_NotFound(t *testing.T) {
	repo := NewBoltBucketRepository(newTestDB(t), newTestLogger(t))

	err := repo.Delete(context.Background(), "does-not-exist")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete() error = %v, want ErrNotFound", err)
	}
}

func TestBoltBucketRepository_Delete_RejectsNonEmptyBucket(t *testing.T) {
	db := newTestDB(t)
	repo := NewBoltBucketRepository(db, newTestLogger(t))
	ctx := context.Background()

	if err := repo.Create(ctx, newTestBucket("my-bucket", "owner-1")); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	seedRawObject(t, db, "my-bucket", "photo.png")

	err := repo.Delete(ctx, "my-bucket")
	if !errors.Is(err, ErrBucketNotEmpty) {
		t.Fatalf("Delete() error = %v, want ErrBucketNotEmpty", err)
	}
}

func seedRawObject(t *testing.T, db *bbolt.DB, bucketName, objectKeyName string) {
	t.Helper()

	key, err := bolt.ObjectKey(bucketName, objectKeyName)
	if err != nil {
		t.Fatalf("ObjectKey() error = %v", err)
	}

	obj := model.Object{
		ID:         "object-id-1",
		BucketName: bucketName,
		ObjectKey:  objectKeyName,
		CreatedAt:  time.Now().UTC(),
	}
	encoded, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("marshal object: %v", err)
	}

	err = db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte(store.BoltBucketObjects)).Put(key, encoded)
	})
	if err != nil {
		t.Fatalf("seed object: %v", err)
	}
}

func TestBoltBucketRepository_Delete_RemovesStaleOwnerIndexEntry(t *testing.T) {
	repo := NewBoltBucketRepository(newTestDB(t), newTestLogger(t))
	ctx := context.Background()

	if err := repo.Create(ctx, newTestBucket("my-bucket", "owner-1")); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := repo.Delete(ctx, "my-bucket"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if err := repo.Create(ctx, newTestBucket("my-bucket", "owner-2")); err != nil {
		t.Fatalf("resurrect Create() error = %v", err)
	}

	got, err := repo.ListByOwner(ctx, "owner-1")
	if err != nil {
		t.Fatalf("ListByOwner(owner-1) error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ListByOwner(owner-1) returned %d buckets, want 0 — leaked owner-2's resurrected bucket: %+v", len(got), got)
	}

	got, err = repo.ListByOwner(ctx, "owner-2")
	if err != nil {
		t.Fatalf("ListByOwner(owner-2) error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ListByOwner(owner-2) returned %d buckets, want 1", len(got))
	}
}
