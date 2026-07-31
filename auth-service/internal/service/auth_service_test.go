package service

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"auth-service/internal/model"
	"auth-service/internal/repository"
)

func testUserWithPassword(t *testing.T, password string) model.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("bcrypt.GenerateFromPassword() error = %v", err)
	}
	return model.User{ID: "user-1", Username: "alice", PasswordHash: string(hash), IsAdmin: true}
}

func TestAuthService_Login_Success(t *testing.T) {
	u := testUserWithPassword(t, "hunter22")
	repo := &fakeUserRepository{
		getByUsernameFn: func(ctx context.Context, username string) (model.User, error) {
			return u, nil
		},
	}
	secret := []byte("test-secret")
	svc := NewAuthService(repo, secret, newTestLogger(t))

	access, refresh, err := svc.Login(context.Background(), "alice", "hunter22")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if access == "" || refresh == "" {
		t.Fatal("Login() should return non-empty tokens")
	}

	issuer := newJWTIssuer(secret)

	accessClaims, err := issuer.parse(access, tokenTypeAccess)
	if err != nil {
		t.Fatalf("parse access token: %v", err)
	}
	if accessClaims.Subject != u.ID {
		t.Errorf("access token sub = %q, want %q", accessClaims.Subject, u.ID)
	}
	if accessClaims.Username != u.Username {
		t.Errorf("access token username = %q, want %q", accessClaims.Username, u.Username)
	}
	if !accessClaims.IsAdmin {
		t.Error("access token is_admin = false, want true")
	}

	refreshClaims, err := issuer.parse(refresh, tokenTypeRefresh)
	if err != nil {
		t.Fatalf("parse refresh token: %v", err)
	}
	if refreshClaims.Subject != u.ID {
		t.Errorf("refresh token sub = %q, want %q", refreshClaims.Subject, u.ID)
	}
	if refreshClaims.Username != "" {
		t.Errorf("refresh token should not carry username, got %q", refreshClaims.Username)
	}
}

func TestAuthService_Login_WrongPassword(t *testing.T) {
	u := testUserWithPassword(t, "hunter22")
	repo := &fakeUserRepository{
		getByUsernameFn: func(ctx context.Context, username string) (model.User, error) {
			return u, nil
		},
	}
	svc := NewAuthService(repo, []byte("test-secret"), newTestLogger(t))

	_, _, err := svc.Login(context.Background(), "alice", "wrong-password")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v, want ErrInvalidCredentials", err)
	}
}

func TestAuthService_Login_UnknownUsername(t *testing.T) {
	repo := &fakeUserRepository{
		getByUsernameFn: func(ctx context.Context, username string) (model.User, error) {
			return model.User{}, repository.ErrNotFound
		},
	}
	svc := NewAuthService(repo, []byte("test-secret"), newTestLogger(t))

	_, _, err := svc.Login(context.Background(), "ghost", "hunter22")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v, want ErrInvalidCredentials", err)
	}
}

func TestAuthService_Login_MissingFields(t *testing.T) {
	svc := NewAuthService(&fakeUserRepository{}, []byte("test-secret"), newTestLogger(t))

	if _, _, err := svc.Login(context.Background(), "", "hunter22"); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("Login() with empty username error = %v, want ErrInvalidInput", err)
	}
	if _, _, err := svc.Login(context.Background(), "alice", ""); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("Login() with empty password error = %v, want ErrInvalidInput", err)
	}
}

func TestAuthService_Refresh_Success(t *testing.T) {
	u := testUserWithPassword(t, "hunter22")
	secret := []byte("test-secret")
	issuer := newJWTIssuer(secret)

	refreshToken, err := issuer.mintRefreshToken(u)
	if err != nil {
		t.Fatalf("mintRefreshToken() error = %v", err)
	}

	repo := &fakeUserRepository{
		getByIDFn: func(ctx context.Context, id string) (model.User, error) {
			return u, nil
		},
	}
	svc := NewAuthService(repo, secret, newTestLogger(t))

	access, err := svc.Refresh(context.Background(), refreshToken)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	claims, err := issuer.parse(access, tokenTypeAccess)
	if err != nil {
		t.Fatalf("parse new access token: %v", err)
	}
	if claims.Subject != u.ID {
		t.Errorf("new access token sub = %q, want %q", claims.Subject, u.ID)
	}
	if claims.Username != u.Username {
		t.Errorf("new access token username = %q, want %q", claims.Username, u.Username)
	}
}

func TestAuthService_Refresh_RejectsAccessTokenAsRefreshToken(t *testing.T) {
	u := testUserWithPassword(t, "hunter22")
	secret := []byte("test-secret")
	issuer := newJWTIssuer(secret)

	accessToken, err := issuer.mintAccessToken(u)
	if err != nil {
		t.Fatalf("mintAccessToken() error = %v", err)
	}

	svc := NewAuthService(&fakeUserRepository{}, secret, newTestLogger(t))

	_, err = svc.Refresh(context.Background(), accessToken)
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Refresh() error = %v, want ErrInvalidCredentials", err)
	}
}

func TestAuthService_Refresh_RejectsGarbageToken(t *testing.T) {
	svc := NewAuthService(&fakeUserRepository{}, []byte("test-secret"), newTestLogger(t))

	_, err := svc.Refresh(context.Background(), "not-a-jwt")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Refresh() error = %v, want ErrInvalidCredentials", err)
	}
}

func TestAuthService_Refresh_RejectsWrongSecret(t *testing.T) {
	u := testUserWithPassword(t, "hunter22")
	wrongIssuer := newJWTIssuer([]byte("wrong-secret"))
	refreshToken, err := wrongIssuer.mintRefreshToken(u)
	if err != nil {
		t.Fatalf("mintRefreshToken() error = %v", err)
	}

	svc := NewAuthService(&fakeUserRepository{}, []byte("test-secret"), newTestLogger(t))

	_, err = svc.Refresh(context.Background(), refreshToken)
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Refresh() error = %v, want ErrInvalidCredentials", err)
	}
}

func TestAuthService_Refresh_UserNoLongerExists(t *testing.T) {
	u := testUserWithPassword(t, "hunter22")
	secret := []byte("test-secret")
	issuer := newJWTIssuer(secret)

	refreshToken, err := issuer.mintRefreshToken(u)
	if err != nil {
		t.Fatalf("mintRefreshToken() error = %v", err)
	}

	repo := &fakeUserRepository{
		getByIDFn: func(ctx context.Context, id string) (model.User, error) {
			return model.User{}, repository.ErrNotFound
		},
	}
	svc := NewAuthService(repo, secret, newTestLogger(t))

	_, err = svc.Refresh(context.Background(), refreshToken)
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Refresh() error = %v, want ErrInvalidCredentials", err)
	}
}

func TestAuthService_Refresh_EmptyToken(t *testing.T) {
	svc := NewAuthService(&fakeUserRepository{}, []byte("test-secret"), newTestLogger(t))

	_, err := svc.Refresh(context.Background(), "")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Refresh() error = %v, want ErrInvalidInput", err)
	}
}
