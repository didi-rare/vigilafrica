package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"vigilafrica/api/internal/database"
)

type EnrichmentStatsHandler struct {
	repo database.Repository
}

func NewEnrichmentStatsHandler(repo database.Repository) *EnrichmentStatsHandler {
	return &EnrichmentStatsHandler{repo: repo}
}

// ServeHTTP handles GET /v1/enrichment-stats.
// Returns per-country enrichment success rates — used to verify the ≥85% target.
func (h *EnrichmentStatsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	stats, err := h.repo.GetEnrichmentStats(r.Context())
	if err != nil {
		slog.Error("enrichment-stats: query failed", "err", err)
		respondWithError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{"stats": stats}); err != nil {
		slog.Error("enrichment-stats: failed to encode response", "err", err)
	}
}

