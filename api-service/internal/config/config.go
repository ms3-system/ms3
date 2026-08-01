package config

import (
	"fmt"
	"os"
)

const (
	defaultServerPort  = "8080"
	defaultMetadataURL = "http://localhost:8083"
	defaultDataURL     = "http://localhost:8081"
	defaultAuthURL     = "http://localhost:8082"
)

type Config struct {
	ServerPort    string
	MetadataURL   string
	DataURL       string
	AuthURL       string
	InternalToken string
}

func Load() (Config, error) {
	port := os.Getenv("API_SERVICE_SERVER_PORT")
	if port == "" {
		port = defaultServerPort
	}

	metadataURL := os.Getenv("API_SERVICE_METADATA_URL")
	if metadataURL == "" {
		metadataURL = defaultMetadataURL
	}

	dataURL := os.Getenv("API_SERVICE_DATA_URL")
	if dataURL == "" {
		dataURL = defaultDataURL
	}

	authURL := os.Getenv("API_SERVICE_AUTH_URL")
	if authURL == "" {
		authURL = defaultAuthURL
	}

	// InternalToken is a shared secret with auth-service (must match its
	// AUTH_SERVICE_INTERNAL_TOKEN) — no safe default exists for a shared
	// security credential, so unlike the URLs above, this is required.
	internalToken := os.Getenv("API_SERVICE_INTERNAL_TOKEN")
	if internalToken == "" {
		return Config{}, fmt.Errorf("API_SERVICE_INTERNAL_TOKEN is required (must match auth-service's AUTH_SERVICE_INTERNAL_TOKEN)")
	}

	return Config{
		ServerPort:    port,
		MetadataURL:   metadataURL,
		DataURL:       dataURL,
		AuthURL:       authURL,
		InternalToken: internalToken,
	}, nil
}
