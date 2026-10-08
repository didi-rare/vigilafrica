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
	// Defensive nil-check: all current callers guard with `if run != nil`, but
	// a future caller that forgets would otherwise panic on the dereference
	// below. See chore-post-v11-quality-sweep B3.
	if run == nil {
		return nil
	}
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
// Returns status "degraded" if any country's last ingestion run failed OR was
// degraded (GDACS unreachable for some polygon events).
//
// ⚠️ The two are reported the same way on /health but NOT on /ready: readiness
// answers "can this API serve requests?", and an upstream data provider being
// down does not make it unable to. /ready returns 503 for a real failure only
// (fix-gdacs-degraded-run-status).
func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	resp := HealthResponse{
		Status:  "ok",
		Version: h.Version,
	}
	anyFailure := false
	// queryFailed: the ingestion-run lookups themselves errored, i.e. the database
	// is unreachable. /health stays 200 (it is the liveness probe the container
	// healthcheck uses, and must not restart the API over a DB blip), but /ready
	// must not claim readiness it cannot verify (independent review, PR #280;
	// pre-existing — errors here were only ever logged).
	queryFailed := false
	note := func(s models.IngestionRunStatus) {
		switch s {
		case models.RunStatusFailure:
			anyFailure = true
			resp.Status = "degraded"
		case models.RunStatusDegraded:
			resp.Status = "degraded"
		}
	}

	if h.repo != nil {
		// Global last run (backward compat)
		run, err := h.repo.GetLastIngestionRun(r.Context())
		if err != nil {
			queryFailed = true
			slog.Error("health: failed to query last ingestion run", "err", err)
		} else if run != nil {
			resp.LastIngestion = runToResponse(run, h.includeErrors)
			note(run.Status)
		}

		// Per-country map
		byCountry, err := h.repo.GetLastIngestionRunAllCountries(r.Context())
		if err != nil {
			queryFailed = true
			slog.Error("health: failed to query per-country runs", "err", err)
		} else if len(byCountry) > 0 {
			resp.LastIngestionByCountry = make(map[string]*lastIngestionResponse, len(byCountry))
			for code, cr := range byCountry {
				resp.LastIngestionByCountry[code] = runToResponse(cr, h.includeErrors)
				note(cr.Status)
			}
		}
	}

	statusCode := http.StatusOK
	if h.readiness && (anyFailure || queryFailed) {
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
