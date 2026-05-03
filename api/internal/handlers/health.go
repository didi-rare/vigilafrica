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
	CountryCode   string     `json:"country_code,omitempty"`
	Status        *string    `json:"status"`
	StartedAt     *time.Time `json:"started_at"`
	CompletedAt   *time.Time `json:"completed_at"`
	EventsFetched *int       `json:"events_fetched"`
	EventsStored  *int       `json:"events_stored"`
	Error         *string    `json:"error"`
}

// HealthResponse is the response body for GET /health.
type HealthResponse struct {
	Status                 string                            `json:"status"`
	Version                string                            `json:"version"`
	LastIngestion          *lastIngestionResponse            `json:"last_ingestion"`
	LastIngestionByCountry map[string]*lastIngestionResponse `json:"last_ingestion_by_country,omitempty"`
}

// HealthHandler encapsulates the health check logic.
type HealthHandler struct {
	Version       string
	repo          database.Repository
	includeErrors bool
	readiness     bool
}

// NewHealthHandler creates a new HealthHandler.
// repo may be nil (pre-DB startup), in which case last_ingestion is omitted.
func NewHealthHandler(version string, repo database.Repository) *HealthHandler {
	return &HealthHandler{Version: version, repo: repo}
}

func NewReadinessHandler(version string, repo database.Repository) *HealthHandler {
	return &HealthHandler{Version: version, repo: repo, readiness: true}
}

func runToResponse(run *models.IngestionRun, includeErrors bool) *lastIngestionResponse {
	statusStr := string(run.Status)
	resp := &lastIngestionResponse{
		CountryCode: run.CountryCode,
		Status:      &statusStr,
		CompletedAt: run.CompletedAt,
	}
	if includeErrors {
		resp.StartedAt = &run.StartedAt
		resp.EventsFetched = &run.EventsFetched
		resp.EventsStored = &run.EventsStored
		resp.Error = run.Error
	}
	return resp
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
		// Global last run (backward compat)
		run, err := h.repo.GetLastIngestionRun(r.Context())
		if err != nil {
			slog.Error("health: failed to query last ingestion run", "err", err)
		} else if run != nil {
			resp.LastIngestion = runToResponse(run, h.includeErrors)
			if run.Status == models.RunStatusFailure {
				resp.Status = "degraded"
			}
		}

		// Per-country map
		byCountry, err := h.repo.GetLastIngestionRunAllCountries(r.Context())
		if err != nil {
			slog.Error("health: failed to query per-country runs", "err", err)
		} else if len(byCountry) > 0 {
			resp.LastIngestionByCountry = make(map[string]*lastIngestionResponse, len(byCountry))
			for code, cr := range byCountry {
				resp.LastIngestionByCountry[code] = runToResponse(cr, h.includeErrors)
				// Upgrade to degraded if any country's last run failed
				if cr.Status == models.RunStatusFailure && resp.Status != "degraded" {
					resp.Status = "degraded"
				}
			}
		}
	}

	statusCode := http.StatusOK
	if h.readiness && resp.Status == "degraded" {
		statusCode = http.StatusServiceUnavailable
	}
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("health: failed to encode response", "err", err)
	}
}

func LiveHandler(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"version": version,
		}); err != nil {
			slog.Error("live: failed to encode response", "err", err)
		}
	}
}
