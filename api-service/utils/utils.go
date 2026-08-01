package utils

import (
	"encoding/json"
	"errors"
	"net/http"

	api "github.com/naseyro/ms3/api-service/clients"
)

type errorResponse struct {
	Error string `json:"error"`
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, errorResponse{Error: message})
}

func WriteUpstreamError(w http.ResponseWriter, err error) {
	if notFound, ok := errors.AsType[*api.NotFoundError](err); ok {
		WriteError(w, http.StatusNotFound, notFound.Error())
		return
	}

	if conflict, ok := errors.AsType[*api.ConflictError](err); ok {
		WriteError(w, http.StatusConflict, conflict.Error())
		return
	}

	if invalidInput, ok := errors.AsType[*api.InvalidInputError](err); ok {
		WriteError(w, http.StatusBadRequest, invalidInput.Error())
		return
	}

	if forbidden, ok := errors.AsType[*api.ForbiddenError](err); ok {
		WriteError(w, http.StatusForbidden, forbidden.Error())
		return
	}

	WriteError(w, http.StatusBadGateway, "upstream service error")
}
