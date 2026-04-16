package models

import "time"

// IngestionRunStatus represents the lifecycle state of a single ingestion run.
type IngestionRunStatus string

const (
	RunStatusRunning IngestionRunStatus = "running"
	RunStatusSuccess IngestionRunStatus = "success"
	RunStatusFailure IngestionRunStatus = "failure"
)

// IngestionRun records a single EONET ingestion cycle.
// One row is written per run — at start (status=running) and updated at end.
// Used by: /health endpoint, Resend failure alerter, staleness watchdog.
type IngestionRun struct {
	ID            int64              `json:"id"`
	StartedAt     time.Time          `json:"started_at"`
	CompletedAt   *time.Time         `json:"completed_at"`
	Status        IngestionRunStatus `json:"status"`
	EventsFetched int                `json:"events_fetched"`
	EventsStored  int                `json:"events_stored"`
	Error         *string            `json:"error"`
	CreatedAt     time.Time          `json:"created_at"`
}
