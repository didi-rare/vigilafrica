---
change_id: roadmap-v05-operational-prototype
status: spec
created_date: 2026-04-15
author: Claude Code
---

# Spec: v0.5 · Operational Prototype

## Objective

Make VigilAfrica run unattended. Implement scheduled EONET ingestion (F-012), idempotent
deduplication via source_id upsert (F-013), and all operational requirements defined in
`openspec/specs/vigilafrica/roadmap.md:159–178`.

---

## F-012: Scheduled Ingestion

### Package: `api/internal/scheduler/`

```
api/internal/scheduler/
  scheduler.go   — gocron setup, job registration, graceful shutdown
```

**Dependency:** `github.com/go-co-op/gocron/v2`

**Scheduler setup (in `api/cmd/server/main.go`):**

```go
s, _ := gocron.NewScheduler()
s.NewJob(
    gocron.DurationJob(cfg.IngestInterval),
    gocron.NewTask(ingestor.Run, ctx),
)
s.Start()
defer s.Shutdown()
```

**Environment variable:**

| Variable | Default | Description |
|---|---|---|
| `INGEST_INTERVAL_MIN` | `60` | Ingestion poll interval in minutes |

**Structured log output per run (zerolog JSON):**

```json
{"level":"info","event":"ingest_start","ts":"..."}
{"level":"info","event":"ingest_complete","fetched":12,"stored":3,"skipped":9,"duration_ms":430,"ts":"..."}
{"level":"error","event":"ingest_error","error":"connection refused","ts":"..."}
```

---

## F-013: Deduplication via Upsert

### Migration: `api/db/migrations/003_upsert_on_source_id.sql`

```sql
-- Add unique constraint if not already present
ALTER TABLE events ADD CONSTRAINT events_source_id_key UNIQUE (source_id);
```

### Query change (`api/internal/database/queries.go`):

Replace `INSERT INTO events` with:

```sql
INSERT INTO events (source_id, title, category, status, geometry, country_name, state_name, event_date, raw_payload)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (source_id) DO UPDATE SET
  status       = EXCLUDED.status,
  geometry     = EXCLUDED.geometry,
  state_name   = EXCLUDED.state_name,
  country_name = EXCLUDED.country_name,
  raw_payload  = EXCLUDED.raw_payload;
```

This ensures:
- New events are inserted
- Existing events (same `source_id`) are updated in place
- Running ingestion N times yields the same row count

---

## Operational Requirements

### Rate Limiting (`api/internal/middleware/ratelimit.go`)

Use `golang.org/x/time/rate` (stdlib-adjacent, no external dep):

```go
limiter := rate.NewLimiter(rate.Every(time.Minute/time.Duration(cfg.RateLimitRPM)), cfg.RateLimitRPM)
```

| Variable | Default | Description |
|---|---|---|
| `RATE_LIMIT_RPM` | `60` | Max API requests per minute per IP |

### Response Caching (`api/internal/middleware/cache.go`)

In-memory cache for `GET /v1/events` using a simple `sync.Map` with TTL:

| Variable | Default | Description |
|---|---|---|
| `CACHE_TTL_SECONDS` | `300` | Events list cache TTL (5 min default) |

### CORS (`api/cmd/server/main.go`)

| Variable | Required | Description |
|---|---|---|
| `CORS_ORIGIN` | Yes | Allowed origin (e.g. `https://vigilafrica.vercel.app`) |

### Seed Dataset

File: `api/db/seeds/sample_events_nigeria.sql`

- Minimum 5 synthetic events covering ≥ 3 Nigerian states
- Covers both Flood and Wildfire categories
- Safe to run idempotently (`INSERT ... ON CONFLICT DO NOTHING`)
- Enables full local development without EONET connectivity

### Documentation Files

| File | Contents |
|---|---|
| `CONTRIBUTING.md` | Prerequisites, local setup steps, how to run tests, how to add a country |
| `CODE_OF_CONDUCT.md` | Standard Contributor Covenant v2.1 |
| `docs/deployment/vps.md` | Caddy reverse-proxy config, Docker Compose production config, env var reference |

---

## .env.example additions

```
# Scheduler
INGEST_INTERVAL_MIN=60

# Rate limiting
RATE_LIMIT_RPM=60

# Response caching
CACHE_TTL_SECONDS=300

# CORS
CORS_ORIGIN=https://your-vercel-domain.vercel.app
```

---

## Acceptance Criteria

### F-012 — Scheduled Ingestion
- [ ] Server starts and automatically runs ingestion on first tick without manual trigger
- [ ] `INGEST_INTERVAL_MIN` controls the polling interval
- [ ] Every ingestion run emits structured JSON log lines: `ingest_start`, `ingest_complete` (with counts), `ingest_error` (on failure)
- [ ] Server does not crash if EONET is unreachable — error is logged, scheduler continues

### F-013 — Deduplication
- [ ] Running ingestion twice on the same EONET snapshot yields identical event counts (no duplicates)
- [ ] A previously-open event whose status changes to "closed" in EONET is updated in the DB, not duplicated
- [ ] Migration `003_upsert_on_source_id.sql` applies cleanly against a fresh database

### Rate Limiting
- [ ] `RATE_LIMIT_RPM=10` limits a client to 10 requests/minute; 11th request receives HTTP 429
- [ ] Rate limit is per IP, not global

### Response Caching
- [ ] `GET /v1/events` response is cached; two identical requests within the TTL window return same data without a DB query
- [ ] Cache respects `CACHE_TTL_SECONDS` env var

### CORS
- [ ] Only the domain in `CORS_ORIGIN` receives a permissive CORS response
- [ ] Requests from other origins receive a CORS error (not silently allowed)

### Operational Docs
- [ ] A new contributor can run the full stack locally by following `CONTRIBUTING.md` without external help
- [ ] Seed dataset loads with `psql < api/db/seeds/sample_events_nigeria.sql` and is idempotent
- [ ] `CODE_OF_CONDUCT.md` is present at repo root

---

## Out of Scope

- Alert subscriptions or email delivery (post-v1.0 per roadmap governance)
- Multi-country events
- Authentication or user accounts
- Frontend changes beyond banner/milestone text (handled in `feature-dynamic-milestones`)
