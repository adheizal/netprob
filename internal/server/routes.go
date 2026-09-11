package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *APIServer) routes() {
	r := s.router

	// Authentication discovery and login remain reachable before authentication.
	r.Get("/api/auth/session", s.HandleGetAuthSession)
	r.Post("/api/auth/login", s.HandleLogin)
	r.Post("/api/auth/logout", s.HandleLogout)

	r.Group(func(r chi.Router) {
		r.Use(s.RequireAdmin)
		r.Put("/api/auth/account", s.HandleUpdateAdminAccount)

		// Agent registration API (browser-side management)
		r.Post("/api/agents/register", s.HandleRegisterAgent)
		r.Get("/api/agents", s.HandleListAgents)
		r.Route("/api/agents/{id}", func(r chi.Router) {
			r.Get("/", s.HandleGetAgent)
			r.Delete("/", s.HandleDeleteAgent)
		})

		// Links
		r.Post("/api/links", s.HandleCreateLink)
		r.Get("/api/links", s.HandleListLinks)
		r.Route("/api/links/{id}", func(r chi.Router) {
			r.Get("/", s.HandleGetLink)
			r.Put("/", s.HandleUpdateLink)
			r.Delete("/", s.HandleDeleteLink)
		})

		// Link directions
		r.Post("/api/links/{link_id}/directions", s.HandleCreateDirection)
		r.Get("/api/links/{link_id}/directions", s.HandleListDirections)
		r.Put("/api/links/{link_id}/directions/{direction_id}", s.HandleUpdateDirection)

		// Probe results
		r.Get("/api/links/{link_id}/directions/{direction_id}/ping", s.HandleGetPingResults)
		r.Get("/api/links/{link_id}/directions/{direction_id}/mtr", s.HandleGetMTRRuns)
		r.Get("/api/mtr-runs/{id}", s.HandleGetMTRRun)

		// Application settings
		r.Get("/api/settings/retention", s.HandleGetRetentionSettings)
		r.Put("/api/settings/retention", s.HandleUpdateRetentionSettings)
		r.Put("/api/settings/security", s.HandleUpdateSecuritySettings)
	})

	// WebSocket for agents
	r.Get("/ws", s.HandleWebSocket)

	// Simple health
	r.Get("/health", s.HandleHealth)
}

func (s *APIServer) Handler() http.Handler {
	return s.router
}
