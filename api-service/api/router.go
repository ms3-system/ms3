package api

import "github.com/go-chi/chi/v5"

func (s *Server) routes(r chi.Router) {
	r.Get("/", s.handleListBuckets)

	r.Put("/buckets/{bucket}", s.handleCreateBucket)
	r.Delete("/buckets/{bucket}", s.handleDeleteBucket)
	r.Get("/buckets/{bucket}", s.handleListObjects)

	r.Put("/buckets/{bucket}/objects/*", s.handlePutObject)
	r.Get("/buckets/{bucket}/objects/*", s.handleGetObject)
	r.Head("/buckets/{bucket}/objects/*", s.handleHeadObject)
	r.Delete("/buckets/{bucket}/objects/*", s.handleDeleteObject)
}
