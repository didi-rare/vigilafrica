-- Migration: 000016_add_degraded_ingestion_status
-- Purpose: allow ingestion_runs.status = 'degraded' (fix-gdacs-degraded-run-status).
--
-- A run is degraded when it completed and stored what it could, but GDACS could
-- not be reached for at least one polygon event, so those flood areas are
-- missing from that run. Before this, such a run recorded 'success' and /health
-- stayed green while floods silently dropped off the map.
--
-- The original CHECK was declared inline in 000004, so Postgres named it
-- ingestion_runs_status_check.
--
-- Replayable per developers-go.md §11.4 (independent review, PR #280, round 2):
-- IF EXISTS / IF NOT EXISTS throughout, so re-running after a dirty-version
-- recovery does not wedge. The cost is that a wrong constraint name would no
-- longer fail here; it is caught instead by the integration test that writes a
-- 'degraded' run, which fails if the old constraint survived.

ALTER TABLE ingestion_runs DROP CONSTRAINT IF EXISTS ingestion_runs_status_check;

ALTER TABLE ingestion_runs
    ADD CONSTRAINT ingestion_runs_status_check
    CHECK (status IN ('running', 'success', 'failure', 'degraded'));

-- When the degraded alert for this run's streak was actually delivered.
-- Deduplication keys on DELIVERY, not on status: an earlier design compared only
-- the previous run's status, so a failed send left the streak marked degraded
-- and every later run suppressed the alert forever (independent review, PR #280).
ALTER TABLE ingestion_runs ADD COLUMN IF NOT EXISTS alert_sent_at TIMESTAMPTZ;
