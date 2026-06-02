package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"vigilafrica/api/internal/database"
	"vigilafrica/api/internal/digest"
)

// DigestHandler serves the daily flood digest as JSON. It shares
// digest.BuildTodayDigest with the scheduled email so the two never drift.
type DigestHandler struct {
	repo database.Repository
}

func NewDigestHandler(repo database.Repository) *DigestHandler {
	return &DigestHandler{repo: repo}
}

// GetTodayDigest handles GET /v1/digest/today.json. An empty day is a valid
// 200 with total 0 — never a 404 or 500.
func (h *DigestHandler) GetTodayDigest(w http.ResponseWriter, r *http.Request) {
	d, err := digest.BuildTodayDigest(r.Context(), h.repo, time.Now())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	// best-effort — status code already framed; encode errors are network-level (§4.7).
	_ = json.NewEncoder(w).Encode(d)
}
