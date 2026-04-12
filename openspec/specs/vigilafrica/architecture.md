# VigilAfrica — Architecture Specification

**Version**: 1.0
**Status**: LOCKED — Approved 2026-04-12
**Maintained by**: @didi-rare

> **Governance rule**: Technology choices in this document are locked by ADRs in `decisions.md`. Any deviation from the confirmed stack requires a new ADR. No implementation may introduce technologies not listed here.

---

## 1. Architecture Pattern

VigilAfrica follows a **Poll → Enrich → Serve** architecture:

1. **Poll** — Go backend periodically fetches raw events from NASA EONET v3 (Floods + Wildfires, Nigeria bounding box)
2. **Enrich** — Events are normalized into the internal model and spatially matched to Nigerian administrative boundaries via PostGIS
3. **Serve** — Enriched events are served via a REST API consumed by the React frontend

---

## 2. Confirmed Technology Stack

| Layer                | Technology                          | ADR         | Status      |
|----------------------|-------------------------------------|-------------|-------------|
| Backend language     | Go 1.26                             | ADR-008     | Locked      |
| Frontend framework   | React 19 + Vite + TypeScript        | —           | Locked      |
| Database             | PostgreSQL 15 + PostGIS 3           | —           | Locked      |
| Map library          | **MapLibre GL JS v3+**              | ADR-001     | Locked      |
| IP geolocation       | MaxMind GeoLite2-City (local .mmdb) | —           | Locked      |
| Frontend hosting     | **Vercel**                          | ADR-002     | Locked      |
| Backend + DB hosting | **Single VPS (DigitalOcean/Hetzner)** | ADR-003   | Locked      |
| Container runtime    | Docker Compose                      | ADR-003     | Locked      |
| Reverse proxy        | Caddy                               | ADR-003     | Locked      |
| PostgreSQL driver    | `jackc/pgx` (no ORM)               | ADR-009     | Locked      |
| API protocol         | REST / JSON                         | —           | Locked      |
| Go scheduler         | `go-co-op/gocron` v2               | —           | Locked      |
| GeoIP Go library     | `oschwald/geoip2-golang`           | —           | Locked      |

---

## 3. System Architecture

```mermaid
graph TD
    subgraph External["External Data Sources"]
        EONET["NASA EONET v3 API<br/>eonet.gsfc.nasa.gov"]
        MMDB["MaxMind GeoLite2-City<br/>.mmdb — local file on VPS"]
    end

    subgraph VPS["VPS — DigitalOcean / Hetzner"]
        Caddy["Caddy<br/>SSL termination + reverse proxy<br/>:443 → :8080"]
        subgraph Docker["Docker Compose"]
            GoAPI["Go Backend :8080<br/>─────────────────<br/>• Poll Worker (gocron)<br/>• Normalizer<br/>• Enricher<br/>• REST API (chi router)"]
            PG[("PostgreSQL 15 + PostGIS 3<br/>:5432<br/>─────────────────<br/>• events table<br/>• admin_boundaries table")]
        end
    end

    subgraph Vercel["Vercel CDN"]
        React["React + Vite<br/>Static Build<br/>─────────────────<br/>• Landing page<br/>• Event list + filters<br/>• MapLibre GL JS map<br/>• Event detail view"]
    end

    subgraph Seed["One-Time Seed Data"]
        HDX["HDX Nigeria<br/>ADM0 + ADM1 GeoJSON"]
    end

    EONET -->|"Periodic poll (HTTPS/JSON)"| GoAPI
    HDX -->|"SQL seed migration"| PG
    GoAPI -->|"INSERT events"| PG
    PG -->|"ST_Intersects enrichment"| GoAPI
    MMDB -->|"Local file read (no network)"| GoAPI
    React -->|"GET /v1/* (HTTPS)"| Caddy
    Caddy -->|"Proxy to :8080"| GoAPI
    GoAPI -->|"SELECT events"| PG
```

---

## 4. Deployment Topology

```
[User Browser]
      │
      ▼
[Vercel CDN Edge]
  React static build (HTML/JS/CSS)
  VITE_API_BASE_URL = https://api.vigilafrica.io
      │
      │ API calls: GET /health, GET /v1/events, GET /v1/context
      ▼
[Caddy :443 on VPS]
  - Auto SSL (Let's Encrypt)
  - HTTP → HTTPS redirect
  - /v1/* and /health → proxy :8080
      │
      ▼
[Go binary :8080 — Docker container]
  - Chi HTTP router
  - Poll worker (gocron, every 60 min)
  - MaxMind .mmdb local file
      │
      ▼
[PostgreSQL 15 + PostGIS 3 :5432 — Docker container]
  - Named Docker volume (persistent)
  - events table
  - admin_boundaries table (Nigeria ADM0 + ADM1)
```

### Environment Separation

| Git Branch    | Environment | Target                    |
|---------------|-------------|---------------------------|
| `development` | Local dev   | Docker Compose (localhost) |
| `main`        | Staging     | Staging VPS instance       |
| `releases`    | Production  | Production VPS instance    |

---

## 5. Data Flow: Ingestion

```mermaid
sequenceDiagram
    participant Scheduler as gocron Scheduler
    participant EONET as NASA EONET v3
    participant Normalizer as Go Normalizer
    participant DB as PostgreSQL + PostGIS

    Scheduler->>EONET: GET /api/v3/events?category=floods,wildfires&bbox=2,4,15,14
    EONET-->>Scheduler: JSON events array
    Scheduler->>Normalizer: Raw EONET payloads
    Normalizer->>DB: INSERT INTO events ON CONFLICT (source_id) DO UPDATE SET status = EXCLUDED.status
    DB-->>Normalizer: Upserted event IDs
    Normalizer->>DB: UPDATE events SET country_name, state_name, enriched_at WHERE ST_Intersects(geom, boundary) AND enriched_at IS NULL
    DB-->>Normalizer: Enriched row count
```

---

## 6. Data Flow: Context (Near You)

```mermaid
sequenceDiagram
    participant Browser as User Browser
    participant Vercel as Vercel Edge
    participant API as Go REST API
    participant GeoIP as MaxMind GeoLite2 (local file)
    participant DB as PostgreSQL + PostGIS

    Browser->>Vercel: Load dashboard
    Vercel-->>Browser: React static bundle
    Browser->>API: GET /v1/context (X-Forwarded-For: <user IP>)
    API->>GeoIP: Lookup IP → country + state (local .mmdb read)
    GeoIP-->>API: {country: "Nigeria", state: "Benue"}
    API->>DB: SELECT * FROM events WHERE state_name = 'Benue' AND status = 'open'
    DB-->>API: Matched events (enriched)
    API-->>Browser: {location: {country, state}, events: [...]}
```

If IP resolution fails at any step, the API returns `{"location": null, "events": []}` with HTTP 200. It never returns an error for location failure.

---

## 7. Repository Structure

```
vigilafrica/
├── api/                             # Go backend
│   ├── cmd/
│   │   └── server/
│   │       └── main.go              # Entry point — wires all internal packages
│   ├── internal/
│   │   ├── ingestor/                # EONET fetch + poll worker
│   │   ├── normalizer/              # Raw payload → internal Event model
│   │   ├── enricher/                # PostGIS spatial enrichment
│   │   ├── geoip/                   # MaxMind GeoLite2 wrapper
│   │   └── api/                     # HTTP handlers, router (chi), middleware
│   ├── db/
│   │   ├── migrations/              # 001_create_events.sql, 002_create_admin_boundaries.sql ...
│   │   └── seeds/                   # sample_events_nigeria.sql (local dev only)
│   └── go.mod
│
├── web/                             # React + Vite + TypeScript frontend
│   ├── src/
│   │   ├── components/              # Reusable UI components
│   │   ├── pages/                   # Route-level pages (Home, Events, EventDetail)
│   │   └── main.tsx                 # Vite entry point
│   └── package.json
│
├── openspec/                        # Governance specification layer
│   └── specs/vigilafrica/
│       ├── product.md               # Feature registry + acceptance criteria (LOCKED)
│       ├── roadmap.md               # Milestone plan v0.1–v1.0 (LOCKED)
│       ├── architecture.md          # This file (LOCKED)
│       ├── api-contract.md          # API endpoint contracts (LOCKED)
│       ├── data-model.md            # Database schema + Go structs (LOCKED)
│       └── decisions.md             # Architecture Decision Records (LOCKED)
│
├── .github/
│   └── workflows/
│       ├── ci-cd.yml                # Build, test, deploy
│       ├── openspec-verify.yml      # Spec drift detection
│       └── community.yml            # First-interaction welcome
│
├── docker-compose.yml               # Local dev: PostgreSQL + PostGIS
├── openspec.yaml                    # OpenSpec project configuration
├── package.json                     # Root scripts (api:dev, web:dev, spec:validate)
├── .env.example                     # All required environment variables
├── LICENSE                          # Apache 2.0
└── README.md                        # Project-stage README (prototype)
```

> **Note**: There is no `/infra` directory. Deployment configuration is `docker-compose.yml` (root) and VPS-level Caddy/Docker configs managed outside the repository. See ADR-003.

---

## 8. API Design Principles

- All API routes prefixed `/v1/`
- JSON-only responses: `Content-Type: application/json`
- All errors: `{"error": "<human-readable message>"}` — never expose stack traces
- Pagination via `?limit=<n>&offset=<n>` query params (default 50, max 200)
- CORS configured for the Vercel production domain via `CORS_ORIGIN` environment variable
- No authentication in MVP

See `api-contract.md` for full endpoint definitions.

---

## 9. Security Baseline (MVP)

| Area           | Approach                                                                |
|----------------|-------------------------------------------------------------------------|
| TLS            | Caddy auto-provisioned Let's Encrypt certificate                        |
| CORS           | Allowlist set to Vercel domain only (`CORS_ORIGIN` env var)            |
| IP trust       | Trust `X-Forwarded-For` from Vercel only (not arbitrary proxies)       |
| Secrets        | All secrets via environment variables — never committed to the repo     |
| MaxMind .mmdb  | Not committed to repo (`.gitignore` pattern: `*.mmdb`)                 |
| SQL injection  | `pgx` parameterised queries (`$1`, `$2`) — no string interpolation     |
| Auth           | None in MVP — public read-only API                                      |
