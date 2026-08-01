package config

import "os"

const (
	defaultServerPort = "8081"
	defaultDataDir    = "data"
)

type Config struct {
	ServerPort string
	DataDir    string
}

func Load() (Config, error) {
	port := os.Getenv("DATA_SERVICE_SERVER_PORT")
	if port == "" {
		port = defaultServerPort
	}

	dataDir := os.Getenv("DATA_SERVICE_DATA_DIR")
	if dataDir == "" {
		dataDir = defaultDataDir
	}

	return Config{
		ServerPort: port,
		DataDir:    dataDir,
	}, nil
}
