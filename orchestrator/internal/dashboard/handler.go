package dashboard

import (
	"embed"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/Konk32/acurve/orchestrator/internal/db"
	"github.com/Konk32/acurve/orchestrator/internal/digest"
)

//go:embed templates
var templateFS embed.FS

// Handler serves the HTMX dashboard.
type Handler struct {
	store *db.Store
	tmpl  *template.Template
}

// NewHandler parses the embedded templates and returns a Handler.
func NewHandler(store *db.Store) (*Handler, error) {
	tmpl, err := template.New("").ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &Handler{store: store, tmpl: tmpl}, nil
}

// Routes returns an http.Handler for all dashboard endpoints (mounted at /dashboard).
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/", h.handleIndex)
	r.Post("/sources", h.handleCreateSource)
	r.Delete("/sources/{id}", h.handleDeleteSource)
	r.Patch("/sources/{id}/toggle", h.handleToggleSource)
	r.Get("/digest-preview", h.handleDigestPreview)
	return r
}

// HandleIndex is exported so the server can also serve it at GET /.
func (h *Handler) HandleIndex(w http.ResponseWriter, r *http.Request) {
	h.handleIndex(w, r)
}

// handleIndex renders the full dashboard page.
func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	sources, err := h.store.ListSources(r.Context())
	if err != nil {
		slog.Error("dashboard list sources", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	data := struct{ Sources []db.Source }{Sources: sources}
	h.renderFull(w, "index.html", data)
}

// handleCreateSource creates a new source from a form POST and returns
// the updated sources table partial.
func (h *Handler) handleCreateSource(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	kind := r.FormValue("kind")
	url := r.FormValue("url")
	name := r.FormValue("name")

	if kind == "" || url == "" || name == "" {
		http.Error(w, "kind, url, and name are required", http.StatusBadRequest)
		return
	}
	if kind != "rss" && kind != "youtube" && kind != "reddit" {
		http.Error(w, "kind must be rss, youtube, or reddit", http.StatusBadRequest)
		return
	}

	if _, err := h.store.CreateSource(r.Context(), kind, url, name); err != nil {
		slog.Error("dashboard create source", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	h.renderSourcesTable(w, r)
}

// handleDeleteSource deletes a source and returns the updated sources table.
func (h *Handler) handleDeleteSource(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := h.store.DeleteSource(r.Context(), id); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		slog.Error("dashboard delete source", "id", id, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	h.renderSourcesTable(w, r)
}

// handleToggleSource flips the enabled flag on a source.
func (h *Handler) handleToggleSource(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	src, err := h.store.GetSource(r.Context(), id)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		slog.Error("dashboard get source for toggle", "id", id, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	flipped := !src.Enabled
	if _, err := h.store.UpdateSource(r.Context(), id, db.UpdateSourceParams{
		Enabled: &flipped,
	}); err != nil {
		slog.Error("dashboard toggle source", "id", id, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	h.renderSourcesTable(w, r)
}

// handleDigestPreview returns an HTML snippet with the upcoming digest stats.
func (h *Handler) handleDigestPreview(w http.ResponseWriter, r *http.Request) {
	const primaryMinScore = 70

	items, err := h.store.GetDigestItems(r.Context(), primaryMinScore)
	if err != nil {
		slog.Error("dashboard digest preview", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	result := digest.Compose(items)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if len(result.ItemIDs) == 0 {
		w.Write([]byte(`<p class="muted">No items qualify yet (score ≥ 70). Run the summarizer first.</p>`)) //nolint:errcheck
		return
	}

	w.Write([]byte(`<p>` + //nolint:errcheck
		strconv.Itoa(len(result.ItemIDs)) + ` items across ` +
		strconv.Itoa(len(result.Embeds)) + ` categories ready to send.</p>`))
}

// renderSourcesTable re-fetches sources and renders the sources-table partial.
func (h *Handler) renderSourcesTable(w http.ResponseWriter, r *http.Request) {
	sources, err := h.store.ListSources(r.Context())
	if err != nil {
		slog.Error("dashboard list sources", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "sources-table", sources); err != nil {
		slog.Error("dashboard render sources-table", "err", err)
	}
}

// renderFull renders a full-page template by filename.
func (h *Handler) renderFull(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, name, data); err != nil {
		slog.Error("dashboard render", "template", name, "err", err)
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}
