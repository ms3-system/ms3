package config

import (
	"os"

	"metadata-service/internal/store"
)

const defaultServerPort = "8083"

type Config struct {
	ServerPort string
	DBPath     string
}

func Load() (Config, error) {
	port := os.Getenv("METADATA_SERVICE_SERVER_PORT")
	if port == "" {
		port = defaultServerPort
	}

	dbPath := os.Getenv("METADATA_SERVICE_DB_PATH")
	if dbPath == "" {
		dbPath = store.DefaultDBPath
	}

	return Config{
		ServerPort: port,
		DBPath:     dbPath,
	}, nil
}
