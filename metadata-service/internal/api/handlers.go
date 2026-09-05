package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"metadata-service/internal/service"
)

type ReadinessChecker interface {
	Ready(ctx context.Context) error
}

type Handler struct {
	buckets service.BucketService
	objects service.ObjectService
	ready   ReadinessChecker
	logger  *slog.Logger
}

func NewHandler(buckets service.BucketService, objects service.ObjectService, ready ReadinessChecker, logger *slog.Logger) *Handler {
	return &Handler{
		buckets: buckets,
		objects: objects,
		ready:   ready,
		logger:  logger.With(slog.String("component", "api.handler")),
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

func (h *Handler) createBucket(w http.ResponseWriter, r *http.Request) {
	var req createBucketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Debug("create bucket rejected: invalid JSON body", slog.Any("error", err))
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}

	b, err := h.buckets.CreateBucket(r.Context(), req.Name, req.OwnerID)
	if err != nil {
		writeError(w, h.logger, err)
		return
	}

	writeJSON(w, http.StatusCreated, toBucketResponse(b))
}

func (h *Handler) listBuckets(w http.ResponseWriter, r *http.Request) {
	ownerID := r.URL.Query().Get("owner_id")

	buckets, err := h.buckets.ListBuckets(r.Context(), ownerID)
	if err != nil {
		writeError(w, h.logger, err)
		return
	}

	resp := make([]bucketResponse, len(buckets))
	for i, b := range buckets {
		resp[i] = toBucketResponse(b)
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) getBucket(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "bucket")

	b, err := h.buckets.GetBucket(r.Context(), name)
	if err != nil {
		writeError(w, h.logger, err)
		return
	}

	writeJSON(w, http.StatusOK, toBucketResponse(b))
}

func (h *Handler) deleteBucket(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "bucket")

	if err := h.buckets.DeleteBucket(r.Context(), name); err != nil {
		writeError(w, h.logger, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) putObject(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")

	var req putObjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Debug("put object rejected: invalid JSON body", slog.Any("error", err))
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}

	o, err := h.objects.PutObject(r.Context(), bucket, req.Key, req.SizeBytes, req.ETag, req.ContentType, req.StorageRef)
	if err != nil {
		writeError(w, h.logger, err)
		return
	}

	writeJSON(w, http.StatusCreated, toObjectResponse(o))
}

func (h *Handler) listObjects(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	prefix := r.URL.Query().Get("prefix")

	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 0 {
			h.logger.Debug("list objects rejected: invalid limit", slog.String("limit", raw))
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "limit must be a non-negative integer"})
			return
		}
		limit = v
	}

	objects, err := h.objects.ListObjects(r.Context(), bucket, prefix, limit)
	if err != nil {
		writeError(w, h.logger, err)
		return
	}

	resp := make([]objectResponse, len(objects))
	for i, o := range objects {
		resp[i] = toObjectResponse(o)
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) getObject(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	key := chi.URLParam(r, "*")

	o, err := h.objects.GetObject(r.Context(), bucket, key)
	if err != nil {
		writeError(w, h.logger, err)
		return
	}

	writeJSON(w, http.StatusOK, toObjectResponse(o))
}

func (h *Handler) deleteObject(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	key := chi.URLParam(r, "*")

	if err := h.objects.DeleteObject(r.Context(), bucket, key); err != nil {
		writeError(w, h.logger, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
