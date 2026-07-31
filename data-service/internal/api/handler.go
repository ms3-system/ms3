package api

import (
	"encoding/json"
	"io"
	"net/http"
	"os"

	"github.com/naseyro/ms3/data-service/internal/storage"
)

type Handler struct {
	store storage.Backend
}

func NewHandler(store storage.Backend) *Handler {
	return &Handler{store: store}
}

// HandleUpload handles HTTP POST/PUT streams
func (h *Handler) HandleUpload(w http.ResponseWriter, r *http.Request) {
	namespace := r.PathValue("namespace")
	if namespace == "" {
		namespace = "default-bucket"
	}
	hash, size, err := h.store.Write(r.Context(), namespace, r.Body)
	if err != nil {
		http.Error(w, "Failed to write data", http.StatusInternalServerError)
		return
	}

	// Return the CAS results back to the API Gateway
	// API Gateway should send this to Metadata Service to MetaDB
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"hash":   hash,
		"size":   size,
		"bucket": namespace,
	})
}

// HandleDownload translates an HTTP request into a Storage Read operation
func (h *Handler) HandleDownload(w http.ResponseWriter, r *http.Request) {
	hash := r.PathValue("objectHash")
	namespace := r.PathValue("namespace")

	// 2. Ask the storage interface for the stream
	file, err := h.store.Read(r.Context(), namespace, hash)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	// 3. Stream the bytes back to the API Gateway
	w.Header().Set("Content-Type", "application/octet-stream")
	io.Copy(w, file)
}

func (h *Handler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	hash := r.PathValue("objectHash")
	namespace := r.PathValue("namespace")

	err := h.store.Delete(r.Context(), namespace, hash)
	if err != nil {
		// File does not exist
		if os.IsNotExist(err) {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		http.Error(w, "Failed to delete physical file", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
