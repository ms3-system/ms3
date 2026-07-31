package api

import (
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	api "github.com/naseyro/ms3/api-service/clients"
	"github.com/naseyro/ms3/api-service/utils"
)

func (s *Server) handleCreateBucket(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")

	b, err := s.metadata.CreateBucket(r.Context(), bucket)
	if err != nil {
		utils.WriteUpstreamError(w, err)
		return
	}
	utils.WriteJSON(w, http.StatusCreated, b)
}

func (s *Server) handleDeleteBucket(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")

	if err := s.metadata.DeleteBucket(r.Context(), bucket); err != nil {
		utils.WriteUpstreamError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListObjects(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")

	objs, err := s.metadata.ListObjects(r.Context(), bucket)
	if err != nil {
		utils.WriteUpstreamError(w, err)
		return
	}
	utils.WriteJSON(w, http.StatusOK, objs)
}

func (s *Server) handlePutObject(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	key := chi.URLParam(r, "object")

	const maxUploadBytes = 5 << 30
	body := http.MaxBytesReader(w, r.Body, maxUploadBytes)
	hash, size, err := s.data.Write(r.Context(), bucket, body)
	if err != nil {
		http.Error(w, "failed to store object data", http.StatusBadGateway)
		return
	}

	meta := api.ObjectInfo{
		Key:         key,
		Bucket:      bucket,
		SizeBytes:   size,
		StorageRef:  hash,
		ContentType: r.Header.Get("Content-Type"),
	}

	saved, err := s.metadata.PutObjectMeta(r.Context(), meta)
	if err != nil {
		_ = s.data.Delete(r.Context(), bucket, hash)
		utils.WriteUpstreamError(w, err)
		return
	}

	utils.WriteJSON(w, http.StatusCreated, saved)
}

func (s *Server) handleGetObject(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	key := chi.URLParam(r, "object")

	meta, err := s.metadata.GetObjectMeta(r.Context(), bucket, key)
	if err != nil {
		utils.WriteUpstreamError(w, err)
		return
	}

	body, err := s.data.Read(r.Context(), bucket, meta.StorageRef)
	if err != nil {
		utils.WriteUpstreamError(w, err)
		return
	}
	defer body.Close()

	if meta.ContentType != "" {
		w.Header().Set("Content-Type", meta.ContentType)
	}
	io.Copy(w, body)
}

func (s *Server) handleDeleteObject(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	key := chi.URLParam(r, "object")

	meta, err := s.metadata.GetObjectMeta(r.Context(), bucket, key)
	if err != nil {
		utils.WriteUpstreamError(w, err)
		return
	}

	if err := s.data.Delete(r.Context(), bucket, meta.StorageRef); err != nil {
		http.Error(w, "failed to delete object data", http.StatusBadGateway)
		return
	}

	if err := s.metadata.DeleteObjectMeta(r.Context(), bucket, key); err != nil {
		utils.WriteUpstreamError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
