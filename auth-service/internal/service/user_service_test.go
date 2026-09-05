package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"auth-service/internal/model"
	"auth-service/internal/repository"
)

func TestUserService_Register(t *testing.T) {
	var created model.User
	repo := &fakeUserRepository{
		createFn: func(_ context.Context, u *model.User) error {
			created = *u
			return nil
		},
	}
	svc := NewUserService(repo, newTestLogger(t))

	u, err := svc.Register(context.Background(), "alice", "hunter22")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if u.ID == "" {
		t.Error("Register() should assign an ID")
	}
	if u.Username != "alice" {
		t.Errorf("Username = %q, want %q", u.Username, "alice")
	}
	if u.PasswordHash == "" || u.PasswordHash == "hunter22" {
		t.Errorf("PasswordHash should be a bcrypt hash, got %q", u.PasswordHash)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte("hunter22")); err != nil {
		t.Errorf("stored hash does not match password: %v", err)
	}
	if created.ID != u.ID {
		t.Error("Register() should pass the same user through to the repository")
	}
}

func TestUserService_Register_InvalidUsername(t *testing.T) {
	svc := NewUserService(&fakeUserRepository{}, newTestLogger(t))

	tests := []string{"", "ab", "has spaces", "way-too-long-a-username-to-be-valid-honestly"}
	for _, username := range tests {
		_, err := svc.Register(context.Background(), username, "hunter22")
		if !errors.Is(err, ErrInvalidInput) {
			t.Errorf("Register(%q) error = %v, want ErrInvalidInput", username, err)
		}
	}
}

func TestUserService_Register_PasswordTooShort(t *testing.T) {
	svc := NewUserService(&fakeUserRepository{}, newTestLogger(t))

	_, err := svc.Register(context.Background(), "alice", "short")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Register() error = %v, want ErrInvalidInput", err)
	}
}

func TestUserService_Register_PasswordTooLong(t *testing.T) {
	svc := NewUserService(&fakeUserRepository{}, newTestLogger(t))

	_, err := svc.Register(context.Background(), "alice", strings.Repeat("a", maxPasswordLength+1))
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Register() error = %v, want ErrInvalidInput", err)
	}
}

func TestUserService_Register_PasswordAtMaxLengthIsAccepted(t *testing.T) {
	svc := NewUserService(&fakeUserRepository{}, newTestLogger(t))

	_, err := svc.Register(context.Background(), "alice", strings.Repeat("a", maxPasswordLength))
	if err != nil {
		t.Fatalf("Register() error = %v, want nil", err)
	}
}

func TestUserService_Register_PropagatesRepositoryError(t *testing.T) {
	repo := &fakeUserRepository{
		createFn: func(_ context.Context, _ *model.User) error {
			return repository.ErrAlreadyExists
		},
	}
	svc := NewUserService(repo, newTestLogger(t))

	_, err := svc.Register(context.Background(), "alice", "hunter22")
	if !errors.Is(err, repository.ErrAlreadyExists) {
		t.Fatalf("Register() error = %v, want ErrAlreadyExists", err)
	}
}

func TestUserService_GetUser(t *testing.T) {
	repo := &fakeUserRepository{
		getByIDFn: func(_ context.Context, id string) (model.User, error) {
			return model.User{ID: id, Username: "alice"}, nil
		},
	}
	svc := NewUserService(repo, newTestLogger(t))

	u, err := svc.GetUser(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("GetUser() error = %v", err)
	}
	if u.Username != "alice" {
		t.Errorf("Username = %q, want %q", u.Username, "alice")
	}
}

func TestUserService_GetUser_EmptyID(t *testing.T) {
	svc := NewUserService(&fakeUserRepository{}, newTestLogger(t))

	_, err := svc.GetUser(context.Background(), "")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("GetUser() error = %v, want ErrInvalidInput", err)
	}
}
