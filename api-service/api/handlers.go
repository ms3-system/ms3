package api

import (
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	api "github.com/naseyro/ms3/api-service/clients"
	"github.com/naseyro/ms3/api-service/utils"
)

// TODO: requireSigV4 is currently disabled in server.go for local testing.
// Fall back to a fixed principal instead of 401ing every request so
// bucket/object flows stay testable without signing. Remove this fallback
// once requireSigV4 is re-enabled.
func requirePrincipal(_ http.ResponseWriter, r *http.Request) (Principal, bool) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		return Principal{UserID: "local-test-user", AccessKey: "local-test"}, true
	}
	return principal, true
}

func (s *Server) handleListBuckets(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	buckets, err := s.metadata.ListBucketsByOwner(r.Context(), principal.UserID)
	if err != nil {
		utils.WriteUpstreamError(w, err)
		return
	}
	utils.WriteJSON(w, http.StatusOK, buckets)
}

func (s *Server) handleCreateBucket(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	bucket := chi.URLParam(r, "bucket")

	b, err := s.metadata.CreateBucket(r.Context(), bucket, principal.UserID)
	if err != nil {
		utils.WriteUpstreamError(w, err)
		return
	}
	utils.WriteJSON(w, http.StatusCreated, b)
}

func (s *Server) handleDeleteBucket(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	bucket := chi.URLParam(r, "bucket")

	if !s.authorizeBucketOwner(w, r, bucket, principal) {
		return
	}

	if err := s.metadata.DeleteBucket(r.Context(), bucket); err != nil {
		utils.WriteUpstreamError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListObjects(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	bucket := chi.URLParam(r, "bucket")
	prefix := r.URL.Query().Get("prefix")

	if !s.authorizeBucketOwner(w, r, bucket, principal) {
		return
	}

	objs, err := s.metadata.ListObjects(r.Context(), bucket, prefix)
	if err != nil {
		utils.WriteUpstreamError(w, err)
		return
	}
	utils.WriteJSON(w, http.StatusOK, objs)
}

func (s *Server) handlePutObject(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	bucket := chi.URLParam(r, "bucket")
	key := chi.URLParam(r, "*")

	// Authorize before touching the body — an unauthorized caller
	// shouldn't be able to push bytes into data-service at all.
	if !s.authorizeBucketOwner(w, r, bucket, principal) {
		return
	}

	const maxUploadBytes = 5 << 30
	body := http.MaxBytesReader(w, r.Body, maxUploadBytes)
	hash, size, err := s.data.Write(r.Context(), bucket, body)
	if err != nil {
		utils.WriteError(w, http.StatusBadGateway, "failed to store object data")
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
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	bucket := chi.URLParam(r, "bucket")
	key := chi.URLParam(r, "*")

	if !s.authorizeBucketOwner(w, r, bucket, principal) {
		return
	}

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
	_, _ = io.Copy(w, body)
}

// handleHeadObject returns the same headers handleGetObject would set,
// with no body — for a caller that only wants to check an object's
// existence and metadata (size, content type) without downloading it.
func (s *Server) handleHeadObject(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	bucket := chi.URLParam(r, "bucket")
	key := chi.URLParam(r, "*")

	if !s.authorizeBucketOwner(w, r, bucket, principal) {
		return
	}

	meta, err := s.metadata.GetObjectMeta(r.Context(), bucket, key)
	if err != nil {
		utils.WriteUpstreamError(w, err)
		return
	}

	if meta.ContentType != "" {
		w.Header().Set("Content-Type", meta.ContentType)
	}
	if meta.ETag != "" {
		w.Header().Set("ETag", meta.ETag)
	}
	w.Header().Set("Content-Length", strconv.FormatInt(meta.SizeBytes, 10))
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleDeleteObject(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	bucket := chi.URLParam(r, "bucket")
	key := chi.URLParam(r, "*")

	if !s.authorizeBucketOwner(w, r, bucket, principal) {
		return
	}

	meta, err := s.metadata.GetObjectMeta(r.Context(), bucket, key)
	if err != nil {
		utils.WriteUpstreamError(w, err)
		return
	}

	if err := s.data.Delete(r.Context(), bucket, meta.StorageRef); err != nil {
		utils.WriteError(w, http.StatusBadGateway, "failed to delete object data")
		return
	}

	if err := s.metadata.DeleteObjectMeta(r.Context(), bucket, key); err != nil {
		utils.WriteUpstreamError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
