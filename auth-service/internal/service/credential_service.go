package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"

	"auth-service/internal/model"
	"auth-service/internal/repository"
)

const (
	accessKeyPrefix     = "AKIA"
	accessKeyRandLength = 16
	secretKeyByteLength = 30
	maxAccessKeyRetries = 5

	// base32Alphabet is RFC 4648's alphabet — the same charset AWS uses for
	// its own access key IDs. 32 divides 256 evenly, so mapping a random
	// byte into it via modulo introduces no bias.
	base32Alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
)

type credentialService struct {
	credentials repository.CredentialRepository
	users       repository.UserRepository
	masterKey   []byte
	logger      *slog.Logger
}

func NewCredentialService(credentials repository.CredentialRepository, users repository.UserRepository, masterKey []byte, logger *slog.Logger) CredentialService {
	return &credentialService{
		credentials: credentials,
		users:       users,
		masterKey:   masterKey,
		logger:      logger.With(slog.String("component", "service.credential")),
	}
}

func (s *credentialService) IssueCredential(ctx context.Context, userID string) (IssuedCredential, error) {
	log := s.logger.With(slog.String("user_id", userID))

	if userID == "" {
		return IssuedCredential{}, fmt.Errorf("%w: user id is required", ErrInvalidInput)
	}

	if _, err := s.users.GetByID(ctx, userID); err != nil {
		return IssuedCredential{}, err
	}

	secretKey, err := generateSecretKey()
	if err != nil {
		return IssuedCredential{}, fmt.Errorf("generate secret key: %w", err)
	}

	encrypted, err := s.encrypt(secretKey)
	if err != nil {
		return IssuedCredential{}, fmt.Errorf("encrypt secret key: %w", err)
	}

	var c model.Credential
	for attempt := 0; ; attempt++ {
		accessKey, genErr := generateAccessKey()
		if genErr != nil {
			return IssuedCredential{}, fmt.Errorf("generate access key: %w", genErr)
		}

		c = model.Credential{
			AccessKey:          accessKey,
			UserID:             userID,
			SecretKeyEncrypted: encrypted,
		}

		createErr := s.credentials.Create(ctx, &c)
		if createErr == nil {
			break
		}
		if errors.Is(createErr, repository.ErrAlreadyExists) && attempt < maxAccessKeyRetries-1 {
			log.Warn("access key collision, regenerating", slog.Int("attempt", attempt))
			continue
		}
		return IssuedCredential{}, createErr
	}

	log.Info("credential issued", slog.String("access_key", c.AccessKey))
	return IssuedCredential{
		AccessKey: c.AccessKey,
		SecretKey: secretKey,
		UserID:    userID,
		CreatedAt: c.CreatedAt,
	}, nil
}

func (s *credentialService) RevokeCredential(ctx context.Context, accessKey string) error {
	if accessKey == "" {
		return fmt.Errorf("%w: access key is required", ErrInvalidInput)
	}
	return s.credentials.Revoke(ctx, accessKey)
}

func (s *credentialService) GetCredentialOwner(ctx context.Context, accessKey string) (string, error) {
	if accessKey == "" {
		return "", fmt.Errorf("%w: access key is required", ErrInvalidInput)
	}

	c, err := s.credentials.GetByAccessKey(ctx, accessKey)
	if err != nil {
		return "", err
	}
	return c.UserID, nil
}

func (s *credentialService) LookupCredential(ctx context.Context, accessKey string) (LookedUpCredential, error) {
	if accessKey == "" {
		return LookedUpCredential{}, fmt.Errorf("%w: access key is required", ErrInvalidInput)
	}

	c, err := s.credentials.GetByAccessKey(ctx, accessKey)
	if err != nil {
		return LookedUpCredential{}, err
	}

	secretKey, err := s.decrypt(c.SecretKeyEncrypted)
	if err != nil {
		return LookedUpCredential{}, fmt.Errorf("decrypt secret key: %w", err)
	}

	return LookedUpCredential{UserID: c.UserID, SecretKey: secretKey}, nil
}

func (s *credentialService) encrypt(plaintext string) (string, error) {
	gcm, err := s.gcm()
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (s *credentialService) decrypt(encoded string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}

	gcm, err := s.gcm()
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(raw) < nonceSize {
		return "", errors.New("ciphertext shorter than nonce")
	}
	nonce, ciphertext := raw[:nonceSize], raw[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plaintext), nil
}

func (s *credentialService) gcm() (cipher.AEAD, error) {
	block, err := aes.NewCipher(s.masterKey)
	if err != nil {
		return nil, fmt.Errorf("init AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("init GCM: %w", err)
	}
	return gcm, nil
}

func generateAccessKey() (string, error) {
	suffix, err := randomStringFromAlphabet(base32Alphabet, accessKeyRandLength)
	if err != nil {
		return "", err
	}
	return accessKeyPrefix + suffix, nil
}

func randomStringFromAlphabet(alphabet string, length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i, v := range b {
		b[i] = alphabet[int(v)%len(alphabet)]
	}
	return string(b), nil
}

func generateSecretKey() (string, error) {
	b := make([]byte, secretKeyByteLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}
