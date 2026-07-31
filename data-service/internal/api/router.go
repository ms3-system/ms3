package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter(h *Handler) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Get("/namespaces/{namespace}/objects/{objectHash}", h.HandleDownload)
	r.Delete("/namespaces/{namespace}/objects/{objectHash}", h.HandleDelete)
	r.Post("/namespaces/{namespace}/objects", h.HandleUpload)
	return r
}
