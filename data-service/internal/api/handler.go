package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"

	"github.com/naseyro/ms3/data-service/internal/storage"
)

type readinessChecker interface {
	Ready(ctx context.Context) error
}

type Handler struct {
	store  storage.Backend
	logger *slog.Logger
}

func NewHandler(store storage.Backend, logger *slog.Logger) *Handler {
	return &Handler{
		store:  store,
		logger: logger.With(slog.String("component", "api.handler")),
	}
}

func (h *Handler) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) healthzLive(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) healthzReady(w http.ResponseWriter, r *http.Request) {
	if checker, ok := h.store.(readinessChecker); ok {
		if err := checker.Ready(r.Context()); err != nil {
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

func (h *Handler) HandleUpload(w http.ResponseWriter, r *http.Request) {
	namespace := r.PathValue("namespace")
	if namespace == "" {
		namespace = "default-bucket"
	}
	hash, size, err := h.store.Write(r.Context(), namespace, r.Body)
	if err != nil {
		h.logger.Error("write failed", slog.Any("error", err))
		writeError(w, http.StatusInternalServerError, "failed to write data")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"hash":   hash,
		"size":   size,
		"bucket": namespace,
	})
}

func (h *Handler) HandleDownload(w http.ResponseWriter, r *http.Request) {
	hash := r.PathValue("objectHash")
	namespace := r.PathValue("namespace")

	file, err := h.store.Read(r.Context(), namespace, hash)
	if err != nil {
		writeError(w, http.StatusNotFound, "object not found")
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	if _, err := io.Copy(w, file); err != nil {
		h.logger.Warn("download interrupted", slog.Any("error", err))
	}
}

func (h *Handler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	hash := r.PathValue("objectHash")
	namespace := r.PathValue("namespace")

	err := h.store.Delete(r.Context(), namespace, hash)
	if err != nil {
		if os.IsNotExist(err) {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		h.logger.Error("delete failed", slog.Any("error", err))
		writeError(w, http.StatusInternalServerError, "failed to delete physical file")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
