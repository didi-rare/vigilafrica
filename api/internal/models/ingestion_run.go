package models

import "time"

// IngestionRunStatus represents the lifecycle state of a single ingestion run.
type IngestionRunStatus string

const (
	RunStatusRunning IngestionRunStatus = "running"
	RunStatusSuccess IngestionRunStatus = "success"
	RunStatusFailure IngestionRunStatus = "failure"
	// RunStatusDegraded: the run completed and stored what it could, but GDACS
	// returned no usable answer for at least one polygon event — new ones were
	// not stored, existing ones kept their last verified geometry
	// (fix-gdacs-degraded-run-status). Distinct from
	// failure: data did arrive, and staleness logic treats it as a completed run.
	RunStatusDegraded IngestionRunStatus = "degraded"
)

// IngestionRun records a single EONET ingestion cycle.
// One row is written per run — at start (status=running) and updated at end.
// Used by: /health endpoint, Resend failure alerter, staleness watchdog.
type IngestionRun struct {
	ID            int64              `json:"id"`
	CountryCode   string             `json:"country_code"`
	StartedAt     time.Time          `json:"started_at"`
	CompletedAt   *time.Time         `json:"completed_at"`
	Status        IngestionRunStatus `json:"status"`
	EventsFetched int                `json:"events_fetched"`
	EventsStored  int                `json:"events_stored"`
	Error         *string            `json:"error"`
	CreatedAt     time.Time          `json:"created_at"`
	// AlertSentAt is when the degraded alert covering this run's streak was
	// delivered — the ORIGINAL delivery time, carried forward unchanged while the
	// streak lasts. Internal: never serialised.
	AlertSentAt *time.Time `json:"-"`
}
