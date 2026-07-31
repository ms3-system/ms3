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

type BoltObjectRepository struct {
	db     *bbolt.DB
	logger *slog.Logger
}

var _ ObjectRepository = (*BoltObjectRepository)(nil)

func NewBoltObjectRepository(db *bbolt.DB, logger *slog.Logger) *BoltObjectRepository {
	return &BoltObjectRepository{
		db:     db,
		logger: logger.With(slog.String("component", "repository.object")),
	}
}

func (r *BoltObjectRepository) Put(ctx context.Context, o model.Object) error {
	log := r.logger.With(slog.String("bucket_name", o.BucketName), slog.String("object_key", o.ObjectKey))

	err := r.db.Update(func(tx *bbolt.Tx) error {
		key, err := bolt.ObjectKey(o.BucketName, o.ObjectKey)
		if err != nil {
			return fmt.Errorf("build object key: %w", err)
		}

		if o.CreatedAt.IsZero() {
			o.CreatedAt = time.Now().UTC()
		}

		encoded, err := json.Marshal(o)
		if err != nil {
			return fmt.Errorf("marshal object: %w", err)
		}

		return tx.Bucket([]byte(store.BoltBucketObjects)).Put(key, encoded)
	})

	if err != nil {
		log.Error("put failed", slog.Any("error", err))
		return err
	}

	log.Info("object put", slog.Int64("size_bytes", o.SizeBytes))
	return nil
}

func (r *BoltObjectRepository) Get(ctx context.Context, bucketName, key string) (model.Object, error) {
	log := r.logger.With(slog.String("bucket_name", bucketName), slog.String("object_key", key))

	var o model.Object
	err := r.db.View(func(tx *bbolt.Tx) error {
		objKey, err := bolt.ObjectKey(bucketName, key)
		if err != nil {
			return fmt.Errorf("build object key: %w", err)
		}

		raw := tx.Bucket([]byte(store.BoltBucketObjects)).Get(objKey)
		if raw == nil {
			return ErrNotFound
		}
		if err := json.Unmarshal(raw, &o); err != nil {
			return fmt.Errorf("unmarshal object at key %q: %w", objKey, err)
		}
		if o.DeletedAt != nil {
			return ErrNotFound
		}
		return nil
	})

	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			log.Error("get failed", slog.Any("error", err))
		}
		return model.Object{}, err
	}
	return o, nil
}

func (r *BoltObjectRepository) List(ctx context.Context, bucketName, prefix string, limit int) ([]model.Object, error) {
	log := r.logger.With(slog.String("bucket_name", bucketName), slog.String("prefix", prefix))

	if limit <= 0 {
		limit = 1000
	}

	scanPrefix, err := bolt.ObjectListPrefix(bucketName, prefix)
	if err != nil {
		return nil, fmt.Errorf("build object list prefix: %w", err)
	}

	var objects []model.Object
	err = r.db.View(func(tx *bbolt.Tx) error {
		c := tx.Bucket([]byte(store.BoltBucketObjects)).Cursor()

		for k, v := c.Seek(scanPrefix); k != nil && bytes.HasPrefix(k, scanPrefix); k, v = c.Next() {
			if len(objects) >= limit {
				break
			}
			if err := ctx.Err(); err != nil {
				return err
			}

			var o model.Object
			if err := json.Unmarshal(v, &o); err != nil {
				return fmt.Errorf("unmarshal object at key %q: %w", k, err)
			}
			if o.DeletedAt != nil {
				continue
			}
			objects = append(objects, o)
		}
		return nil
	})

	if err != nil {
		log.Error("list failed", slog.Any("error", err))
		return nil, err
	}

	log.Debug("listed objects", slog.Int("count", len(objects)))
	return objects, nil
}

func (r *BoltObjectRepository) Delete(ctx context.Context, bucketName, key string) error {
	log := r.logger.With(slog.String("bucket_name", bucketName), slog.String("object_key", key))

	err := r.db.Update(func(tx *bbolt.Tx) error {
		objects := tx.Bucket([]byte(store.BoltBucketObjects))

		objKey, err := bolt.ObjectKey(bucketName, key)
		if err != nil {
			return fmt.Errorf("build object key: %w", err)
		}

		raw := objects.Get(objKey)
		if raw == nil {
			return ErrNotFound
		}

		var o model.Object
		if err := json.Unmarshal(raw, &o); err != nil {
			return fmt.Errorf("unmarshal object at key %q: %w", objKey, err)
		}
		if o.DeletedAt != nil {
			return ErrNotFound
		}

		now := time.Now().UTC()
		o.DeletedAt = &now

		encoded, err := json.Marshal(o)
		if err != nil {
			return fmt.Errorf("marshal object: %w", err)
		}
		return objects.Put(objKey, encoded)
	})

	if err != nil {
		if errors.Is(err, ErrNotFound) {
			log.Debug("delete rejected: not found")
		} else {
			log.Error("delete failed", slog.Any("error", err))
		}
		return err
	}

	log.Info("object deleted")
	return nil
}

func (r *BoltObjectRepository) CountByBucketName(ctx context.Context, bucketName string) (int, error) {
	log := r.logger.With(slog.String("bucket_name", bucketName))

	scanPrefix, err := bolt.ObjectListPrefix(bucketName, "")
	if err != nil {
		return 0, fmt.Errorf("build object list prefix: %w", err)
	}

	var count int
	err = r.db.View(func(tx *bbolt.Tx) error {
		c := tx.Bucket([]byte(store.BoltBucketObjects)).Cursor()

		for k, v := c.Seek(scanPrefix); k != nil && bytes.HasPrefix(k, scanPrefix); k, v = c.Next() {
			if err := ctx.Err(); err != nil {
				return err
			}

			var o model.Object
			if err := json.Unmarshal(v, &o); err != nil {
				return fmt.Errorf("unmarshal object at key %q: %w", k, err)
			}
			if o.DeletedAt == nil {
				count++
			}
		}
		return nil
	})

	if err != nil {
		log.Error("count failed", slog.Any("error", err))
		return 0, err
	}
	return count, nil
}
