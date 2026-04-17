-- Migration: 000004_create_ingestion_runs
-- Purpose: Track every EONET ingestion run for observability and alerting (ADR-011)

CREATE TABLE ingestion_runs (
    id             SERIAL       PRIMARY KEY,
    started_at     TIMESTAMPTZ  NOT NULL,
    completed_at   TIMESTAMPTZ,
    status         TEXT         NOT NULL CHECK (status IN ('running', 'success', 'failure')),
    events_fetched INT          NOT NULL DEFAULT 0,
    events_stored  INT          NOT NULL DEFAULT 0,
    error          TEXT,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Index for fast "get most recent run" queries used by /health and watchdog
CREATE INDEX idx_ingestion_runs_started_at ON ingestion_runs(started_at DESC);
CREATE INDEX idx_ingestion_runs_status     ON ingestion_runs(status);
