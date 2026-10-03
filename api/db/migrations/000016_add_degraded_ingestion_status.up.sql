-- Migration: 000016_add_degraded_ingestion_status
-- Purpose: allow ingestion_runs.status = 'degraded' (fix-gdacs-degraded-run-status).
--
-- A run is degraded when it completed and stored what it could, but GDACS could
-- not be reached for at least one polygon event, so those flood areas are
-- missing from that run. Before this, such a run recorded 'success' and /health
-- stayed green while floods silently dropped off the map.
--
-- The original CHECK was declared inline in 000004, so Postgres named it
-- ingestion_runs_status_check. DROP CONSTRAINT without IF EXISTS on purpose:
-- if that assumption is ever wrong this migration must fail loudly, not leave
-- the old constraint in place and then reject every degraded write at runtime.

ALTER TABLE ingestion_runs DROP CONSTRAINT ingestion_runs_status_check;

ALTER TABLE ingestion_runs
    ADD CONSTRAINT ingestion_runs_status_check
    CHECK (status IN ('running', 'success', 'failure', 'degraded'));
