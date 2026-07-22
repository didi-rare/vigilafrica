---
id: fix-api-observability-backstop
status: proposed
branch: fix/api-observability-backstop
---

# Proposal: Log the 500s and Catch the Panics (fix-api-observability-backstop)

## Why

Two gaps found by the 2026-07-22 standards review (shipped as #167/#168/#169). Both are cases where a production failure produces **no evidence** — the worst class of defect for a service with one maintainer and no on-call rotation.

**1. `EventHandler` has no logger, so its 500s are silent.**

[docs/standards/developers-go.md §4.5](../../docs/standards/developers-go.md) requires:

> Handlers must not leak internal error text to clients. **Log the real error**; return a sanitised message.

`GET /v1/events` and `GET /v1/events/{id}` did the second half and skipped the first. `EventHandler` held only `repo` — no `*slog.Logger` — so a repository failure returned `{"error":"internal server error"}` to the client and wrote **nothing** anywhere. The two most-trafficked endpoints in the API could fail continuously and the only signal would be user complaints. `DigestHandler` already had the right shape (`NewDigestHandler(repo, logger)`), so this is an inconsistency, not an unsolved problem.

**2. There is no recovery middleware, so §4.4 has no backstop.**

§4.4 says handlers must never panic, and §6.7 previously claimed a recovery middleware sat outermost in the chain. It never existed — the chain is security headers → CORS → global rate limit. #167 corrected the doc to say so; this closes it for real.

Relying on `net/http`'s built-in per-connection recovery is not sufficient: it **closes the connection without writing a response** and prints to the server's error log rather than our slog handler. The client sees a dropped request rather than a 500, and the panic never reaches structured logs. A nil-map write or an out-of-range index in a parser would be invisible in exactly the same way as (1).

## What Changes

1. `NewEventHandler(repo, logger)` — add a `*slog.Logger` field, nil falling back to `slog.Default()`, mirroring `NewDigestHandler` exactly.
2. Log at both 500 sites in `events.go` with the correlation keys already available (`category`/`country`/`state` for the list path, `event_id` for the detail path). The `pgx.ErrNoRows` → 404 branch stays unlogged — a 404 is a correct answer, not a fault.
3. `handlers.RecoveryMiddleware` in `middleware.go`, wired **outermost** in `cmd/server/main.go`:
   - recovers, logs at Error with method, path and `debug.Stack()`, returns the standard sanitised 500 envelope;
   - re-panics `http.ErrAbortHandler`, which is the documented way for a handler to abort a connection deliberately;
   - tracks whether the response was already framed via a small `recoveryWriter`, and does not attempt a 500 over a partially-written body.
4. Update §6.7 (real chain, recovery now present) and §8.6 (`EventHandler` now injects) in `developers-go.md`.

**Ordering note:** recovery goes outermost so it also covers panics raised inside the other middleware. Security headers still apply to a recovered 500 — `SecurityHeadersMiddleware` populates the header map before calling `next`, and the recovery path writes the status after that, so the headers go out with it.

## Out of Scope

- **The remaining `slog.Default()` fallbacks.** `enrichment_stats.go`, `health.go`, `alert/resend.go`, `alert/watchdog.go` and `digest/scheduler.go` all still reach for the package logger instead of an injected one. They do at least log, so they are a consistency problem rather than a blindness problem. Worth a follow-up chore.
- **An access-log middleware.** §8.7 says "middleware handles the access log" and none exists. That is a separate decision with real volume/cost implications, not a bug fix.
- **Persisting ingest counters** (`EventsSkippedBBox` / `EventsUnverifiedGeom` are log-only) — related observability gap, different subsystem, needs a migration.

## Verification

- [ ] `go build ./...` and `go vet ./...` clean
- [ ] New test: a repository failure on `GET /v1/events` returns 500, the body does **not** contain the underlying error, and the log **does**
- [ ] New test: a panicking handler yields a logged 500 with a stack, and the panic value never reaches the response body
- [ ] New test: a panic after the response is framed leaves the original status intact and still logs
- [ ] New test: `http.ErrAbortHandler` propagates rather than being swallowed
- [ ] Full suite green, including under `-race` in CI (gate added in #168)

## Origin

Residual findings 1 and 2 from the Fable-5 review of `docs/standards/developers-{go,react}.md`, 2026-07-22. Findings 3 (`@types/react-router-dom` misplaced) and 4 (test files type-checked by nothing) are tracked separately.
