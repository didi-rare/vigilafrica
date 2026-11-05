-- Migration: 000016_add_degraded_ingestion_status (down)
-- Purpose: revert 000016 — drop alert_sent_at and restore the three-value status CHECK.
-- Milestone: fix-gdacs-degraded-run-status.
-- Data quality: production-quality. Lossy: rows recorded as 'degraded' DID complete
--   ingestion, so they map back to 'success' — the value they would have carried
--   before 000016; the old constraint cannot represent 'degraded'.

ALTER TABLE ingestion_runs DROP COLUMN IF EXISTS alert_sent_at;

UPDATE ingestion_runs SET status = 'success' WHERE status = 'degraded';

ALTER TABLE ingestion_runs DROP CONSTRAINT IF EXISTS ingestion_runs_status_check;

ALTER TABLE ingestion_runs
    ADD CONSTRAINT ingestion_runs_status_check
    CHECK (status IN ('running', 'success', 'failure'));
