# v0.5 — Operational Prototype Implementation

**Branch**: `feat/v0.5-operational-prototype`
**Spec**: `openspec/specs/vigilafrica/roadmap.md` § v0.5
**Change record**: `openspec/changes/feat-v0.5-implementation.md`

---

## Task List

- [x] Read and validate spec + existing codebase
- [x] Create `openspec/changes/feat-v0.5-implementation.md` (Sentinel gate)
- [x] Migration: `api/db/migrations/000004_create_ingestion_runs.up.sql`
- [x] Model: `api/internal/models/ingestion_run.go`
- [x] Database: add `CreateIngestionRun`, `CompleteIngestionRun`, `GetLastIngestionRun` to Repository
- [x] Ingestor: update `eonet.go` — slog structured logging + run record tracking
- [x] Ingestor: create `alerter.go` — Resend email failure alert + staleness watchdog
- [x] Ingestor: create `scheduler.go` — stdlib time.Ticker scheduler (F-012, no external dep)
- [x] Handlers: extend `health.go` — `last_ingestion` block + `degraded` status
- [x] Handlers: create `middleware.go` — rate limiting, response caching, CORS
- [x] Main: update `cmd/server/main.go` — wire scheduler + middleware + graceful shutdown
- [x] Seeds: create `api/db/seeds/sample_events_nigeria.sql`
- [x] Config: update `.env.example` with v0.5 env vars
- [x] Fix go.mod version to `go 1.26` (ADR-008)
- [x] Verify: `cd api && go build ./...` ✅ + `go vet ./...` ✅

---

## Acceptance Criteria

- [ ] Running ingestion twice yields identical event count (dedup)
- [ ] `GET /health` returns `last_ingestion` block with correct fields
- [ ] `GET /health` returns `status: "degraded"` when last run failed
- [ ] Scheduler starts automatically on server boot
- [ ] Scheduler stops cleanly on SIGTERM/SIGINT
- [ ] Resend failure does not crash the server
- [ ] All ingestion log output is valid JSON (slog)
- [ ] Rate limiter returns HTTP 429 after RPM threshold exceeded
- [ ] `GET /v1/events` served from cache on second request within TTL
