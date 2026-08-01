package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"auth-service/internal/api"
	"auth-service/internal/config"
	"auth-service/internal/repository"
	"auth-service/internal/service"
	"auth-service/internal/store"
)

const (
	listenAddr = ":8082"

	readHeaderTimeout = 5 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 120 * time.Second
)

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
	authSvc, err := service.NewAuthService(userRepo, []byte(cfg.JWTSecret), logger)
	if err != nil {
		log.Fatalf("Error creating auth service: %v", err)
	}
	credentialSvc := service.NewCredentialService(credentialRepo, userRepo, cfg.MasterKey, logger)

	router := api.NewRouter(userSvc, authSvc, credentialSvc, cfg.InternalToken, logger)

	srv := &http.Server{
		Addr:              listenAddr,
		Handler:           router,
		ReadHeaderTimeout: readHeaderTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	logger.Info("auth-service listening", slog.String("addr", listenAddr))
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Error running server: %v", err)
	}
}
