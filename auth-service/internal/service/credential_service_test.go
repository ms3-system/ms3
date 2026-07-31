package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"auth-service/internal/model"
	"auth-service/internal/repository"
)

func testMasterKey() []byte {
	return make([]byte, 32)
}

func TestCredentialService_IssueCredential(t *testing.T) {
	var stored model.Credential
	credRepo := &fakeCredentialRepository{
		createFn: func(ctx context.Context, c *model.Credential) error {
			stored = *c
			return nil
		},
	}
	userRepo := &fakeUserRepository{
		getByIDFn: func(ctx context.Context, id string) (model.User, error) {
			return model.User{ID: id}, nil
		},
	}
	svc := NewCredentialService(credRepo, userRepo, testMasterKey(), newTestLogger(t))

	issued, err := svc.IssueCredential(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("IssueCredential() error = %v", err)
	}

	if !strings.HasPrefix(issued.AccessKey, "AKIA") {
		t.Errorf("AccessKey = %q, want AKIA prefix", issued.AccessKey)
	}
	if len(issued.AccessKey) != 20 {
		t.Errorf("len(AccessKey) = %d, want 20", len(issued.AccessKey))
	}
	if len(issued.SecretKey) != 40 {
		t.Errorf("len(SecretKey) = %d, want 40", len(issued.SecretKey))
	}
	if issued.UserID != "user-1" {
		t.Errorf("UserID = %q, want %q", issued.UserID, "user-1")
	}
	if stored.SecretKeyEncrypted == issued.SecretKey {
		t.Error("stored SecretKeyEncrypted must not equal the plaintext secret key")
	}
	if stored.SecretKeyEncrypted == "" {
		t.Error("stored SecretKeyEncrypted must not be empty")
	}
}

func TestCredentialService_IssueCredential_UnknownUser(t *testing.T) {
	userRepo := &fakeUserRepository{
		getByIDFn: func(ctx context.Context, id string) (model.User, error) {
			return model.User{}, repository.ErrNotFound
		},
	}
	svc := NewCredentialService(&fakeCredentialRepository{}, userRepo, testMasterKey(), newTestLogger(t))

	_, err := svc.IssueCredential(context.Background(), "ghost")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("IssueCredential() error = %v, want ErrNotFound", err)
	}
}

func TestCredentialService_IssueCredential_EmptyUserID(t *testing.T) {
	svc := NewCredentialService(&fakeCredentialRepository{}, &fakeUserRepository{}, testMasterKey(), newTestLogger(t))

	_, err := svc.IssueCredential(context.Background(), "")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("IssueCredential() error = %v, want ErrInvalidInput", err)
	}
}

func TestCredentialService_IssueCredential_RetriesOnAccessKeyCollision(t *testing.T) {
	attempts := 0
	credRepo := &fakeCredentialRepository{
		createFn: func(ctx context.Context, c *model.Credential) error {
			attempts++
			if attempts < 3 {
				return repository.ErrAlreadyExists
			}
			return nil
		},
	}
	userRepo := &fakeUserRepository{
		getByIDFn: func(ctx context.Context, id string) (model.User, error) {
			return model.User{ID: id}, nil
		},
	}
	svc := NewCredentialService(credRepo, userRepo, testMasterKey(), newTestLogger(t))

	issued, err := svc.IssueCredential(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("IssueCredential() error = %v", err)
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
	if issued.AccessKey == "" {
		t.Error("IssueCredential() should return the access key that finally succeeded")
	}
}

func TestCredentialService_IssueCredential_GivesUpAfterMaxRetries(t *testing.T) {
	credRepo := &fakeCredentialRepository{
		createFn: func(ctx context.Context, c *model.Credential) error {
			return repository.ErrAlreadyExists
		},
	}
	userRepo := &fakeUserRepository{
		getByIDFn: func(ctx context.Context, id string) (model.User, error) {
			return model.User{ID: id}, nil
		},
	}
	svc := NewCredentialService(credRepo, userRepo, testMasterKey(), newTestLogger(t))

	_, err := svc.IssueCredential(context.Background(), "user-1")
	if !errors.Is(err, repository.ErrAlreadyExists) {
		t.Fatalf("IssueCredential() error = %v, want ErrAlreadyExists", err)
	}
}

func TestCredentialService_LookupCredential_DecryptsSecret(t *testing.T) {
	var stored model.Credential
	credRepo := &fakeCredentialRepository{
		createFn: func(ctx context.Context, c *model.Credential) error {
			stored = *c
			return nil
		},
		getByAccessKeyFn: func(ctx context.Context, accessKey string) (model.Credential, error) {
			return stored, nil
		},
	}
	userRepo := &fakeUserRepository{
		getByIDFn: func(ctx context.Context, id string) (model.User, error) {
			return model.User{ID: id}, nil
		},
	}
	svc := NewCredentialService(credRepo, userRepo, testMasterKey(), newTestLogger(t))

	issued, err := svc.IssueCredential(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("IssueCredential() error = %v", err)
	}

	looked, err := svc.LookupCredential(context.Background(), issued.AccessKey)
	if err != nil {
		t.Fatalf("LookupCredential() error = %v", err)
	}
	if looked.SecretKey != issued.SecretKey {
		t.Errorf("LookupCredential() SecretKey = %q, want %q (round-trip should match)", looked.SecretKey, issued.SecretKey)
	}
	if looked.UserID != "user-1" {
		t.Errorf("LookupCredential() UserID = %q, want %q", looked.UserID, "user-1")
	}
}

func TestCredentialService_LookupCredential_WrongMasterKeyFailsToDecrypt(t *testing.T) {
	var stored model.Credential
	credRepo := &fakeCredentialRepository{
		createFn: func(ctx context.Context, c *model.Credential) error {
			stored = *c
			return nil
		},
		getByAccessKeyFn: func(ctx context.Context, accessKey string) (model.Credential, error) {
			return stored, nil
		},
	}
	userRepo := &fakeUserRepository{
		getByIDFn: func(ctx context.Context, id string) (model.User, error) {
			return model.User{ID: id}, nil
		},
	}
	issuingKey := testMasterKey()
	issuingSvc := NewCredentialService(credRepo, userRepo, issuingKey, newTestLogger(t))

	issued, err := issuingSvc.IssueCredential(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("IssueCredential() error = %v", err)
	}

	wrongKey := make([]byte, 32)
	wrongKey[0] = 0xFF
	lookupSvc := NewCredentialService(credRepo, userRepo, wrongKey, newTestLogger(t))

	if _, err := lookupSvc.LookupCredential(context.Background(), issued.AccessKey); err == nil {
		t.Fatal("LookupCredential() with the wrong master key should fail to decrypt")
	}
}

func TestCredentialService_LookupCredential_NotFound(t *testing.T) {
	credRepo := &fakeCredentialRepository{
		getByAccessKeyFn: func(ctx context.Context, accessKey string) (model.Credential, error) {
			return model.Credential{}, repository.ErrNotFound
		},
	}
	svc := NewCredentialService(credRepo, &fakeUserRepository{}, testMasterKey(), newTestLogger(t))

	_, err := svc.LookupCredential(context.Background(), "missing")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("LookupCredential() error = %v, want ErrNotFound", err)
	}
}

func TestCredentialService_RevokeCredential(t *testing.T) {
	var revoked string
	credRepo := &fakeCredentialRepository{
		revokeFn: func(ctx context.Context, accessKey string) error {
			revoked = accessKey
			return nil
		},
	}
	svc := NewCredentialService(credRepo, &fakeUserRepository{}, testMasterKey(), newTestLogger(t))

	if err := svc.RevokeCredential(context.Background(), "AKIAEXAMPLE"); err != nil {
		t.Fatalf("RevokeCredential() error = %v", err)
	}
	if revoked != "AKIAEXAMPLE" {
		t.Errorf("revoked access key = %q, want %q", revoked, "AKIAEXAMPLE")
	}
}

func TestCredentialService_RevokeCredential_EmptyAccessKey(t *testing.T) {
	svc := NewCredentialService(&fakeCredentialRepository{}, &fakeUserRepository{}, testMasterKey(), newTestLogger(t))

	err := svc.RevokeCredential(context.Background(), "")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("RevokeCredential() error = %v, want ErrInvalidInput", err)
	}
}
