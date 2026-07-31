package repository

import (
	"context"
	"errors"
	"testing"

	"auth-service/internal/model"
)

func TestBoltCredentialRepository_CreateAndGet(t *testing.T) {
	repo := NewBoltCredentialRepository(newTestDB(t), newTestLogger(t))
	ctx := context.Background()

	c := &model.Credential{AccessKey: "AKIAEXAMPLE", UserID: "user-1", SecretKeyEncrypted: "cipher"}
	if err := repo.Create(ctx, c); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if c.CreatedAt.IsZero() {
		t.Error("Create() should stamp CreatedAt on caller's pointer")
	}

	got, err := repo.GetByAccessKey(ctx, "AKIAEXAMPLE")
	if err != nil {
		t.Fatalf("GetByAccessKey() error = %v", err)
	}
	if got.UserID != "user-1" {
		t.Errorf("UserID = %q, want %q", got.UserID, "user-1")
	}
	if got.SecretKeyEncrypted != "cipher" {
		t.Errorf("SecretKeyEncrypted = %q, want %q", got.SecretKeyEncrypted, "cipher")
	}
}

func TestBoltCredentialRepository_CreateRejectsDuplicateAccessKey(t *testing.T) {
	repo := NewBoltCredentialRepository(newTestDB(t), newTestLogger(t))
	ctx := context.Background()

	if err := repo.Create(ctx, &model.Credential{AccessKey: "AKIAEXAMPLE", UserID: "user-1"}); err != nil {
		t.Fatalf("first Create() error = %v", err)
	}

	err := repo.Create(ctx, &model.Credential{AccessKey: "AKIAEXAMPLE", UserID: "user-2"})
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("Create() error = %v, want ErrAlreadyExists", err)
	}
}

func TestBoltCredentialRepository_GetByAccessKey_NotFound(t *testing.T) {
	repo := NewBoltCredentialRepository(newTestDB(t), newTestLogger(t))

	_, err := repo.GetByAccessKey(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetByAccessKey() error = %v, want ErrNotFound", err)
	}
}

func TestBoltCredentialRepository_Revoke(t *testing.T) {
	repo := NewBoltCredentialRepository(newTestDB(t), newTestLogger(t))
	ctx := context.Background()

	if err := repo.Create(ctx, &model.Credential{AccessKey: "AKIAEXAMPLE", UserID: "user-1"}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := repo.Revoke(ctx, "AKIAEXAMPLE"); err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}

	_, err := repo.GetByAccessKey(ctx, "AKIAEXAMPLE")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetByAccessKey() after revoke error = %v, want ErrNotFound (revoked = not found)", err)
	}
}

func TestBoltCredentialRepository_Revoke_NotFound(t *testing.T) {
	repo := NewBoltCredentialRepository(newTestDB(t), newTestLogger(t))

	err := repo.Revoke(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Revoke() error = %v, want ErrNotFound", err)
	}
}

func TestBoltCredentialRepository_Revoke_AlreadyRevoked(t *testing.T) {
	repo := NewBoltCredentialRepository(newTestDB(t), newTestLogger(t))
	ctx := context.Background()

	if err := repo.Create(ctx, &model.Credential{AccessKey: "AKIAEXAMPLE", UserID: "user-1"}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := repo.Revoke(ctx, "AKIAEXAMPLE"); err != nil {
		t.Fatalf("first Revoke() error = %v", err)
	}

	err := repo.Revoke(ctx, "AKIAEXAMPLE")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("second Revoke() error = %v, want ErrNotFound", err)
	}
}
