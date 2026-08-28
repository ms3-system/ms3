package api

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/naseyro/ms3/api-service/clients"
	"github.com/naseyro/ms3/api-service/utils"
)

const requestTimeout = 30 * time.Second

const readinessTimeout = 5 * time.Second

type Server struct {
	Router *chi.Mux

	metadata clients.MetadataClient
	data     clients.DataClient
	auth     clients.AuthClient
	logger   *slog.Logger
}

func NewServer(metadata clients.MetadataClient, data clients.DataClient, auth clients.AuthClient, logger *slog.Logger) *Server {
	s := &Server{
		Router:   chi.NewRouter(),
		metadata: metadata,
		data:     data,
		auth:     auth,
		logger:   logger,
	}

	s.Router.Use(middleware.RequestID)
	s.Router.Use(middleware.Recoverer)
	s.Router.Use(requestLogger(logger))
	s.Router.Use(middleware.Timeout(requestTimeout))

	s.Router.Get("/healthz", s.handleHealthz)
	s.Router.Get("/healthz/live", s.handleHealthzLive)
	s.Router.Get("/healthz/ready", s.handleHealthzReady)
	s.Router.Get("/healthz/startup", s.handleHealthzStartup)

	s.Router.Group(func(r chi.Router) {
		//r.Use(requireSigV4(auth, logger))
		s.routes(r)
	})

	return s
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	utils.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleHealthzLive(w http.ResponseWriter, r *http.Request) {
	utils.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleHealthzReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), readinessTimeout)
	defer cancel()

	checks := map[string]func(context.Context) error{
		"metadata": s.metadata.Ping,
		"data":     s.data.Ping,
		"auth":     s.auth.Ping,
	}

	var mu sync.Mutex
	failures := map[string]string{}
	var wg sync.WaitGroup
	for name, ping := range checks {
		wg.Add(1)
		go func(name string, ping func(context.Context) error) {
			defer wg.Done()
			if err := ping(ctx); err != nil {
				mu.Lock()
				failures[name] = err.Error()
				mu.Unlock()
			}
		}(name, ping)
	}
	wg.Wait()

	if len(failures) > 0 {
		s.logger.Warn("readiness check failed", slog.Any("failures", failures))
		utils.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "unavailable", "errors": failures})
		return
	}
	utils.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleHealthzStartup(w http.ResponseWriter, r *http.Request) {
	s.handleHealthzReady(w, r)
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	log := logger.With(slog.String("component", "api.request"))

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			fields := []any{
				slog.String("request_id", middleware.GetReqID(r.Context())),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", ww.Status()),
				slog.Int("bytes", ww.BytesWritten()),
				slog.Duration("duration", time.Since(start)),
			}

			if ww.Status() >= http.StatusInternalServerError {
				log.Error("request completed", fields...)
			} else {
				log.Info("request completed", fields...)
			}
		})
	}
}
