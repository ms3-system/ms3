package repository

import (
	"context"
	"errors"
	"testing"

	"auth-service/internal/model"
)

func TestBoltUserRepository_CreateAndGetByID(t *testing.T) {
	repo := NewBoltUserRepository(newTestDB(t), newTestLogger(t))
	ctx := context.Background()

	u := &model.User{ID: "user-1", Username: "alice", PasswordHash: "hash"}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if u.CreatedAt.IsZero() {
		t.Error("Create() should stamp CreatedAt on caller's pointer")
	}

	got, err := repo.GetByID(ctx, "user-1")
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.Username != "alice" {
		t.Errorf("Username = %q, want %q", got.Username, "alice")
	}
}

func TestBoltUserRepository_CreateRejectsDuplicateUsername(t *testing.T) {
	repo := NewBoltUserRepository(newTestDB(t), newTestLogger(t))
	ctx := context.Background()

	if err := repo.Create(ctx, &model.User{ID: "user-1", Username: "alice"}); err != nil {
		t.Fatalf("first Create() error = %v", err)
	}

	err := repo.Create(ctx, &model.User{ID: "user-2", Username: "alice"})
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("Create() error = %v, want ErrAlreadyExists", err)
	}
}

func TestBoltUserRepository_GetByID_NotFound(t *testing.T) {
	repo := NewBoltUserRepository(newTestDB(t), newTestLogger(t))

	_, err := repo.GetByID(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetByID() error = %v, want ErrNotFound", err)
	}
}

func TestBoltUserRepository_GetByUsername(t *testing.T) {
	repo := NewBoltUserRepository(newTestDB(t), newTestLogger(t))
	ctx := context.Background()

	if err := repo.Create(ctx, &model.User{ID: "user-1", Username: "alice"}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := repo.GetByUsername(ctx, "alice")
	if err != nil {
		t.Fatalf("GetByUsername() error = %v", err)
	}
	if got.ID != "user-1" {
		t.Errorf("ID = %q, want %q", got.ID, "user-1")
	}
}

func TestBoltUserRepository_GetByUsername_NotFound(t *testing.T) {
	repo := NewBoltUserRepository(newTestDB(t), newTestLogger(t))

	_, err := repo.GetByUsername(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetByUsername() error = %v, want ErrNotFound", err)
	}
}
