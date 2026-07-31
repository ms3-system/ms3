package api

func (s *Server) routes() {
	s.Router.Put("/buckets/{bucket}", s.handleCreateBucket)
	s.Router.Delete("/buckets/{bucket}", s.handleDeleteBucket)
	s.Router.Get("/buckets/{bucket}", s.handleListObjects)

	s.Router.Put("/buckets/{bucket}/objects/{object}", s.handlePutObject)
	s.Router.Get("/buckets/{bucket}/objects/{object}", s.handleGetObject)
	s.Router.Delete("/buckets/{bucket}/objects/{object}", s.handleDeleteObject)
}
