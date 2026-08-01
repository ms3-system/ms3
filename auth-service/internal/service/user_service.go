package service

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"auth-service/internal/model"
	"auth-service/internal/repository"
)

var usernameRE = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,32}$`)

const (
	minPasswordLength = 8
	// maxPasswordLength matches bcrypt's hard limit: GenerateFromPassword
	// rejects anything longer with ErrPasswordTooLong. Checking it here
	// keeps that a normal 400 (ErrInvalidInput) instead of a 500 leaking a
	// bcrypt-specific error out of the service layer.
	maxPasswordLength = 72
)

type userService struct {
	users  repository.UserRepository
	logger *slog.Logger
}

func NewUserService(users repository.UserRepository, logger *slog.Logger) UserService {
	return &userService{
		users:  users,
		logger: logger.With(slog.String("component", "service.user")),
	}
}

func (s *userService) Register(ctx context.Context, username, password string) (model.User, error) {
	log := s.logger.With(slog.String("username", username))

	if !usernameRE.MatchString(username) {
		log.Debug("register rejected: invalid username")
		return model.User{}, fmt.Errorf("%w: username must be 3-32 alphanumeric/underscore/hyphen characters, got %q", ErrInvalidInput, username)
	}
	if len(password) < minPasswordLength {
		log.Debug("register rejected: password too short")
		return model.User{}, fmt.Errorf("%w: password must be at least %d characters", ErrInvalidInput, minPasswordLength)
	}
	if len(password) > maxPasswordLength {
		log.Debug("register rejected: password too long")
		return model.User{}, fmt.Errorf("%w: password must be at most %d bytes", ErrInvalidInput, maxPasswordLength)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return model.User{}, fmt.Errorf("hash password: %w", err)
	}

	u := model.User{
		ID:           uuid.NewString(),
		Username:     username,
		PasswordHash: string(hash),
	}

	if err := s.users.Create(ctx, &u); err != nil {
		return model.User{}, err
	}

	log.Info("user registered")
	return u, nil
}

func (s *userService) GetUser(ctx context.Context, id string) (model.User, error) {
	if id == "" {
		return model.User{}, fmt.Errorf("%w: user id is required", ErrInvalidInput)
	}
	return s.users.GetByID(ctx, id)
}
