# Proposal: Graceful EONET Rate Limiting (feature-eonet-rate-limiting)

## Why
NASA's EONET API sporadically returns HTTP 429 / 503 errors during high demand, typically with a JSON body specifying a `retry_after` period. Currently, VigilAfrica's ingestion immediately fails and fires administrative alerts. This causes false-positive operational noise, skips ingest cycles unnecessarily, and presents a non-specific "Data Stale" error on the dashboard.

## What changes
1. **Ingestor Retry (Go)**: Update `api/internal/ingestor/eonet.go` to intercept HTTP 429 and 503 responses.
2. **Backoff Logic**: Parse the JSON for `retry_after`. Pause ingestion for `retry_after + 5` seconds. If `retry_after` is absent, utilize an exponential backoff. Retries are strictly capped at a maximum of 3 attempts.
3. **Frontend Presentation (React)**: Update the frontend data dashboard's banner to actively present the error details from `/health` so admins and users accurately understand upstream EONET failures.

## Out of scope
- Distributed locking for rate limit tracking across multiple instances (VigilAfrica assumes a single singleton scheduler).
- Changing the primary polling interval (`INGEST_INTERVAL_MIN`).
- Persistent caching of EONET API request parameters.

## User impact
End users will see transparent error messages via the dashboard if EONET goes down. System Administrators will receive far fewer false-positive email alerts for transient NASA API rate-limits.
