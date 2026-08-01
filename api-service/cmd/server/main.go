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

	"github.com/naseyro/ms3/api-service/api"
	"github.com/naseyro/ms3/api-service/clients/auth"
	"github.com/naseyro/ms3/api-service/clients/data"
	"github.com/naseyro/ms3/api-service/clients/metadata"
	"github.com/naseyro/ms3/api-service/internal/config"
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

	metadataClient := metadata.NewHTTPMetadataClient(cfg.MetadataURL)
	dataClient := data.NewHTTPDataClient(cfg.DataURL)
	authClient := auth.NewHTTPAuthClient(cfg.AuthURL, cfg.InternalToken)

	srv := api.NewServer(metadataClient, dataClient, authClient, logger)

	addr := ":" + cfg.ServerPort
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           srv.Router,
		ReadTimeout:       readTimeout,
		ReadHeaderTimeout: readHeaderTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	go func() {
		logger.Info("api-service listening", slog.String("addr", addr),
			slog.String("metadata_url", cfg.MetadataURL), slog.String("data_url", cfg.DataURL))
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	waitForShutdown(httpSrv, logger)
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
