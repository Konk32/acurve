package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/Konk32/acurve/orchestrator/internal/db"
	"github.com/Konk32/acurve/orchestrator/internal/digest"
	"github.com/Konk32/acurve/orchestrator/internal/discord"
)

type Server struct {
	store   *db.Store
	discord *discord.Client
}

// NewServer wires up the HTTP handler. discordClient may be nil; if so,
// digest/send will compose but not deliver (useful for testing).
func NewServer(store *db.Store, discordClient *discord.Client) http.Handler {
	s := &Server{store: store, discord: discordClient}
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
	// TODO: trigger scraper Job in Kubernetes (Phase 3)
	slog.Info("scrape trigger requested (stub)")
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

func (s *Server) handleDigestSend(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	const primaryMinScore = 70

	items, err := s.store.GetDigestItems(ctx, primaryMinScore)
	if err != nil {
		slog.Error("get digest items", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Fallback: lower the score threshold if too few items qualify.
	if len(items) < digest.MinItemsBeforeFallback() {
		slog.Info("few items at primary score, trying fallback",
			"primary", primaryMinScore, "fallback", digest.FallbackMinScore())
		items, err = s.store.GetDigestItems(ctx, digest.FallbackMinScore())
		if err != nil {
			slog.Error("get digest items (fallback)", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}

	if len(items) == 0 {
		slog.Info("no items qualify for digest")
		writeJSON(w, http.StatusOK, map[string]any{"sent": false, "reason": "no qualifying items"})
		return
	}

	result := digest.Compose(items)
	slog.Info("digest composed", "embeds", len(result.Embeds), "items", len(result.ItemIDs))

	success := false
	if s.discord != nil {
		if err := s.sendDigestEmbeds(ctx, result.Embeds); err != nil {
			slog.Error("discord send failed", "err", err)
			// Record the attempt as failed but don't return an error to the caller —
			// the digest CronJob shouldn't retry on Discord failures.
		} else {
			success = true
			slog.Info("digest sent to Discord", "items", len(result.ItemIDs))
		}
	} else {
		slog.Warn("discord client not configured — digest composed but not sent")
	}

	if err := s.store.InsertDigest(ctx, "discord", result.ItemIDs, success); err != nil {
		slog.Error("insert digest record", "err", err)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"sent":    success,
		"items":   len(result.ItemIDs),
		"embeds":  len(result.Embeds),
	})
}

// sendDigestEmbeds splits embeds into batches of 10 (Discord limit) and
// POSTs each batch to the webhook.
func (s *Server) sendDigestEmbeds(ctx context.Context, embeds []discord.Embed) error {
	const batchSize = 10
	for i := 0; i < len(embeds); i += batchSize {
		end := i + batchSize
		if end > len(embeds) {
			end = len(embeds)
		}
		payload := discord.WebhookPayload{Embeds: embeds[i:end]}
		if err := s.discord.Send(ctx, payload); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) handleDigestPreview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	items, err := s.store.GetDigestItems(ctx, 70)
	if err != nil {
		slog.Error("get digest items for preview", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	result := digest.Compose(items)
	writeJSON(w, http.StatusOK, map[string]any{
		"item_count":  len(result.ItemIDs),
		"embed_count": len(result.Embeds),
		"item_ids":    result.ItemIDs,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write json", "err", err)
	}
}
