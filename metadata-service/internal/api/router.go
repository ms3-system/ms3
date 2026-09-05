package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"metadata-service/internal/service"
)

const requestTimeout = 30 * time.Second

func NewRouter(buckets service.BucketService, objects service.ObjectService, ready ReadinessChecker, logger *slog.Logger) http.Handler {
	h := NewHandler(buckets, objects, ready, logger)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(requestLogger(logger))
	r.Use(middleware.Timeout(requestTimeout))

	r.Get("/healthz", h.healthz)
	r.Get("/healthz/live", h.healthzLive)
	r.Get("/healthz/ready", h.healthzReady)
	r.Get("/healthz/startup", h.healthzStartup)

	r.Route("/buckets", func(r chi.Router) {
		r.Post("/", h.createBucket)
		r.Get("/", h.listBuckets)

		r.Route("/{bucket}", func(r chi.Router) {
			r.Get("/", h.getBucket)
			r.Delete("/", h.deleteBucket)

			r.Route("/objects", func(r chi.Router) {
				r.Post("/", h.putObject)
				r.Get("/", h.listObjects)
				r.Get("/*", h.getObject)
				r.Delete("/*", h.deleteObject)
			})
		})
	})

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
