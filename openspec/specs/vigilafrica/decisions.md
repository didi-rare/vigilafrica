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
| ADR-010 | Automated Governance: The Sentinel        | ACCEPTED | 2026-04-14 |
| ADR-011 | Ingestion Observability: Resend Alerting  | ACCEPTED | 2026-04-16 |
| ADR-012 | Frontend Server State: TanStack Query     | ACCEPTED | 2026-04-18 |
| ADR-013 | Frontend Styling: Plain CSS over CSS-in-JS | ACCEPTED | 2026-04-18 |
| ADR-014 | Single-VPS Two-Stack Deployment Model     | ACCEPTED | 2026-04-24 |

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
---

## ADR-010 — Automated Governance: The Sentinel

**Date**: 2026-04-14
**Status**: ACCEPTED

### Decision

Implement an automated governance gate ("The Sentinel") that prevents code changes to critical packages from being merged into `development` or `main` without a corresponding OpenSpec change record.

### Rationale

- **Ghost Implementations**: As the project grows, AI agents and human contributors might add features without formal design review (ADRs/Specs).
- **Traceability**: Every architectural decision and feature must be traceable to a specific proposal in `openspec/changes/`.
- **System Intelligence**: The repository must be "intelligent" enough to enforce its own governance rules without manual overhead.

### Enforcement Rules

1. **Critical Packages**: Any change to `api/internal/*`, `api/cmd/*`, or `web/src/*` triggers an audit.
2. **Governance Link**: The audit passes IF at least one file is added or modified in `openspec/changes/`.
3. **Exemptions**: 
   - **Trivial Fixes**: Commits containing `[trivial]` in the message skip the audit (for typos, linting, etc.).
   - **Maintenance**: Changes to `api/db/migrations/`, `docs/`, or root configuration files are exempt.

### Consequences

- **CI Failure**: Pull Requests that violate these rules will fail the `openspec-verify` workflow.
- **Workflow Dependency**: Developers must run `/opsx-propose` before starting implementation on a new feature.
- **Improved Scannability**: The `openspec/changes/archive` becomes a reliable history of *why* every part of the codebase exists.

---

## ADR-011 — Ingestion Observability: Resend Alerting

**Date**: 2026-04-16
**Status**: ACCEPTED

### Decision

Implement ingestion run tracking and email alerting via **Resend** as part of the v0.5 operational prototype. The system must be capable of detecting and reporting both immediate ingestion failures and prolonged ingestion staleness without manual log inspection.

### Context

v0.5 introduces scheduled ingestion via gocron. A scheduled job that fails silently — EONET unreachable, PostGIS enrichment broken, goroutine hung — presents stale data as current, which is worse than no data for target users (NGOs, journalists, civic responders). A solo-maintained VPS requires automated alerting to be operationally viable at zero cost.

### Decision Components

**1. Ingestion run tracking**
- An `ingestion_runs` table records every run: `started_at`, `completed_at`, `status` (success/failure), `events_fetched`, `events_stored`, `error` message
- Written by the ingestor at the start and end of each cycle

**2. Extended `/health` endpoint**
- Response includes a `last_ingestion` block with the most recent run record
- Top-level `status` field returns `"degraded"` if the last run status is `"failure"`, `"ok"` otherwise
- Frontend reads `last_ingestion.completed_at` to display a "last updated X min ago" freshness indicator — warns if > 2 hours stale

**3. Failure alert**
- On every failed ingestion run, an email is sent immediately via Resend
- Subject: `[VigilAfrica] Ingestion failed at {time}`
- Body includes: error message, events fetched, events stored, run duration

**4. Staleness watchdog**
- A separate goroutine runs on a configurable interval (default: every 30 minutes, offset from the ingestion schedule)
- Queries `ingestion_runs` for the most recent successful run
- If `last_success_at < now - ALERT_STALENESS_THRESHOLD_HOURS`, sends a staleness email
- Catches failures the immediate alert misses: scheduler process died, gocron stopped, goroutine hung silently

### Email Provider: Resend

Four options evaluated: SMTP (self-configured), SendGrid, Mailgun, Resend.

- **Resend chosen**: single `RESEND_API_KEY` env var, no SMTP config, 3,000 emails/month free tier — sufficient for hourly ingestion monitoring indefinitely
- **SMTP rejected**: requires per-provider configuration (Gmail app password, port, TLS settings) — unnecessary complexity for a zero-cost operational goal
- **SendGrid / Mailgun rejected**: more complex onboarding for equivalent free tier functionality

### Environment Variables

| Variable | Default | Purpose |
|---|---|---|
| `RESEND_API_KEY` | — | Required. Resend API key for email delivery |
| `ALERT_EMAIL_TO` | — | Required. Recipient address for all alerts |
| `ALERT_STALENESS_THRESHOLD_HOURS` | `2` | Hours without successful ingestion before staleness alert fires |

### Consequences

- `RESEND_API_KEY` and `ALERT_EMAIL_TO` are required env vars from v0.5 onward — documented in `.env.example`
- The `ingestion_runs` table requires a new migration: `api/db/migrations/` (numbered after existing migrations)
- Resend email delivery and staleness watchdog logic live in `api/internal/alert/`; scheduled ingestion remains in `api/internal/ingestor/`
- The `/health` handler in `api/internal/handlers/` queries `ingestion_runs` for the last run record
- If Resend is unreachable when sending an alert, the failure is logged but does not crash the ingestor or scheduler

---

## ADR-012 — Frontend Server State: TanStack Query

**Date**: 2026-04-18
**Status**: ACCEPTED

### Decision

Use **TanStack Query v5** (`@tanstack/react-query`) as the sole server-state management layer for the React frontend.

### Context

The dashboard fetches event lists, applies filters, and needs background refetch to stay current with ingestion runs. Options evaluated: `useEffect` + `useState`, SWR, TanStack Query, Zustand with async actions.

### Rationale

- **Eliminates `useEffect` data fetching** — the most common React anti-pattern; TanStack Query replaces the `useEffect` + `useState` + error handling triple with a single hook
- **Built-in caching and deduplication** — multiple components requesting the same query share one network call; cache is invalidated explicitly after mutations
- **Background refetch** — events dashboard stays live without manual polling logic
- **Suspense integration** — `useSuspenseQuery` works natively with React 19 Suspense boundaries
- **SWR rejected** — smaller API surface but less control over cache invalidation and key conventions; TanStack Query's key factory pattern is more explicit
- **Zustand rejected** — appropriate for complex client state machines, not server state; would require manual loading/error/cache management

### Consequences

- All server data fetching must use TanStack Query hooks — `useQuery`, `useSuspenseQuery`, `useMutation`
- Query keys must follow the hierarchical key factory pattern (§5.2 of `developers-react.md`)
- No `useEffect` + `setState` for fetching data
- `QueryClientProvider` wraps the app root with a single shared `QueryClient`
- Cache invalidation is explicit via `queryClient.invalidateQueries`

---

## ADR-013 — Frontend Styling: Plain CSS over CSS-in-JS

**Date**: 2026-04-18
**Status**: ACCEPTED

### Decision

Use **plain CSS files co-located with components** as the sole styling mechanism. No CSS-in-JS, no CSS Modules, no Tailwind.

### Context

Styling approach was evaluated at project start. Options considered: Tailwind CSS, CSS Modules, styled-components / Emotion (CSS-in-JS), plain CSS.

### Rationale

- **Zero runtime cost** — CSS-in-JS (styled-components, Emotion) injects styles at runtime, adding JS bundle size and runtime overhead; plain CSS has none
- **No build transformation** — CSS Modules require a PostCSS pipeline; Tailwind requires a purge step; plain CSS works natively in Vite with no config
- **Browser DevTools native** — plain CSS class names are readable in DevTools without source maps; CSS-in-JS generates hashed names that obscure origin
- **Contributor accessibility** — plain CSS is the baseline skill; CSS-in-JS APIs are framework-specific knowledge
- **Tailwind rejected** — utility-class proliferation makes diffs noisy and component JSX harder to read; also requires the Tailwind PostCSS plugin
- **CSS Modules rejected** — adds a build-time step and a module import pattern for marginal benefit over class prefixing convention
- **Scoping achieved via convention** — BEM-style component-prefix class names (`.events-dashboard__filter`) prevent global collisions without a build tool

### Consequences

- Each component has one co-located CSS file, imported at the top of the `.tsx`
- Class names are `kebab-case` prefixed with the component name (§7.3 of `developers-react.md`)
- CSS custom properties in `index.css` are the design token layer
- No `styled-components`, `@emotion/react`, `@emotion/styled`, or Tailwind in `package.json`
- Adding any CSS-in-JS library or Tailwind requires superseding this ADR

---

## ADR-014 — Single-VPS Two-Stack Deployment Model

**Date**: 2026-04-24
**Status**: ACCEPTED

### Context

VigilAfrica needs staging and production environments before v1.0 can be tagged. The project is still solo-maintained and cost-sensitive, but production changes need a real promotion path, versioned deploys, and a rollback mechanism.

### Decision

Run staging and production on one VPS as two isolated Docker Compose stacks behind one host-level Caddy instance:

- Staging API: `api.staging.vigilafrica.org` -> `127.0.0.1:8081`, deployed from `main`
- Production API: `api.vigilafrica.org` -> `127.0.0.1:8080`, deployed from SemVer tags on `release`
- Frontend: two Vercel projects, `staging.vigilafrica.org` from `main` and `vigilafrica.org` from `release`
- Production deploys require GitHub Environment approval

### Options Considered

- **Railway**: rejected because managed Postgres does not provide the same PostGIS story, and custom containers erase much of the managed-platform benefit.
- **Supabase database + VPS API**: rejected for v1.0 because it splits the operational surface across vendors and adds latency for ingestion upserts.
- **Fly.io**: rejected because it does not materially simplify the PostGIS + low-cost deployment story for this stage.
- **Branch-push production deploys**: rejected because tags provide a clearer release artifact and rollback target.

### Consequences

- One VPS remains a single point of failure, accepted for v1.0 scale.
- Staging and production must use separate `.env` files, Docker networks, and volumes.
- `/health.version` is stamped at build time with a commit SHA for staging and a SemVer tag for production.
- Rollback is performed by redeploying a prior tag through the production workflow.
