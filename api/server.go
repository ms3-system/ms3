package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Server struct {
	Router *chi.Mux
}

func NewServer() *Server {
	router := chi.NewRouter()
	router.Use(middleware.Logger)
	return &Server{
		Router: router,
	}
}

func (s *Server) Serve() {
	s.Router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello from Serve function!!"))
	})
	http.ListenAndServe(":8080", s.Router)
}
