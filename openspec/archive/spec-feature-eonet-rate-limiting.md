# Spec: Graceful EONET Rate Limiting (feature-eonet-rate-limiting)

## Context
EONET sporadically responds with a rate-limit/high-demand JSON payload containing a `retry_after` parameter. The ingestor currently fails immediately. We need to introduce a resilient retry mechanism directly into `runIngest` to pause and re-attempt transparently. Concurrently, the frontend dashboard requires logic to present these errors gracefully rather than a generic "data stale" message.

## Components to touch
- **`api/internal/ingestor/eonet.go`**:
  - Update `runIngest` function. Wrap the HTTP request in a limited retry loop.
  - Parse the EONET backoff JSON payload `{"retry_after": int}`.
- **`web/src/components/StatusBanner.tsx`** (or equivalent):
  - Read `data.last_ingestion.error` from the `/health` endpoint and safely display it to users.

## Implementation plan
1. Introduce a constant `maxEONETRetries = 3`.
2. Wrap `http.NewRequestWithContext` and `client.Do` inside a 0-to-3 retry loop.
3. On HTTP `429` or `503`:
   - Try decoding the JSON into a struct with a `retry_after` integer.
   - If decoded successfully: `time.Sleep(time.Duration(retry_after + 5) * time.Second)`.
   - If not decoded: apply exponential backoff `time.Sleep(time.Duration(5 * (2^attempt)) * time.Second)`.
4. If loop breaks without success (over 3 retries), exit naturally and return the error.
5. In the React frontend, modify the health check polling logic to display `error` directly within the staleness banner if the health status is `degraded` and the string is present.

## Acceptance criteria
- When encountering a `429/503`, the ingestor pauses for `retry_after + 5` seconds and retries.
- Retries are properly restricted to a maximum of 3 attempts.
- An accurate error string indicating the EONET outage is prominently shown on the frontend.

## Verification plan
- Provide a Go unit test featuring a mocked HTTP server that simulates rate limiting and returns a JSON payload with `retry_after`.
- Perform a manual UI test locally by fabricating an error in the `/health` response and verifying the text rendering.
