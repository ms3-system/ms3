package api

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"auth-service/internal/service"
)

type contextKey int

const principalContextKey contextKey = iota

// requireAuth authenticates a JWT bearer access token and stores the
// resulting service.Principal in the request context for handlers to make
// self-or-admin authorization decisions with. It does not itself authorize
// anything beyond "this is a validly signed, unexpired access token."
func requireAuth(auth service.AuthService, logger *slog.Logger) func(http.Handler) http.Handler {
	log := logger.With(slog.String("component", "api.auth"))

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || token == "" {
				log.Debug("rejected request: missing or malformed Authorization header")
				writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "unauthorized"})
				return
			}

			principal, err := auth.VerifyAccessToken(token)
			if err != nil {
				log.Debug("rejected request: invalid access token", slog.Any("error", err))
				writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "unauthorized"})
				return
			}

			ctx := context.WithValue(r.Context(), principalContextKey, principal)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func principalFromContext(ctx context.Context) (service.Principal, bool) {
	p, ok := ctx.Value(principalContextKey).(service.Principal)
	return p, ok
}

// requireSelfOrAdmin writes 403 and returns false unless the authenticated
// principal is an admin or the principal whose resource is being accessed.
// Callers must run requireAuth first — a missing principal is treated as
// unauthorized rather than forbidden, since it means the middleware chain
// itself is misconfigured.
func requireSelfOrAdmin(w http.ResponseWriter, r *http.Request, targetUserID string) bool {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "unauthorized"})
		return false
	}
	if principal.IsAdmin || principal.UserID == targetUserID {
		return true
	}
	writeJSON(w, http.StatusForbidden, errorResponse{Error: "forbidden"})
	return false
}
