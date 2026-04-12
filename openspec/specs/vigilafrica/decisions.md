# VigilAfrica — Architecture Decision Records (ADRs)

**Version**: 1.0
**Status**: LOCKED — Approved 2026-04-12
**Maintained by**: @didi-rare

> **Governance rule**: ADRs capture confirmed, locked decisions. Reopening a decision requires a new ADR with status `SUPERSEDES ADR-XXX` and explicit maintainer sign-off. No implementation may contradict an ACCEPTED ADR.

---

## ADR Index

| ID      | Decision                                  | Status   | Date       |
|---------|-------------------------------------------|----------|------------|
| ADR-001 | Map Library: MapLibre GL JS               | ACCEPTED | 2026-04-12 |
| ADR-002 | Frontend Deployment: Vercel               | ACCEPTED | 2026-04-12 |
| ADR-003 | Backend Deployment: Single VPS            | ACCEPTED | 2026-04-12 |
| ADR-004 | Initial Country Scope: Nigeria First      | ACCEPTED | 2026-04-12 |
| ADR-005 | GoFundMe / Fundraising: Deferred          | ACCEPTED | 2026-04-12 |
| ADR-006 | Contact / Community: GitHub Issues Only   | ACCEPTED | 2026-04-12 |
| ADR-007 | Go Backend Package Structure              | ACCEPTED | 2026-04-12 |
| ADR-008 | Go Version: 1.26                          | ACCEPTED | 2026-04-12 |
| ADR-009 | Database Driver: pgx (No ORM)             | ACCEPTED | 2026-04-12 |

---

## ADR-001 — Map Library: MapLibre GL JS

**Date**: 2026-04-12
**Status**: ACCEPTED

### Decision

Use **MapLibre GL JS v3+** as the sole map rendering library for the VigilAfrica frontend.

### Context

Four options were evaluated: Leaflet.js, MapLibre GL JS, Google Maps JavaScript API, and Mapbox GL JS.

### Rationale

- **Free and open-source** — no API key cost at any traffic level
- **WebGL / GPU-accelerated** — performant on lower-end Android devices common in target markets
- **Custom tile sources** — can switch from OpenStreetMap to custom African tile sets (e.g., OpenAfrica) without library change
- **Active fork** — MapLibre is the community-maintained fork of Mapbox GL JS, with a strong release cadence
- **Mapbox disqualified** — Mapbox GL JS v2+ requires a paid API key; unacceptable for an open-source public-good project
- **Leaflet disqualified** — canvas-based rendering is less performant for large event datasets; no WebGL support

### Consequences

- All map components must use MapLibre GL JS v3+ APIs exclusively
- No Leaflet, Google Maps, or Mapbox GL JS dependencies permitted in `web/package.json`
- Custom markers, popups, and controls must use MapLibre's native API
- Tile source must default to a free provider (MapLibre's built-in style or OpenStreetMap raster tiles)

---

## ADR-002 — Frontend Deployment: Vercel

**Date**: 2026-04-12
**Status**: ACCEPTED

### Decision

Deploy the React + Vite frontend as a static build to **Vercel**.

### Rationale

- Zero-config deployment for Vite static builds — no build server required
- Global CDN edge network — critical for African users where round-trip latency to European/US servers is high
- Free tier is sufficient for MVP traffic volumes
- Automatic preview deployments per pull request — useful for contributor review
- `X-Forwarded-For` header from Vercel is reliable for IP geolocation (F-008)

### Consequences

- The frontend is a pure static SPA — no SSR, no Vercel Edge Functions in MVP
- All API calls from the frontend use `fetch()` to the VPS backend API over HTTPS
- CORS on the Go backend must be configured to allow the Vercel deployment domain
- Vercel secrets (`VERCEL_TOKEN`, `VERCEL_ORG_ID`, `VERCEL_PROJECT_ID`) must be configured in GitHub Actions before CI deployment steps run
- Frontend environment variable for API base URL: `VITE_API_BASE_URL`

---

## ADR-003 — Backend Deployment: Single VPS

**Date**: 2026-04-12
**Status**: ACCEPTED

### Decision

Deploy the Go backend and PostgreSQL/PostGIS database on a **single small VPS** (DigitalOcean or Hetzner), co-located and orchestrated with Docker Compose. Caddy handles SSL termination and reverse proxy.

### Rationale

- **Lowest cost** — a $6–12/month Droplet or Hetzner CX22 covers Go + PostgreSQL for MVP traffic
- **Co-location is critical** — PostGIS spatial queries are latency-sensitive; keeping the API and database on the same machine eliminates network round-trips between them
- **Simple ops** — Docker Compose is appropriate for a single-maintainer project at prototype scale
- **No managed DB costs** — avoids $15–50/month managed PostgreSQL cost before the project has validated demand

### Consequences

- No horizontal scaling in MVP — single point of failure is acceptable for a prototype stage project
- Database backups must be manually configured (cron + `pg_dump` to object storage)
- Caddy handles: SSL certificate (Let's Encrypt auto-renew), HTTP→HTTPS redirect, `/v1/*` reverse proxy to Go
- Docker Compose manages: `go-api` container, `postgres` container (with PostGIS), named volume for data persistence
- The `/infra` directory referenced in early documentation does not exist and is not planned — deployment config lives in `docker-compose.yml` at the repo root

### Migration Path

When scale requires it: move PostgreSQL to a managed provider (Supabase, Neon, or DigitalOcean Managed Databases) by changing only `DATABASE_URL`. No application code changes needed.

---

## ADR-004 — Initial Country Scope: Nigeria First

**Date**: 2026-04-12
**Status**: ACCEPTED

### Decision

The MVP delivers value **with Nigeria only**. No other African country is added until Nigeria is working end-to-end.

### Rationale

- Nigeria is the largest African economy by GDP and has significant, well-documented flood and wildfire risk affecting large populations
- HDX boundary data for Nigeria (ADM0 + ADM1) is high-quality and freely available
- "Prove value deep in one country" is more compelling than "claim coverage of 54 countries with nothing behind it"
- Nigeria's large NGO and civic-tech community provides a natural early user base

### Consequences

- EONET ingestion bounding box locked to Nigeria (Lat 4.0–14.0, Long 2.0–15.0) in MVP
- Only Nigeria ADM0 and ADM1 boundary GeoJSON is loaded into `admin_boundaries` table for MVP
- Country expansion follows the Country Onboarding Template process defined at v0.6
- Any agent adding non-Nigeria boundary data before v0.6 is out of scope

---

## ADR-005 — GoFundMe / Fundraising: Deferred

**Date**: 2026-04-12
**Status**: ACCEPTED

### Decision

No fundraising links, donation prompts, or GoFundMe references appear in the repository, README, or application until the project has a working, publicly demonstrated product.

### Rationale

- A fundraising link on a scaffold repository with no working features damages credibility with potential contributors, NGO partners, and funders
- Engineering focus should deliver v0.1 before any sustainability conversation
- The fundraising conversation is more credible after a working demo exists

### Consequences

- Remove GoFundMe placeholder from `README.md`
- Remove GoFundMe mention from all `openspec/specs/` documents ← superseded by this document
- "Fundraising / sustainability model" is added to the post-v1.0 backlog
- Revisit at v0.6–v1.0 when the project has demonstrable value

---

## ADR-006 — Contact / Community: GitHub Issues Only

**Date**: 2026-04-12
**Status**: ACCEPTED

### Decision

No personal email address is published in the project repository or its documentation. Community contact is via **GitHub Issues** only at the current stage.

### Rationale

- Publishing a personal email on a public repository invites spam before any community exists
- GitHub Issues is searchable, public, and creates a durable discussion record
- Appropriate for the current scale (no users yet)

### Consequences

- Remove personal email from `README.md`
- Contact section in README reads: "For collaboration or project discussions, open a GitHub Issue"

### Migration Path

- Enable **GitHub Discussions** when issue volume warrants structured community conversation
- Create a project-specific email address (e.g., `vigilafrica@proton.me`) when the project has real users, maintainers, or partnership enquiries

---

## ADR-007 — Go Backend Package Structure

**Date**: 2026-04-12
**Status**: ACCEPTED

### Decision

The Go backend follows the standard Go project layout with a `cmd/` entry point and `internal/` packages.

### Structure

```
api/
├── cmd/
│   └── server/
│       └── main.go          # Entry point: wires all internal packages
├── internal/
│   ├── ingestor/            # NASA EONET fetch logic + poll worker
│   ├── normalizer/          # Raw EONET payload → internal Event model
│   ├── enricher/            # PostGIS spatial enrichment queries
│   ├── geoip/               # MaxMind GeoLite2 .mmdb wrapper
│   └── api/                 # HTTP handlers, router, middleware
├── db/
│   ├── migrations/          # Numbered SQL files: 001_*.sql, 002_*.sql ...
│   └── seeds/               # Sample data for local development
└── go.mod
```

### Consequences

- `api/main.go` at the root of the `api/` directory does NOT exist and must not be created
- Root `package.json` script `api:dev` must reference: `go run ./api/cmd/server/`
- Root `package.json` script `build` must reference: `go build -o vigilafrica ./api/cmd/server/`
- CI build step references: `cd api && go build -o ../vigilafrica ./cmd/server/`
- All `internal/` packages are unexported — they are not importable by external consumers

---

## ADR-008 — Go Version: 1.26

**Date**: 2026-04-12
**Status**: ACCEPTED

### Decision

Target **Go 1.26** as the minimum Go version. The `go` directive in `api/go.mod` is set to `go 1.26`.

### Context

The original `api/go.mod` contained `go 1.26.2`. This is invalid syntax — the `go` directive in `go.mod` accepts only `MAJOR.MINOR` format (e.g., `go 1.26`). The `.2` patch suffix was a typo and has been corrected.

The CI workflow `ci-cd.yml` previously specified `go-version: '1.21'`, which was 5 minor versions behind `go.mod`. This has been aligned.

### Consequences

- `api/go.mod` directive: `go 1.26`
- CI `ci-cd.yml` `go-version`: `'1.26'`
- All Go toolchain features up to 1.26 are available for use

---

## ADR-009 — Database Driver: pgx (No ORM)

**Date**: 2026-04-12
**Status**: ACCEPTED

### Decision

Use **`jackc/pgx`** as the PostgreSQL driver. No ORM (no GORM, no sqlc, no ent).

### Rationale

- `pgx` has native support for PostGIS geometry types and JSONB — both critical for VigilAfrica
- ORMs add a layer of abstraction that complicates PostGIS spatial queries (`ST_Intersects`, `ST_Within`)
- Plain SQL is more readable for spatial queries and easier for contributors to understand without ORM knowledge
- `pgx` is the de-facto standard Go PostgreSQL driver with full `pgtype` support

### Consequences

- All database queries are written as raw SQL strings
- Query parameters use `pgx` named or positional parameters (`$1`, `$2`, ...)
- No auto-migrations — all schema changes via numbered SQL files in `api/db/migrations/`
- `pgxpool` used for connection pooling (not `database/sql` interface)
