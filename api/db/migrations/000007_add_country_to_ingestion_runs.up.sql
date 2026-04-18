-- Add country_code to ingestion_runs — v0.7 per-country observability (Note B).
--
-- Prior to v0.7 all runs were Nigeria-only, so DEFAULT 'NG' correctly backfills
-- the historical rows. The index enables the DISTINCT ON query used by /health.

ALTER TABLE ingestion_runs
  ADD COLUMN IF NOT EXISTS country_code TEXT NOT NULL DEFAULT 'NG';

CREATE INDEX IF NOT EXISTS idx_ingestion_runs_country_started
  ON ingestion_runs (country_code, started_at DESC);
