package repository

import (
	"context"
	"errors"

	"auth-service/internal/model"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
)

type UserRepository interface {
	Create(ctx context.Context, u *model.User) error
	GetByID(ctx context.Context, id string) (model.User, error)
	GetByUsername(ctx context.Context, username string) (model.User, error)
}

type CredentialRepository interface {
	Create(ctx context.Context, c *model.Credential) error
	GetByAccessKey(ctx context.Context, accessKey string) (model.Credential, error)
	Revoke(ctx context.Context, accessKey string) error
}
