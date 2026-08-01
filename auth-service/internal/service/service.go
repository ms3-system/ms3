package service

import (
	"context"
	"time"

	"auth-service/internal/model"
)

type UserService interface {
	Register(ctx context.Context, username, password string) (model.User, error)
	GetUser(ctx context.Context, id string) (model.User, error)
}

type AuthService interface {
	Login(ctx context.Context, username, password string) (accessToken, refreshToken string, err error)
	Refresh(ctx context.Context, refreshToken string) (accessToken string, err error)
	// VerifyAccessToken authenticates a bearer access token for the API
	// layer's own auth middleware — see internal/api/auth_middleware.go.
	VerifyAccessToken(tokenString string) (Principal, error)
}

// Principal is the authenticated caller identity extracted from a verified
// access token.
type Principal struct {
	UserID  string
	IsAdmin bool
}

type CredentialService interface {
	IssueCredential(ctx context.Context, userID string) (IssuedCredential, error)
	RevokeCredential(ctx context.Context, accessKey string) error
	LookupCredential(ctx context.Context, accessKey string) (LookedUpCredential, error)
	// GetCredentialOwner is a lightweight ownership check for the API
	// layer's self-or-admin authorization — it never decrypts the secret.
	GetCredentialOwner(ctx context.Context, accessKey string) (userID string, err error)
}

type IssuedCredential struct {
	AccessKey string
	SecretKey string
	UserID    string
	CreatedAt time.Time
}

type LookedUpCredential struct {
	UserID    string
	SecretKey string
}
