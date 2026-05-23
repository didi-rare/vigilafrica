---
id: chore-eonet-retry-backoff
status: proposed
branch: chore/eonet-retry-backoff
---

# Spec: EONET Client Retry with Backoff (chore-eonet-retry-backoff)

## Context

[api/internal/ingestor/eonet.go](api/internal/ingestor/eonet.go) currently retries only `429 Too Many Requests` and `503 Service Unavailable` responses (with retry_after-aware backoff). It returns immediately on:

- Network errors at [eonet.go:159-161](api/internal/ingestor/eonet.go#L159-L161) (`reqErr != nil` from `client.Do(req)`)
- 5xx responses other than 503 at [eonet.go:207-208](api/internal/ingestor/eonet.go#L207-L208) — catch-all "unexpected EONET status"

On 2026-05-11 EONET stalled upstream for one hourly tick. TCP connected but no response headers within 30s; the request timed out as a network error. Both Nigeria and Ghana ingestion runs failed back-to-back, generating two alert emails for what was effectively one transient blip that recovered ~1h later. This is exactly the noise the watchdog/alert design was meant to eliminate.

A small retry-with-backoff for transient conditions absorbs this class of incident silently.

Companion: [openspec/archive/proposal-chore-eonet-retry-backoff.md](proposal-chore-eonet-retry-backoff.md).

## Decision Log

| # | Decision | Alternatives | Why |
|---|---|---|---|
| D1 | Two retry sub-budgets in the same loop: 429 keeps its existing 3 retries with retry_after backoff; transient (network errors + 5xx non-503) gets a separate 2-retry budget with fixed 5s/15s waits | One unified budget covering both classes | 429 has explicit server guidance via `retry_after` — more retries are useful because the server is telling us when it'll be ready. Transient errors have no such signal, so a tighter budget is more conservative |
| D2 | 503 stays in the existing 429 path (retry_after-aware) | Move 503 to the new transient path | 503 commonly carries `retry_after` (it's the Service Unavailable code). Keeping it in the 429 branch preserves the retry_after-honouring behaviour. Other 5xx (500, 502, 504) don't have that convention |
| D3 | Network errors (`reqErr != nil` from `client.Do`) are treated as transient | Distinguish timeout vs. DNS vs. connection-reset for different policies | The 2026-05-11 incident showed Go presents all of these as the same `err != nil` shape from `client.Do`. Different policies per error type adds complexity without observed value |
| D4 | Per-attempt request timeout stays 30s | Lower to 10s for faster failover | The 30s budget is already short for a public-API JSON fetch over the open internet. Lowering it would convert "slow but successful" into "spurious retry" |
| D5 | Add `eonetHTTPClient` as a package-level var (mirrors existing `eonetURL` / `eonetSleepFn` pattern) | Refactor into an `Ingestor` struct first | The `Ingestor` struct refactor is captured as B6 in `chore-post-v11-quality-sweep`. Adding one more package global is consistent with the current code shape; refactor lands together with B6 |
| D6 | Fixed 5s/15s delays for transient retries | Exponential 5/10/20s OR jitter | Matches the proposal stub. Two retries is a small enough budget that exponential vs. linear barely differs in total wall-clock; jitter is unnecessary at this scale (one client, not a fleet) — the proposal explicitly puts jitter out of scope |
| D7 | New tests share existing `installInstantSleep` + `installTestServer` helpers in `eonet_test.go` | Add a separate test file | Helpers are already there and idiomatic for the package; splitting files would fragment the retry-related tests |

## Components to Touch

### Modified files

| File | Change |
|---|---|
| [api/internal/ingestor/eonet.go](api/internal/ingestor/eonet.go) | Extract `client` to package-level `eonetHTTPClient` var (D5). Replace the `if reqErr != nil { return ... }` early-return at line 159 with a transient-retry branch. Add a new branch after the 429/503 check that retries on 5xx (non-503) using the same transient budget. Add WARN logs for each retry with the cause |
| [api/internal/ingestor/eonet_test.go](api/internal/ingestor/eonet_test.go) | Remove `internal server error` (500) row from `TestRunIngest_NonRetryableStatus` (4xx-only now: 401, 404). Add `TestRunIngest_5xx_ThenSuccess` (500 → 200). Add `TestRunIngest_TransientExhausted` (always 500, asserts attempts = 3). Add `TestRunIngest_NetworkError_ThenSuccess` (first request closes the connection abruptly → retry succeeds) |

### Untouched

- The existing 429/503 retry path — unchanged
- Per-attempt 30s timeout — unchanged
- `maxRetries` constant (3 for 429/503) — unchanged
- Maximum response body size, raw event payload cap, retry_after upper bound — all unchanged
- Scheduler, watchdog, alert client — out of scope
- The `Ingestor` struct refactor — deferred to `chore-post-v11-quality-sweep` B6

## Behaviour Contract

- **B1** — A network error from `client.Do(req)` (TCP stall, connection reset, DNS failure, request timeout) MUST cause a retry, up to 2 retries, with 5s before the first retry and 15s before the second
- **B2** — An HTTP 5xx response **other than 503** (500, 502, 504, etc.) MUST cause a retry, sharing the same 2-retry / 5s,15s budget as network errors
- **B3** — A 429 response MUST continue to use the existing retry_after-aware loop with 3 retries — unchanged from current behaviour
- **B4** — A 503 response MUST continue to use the same path as 429 — unchanged
- **B5** — A 4xx response **other than 429** MUST NOT cause a retry. The function returns an error immediately
- **B6** — Each retry MUST emit a `slog.Warn` log entry tagged with `country`, `attempt`, `cause` (`network-error` / `server-error-5xx` / etc.), and `sleep_sec`
- **B7** — Context cancellation during a retry sleep MUST be honoured immediately (existing `eonetSleepFn` already does this; new path uses the same helper)
- **B8** — Maximum total request count for a single `runIngest` call: 4 attempts on the 429/503 path (3 retries) OR 3 attempts on the transient path (2 retries). These are independent budgets — if a run hits a mix (e.g. 429 → 500), each class's counter advances independently
- **B9** — No retry on `2xx` responses other than 200 — the existing code treats anything outside `[200]` as failure, behaviour unchanged

## Phase 1 — Implementation

- [ ] Promote `client` to package-level `eonetHTTPClient` var
- [ ] Add constants `maxTransientRetries = 2` and `transientRetryDelays = [5s, 15s]`
- [ ] Refactor the retry loop:
  - [ ] On `reqErr != nil`: classify as network-error transient, retry up to `maxTransientRetries`
  - [ ] After the 429/503 branch, add 5xx-non-503 branch that uses the transient retry budget
  - [ ] 4xx (non-429): immediate error return, no retry
- [ ] WARN-log each retry with country / attempt / cause / sleep_sec

## Phase 2 — Tests

- [ ] Update `TestRunIngest_NonRetryableStatus`: drop 500 from cases (now retryable), keep 401 and 404
- [ ] Add `TestRunIngest_5xx_ThenSuccess`: first request 500, second 200, asserts request count and success
- [ ] Add `TestRunIngest_TransientExhausted`: always 500, asserts `requestCount == maxTransientRetries+1` and a non-nil error
- [ ] Add `TestRunIngest_NetworkError_ThenSuccess`: install a `eonetHTTPClient` override (or use `srv.CloseClientConnections()` on the first request) so the first attempt errors at the transport layer; assert recovery on the retry

## Phase 3 — Verification

- [ ] `go test ./internal/ingestor/...` (from `api/`) — all existing tests pass plus the 3 new ones
- [ ] `go vet ./...` clean
- [ ] `go build ./...` clean
- [ ] Manual reasoning check: a transient 500 → 200 cycle now silently absorbs instead of paging. The 2026-05-11 incident (network timeout → 1h blip) would no longer generate two alert emails

## Acceptance Criteria

- [ ] B1-B9 of the behaviour contract verified via unit tests
- [ ] `go test ./...` from `api/` is green
- [ ] No regression in the existing 429/503 retry tests
- [ ] The package-level `eonetHTTPClient` is documented with the same TODO comment style used for `eonetURL` and `eonetSleepFn` so it's covered when the B6 struct refactor lands
- [ ] PR description includes a reasoning trace explaining how the 2026-05-11 incident would have been absorbed by the new logic

## Out of Scope (reaffirmed)

- Circuit breaker / outage-mode behaviour — the staleness watchdog handles sustained outages
- Jitter — 2 retries at fixed delays is sufficient
- Per-country retry budgets — single shared budget per run
- Retrying inside the alerter / database paths — different failure modes
- Refactoring `eonet.go` package globals into a struct — captured as B6 in `chore-post-v11-quality-sweep`
- Adjusting `maxRetries` (429/503 path) — unchanged at 3

## Risks

- **R1 — Hidden upstream regressions**: silent retry on 5xx might mask a real upstream change (e.g. EONET API breaking). Mitigation: B6 mandates WARN-level logs for every retry — operators can grep production logs for retry patterns
- **R2 — Test for network errors is awkward**: simulating a network error in Go's httptest is non-trivial. Mitigation: use `httptest.Server.CloseClientConnections()` after sending the first request OR replace `eonetHTTPClient` with a custom round-tripper that returns an error on the first call. Both are idiomatic
- **R3 — Adding a third package global slightly worsens the §9.5 violation** that B6 wants to fix. Mitigation: D5 explicitly notes this and links to B6 so the refactor lands as one cohesive change
- **R4 — Increased ingestion run duration on transient errors**: worst case +20s (5s + 15s) per failed transient run. Mitigation: per-run timeout is bounded by `maxTransientRetries × max(transient delay) + per-attempt timeouts` ≈ 110s ceiling; well under the 60min ingestion interval

## Verification Plan

1. Implement Phase 1 + Phase 2 on this branch
2. `go test ./internal/ingestor/... -v -count=1` from `api/` — all green
3. `go vet ./...` and `go build ./...` clean
4. Open PR to `development`; reviewer focuses on the behaviour contract clauses
5. Post-merge → main → staging: observe production logs for `retry` patterns over the next ingestion cycles to confirm the retry behaviour fires when expected (no synthetic test on staging needed — the existing alert flow continues to be the regression guard)

No new automated CI changes required.
