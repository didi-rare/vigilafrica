package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"vigilafrica/api/internal/database"
	"vigilafrica/api/internal/digest"
)

// DigestHandler serves the daily flood digest as JSON. It shares
// digest.BuildTodayDigest with the scheduled email so the two never drift.
type DigestHandler struct {
	repo database.Repository
	log  *slog.Logger
}

// NewDigestHandler builds the handler. A nil logger falls back to slog.Default()
// (mirrors alert.NewClient), so tests can pass nil.
func NewDigestHandler(repo database.Repository, logger *slog.Logger) *DigestHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &DigestHandler{repo: repo, log: logger}
}

// GetTodayDigest handles GET /v1/digest/today.json. An empty day is a valid
// 200 with total 0 — never a 404 or 500.
func (h *DigestHandler) GetTodayDigest(w http.ResponseWriter, r *http.Request) {
	d, err := digest.BuildTodayDigest(r.Context(), h.repo, time.Now())
	if err != nil {
		// Log the real cause (§4.5/§8.6); return a sanitised message to the client.
		h.log.Error("today digest failed", "err", err)
		respondWithError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	// best-effort — status code already framed; encode errors are network-level (§4.7).
	_ = json.NewEncoder(w).Encode(d)
}
