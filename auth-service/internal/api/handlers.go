package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"auth-service/internal/service"
)

type ReadinessChecker interface {
	Ready(ctx context.Context) error
}

type Handler struct {
	users       service.UserService
	auth        service.AuthService
	credentials service.CredentialService
	ready       ReadinessChecker
	logger      *slog.Logger
}

func NewHandler(users service.UserService, auth service.AuthService, credentials service.CredentialService, ready ReadinessChecker, logger *slog.Logger) *Handler {
	return &Handler{
		users:       users,
		auth:        auth,
		credentials: credentials,
		ready:       ready,
		logger:      logger.With(slog.String("component", "api.handler")),
	}
}

func (h *Handler) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) healthzLive(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) healthzReady(w http.ResponseWriter, r *http.Request) {
	if h.ready != nil {
		if err := h.ready.Ready(r.Context()); err != nil {
			h.logger.Warn("readiness check failed", slog.Any("error", err))
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable", "error": err.Error()})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) healthzStartup(w http.ResponseWriter, r *http.Request) {
	h.healthzReady(w, r)
}

func (h *Handler) createUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.logger.Debug("create user rejected: invalid JSON body", slog.Any("error", err))
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}

	u, err := h.users.Register(r.Context(), req.Username, req.Password)
	if err != nil {
		writeError(w, h.logger, err)
		return
	}

	writeJSON(w, http.StatusCreated, toUserResponse(u))
}

func (h *Handler) getUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if !requireSelfOrAdmin(w, r, id) {
		return
	}

	u, err := h.users.GetUser(r.Context(), id)
	if err != nil {
		writeError(w, h.logger, err)
		return
	}

	writeJSON(w, http.StatusOK, toUserResponse(u))
}

func (h *Handler) createCredential(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")

	if !requireSelfOrAdmin(w, r, userID) {
		return
	}

	c, err := h.credentials.IssueCredential(r.Context(), userID)
	if err != nil {
		writeError(w, h.logger, err)
		return
	}

	writeJSON(w, http.StatusCreated, toCredentialIssuedResponse(c))
}

func (h *Handler) revokeCredential(w http.ResponseWriter, r *http.Request) {
	accessKey := chi.URLParam(r, "access_key")

	// Ownership is only known once the credential is looked up, so the
	// self-or-admin check happens here rather than via requireSelfOrAdmin
	// on the target path param (there isn't one — the path only has the
	// access key).
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "unauthorized"})
		return
	}
	if !principal.IsAdmin {
		ownerID, err := h.credentials.GetCredentialOwner(r.Context(), accessKey)
		if err != nil {
			writeError(w, h.logger, err)
			return
		}
		if ownerID != principal.UserID {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "forbidden"})
			return
		}
	}

	if err := h.credentials.RevokeCredential(r.Context(), accessKey); err != nil {
		writeError(w, h.logger, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.logger.Debug("login rejected: invalid JSON body", slog.Any("error", err))
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}

	access, refresh, err := h.auth.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		writeError(w, h.logger, err)
		return
	}

	writeJSON(w, http.StatusOK, loginResponse{AccessToken: access, RefreshToken: refresh})
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.logger.Debug("refresh rejected: invalid JSON body", slog.Any("error", err))
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}

	access, err := h.auth.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		writeError(w, h.logger, err)
		return
	}

	writeJSON(w, http.StatusOK, refreshResponse{AccessToken: access})
}

func (h *Handler) lookupCredentialInternal(w http.ResponseWriter, r *http.Request) {
	accessKey := chi.URLParam(r, "access_key")

	c, err := h.credentials.LookupCredential(r.Context(), accessKey)
	if err != nil {
		writeError(w, h.logger, err)
		return
	}

	writeJSON(w, http.StatusOK, toInternalCredentialResponse(c))
}
