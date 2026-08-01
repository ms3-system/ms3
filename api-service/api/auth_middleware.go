package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/naseyro/ms3/api-service/clients"
	"github.com/naseyro/ms3/api-service/internal/sigv4"
	"github.com/naseyro/ms3/api-service/utils"
)

// sigv4Region and sigv4Service are fixed rather than configurable — this is
// a small local S3-compatible system, not a multi-region deployment, so
// there's nothing for a client to legitimately pick beyond these.
const (
	sigv4Region  = "us-east-1"
	sigv4Service = "s3"
	maxClockSkew = 15 * time.Minute
)

type contextKey int

const principalContextKey contextKey = iota

// Principal identifies the caller a request was signed by, resolved via
// auth-service's internal credential lookup.
type Principal struct {
	UserID    string
	AccessKey string
}

func principalFromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalContextKey).(Principal)
	return p, ok
}

// requireSigV4 authenticates every request with an AWS SigV4 signature:
// it parses the Authorization header, looks up the signing credential's
// secret key via auth-service, verifies the signature against the live
// request, and stores the resulting Principal in the request context. It
// does not itself authorize any action beyond "this request was validly
// signed by a known access key" — see authorizeBucketOwner for that.
func requireSigV4(auth clients.AuthClient, logger *slog.Logger) func(http.Handler) http.Handler {
	log := logger.With(slog.String("component", "api.auth"))

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				log.Debug("rejected request: missing Authorization header")
				utils.WriteError(w, http.StatusUnauthorized, "missing Authorization header")
				return
			}

			cred, err := sigv4.ParseAuthorization(authHeader)
			if err != nil {
				log.Debug("rejected request: malformed Authorization header", slog.Any("error", err))
				utils.WriteError(w, http.StatusUnauthorized, "malformed Authorization header")
				return
			}
			if cred.Region != sigv4Region || cred.Service != sigv4Service {
				log.Debug("rejected request: unsupported signing region/service",
					slog.String("region", cred.Region), slog.String("service", cred.Service))
				utils.WriteError(w, http.StatusUnauthorized, "unsupported signing region/service")
				return
			}

			credential, err := auth.LookupCredential(r.Context(), cred.AccessKey)
			if err != nil {
				if _, ok := errors.AsType[*clients.NotFoundError](err); ok {
					log.Debug("rejected request: unknown access key", slog.String("access_key", cred.AccessKey))
					utils.WriteError(w, http.StatusUnauthorized, "unknown access key")
					return
				}
				log.Error("credential lookup failed", slog.Any("error", err))
				utils.WriteError(w, http.StatusBadGateway, "auth-service unavailable")
				return
			}

			if err := sigv4.Verify(r, cred, credential.SecretKey, maxClockSkew); err != nil {
				log.Debug("rejected request: signature verification failed", slog.Any("error", err))
				utils.WriteError(w, http.StatusUnauthorized, "signature verification failed")
				return
			}

			ctx := context.WithValue(r.Context(), principalContextKey,
				Principal{UserID: credential.UserID, AccessKey: cred.AccessKey})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// authorizeBucketOwner fetches the named bucket and writes 403 unless the
// principal is its owner. It writes its own error response and returns
// false when the bucket doesn't exist, on an upstream error, or when the
// caller isn't the owner — callers should return immediately when it
// returns false.
func (s *Server) authorizeBucketOwner(w http.ResponseWriter, r *http.Request, bucket string, principal Principal) bool {
	b, err := s.metadata.GetBucket(r.Context(), bucket)
	if err != nil {
		utils.WriteUpstreamError(w, err)
		return false
	}
	if b.OwnerID != principal.UserID {
		utils.WriteUpstreamError(w, &clients.ForbiddenError{Message: "not the bucket owner"})
		return false
	}
	return true
}
