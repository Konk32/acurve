package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/Konk32/acurve/orchestrator/internal/db"
)

func (s *Server) handleListSources(w http.ResponseWriter, r *http.Request) {
	sources, err := s.store.ListSources(r.Context())
	if err != nil {
		slog.Error("list sources", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if sources == nil {
		sources = []db.Source{}
	}
	writeJSON(w, http.StatusOK, sources)
}

func (s *Server) handleCreateSource(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Kind string `json:"kind"`
		URL  string `json:"url"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if body.Kind == "" || body.URL == "" || body.Name == "" {
		http.Error(w, "kind, url, and name are required", http.StatusBadRequest)
		return
	}
	if body.Kind != "rss" && body.Kind != "youtube" && body.Kind != "reddit" {
		http.Error(w, "kind must be rss, youtube, or reddit", http.StatusBadRequest)
		return
	}

	src, err := s.store.CreateSource(r.Context(), body.Kind, body.URL, body.Name)
	if err != nil {
		slog.Error("create source", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, src)
}

func (s *Server) handleUpdateSource(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var body struct {
		Enabled        *bool   `json:"enabled"`
		ScrapeInterval *string `json:"scrape_interval"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	src, err := s.store.UpdateSource(r.Context(), id, db.UpdateSourceParams{
		Enabled:        body.Enabled,
		ScrapeInterval: body.ScrapeInterval,
	})
	if err != nil {
		slog.Error("update source", "id", id, "err", err)
		if isNotFound(err) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, src)
}

func (s *Server) handleDeleteSource(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := s.store.DeleteSource(r.Context(), id); err != nil {
		slog.Error("delete source", "id", id, "err", err)
		if isNotFound(err) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func isNotFound(err error) bool {
	return errors.Is(err, db.ErrNotFound)
}
