package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"go.etcd.io/bbolt"

	"auth-service/internal/model"
	"auth-service/internal/repository/bolt"
	"auth-service/internal/store"
)

type BoltCredentialRepository struct {
	db     *bbolt.DB
	logger *slog.Logger
}

var _ CredentialRepository = (*BoltCredentialRepository)(nil)

func NewBoltCredentialRepository(db *bbolt.DB, logger *slog.Logger) *BoltCredentialRepository {
	return &BoltCredentialRepository{
		db:     db,
		logger: logger.With(slog.String("component", "repository.credential")),
	}
}

func (r *BoltCredentialRepository) Create(ctx context.Context, c *model.Credential) error {
	log := r.logger.With(slog.String("access_key", c.AccessKey), slog.String("user_id", c.UserID))

	err := r.db.Update(func(tx *bbolt.Tx) error {
		key, err := bolt.CredentialKey(c.AccessKey)
		if err != nil {
			return fmt.Errorf("build credential key: %w", err)
		}

		credentials := tx.Bucket([]byte(store.BoltBucketCredentials))
		if credentials.Get(key) != nil {
			return ErrAlreadyExists
		}

		if c.CreatedAt.IsZero() {
			c.CreatedAt = time.Now().UTC()
		}

		encoded, err := json.Marshal(c)
		if err != nil {
			return fmt.Errorf("marshal credential: %w", err)
		}
		return credentials.Put(key, encoded)
	})

	if err != nil {
		if errors.Is(err, ErrAlreadyExists) {
			log.Debug("create rejected: access key already in use")
		} else {
			log.Error("create failed", slog.Any("error", err))
		}
		return err
	}

	log.Info("credential created")
	return nil
}

func (r *BoltCredentialRepository) GetByAccessKey(ctx context.Context, accessKey string) (model.Credential, error) {
	log := r.logger.With(slog.String("access_key", accessKey))

	var c model.Credential
	err := r.db.View(func(tx *bbolt.Tx) error {
		key, err := bolt.CredentialKey(accessKey)
		if err != nil {
			return fmt.Errorf("build credential key: %w", err)
		}

		raw := tx.Bucket([]byte(store.BoltBucketCredentials)).Get(key)
		if raw == nil {
			return ErrNotFound
		}
		if err := json.Unmarshal(raw, &c); err != nil {
			return fmt.Errorf("unmarshal credential %q: %w", accessKey, err)
		}
		if c.RevokedAt != nil {
			return ErrNotFound
		}
		return nil
	})

	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			log.Error("get failed", slog.Any("error", err))
		}
		return model.Credential{}, err
	}
	return c, nil
}

func (r *BoltCredentialRepository) Revoke(ctx context.Context, accessKey string) error {
	log := r.logger.With(slog.String("access_key", accessKey))

	err := r.db.Update(func(tx *bbolt.Tx) error {
		key, err := bolt.CredentialKey(accessKey)
		if err != nil {
			return fmt.Errorf("build credential key: %w", err)
		}

		credentials := tx.Bucket([]byte(store.BoltBucketCredentials))
		raw := credentials.Get(key)
		if raw == nil {
			return ErrNotFound
		}

		var c model.Credential
		if err := json.Unmarshal(raw, &c); err != nil {
			return fmt.Errorf("unmarshal credential %q: %w", accessKey, err)
		}
		if c.RevokedAt != nil {
			return ErrNotFound
		}

		now := time.Now().UTC()
		c.RevokedAt = &now

		encoded, err := json.Marshal(c)
		if err != nil {
			return fmt.Errorf("marshal credential: %w", err)
		}
		return credentials.Put(key, encoded)
	})

	if err != nil {
		if errors.Is(err, ErrNotFound) {
			log.Debug("revoke rejected: not found")
		} else {
			log.Error("revoke failed", slog.Any("error", err))
		}
		return err
	}

	log.Info("credential revoked")
	return nil
}
