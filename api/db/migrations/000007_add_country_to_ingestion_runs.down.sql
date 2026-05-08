DROP INDEX IF EXISTS idx_ingestion_runs_country_started;
ALTER TABLE ingestion_runs DROP COLUMN IF EXISTS country_code;
