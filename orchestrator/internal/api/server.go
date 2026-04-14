package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/Konk32/acurve/orchestrator/internal/db"
)

type Server struct {
	store *db.Store
}

func NewServer(store *db.Store) http.Handler {
	s := &Server{store: store}
	r := chi.NewRouter()

	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)

	r.Get("/healthz", s.handleHealth)

	r.Route("/api", func(r chi.Router) {
		r.Get("/sources", s.handleListSources)
		r.Post("/sources", s.handleCreateSource)
		r.Patch("/sources/{id}", s.handleUpdateSource)
		r.Delete("/sources/{id}", s.handleDeleteSource)

		r.Post("/scrape/trigger", s.handleTriggerScrape)
		r.Post("/digest/send", s.handleDigestSend)
		r.Get("/digest/preview", s.handleDigestPreview)
	})

	return r
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleTriggerScrape(w http.ResponseWriter, r *http.Request) {
	// TODO: trigger scraper Job in Kubernetes (Phase 2)
	slog.Info("scrape trigger requested (stub)")
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

func (s *Server) handleDigestSend(w http.ResponseWriter, r *http.Request) {
	// TODO: compose and send digest (Phase 2)
	slog.Info("digest send requested (stub)")
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

func (s *Server) handleDigestPreview(w http.ResponseWriter, r *http.Request) {
	// TODO: preview next digest (Phase 2)
	writeJSON(w, http.StatusOK, map[string]any{"items": []any{}})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write json", "err", err)
	}
}
