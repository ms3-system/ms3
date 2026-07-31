package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"auth-service/internal/repository"
	"auth-service/internal/service"
)

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, logger *slog.Logger, err error) {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		writeJSON(w, http.StatusNotFound, errorResponse{Error: err.Error()})
	case errors.Is(err, repository.ErrAlreadyExists):
		writeJSON(w, http.StatusConflict, errorResponse{Error: err.Error()})
	case errors.Is(err, service.ErrInvalidCredentials):
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: err.Error()})
	case errors.Is(err, service.ErrInvalidInput):
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
	default:
		logger.Error("unhandled error", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal server error"})
	}
}
