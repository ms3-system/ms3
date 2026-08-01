package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"metadata-service/internal/api"
	"metadata-service/internal/config"
	"metadata-service/internal/repository"
	"metadata-service/internal/service"
	"metadata-service/internal/store"
)

const (
	readTimeout       = 30 * time.Second
	readHeaderTimeout = 5 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 120 * time.Second
	shutdownTimeout   = 10 * time.Second
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("error loading config", slog.Any("error", err))
		os.Exit(1)
	}

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		logger.Error("error opening store", slog.Any("error", err))
		os.Exit(1)
	}
	defer func() {
		if err := st.Close(); err != nil {
			logger.Error("error closing store", slog.Any("error", err))
		}
	}()

	bucketRepo := repository.NewBoltBucketRepository(st.DB(), logger)
	objectRepo := repository.NewBoltObjectRepository(st.DB(), logger)

	bucketSvc := service.NewBucketService(bucketRepo, logger)
	objectSvc := service.NewObjectService(objectRepo, bucketRepo, logger)

	router := api.NewRouter(bucketSvc, objectSvc, logger)

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
		logger.Info("metadata-service listening", slog.String("addr", addr))
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
