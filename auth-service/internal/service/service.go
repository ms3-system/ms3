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
}

type CredentialService interface {
	IssueCredential(ctx context.Context, userID string) (IssuedCredential, error)
	RevokeCredential(ctx context.Context, accessKey string) error
	LookupCredential(ctx context.Context, accessKey string) (LookedUpCredential, error)
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
