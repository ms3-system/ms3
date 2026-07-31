package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/naseyro/ms3/api-service/clients"
)

type Server struct {
	Router *chi.Mux

	metadata clients.MetadataClient
	data     clients.DataClient
}

func NewServer(metadata clients.MetadataClient, data clients.DataClient) *Server {
	s := &Server{
		Router:   chi.NewRouter(),
		metadata: metadata,
		data:     data,
	}

	s.Router.Use(middleware.Logger)
	s.Router.Use(middleware.Recoverer)

	s.routes()
	return s
}

func (s *Server) Run() error {
	return http.ListenAndServe(":8080", s.Router)
}
