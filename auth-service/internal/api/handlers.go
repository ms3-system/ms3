package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"auth-service/internal/service"
)

type Handler struct {
	users       service.UserService
	auth        service.AuthService
	credentials service.CredentialService
	logger      *slog.Logger
}

func NewHandler(users service.UserService, auth service.AuthService, credentials service.CredentialService, logger *slog.Logger) *Handler {
	return &Handler{
		users:       users,
		auth:        auth,
		credentials: credentials,
		logger:      logger.With(slog.String("component", "api.handler")),
	}
}

func (h *Handler) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) createUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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

	u, err := h.users.GetUser(r.Context(), id)
	if err != nil {
		writeError(w, h.logger, err)
		return
	}

	writeJSON(w, http.StatusOK, toUserResponse(u))
}

func (h *Handler) createCredential(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")

	c, err := h.credentials.IssueCredential(r.Context(), userID)
	if err != nil {
		writeError(w, h.logger, err)
		return
	}

	writeJSON(w, http.StatusCreated, toCredentialIssuedResponse(c))
}

func (h *Handler) revokeCredential(w http.ResponseWriter, r *http.Request) {
	accessKey := chi.URLParam(r, "access_key")

	if err := h.credentials.RevokeCredential(r.Context(), accessKey); err != nil {
		writeError(w, h.logger, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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
