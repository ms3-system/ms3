package api

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"auth-service/internal/service"
)

const requestTimeout = 30 * time.Second

func NewRouter(users service.UserService, auth service.AuthService, credentials service.CredentialService, internalToken string, logger *slog.Logger) http.Handler {
	h := NewHandler(users, auth, credentials, logger)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(requestLogger(logger))
	r.Use(middleware.Timeout(requestTimeout))

	r.Get("/healthz", h.healthz)

	r.Route("/v1", func(r chi.Router) {
		r.Route("/users", func(r chi.Router) {
			r.Post("/", h.createUser)

			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", h.getUser)
				r.Post("/credentials", h.createCredential)
			})
		})

		r.Delete("/access-keys/{access_key}", h.revokeCredential)

		r.Route("/auth", func(r chi.Router) {
			r.Post("/login", h.login)
			r.Post("/refresh", h.refresh)
		})
	})

	r.Route("/internal", func(r chi.Router) {
		r.Use(internalTokenAuth(internalToken, logger))
		r.Get("/credentials/{access_key}", h.lookupCredentialInternal)
	})

	return r
}

func internalTokenAuth(token string, logger *slog.Logger) func(http.Handler) http.Handler {
	log := logger.With(slog.String("component", "api.internal_auth"))

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got := r.Header.Get("X-Internal-Token")
			if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
				log.Warn("rejected request: missing or invalid X-Internal-Token",
					slog.String("path", r.URL.Path), slog.String("remote_addr", r.RemoteAddr))
				writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "unauthorized"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
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
