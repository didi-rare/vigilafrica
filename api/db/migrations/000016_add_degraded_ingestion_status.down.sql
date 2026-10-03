-- Reverts 000016. Rows recorded as 'degraded' DID complete ingestion, so they
-- map back to 'success' — the value they would have carried before 000016.
-- (Lossy by necessity: the old constraint cannot represent 'degraded'.)

ALTER TABLE ingestion_runs DROP COLUMN IF EXISTS alert_sent_at;

UPDATE ingestion_runs SET status = 'success' WHERE status = 'degraded';

ALTER TABLE ingestion_runs DROP CONSTRAINT IF EXISTS ingestion_runs_status_check;

ALTER TABLE ingestion_runs
    ADD CONSTRAINT ingestion_runs_status_check
    CHECK (status IN ('running', 'success', 'failure'));
