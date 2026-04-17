---
change_id: feat-v0.5-implementation
status: complete
created_date: 2026-04-16
author: Claude Code
spec_ref: openspec/specs/vigilafrica/roadmap.md#v05--operational-prototype
adr_refs: ADR-011
---

# Change: v0.5 Operational Prototype Implementation

## What This Change Implements

Full implementation of the v0.5 milestone as specified in the locked roadmap.
All changes are confined to `api/internal/*`, `api/cmd/*`, `api/db/*`, and
root config files. No changes to `web/src/*`.

## Files Created or Modified

### New files
- `api/db/migrations/000004_create_ingestion_runs.up.sql`
- `api/internal/models/ingestion_run.go`
- `api/internal/ingestor/alerter.go`
- `api/internal/ingestor/scheduler.go`
- `api/internal/handlers/middleware.go`
- `api/db/seeds/sample_events_nigeria.sql`

### Modified files
- `api/internal/database/db.go` — extend Repository interface + pgRepo
- `api/internal/database/queries.go` — ingestion run queries
- `api/internal/ingestor/eonet.go` — slog + run record tracking
- `api/internal/handlers/health.go` — last_ingestion block
- `api/cmd/server/main.go` — scheduler wiring + graceful shutdown
- `api/go.mod` — fix go version to 1.26; stdlib `time.Ticker` used for fixed-interval scheduling (no external scheduler dependency)
- `.env.example` — new v0.5 env vars

## Traceability

| Feature / Requirement | Spec ref | ADR ref |
|---|---|---|
| F-012 Scheduled ingestion | roadmap.md §v0.5 | — |
| F-013 Deduplication (upsert) | roadmap.md §v0.5 | ADR-009 |
| ingestion_runs table | roadmap.md §v0.5, ADR-011 | ADR-011 |
| Extended /health endpoint | roadmap.md §v0.5, ADR-011 | ADR-011 |
| Resend email alerting | roadmap.md §v0.5 | ADR-011 |
| Staleness watchdog | roadmap.md §v0.5 | ADR-011 |
| Rate limiting | roadmap.md §v0.5 | — |
| Response caching | roadmap.md §v0.5 | — |
| CORS middleware | roadmap.md §v0.5 | ADR-002 |
| Structured JSON logging (slog) | roadmap.md §v0.5 | ADR-007 |
| Seed dataset | roadmap.md §v0.5 | ADR-004 |
