package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"auth-service/internal/api"
	"auth-service/internal/config"
	"auth-service/internal/repository"
	"auth-service/internal/service"
	"auth-service/internal/store"
)

const (
	readTimeout       = 30 * time.Second
	readHeaderTimeout = 5 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 120 * time.Second
	shutdownTimeout   = 10 * time.Second
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

	addr := ":" + cfg.ServerPort
	srv := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadTimeout:       readTimeout,
		ReadHeaderTimeout: readHeaderTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	go func() {
		logger.Info("auth-service listening", slog.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	waitForShutdown(srv, logger)
}

func waitForShutdown(srv *http.Server, logger *slog.Logger) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	logger.Info("shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("graceful shutdown failed", slog.Any("error", err))
	}
}
