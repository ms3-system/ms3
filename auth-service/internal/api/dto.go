package api

import (
	"time"

	"auth-service/internal/model"
	"auth-service/internal/service"
)

type createUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type userResponse struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	IsAdmin   bool      `json:"is_admin"`
	CreatedAt time.Time `json:"created_at"`
}

func toUserResponse(u model.User) userResponse {
	return userResponse{
		ID:        u.ID,
		Username:  u.Username,
		IsAdmin:   u.IsAdmin,
		CreatedAt: u.CreatedAt,
	}
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type refreshResponse struct {
	AccessToken string `json:"access_token"`
}

type credentialIssuedResponse struct {
	AccessKey string    `json:"access_key"`
	SecretKey string    `json:"secret_key"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

func toCredentialIssuedResponse(c service.IssuedCredential) credentialIssuedResponse {
	return credentialIssuedResponse{
		AccessKey: c.AccessKey,
		SecretKey: c.SecretKey,
		UserID:    c.UserID,
		CreatedAt: c.CreatedAt,
	}
}

type internalCredentialResponse struct {
	UserID    string `json:"user_id"`
	SecretKey string `json:"secret_key"`
}

func toInternalCredentialResponse(c service.LookedUpCredential) internalCredentialResponse {
	return internalCredentialResponse{
		UserID:    c.UserID,
		SecretKey: c.SecretKey,
	}
}
