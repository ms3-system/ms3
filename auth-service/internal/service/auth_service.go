package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"golang.org/x/crypto/bcrypt"

	"auth-service/internal/repository"
)

type authService struct {
	users  repository.UserRepository
	jwt    jwtIssuer
	logger *slog.Logger
}

func NewAuthService(users repository.UserRepository, jwtSecret []byte, logger *slog.Logger) AuthService {
	return &authService{
		users:  users,
		jwt:    newJWTIssuer(jwtSecret),
		logger: logger.With(slog.String("component", "service.auth")),
	}
}

func (s *authService) Login(ctx context.Context, username, password string) (string, string, error) {
	log := s.logger.With(slog.String("username", username))

	if username == "" || password == "" {
		log.Debug("login rejected: missing username or password")
		return "", "", fmt.Errorf("%w: username and password are required", ErrInvalidInput)
	}

	u, err := s.users.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			log.Debug("login rejected: unknown username")
			return "", "", ErrInvalidCredentials
		}
		return "", "", err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		log.Debug("login rejected: password mismatch")
		return "", "", ErrInvalidCredentials
	}

	access, err := s.jwt.mintAccessToken(u)
	if err != nil {
		return "", "", fmt.Errorf("mint access token: %w", err)
	}
	refresh, err := s.jwt.mintRefreshToken(u)
	if err != nil {
		return "", "", fmt.Errorf("mint refresh token: %w", err)
	}

	log.Info("user logged in")
	return access, refresh, nil
}

func (s *authService) Refresh(ctx context.Context, refreshToken string) (string, error) {
	if refreshToken == "" {
		return "", fmt.Errorf("%w: refresh_token is required", ErrInvalidInput)
	}

	claims, err := s.jwt.parse(refreshToken, tokenTypeRefresh)
	if err != nil {
		s.logger.Debug("refresh rejected: invalid or expired token", slog.Any("error", err))
		return "", ErrInvalidCredentials
	}

	u, err := s.users.GetByID(ctx, claims.Subject)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			s.logger.Debug("refresh rejected: user no longer exists", slog.String("user_id", claims.Subject))
			return "", ErrInvalidCredentials
		}
		return "", err
	}

	access, err := s.jwt.mintAccessToken(u)
	if err != nil {
		return "", fmt.Errorf("mint access token: %w", err)
	}

	s.logger.Info("access token refreshed", slog.String("user_id", u.ID))
	return access, nil
}
