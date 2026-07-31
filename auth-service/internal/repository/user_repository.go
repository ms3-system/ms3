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

type BoltUserRepository struct {
	db     *bbolt.DB
	logger *slog.Logger
}

var _ UserRepository = (*BoltUserRepository)(nil)

func NewBoltUserRepository(db *bbolt.DB, logger *slog.Logger) *BoltUserRepository {
	return &BoltUserRepository{
		db:     db,
		logger: logger.With(slog.String("component", "repository.user")),
	}
}

func (r *BoltUserRepository) Create(ctx context.Context, u *model.User) error {
	log := r.logger.With(slog.String("username", u.Username), slog.String("user_id", u.ID))

	err := r.db.Update(func(tx *bbolt.Tx) error {
		usernameKey, err := bolt.UsernameKey(u.Username)
		if err != nil {
			return fmt.Errorf("build username key: %w", err)
		}
		userKey, err := bolt.UserKey(u.ID)
		if err != nil {
			return fmt.Errorf("build user key: %w", err)
		}

		usernames := tx.Bucket([]byte(store.BoltBucketUsernameIdx))
		if usernames.Get(usernameKey) != nil {
			return ErrAlreadyExists
		}

		if u.CreatedAt.IsZero() {
			u.CreatedAt = time.Now().UTC()
		}

		encoded, err := json.Marshal(u)
		if err != nil {
			return fmt.Errorf("marshal user: %w", err)
		}

		if err := tx.Bucket([]byte(store.BoltBucketUsers)).Put(userKey, encoded); err != nil {
			return fmt.Errorf("put user record: %w", err)
		}
		if err := usernames.Put(usernameKey, []byte(u.ID)); err != nil {
			return fmt.Errorf("put username index: %w", err)
		}
		return nil
	})

	if err != nil {
		if errors.Is(err, ErrAlreadyExists) {
			log.Debug("create rejected: username already in use")
		} else {
			log.Error("create failed", slog.Any("error", err))
		}
		return err
	}

	log.Info("user created")
	return nil
}

func (r *BoltUserRepository) GetByID(ctx context.Context, id string) (model.User, error) {
	log := r.logger.With(slog.String("user_id", id))

	var u model.User
	err := r.db.View(func(tx *bbolt.Tx) error {
		userKey, err := bolt.UserKey(id)
		if err != nil {
			return fmt.Errorf("build user key: %w", err)
		}

		raw := tx.Bucket([]byte(store.BoltBucketUsers)).Get(userKey)
		if raw == nil {
			return ErrNotFound
		}
		if err := json.Unmarshal(raw, &u); err != nil {
			return fmt.Errorf("unmarshal user %q: %w", id, err)
		}
		return nil
	})

	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			log.Error("get by id failed", slog.Any("error", err))
		}
		return model.User{}, err
	}
	return u, nil
}

func (r *BoltUserRepository) GetByUsername(ctx context.Context, username string) (model.User, error) {
	log := r.logger.With(slog.String("username", username))

	var u model.User
	err := r.db.View(func(tx *bbolt.Tx) error {
		usernameKey, err := bolt.UsernameKey(username)
		if err != nil {
			return fmt.Errorf("build username key: %w", err)
		}

		id := tx.Bucket([]byte(store.BoltBucketUsernameIdx)).Get(usernameKey)
		if id == nil {
			return ErrNotFound
		}

		userKey, err := bolt.UserKey(string(id))
		if err != nil {
			return fmt.Errorf("build user key: %w", err)
		}

		raw := tx.Bucket([]byte(store.BoltBucketUsers)).Get(userKey)
		if raw == nil {
			log.Warn("username index references missing user record", slog.String("user_id", string(id)))
			return ErrNotFound
		}
		if err := json.Unmarshal(raw, &u); err != nil {
			return fmt.Errorf("unmarshal user %q: %w", id, err)
		}
		return nil
	})

	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			log.Error("get by username failed", slog.Any("error", err))
		}
		return model.User{}, err
	}
	return u, nil
}
