package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

const requestTimeout = 30 * time.Second

func NewRouter(h *Handler, logger *slog.Logger) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(requestLogger(logger))
	r.Use(middleware.Timeout(requestTimeout))

	r.Get("/healthz", h.healthz)
	r.Get("/healthz/live", h.healthzLive)
	r.Get("/healthz/ready", h.healthzReady)
	r.Get("/healthz/startup", h.healthzStartup)

	r.Get("/namespaces/{namespace}/objects/{objectHash}", h.HandleDownload)
	r.Delete("/namespaces/{namespace}/objects/{objectHash}", h.HandleDelete)
	r.Post("/namespaces/{namespace}/objects", h.HandleUpload)
	return r
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
