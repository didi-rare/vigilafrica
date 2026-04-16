package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"vigilafrica/api/internal/database"
	"vigilafrica/api/internal/models"
)

// lastIngestionResponse is the nested block in the health response (ADR-011).
type lastIngestionResponse struct {
	Status        *string    `json:"status"`
	StartedAt     *time.Time `json:"started_at"`
	CompletedAt   *time.Time `json:"completed_at"`
	EventsFetched *int       `json:"events_fetched"`
	EventsStored  *int       `json:"events_stored"`
	Error         *string    `json:"error"`
}

// HealthResponse is the response body for GET /health.
type HealthResponse struct {
	Status        string                 `json:"status"`
	Version       string                 `json:"version"`
	LastIngestion *lastIngestionResponse `json:"last_ingestion"`
}

// HealthHandler encapsulates the health check logic.
type HealthHandler struct {
	Version string
	repo    database.Repository
}

// NewHealthHandler creates a new HealthHandler.
// repo may be nil (pre-DB startup), in which case last_ingestion is omitted.
func NewHealthHandler(version string, repo database.Repository) *HealthHandler {
	return &HealthHandler{Version: version, repo: repo}
}

// ServeHTTP implements http.Handler for GET /health.
// Returns status "degraded" if the last ingestion run failed.
func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	resp := HealthResponse{
		Status:  "ok",
		Version: h.Version,
	}

	if h.repo != nil {
		run, err := h.repo.GetLastIngestionRun(r.Context())
		if err != nil {
			slog.Error("health: failed to query last ingestion run", "err", err)
			// Do not fail health check on DB query error — return ok with no block
		} else if run != nil {
			statusStr := string(run.Status)
			resp.LastIngestion = &lastIngestionResponse{
				Status:        &statusStr,
				StartedAt:     &run.StartedAt,
				CompletedAt:   run.CompletedAt,
				EventsFetched: &run.EventsFetched,
				EventsStored:  &run.EventsStored,
				Error:         run.Error,
			}
			// Top-level degraded if last run failed
			if run.Status == models.RunStatusFailure {
				resp.Status = "degraded"
			}
		}
	}

	statusCode := http.StatusOK
	if resp.Status == "degraded" {
		statusCode = http.StatusOK // Still 200 — degraded is informational, not an HTTP error
	}
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("health: failed to encode response", "err", err)
	}
}
