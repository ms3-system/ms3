package service

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"auth-service/internal/model"
)

func newTestLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type fakeUserRepository struct {
	createFn        func(ctx context.Context, u *model.User) error
	getByIDFn       func(ctx context.Context, id string) (model.User, error)
	getByUsernameFn func(ctx context.Context, username string) (model.User, error)
}

func (f *fakeUserRepository) Create(ctx context.Context, u *model.User) error {
	if f.createFn != nil {
		return f.createFn(ctx, u)
	}
	return nil
}

func (f *fakeUserRepository) GetByID(ctx context.Context, id string) (model.User, error) {
	if f.getByIDFn != nil {
		return f.getByIDFn(ctx, id)
	}
	return model.User{}, nil
}

func (f *fakeUserRepository) GetByUsername(ctx context.Context, username string) (model.User, error) {
	if f.getByUsernameFn != nil {
		return f.getByUsernameFn(ctx, username)
	}
	return model.User{}, nil
}

type fakeCredentialRepository struct {
	createFn         func(ctx context.Context, c *model.Credential) error
	getByAccessKeyFn func(ctx context.Context, accessKey string) (model.Credential, error)
	revokeFn         func(ctx context.Context, accessKey string) error
}

func (f *fakeCredentialRepository) Create(ctx context.Context, c *model.Credential) error {
	if f.createFn != nil {
		return f.createFn(ctx, c)
	}
	return nil
}

func (f *fakeCredentialRepository) GetByAccessKey(ctx context.Context, accessKey string) (model.Credential, error) {
	if f.getByAccessKeyFn != nil {
		return f.getByAccessKeyFn(ctx, accessKey)
	}
	return model.Credential{}, nil
}

func (f *fakeCredentialRepository) Revoke(ctx context.Context, accessKey string) error {
	if f.revokeFn != nil {
		return f.revokeFn(ctx, accessKey)
	}
	return nil
}
