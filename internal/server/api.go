package server

import (
	"netprob/internal/config"
	"netprob/internal/storage"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// APIServer is the REST API HTTP server.
type APIServer struct {
	router *chi.Mux
	store  *storage.Store
	hub    *Hub
	cfg    *config.Config
}

func NewAPIServer(store *storage.Store, hub *Hub, cfg *config.Config) *APIServer {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	s := &APIServer{
		router: r,
		store:  store,
		hub:    hub,
		cfg:    cfg,
	}
	s.routes()
	return s
}
