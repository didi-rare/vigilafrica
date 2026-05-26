---
id: chore-eonet-retry-backoff
status: proposed
branch: chore/eonet-retry-backoff
---

# Proposal: EONET Client Retry with Backoff (chore-eonet-retry-backoff)

## Why

[api/internal/ingestor/eonet.go:135](api/internal/ingestor/eonet.go#L135) configures `http.Client{Timeout: 30 * time.Second}` with **no retry**. The existing retry logic only handles HTTP 429 (rate-limit) responses with explicit `retry_after` hints — transient TCP stalls, 5xx responses, and network timeouts terminate the run immediately.

On 2026-05-11 EONET stalled upstream for one hourly tick (TCP connected, no response headers within 30s). Both Nigeria and Ghana ingestion runs failed back-to-back because the second run hit the same stall window. Every alert recipient received two failure emails for what was effectively one transient upstream blip that recovered ~1h later.

A short retry-with-backoff would absorb this class of incident silently.

## What Changes

1. Wrap the EONET HTTP call in a small retry loop:
   - 2 retries (3 attempts total)
   - 5s wait before first retry, 15s before second
   - Retry on: network errors, timeouts, HTTP 5xx
   - DO NOT retry on: HTTP 4xx (except 429, which keeps its existing dynamic backoff)
2. Keep the per-attempt 30s timeout
3. Log each retry attempt at WARN with the cause
4. Update [api/internal/ingestor/eonet_test.go](api/internal/ingestor/eonet_test.go) with cases for: success-on-retry, retry-exhausted, 4xx-no-retry
5. Update the ingestion alert recipient note: this class of blip will no longer page (good thing — the alert was noise)

## Out of Scope

- Circuit breaker / outage-mode behaviour (the staleness watchdog already handles sustained outages)
- Jitter (5s/15s fixed is fine for 2 retries)
- Retrying inside the alerter / database paths (different failure modes)
- Per-country retry budgets

## Origin

Captured in memory as `project_ingestion_alerting_backlog.md` after the 2026-05-11 staging incident.
