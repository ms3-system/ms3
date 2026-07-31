package config

import (
	"encoding/base64"
	"fmt"
	"os"

	"auth-service/internal/store"
)

const masterKeySize = 32 // AES-256

type Config struct {
	JWTSecret     string
	MasterKey     []byte
	InternalToken string
	DBPath        string
}

func Load() (Config, error) {
	jwtSecret := os.Getenv("AUTH_SERVICE_JWT_SECRET")
	if jwtSecret == "" {
		return Config{}, fmt.Errorf("AUTH_SERVICE_JWT_SECRET is required")
	}

	internalToken := os.Getenv("AUTH_SERVICE_INTERNAL_TOKEN")
	if internalToken == "" {
		return Config{}, fmt.Errorf("AUTH_SERVICE_INTERNAL_TOKEN is required")
	}

	masterKeyRaw := os.Getenv("AUTH_SERVICE_MASTER_KEY")
	if masterKeyRaw == "" {
		return Config{}, fmt.Errorf("AUTH_SERVICE_MASTER_KEY is required")
	}
	masterKey, err := base64.StdEncoding.DecodeString(masterKeyRaw)
	if err != nil {
		return Config{}, fmt.Errorf("AUTH_SERVICE_MASTER_KEY must be base64-encoded: %w", err)
	}
	if len(masterKey) != masterKeySize {
		return Config{}, fmt.Errorf("AUTH_SERVICE_MASTER_KEY must decode to %d bytes, got %d", masterKeySize, len(masterKey))
	}

	dbPath := os.Getenv("AUTH_SERVICE_DB_PATH")
	if dbPath == "" {
		dbPath = store.DefaultDBPath
	}

	return Config{
		JWTSecret:     jwtSecret,
		MasterKey:     masterKey,
		InternalToken: internalToken,
		DBPath:        dbPath,
	}, nil
}
