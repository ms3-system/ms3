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

// maxRequestBodyBytes bounds request bodies decoded by decodeJSON, so a
// client can't force a handler to buffer an unbounded body into memory.
const maxRequestBodyBytes = 1 << 20 // 1 MiB

// decodeJSON decodes r.Body into dst, capping the body size and rejecting
// unknown fields so a typo'd request field fails loudly instead of being
// silently ignored.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
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
