package utils

import (
	"encoding/json"
	"errors"
	"net/http"

	api "github.com/naseyro/ms3/api-service/clients"
)

func WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func WriteUpstreamError(w http.ResponseWriter, err error) {
	var notFound *api.NotFoundError
	if errors.As(err, &notFound) {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	http.Error(w, "upstream service error", http.StatusBadGateway)
}
