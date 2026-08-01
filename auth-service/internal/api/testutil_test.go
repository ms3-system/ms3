package api

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"auth-service/internal/model"
	"auth-service/internal/service"
)

func newTestLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type fakeUserService struct {
	registerFn func(ctx context.Context, username, password string) (model.User, error)
	getUserFn  func(ctx context.Context, id string) (model.User, error)
}

var _ service.UserService = (*fakeUserService)(nil)

func (f *fakeUserService) Register(ctx context.Context, username, password string) (model.User, error) {
	if f.registerFn != nil {
		return f.registerFn(ctx, username, password)
	}
	return model.User{}, nil
}

func (f *fakeUserService) GetUser(ctx context.Context, id string) (model.User, error) {
	if f.getUserFn != nil {
		return f.getUserFn(ctx, id)
	}
	return model.User{}, nil
}

type fakeAuthService struct {
	loginFn             func(ctx context.Context, username, password string) (string, string, error)
	refreshFn           func(ctx context.Context, refreshToken string) (string, error)
	verifyAccessTokenFn func(tokenString string) (service.Principal, error)
}

var _ service.AuthService = (*fakeAuthService)(nil)

func (f *fakeAuthService) Login(ctx context.Context, username, password string) (string, string, error) {
	if f.loginFn != nil {
		return f.loginFn(ctx, username, password)
	}
	return "", "", nil
}

func (f *fakeAuthService) Refresh(ctx context.Context, refreshToken string) (string, error) {
	if f.refreshFn != nil {
		return f.refreshFn(ctx, refreshToken)
	}
	return "", nil
}

func (f *fakeAuthService) VerifyAccessToken(tokenString string) (service.Principal, error) {
	if f.verifyAccessTokenFn != nil {
		return f.verifyAccessTokenFn(tokenString)
	}
	return service.Principal{}, service.ErrInvalidCredentials
}

type fakeCredentialService struct {
	issueFn    func(ctx context.Context, userID string) (service.IssuedCredential, error)
	revokeFn   func(ctx context.Context, accessKey string) error
	lookupFn   func(ctx context.Context, accessKey string) (service.LookedUpCredential, error)
	getOwnerFn func(ctx context.Context, accessKey string) (string, error)
}

var _ service.CredentialService = (*fakeCredentialService)(nil)

func (f *fakeCredentialService) IssueCredential(ctx context.Context, userID string) (service.IssuedCredential, error) {
	if f.issueFn != nil {
		return f.issueFn(ctx, userID)
	}
	return service.IssuedCredential{}, nil
}

func (f *fakeCredentialService) RevokeCredential(ctx context.Context, accessKey string) error {
	if f.revokeFn != nil {
		return f.revokeFn(ctx, accessKey)
	}
	return nil
}

func (f *fakeCredentialService) LookupCredential(ctx context.Context, accessKey string) (service.LookedUpCredential, error) {
	if f.lookupFn != nil {
		return f.lookupFn(ctx, accessKey)
	}
	return service.LookedUpCredential{}, nil
}

func (f *fakeCredentialService) GetCredentialOwner(ctx context.Context, accessKey string) (string, error) {
	if f.getOwnerFn != nil {
		return f.getOwnerFn(ctx, accessKey)
	}
	return "", nil
}
