package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"metadata-service/internal/model"
	"metadata-service/internal/repository/bolt"
	"metadata-service/internal/store"
	"time"

	"go.etcd.io/bbolt"
)

type BoltBucketRepository struct {
	db     *bbolt.DB
	logger *slog.Logger
}

var _ BucketRepository = (*BoltBucketRepository)(nil)

func NewBoltBucketRepository(db *bbolt.DB, logger *slog.Logger) *BoltBucketRepository {
	return &BoltBucketRepository{
		db:     db,
		logger: logger.With(slog.String("component", "repository.bucket")),
	}
}

func (r *BoltBucketRepository) Create(ctx context.Context, b *model.Bucket) error {
	log := r.logger.With(slog.String("bucket_name", b.Name), slog.String("owner_id", b.OwnerID))

	err := r.db.Update(func(tx *bbolt.Tx) error {
		records := tx.Bucket([]byte(store.BoltBucketRecords))

		if existing := records.Get([]byte(b.Name)); existing != nil {
			var current model.Bucket
			if err := json.Unmarshal(existing, &current); err != nil {
				return fmt.Errorf("unmarshal existing bucket %q: %w", b.Name, err)
			}
			if current.DeletedAt == nil {
				return ErrAlreadyExists
			}
			log.Warn("recreating bucket over a soft-deleted entry",
				slog.Time("previously_deleted_at", *current.DeletedAt))
		}

		if b.CreatedAt.IsZero() {
			b.CreatedAt = time.Now().UTC()
		}

		encoded, err := json.Marshal(b)
		if err != nil {
			return fmt.Errorf("marshal bucket: %w", err)
		}
		if err := records.Put([]byte(b.Name), encoded); err != nil {
			return fmt.Errorf("put bucket record: %w", err)
		}

		idxKey, err := bolt.OwnerIndexKey(b.OwnerID, b.Name)
		if err != nil {
			return fmt.Errorf("build owner index key: %w", err)
		}
		if err := tx.Bucket([]byte(store.BoltBucketOwnerIdx)).Put(idxKey, []byte(b.Name)); err != nil {
			return fmt.Errorf("put owner index: %w", err)
		}

		return nil
	})

	if err != nil {
		if errors.Is(err, ErrAlreadyExists) {
			log.Debug("create rejected: name already in use")
		} else {
			log.Error("create failed", slog.Any("error", err))
		}
		return err
	}

	log.Info("bucket created")
	return nil
}

func (r *BoltBucketRepository) GetByName(ctx context.Context, name string) (model.Bucket, error) {
	log := r.logger.With(slog.String("bucket_name", name))

	var b model.Bucket
	err := r.db.View(func(tx *bbolt.Tx) error {
		raw := tx.Bucket([]byte(store.BoltBucketRecords)).Get([]byte(name))
		if raw == nil {
			return ErrNotFound
		}
		if err := json.Unmarshal(raw, &b); err != nil {
			return fmt.Errorf("unmarshal bucket %q: %w", name, err)
		}
		if b.DeletedAt != nil {
			return ErrNotFound
		}
		return nil
	})

	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			log.Error("get failed", slog.Any("error", err))
		}
		return model.Bucket{}, err
	}
	return b, nil
}

func (r *BoltBucketRepository) ListByOwner(ctx context.Context, ownerID string) ([]model.Bucket, error) {
	log := r.logger.With(slog.String("owner_id", ownerID))

	prefix, err := bolt.OwnerIndexPrefix(ownerID)
	if err != nil {
		return nil, fmt.Errorf("build owner index prefix: %w", err)
	}

	var buckets []model.Bucket
	err = r.db.View(func(tx *bbolt.Tx) error {
		records := tx.Bucket([]byte(store.BoltBucketRecords))
		c := tx.Bucket([]byte(store.BoltBucketOwnerIdx)).Cursor()

		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			if err := ctx.Err(); err != nil {
				return err
			}

			raw := records.Get(v)
			if raw == nil {
				log.Warn("owner index references missing bucket record",
					slog.String("bucket_name", string(v)))
				continue
			}

			var b model.Bucket
			if err := json.Unmarshal(raw, &b); err != nil {
				return fmt.Errorf("unmarshal bucket %q: %w", v, err)
			}
			if b.DeletedAt != nil {
				continue
			}
			buckets = append(buckets, b)
		}
		return nil
	})

	if err != nil {
		log.Error("list failed", slog.Any("error", err))
		return nil, err
	}

	log.Debug("listed buckets", slog.Int("count", len(buckets)))
	return buckets, nil
}

func (r *BoltBucketRepository) Delete(ctx context.Context, name string) error {
	log := r.logger.With(slog.String("bucket_name", name))

	err := r.db.Update(func(tx *bbolt.Tx) error {
		records := tx.Bucket([]byte(store.BoltBucketRecords))
		raw := records.Get([]byte(name))
		if raw == nil {
			return ErrNotFound
		}

		var b model.Bucket
		if err := json.Unmarshal(raw, &b); err != nil {
			return fmt.Errorf("unmarshal bucket %q: %w", name, err)
		}
		if b.DeletedAt != nil {
			return ErrNotFound
		}

		idxKey, err := bolt.OwnerIndexKey(b.OwnerID, name)
		if err != nil {
			return fmt.Errorf("build owner index key: %w", err)
		}

		if err := tx.Bucket([]byte(store.BoltBucketOwnerIdx)).Delete(idxKey); err != nil {
			return fmt.Errorf("delete owner index: %w", err)
		}

		objPrefix, err := bolt.ObjectListPrefix(name, "")
		if err != nil {
			return fmt.Errorf("build object list prefix: %w", err)
		}

		c := tx.Bucket([]byte(store.BoltBucketObjects)).Cursor()
		for k, v := c.Seek(objPrefix); k != nil && bytes.HasPrefix(k, objPrefix); k, v = c.Next() {
			if err := ctx.Err(); err != nil {
				return err
			}

			var o model.Object
			if err := json.Unmarshal(v, &o); err != nil {
				return fmt.Errorf("unmarshal object at key %q: %w", k, err)
			}
			if o.DeletedAt == nil {
				return ErrBucketNotEmpty
			}
		}

		now := time.Now().UTC()
		b.DeletedAt = &now

		encoded, err := json.Marshal(b)
		if err != nil {
			return fmt.Errorf("marshal bucket: %w", err)
		}
		return records.Put([]byte(name), encoded)
	})

	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			log.Debug("delete rejected: not found")
		case errors.Is(err, ErrBucketNotEmpty):
			log.Debug("delete rejected: bucket not empty")
		default:
			log.Error("delete failed", slog.Any("error", err))
		}
		return err
	}

	log.Info("bucket deleted")
	return nil
}
