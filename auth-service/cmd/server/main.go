package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"

	"auth-service/internal/api"
	"auth-service/internal/config"
	"auth-service/internal/repository"
	"auth-service/internal/service"
	"auth-service/internal/store"
)

const listenAddr = ":8082"

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("Error opening store: %v", err)
	}
	defer func() {
		if err := st.Close(); err != nil {
			logger.Error("error closing store", slog.Any("error", err))
		}
	}()

	userRepo := repository.NewBoltUserRepository(st.DB(), logger)
	credentialRepo := repository.NewBoltCredentialRepository(st.DB(), logger)

	userSvc := service.NewUserService(userRepo, logger)
	authSvc := service.NewAuthService(userRepo, []byte(cfg.JWTSecret), logger)
	credentialSvc := service.NewCredentialService(credentialRepo, userRepo, cfg.MasterKey, logger)

	router := api.NewRouter(userSvc, authSvc, credentialSvc, cfg.InternalToken, logger)

	logger.Info("auth-service listening", slog.String("addr", listenAddr))
	if err := http.ListenAndServe(listenAddr, router); err != nil {
		log.Fatalf("Error running server: %v", err)
	}
}
