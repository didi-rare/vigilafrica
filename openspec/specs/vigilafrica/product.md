# VigilAfrica — Product Specification

**Version**: 1.0
**Status**: LOCKED — Approved 2026-04-12
**Maintained by**: @didi-rare

> **Governance rule**: Any change to this document requires explicit maintainer review and a corresponding update to `roadmap.md`. No feature may be implemented unless it has an F-ID in this registry. Any implementation diverging from acceptance criteria here is drift.

---

## 1. Problem Statement

Natural event data (floods, wildfires, storms) is globally available via sources like NASA EONET but is presented in satellite coordinates and global administrative units that African communities, responders, NGOs, and logistics operators cannot easily act on.

VigilAfrica translates raw geospatial event signals into familiar African administrative names — countries, states, and local government areas — answering the question: **"What is happening near me?"**

---

## 2. MVP Scope (Locked)

The MVP is intentionally narrow. It proves the core value proposition with the smallest credible implementation before any expansion.

| Dimension         | MVP Value                                                        |
|-------------------|------------------------------------------------------------------|
| **Countries**     | Nigeria only                                                     |
| **Event types**   | Floods and Wildfires only                                        |
| **Admin levels**  | Country (ADM0) + State (ADM1). LGA (ADM2) is post-MVP.          |
| **Data source**   | NASA EONET v3 API only                                           |
| **Primary users** | NGOs, local media, civic responders, logistics planners           |

### 2.1 Explicitly Out of Scope for MVP

The following are **not in scope** and must not be implemented until explicitly promoted by a new ADR in `decisions.md`:

- Historical timeline (events outside the EONET active window)
- Alert webhooks / third-party subscriptions
- SMS or push notifications
- Parametric insurance data exports
- Coverage beyond Nigeria (v0.6+ expansion model)
- Event types beyond Floods and Wildfires
- LGA (ADM2) enrichment
- User accounts or authentication
- GoFundMe or any fundraising UI (ADR-005)
- Multi-language / localisation support
- Mobile native apps

---

## 3. Target Personas

### P-01: NGO Field Coordinator
Needs to quickly understand which Nigerian states have active flood or wildfire events. Works with intermittent internet access. Prioritises clarity and speed over data depth. Does not understand coordinates.

### P-02: Local Journalist / Civic Reporter
Needs to verify event locations by familiar administrative name ("Benue State") rather than coordinates. Needs to share or embed event information via a link.

### P-03: Logistics Planner
Needs to assess route risk based on active events by state. Needs to filter by event type (floods blocking roads vs wildfires near depots).

### P-04: Open Source Contributor
Needs a clear local dev setup that works in under 30 minutes. Needs to understand the codebase without requiring geospatial expertise upfront.

---

## 4. Feature Registry

> **Rules**: Every implementation task must reference an F-ID. Any code not traceable to a feature here is out of scope. Acceptance criteria are the definition of done — all checkboxes must pass for a feature to be considered complete.

---

### F-001 — Health Endpoint
- **Milestone**: v0.1
- **Description**: The Go API exposes a stateless health check endpoint confirming the service is running. No database dependency.
- **Acceptance Criteria**:
  - [ ] `GET /health` returns HTTP 200
  - [ ] Response body is exactly `{"status": "ok", "version": "<semver>"}`
  - [ ] Endpoint responds in under 100ms
  - [ ] Endpoint has no database dependency — it must respond even if the database is down
- **Out of Scope**: Database health check (added in v0.2)
- **Implementation Note**: Stateless. Version injected at build time via `-ldflags "-X main.version=0.1.0"`.

---

### F-002 — NASA EONET Ingestion
- **Milestone**: v0.2
- **Description**: A Go service fetches natural event data from NASA EONET v3 API, filtered to Nigeria and the Floods + Wildfires categories.
- **Acceptance Criteria**:
  - [ ] Fetches from `https://eonet.gsfc.nasa.gov/api/v3/events`
  - [ ] Filters by category slugs: `floods` and `wildfires`
  - [ ] Applies Nigeria bounding box: Lat 4.0–14.0, Long 2.0–15.0
  - [ ] Raw EONET response is captured in `raw_payload` (JSONB) before normalization
  - [ ] Ingestion can be triggered manually via a CLI flag or HTTP endpoint in v0.2
  - [ ] Failed fetches are logged with error detail; the service does not panic or crash
  - [ ] Network timeout is enforced (configurable, default 30s)
- **Out of Scope**: Scheduled/automatic polling (F-012, v0.5)
- **Implementation Note**: Use Go's `net/http` standard library. No third-party HTTP client required for MVP.

---

### F-003 — Event Normalization
- **Milestone**: v0.2
- **Description**: Raw EONET event payloads are converted into VigilAfrica's internal `Event` model (defined in `data-model.md` §1).
- **Acceptance Criteria**:
  - [ ] Every ingested EONET event maps to the internal `Event` struct without data loss
  - [ ] `source_id` preserves the original EONET event ID exactly
  - [ ] `geometry_type` correctly identifies `Point` vs `Polygon` from EONET geometry
  - [ ] `status` is correctly mapped: `"open"` / `"closed"` from EONET `closed` field
  - [ ] Events with missing or null geometry are logged and skipped, not panicked on
  - [ ] Normalization is unit-tested with at least one fixture for each supported category
  - [ ] Normalization is idempotent — running on the same payload twice produces identical output
- **Out of Scope**: Admin boundary enrichment (F-005). `country_name` and `state_name` are null after normalization.
- **Implementation Note**: See `data-model.md` §1 for the canonical Go `Event` struct.

---

### F-004 — PostgreSQL Event Storage
- **Milestone**: v0.2
- **Description**: Normalized events are persisted to PostgreSQL with PostGIS geometry support.
- **Acceptance Criteria**:
  - [ ] Database schema matches `data-model.md` §2 exactly (events table)
  - [ ] `source_id` has a UNIQUE constraint — duplicate inserts are handled gracefully
  - [ ] Event geometry stored as PostGIS `geometry(Geometry, 4326)` (WGS84)
  - [ ] Database migrations are plain SQL files in `api/db/migrations/`, numbered sequentially
  - [ ] `docker-compose.yml` at repo root starts PostgreSQL 15 with PostGIS 3 extension enabled
  - [ ] Connection string is read exclusively from `DATABASE_URL` environment variable
  - [ ] `.env.example` documents `DATABASE_URL` with a localhost example value
- **Out of Scope**: Admin boundary tables (F-005), migration tooling beyond numbered SQL files
- **Implementation Note**: Use `pgx` as the PostgreSQL driver. No ORM.

---

### F-005 — PostGIS Geospatial Enrichment (Nigeria ADM0 + ADM1)
- **Milestone**: v0.3
- **Description**: Each stored event is spatially matched to Nigerian administrative boundaries (country + state) using PostGIS, replacing null `country_name` / `state_name` with real names.
- **Acceptance Criteria**:
  - [ ] `admin_boundaries` table schema matches `data-model.md` §3 exactly
  - [ ] Nigeria ADM0 (country) and ADM1 (states — 36 states + FCT) loaded from HDX source
  - [ ] Each event within Nigeria is tagged with `country_name = 'Nigeria'` and the correct `state_name`
  - [ ] Events outside Nigeria receive `country_name = null`, `state_name = null` — no crash
  - [ ] Enrichment query uses `ST_Intersects` per the canonical query in `data-model.md` §4
  - [ ] `enriched_at` timestamp is set on successful enrichment
  - [ ] Enrichment is re-runnable (idempotent) — only processes `enriched_at IS NULL` rows
  - [ ] At least 5 Nigerian events can be demonstrated as "Flood in [State Name]" after enrichment
- **Out of Scope**: LGA (ADM2) enrichment, countries other than Nigeria
- **Boundary Data Source**: [Humanitarian Data Exchange (HDX) — Nigeria COD-AB](https://data.humdata.org/dataset/cod-ab-nga)
- **Implementation Note**: See `data-model.md` §4 for the canonical enrichment SQL query.

---

### F-006 — API: GET /v1/events
- **Milestone**: v0.3
- **Description**: Paginated REST endpoint returning enriched events with filter support.
- **Acceptance Criteria**:
  - [ ] `GET /v1/events` returns HTTP 200 with JSON body matching `api-contract.md` §2
  - [ ] Supports `?category=floods` and `?category=wildfires` query params
  - [ ] Supports `?state=<state_name>` (case-insensitive match)
  - [ ] Supports `?status=open` (default) and `?status=closed`
  - [ ] Supports pagination via `?limit=<n>&offset=<n>` (default limit: 50, max: 200)
  - [ ] Returns `{"data": [], "meta": {...}}` — `data` is never `null`, only empty array
  - [ ] Invalid `category` value returns HTTP 400 with descriptive error message
  - [ ] Invalid `limit` (>200 or <1) returns HTTP 400
- **Out of Scope**: Authentication, rate limiting (v0.5), full-text search

---

### F-007 — API: GET /v1/events/:id
- **Milestone**: v0.3
- **Description**: Returns the full detail of a single event by internal UUID.
- **Acceptance Criteria**:
  - [ ] `GET /v1/events/:id` returns HTTP 200 with full event JSON matching `api-contract.md` §3
  - [ ] Non-existent UUID returns HTTP 404 with `{"error": "event not found"}`
  - [ ] Non-UUID path parameter returns HTTP 400 with `{"error": "invalid event id: must be a valid UUID"}`
- **Out of Scope**: Related events, event history

---

### F-008 — API: GET /v1/context
- **Milestone**: v0.4
- **Description**: Returns the caller's detected location (via MaxMind GeoLite2 local lookup) and a list of open events in their resolved country/state.
- **Acceptance Criteria**:
  - [ ] Reads caller IP from `X-Forwarded-For` header (Vercel proxy) with fallback to `RemoteAddr`
  - [ ] Resolves IP to country ISO code and state name using MaxMind GeoLite2 local `.mmdb` (no network call)
  - [ ] Returns matched open events for the resolved country/state
  - [ ] If IP resolution fails for any reason: `{"location": null, "events": []}` — HTTP 200, never an error
  - [ ] `.mmdb` file path configurable via `GEOIP_DB_PATH` environment variable
  - [ ] Endpoint responds in under 200ms (GeoIP is a local file lookup)
  - [ ] Response matches `api-contract.md` §4
- **Out of Scope**: Lat/Long radius search, manual location picker, city-level precision
- **Implementation Note**: Use `oschwald/geoip2-golang` library for `.mmdb` file reads.

---

### F-009 — Frontend: VigilAfrica Landing Page
- **Milestone**: v0.1
- **Description**: Replace the Vite starter template with a real VigilAfrica-branded page. No data dependency — static content only.
- **Acceptance Criteria**:
  - [ ] Page renders VigilAfrica project name and tagline ("What is happening near me?")
  - [ ] No Vite logo, React logo, counter button, or generic Vite template content remains
  - [ ] Page explains in plain language what the project does (2–3 sentences max)
  - [ ] Page links to the GitHub repository
  - [ ] Displays a clear "early prototype / work in progress" status notice
  - [ ] Responsive at 375px (mobile) and 1280px (desktop) widths
  - [ ] Page passes basic accessibility check (heading hierarchy, image alt text)
- **Out of Scope**: Real event data, map, any API calls
- **Implementation Note**: Pure static React component. No API calls, no router needed at this stage.

---

### F-010 — Frontend: Event List View
- **Milestone**: v0.3
- **Description**: A page displaying live enriched events fetched from `GET /v1/events`.
- **Acceptance Criteria**:
  - [ ] Fetches events from `GET /v1/events` on page load
  - [ ] Each event card shows: title, category, state name, country, status, event date
  - [ ] Shows a loading skeleton/spinner while fetching
  - [ ] Shows an empty state ("No events found") when the API returns an empty array
  - [ ] Shows an error state ("Unable to load events") if the API call fails, with a retry option
  - [ ] Category filter (F-016) is present and functional on this view
- **Out of Scope**: Map view (F-011), event detail page (F-015), state filter (F-017, v0.4)

---

### F-011 — Frontend: MapLibre GL JS Map
- **Milestone**: v0.4
- **Description**: Interactive map displaying enriched events as coloured markers using MapLibre GL JS.
- **Acceptance Criteria**:
  - [ ] Map renders centred on Nigeria by default (approx. center: Lat 9.0, Long 8.0, zoom 5)
  - [ ] Event markers displayed at correct coordinates
  - [ ] Marker colour indicates category: Floods = blue (`#3B82F6`), Wildfires = orange (`#F97316`)
  - [ ] Clicking a marker opens a popup showing: title, state name, category, status
  - [ ] Map uses a free tile source (MapLibre default or OpenStreetMap raster tiles)
  - [ ] Map is usable on mobile (375px) — no map controls obscuring content
  - [ ] Events without geometry (null coordinates) are excluded from the map gracefully
- **Map Library**: MapLibre GL JS v3+ (confirmed, ADR-001, non-negotiable)
- **Out of Scope**: Clustering, heatmaps, satellite tiles, custom African tile sets (post-MVP)

---

### F-012 — Scheduled Ingestion Job
- **Milestone**: v0.5
- **Description**: Automatic periodic polling of NASA EONET, replacing the manual trigger from F-002.
- **Acceptance Criteria**:
  - [ ] Polling interval configurable via `INGEST_INTERVAL_MINUTES` env var (default: 60)
  - [ ] Scheduler logs each run: start time, end time, events fetched, events stored
  - [ ] A failed poll run does not prevent the next run (errors are isolated per run)
  - [ ] Scheduler handles SIGTERM gracefully — completes the current run before shutting down
- **Implementation Note**: Use `go-co-op/gocron` v2 for the scheduler.

---

### F-013 — Event Deduplication
- **Milestone**: v0.5
- **Description**: Prevent duplicate events across multiple ingestion runs.
- **Acceptance Criteria**:
  - [ ] `source_id` UNIQUE constraint at the database level prevents hard duplicates (enforced at F-004)
  - [ ] Application uses `INSERT ... ON CONFLICT (source_id) DO UPDATE SET status = EXCLUDED.status, ...`
  - [ ] Re-ingesting a closed event correctly updates `status` from `open` to `closed`
  - [ ] Deduplication behaviour is covered by at least one unit test
- **Implementation Note**: See `data-model.md` §2 for the upsert strategy.

---

### F-014 — IP Geolocation (MaxMind GeoLite2)
- **Milestone**: v0.4
- **Description**: Local IP-to-country/state name resolution — no external API calls.
- **Acceptance Criteria**:
  - [ ] MaxMind GeoLite2-City `.mmdb` file is NOT committed to the repo (add pattern to `.gitignore`)
  - [ ] `.env.example` documents `GEOIP_DB_PATH` with instructions to download the file
  - [ ] Startup logs a warning (not an error) if `.mmdb` file is missing; service continues
  - [ ] Lookup resolves IP to country ISO code + subdivision (state) name
  - [ ] Lookup is entirely local — zero outbound network calls
  - [ ] Locale/subdivision lookup falls back to country-only if state is unavailable
- **Implementation Note**: Register at maxmind.com for a free GeoLite2 account to download the `.mmdb`. Use `oschwald/geoip2-golang`.

---

### F-015 — Frontend: Event Detail View
- **Milestone**: v0.4
- **Description**: Full event information page accessible via direct URL.
- **Acceptance Criteria**:
  - [ ] Accessible at route `/events/:id`
  - [ ] Fetches event from `GET /v1/events/:id`
  - [ ] Displays: title, category, status, country, state, coordinates (if available), event date, link to original EONET source
  - [ ] Shows 404 message if event ID is not found (API returns 404)
  - [ ] Back navigation returns to the event list
  - [ ] URL is shareable — loading the URL directly renders the same page

---

### F-016 — Frontend: Filter by Event Type
- **Milestone**: v0.3
- **Description**: UI control to narrow the event list by category.
- **Acceptance Criteria**:
  - [ ] Filter options: "All", "Floods", "Wildfires"
  - [ ] Selection updates the event list without full page reload
  - [ ] Active filter is visually distinct (highlighted/selected state)
  - [ ] Filter state is reflected in the URL query param (`?category=floods`) so links are shareable
  - [ ] "All" resets the filter and shows all categories

---

### F-017 — Frontend: Filter by State
- **Milestone**: v0.4
- **Description**: Dropdown to filter events by Nigerian state name.
- **Acceptance Criteria**:
  - [ ] Dropdown is populated from unique `state_name` values returned in the current event result
  - [ ] Selection updates the event list without page reload
  - [ ] "All States" option resets the state filter
  - [ ] Combined with F-016 category filter using AND logic (both filters active simultaneously)
  - [ ] Filter state reflected in URL (`?state=Benue`)

---

## 5. Design Constraints

| Constraint                    | Value                                           |
|-------------------------------|-------------------------------------------------|
| API response time (p95)       | < 300ms for `GET /v1/events`                   |
| Health endpoint               | < 100ms                                         |
| Context endpoint              | < 200ms (local GeoIP lookup only)              |
| Map library                   | MapLibre GL JS v3+ (ADR-001, non-negotiable)   |
| Backend language              | Go (non-negotiable)                             |
| Frontend framework            | React 19 + Vite + TypeScript (non-negotiable)  |
| Database                      | PostgreSQL 15+ with PostGIS 3+ (non-negotiable)|
| Minimum mobile viewport       | 375px                                           |
| Go driver (PostgreSQL)        | `pgx` — no ORM                                 |
| MaxMind DB                    | GeoLite2-City `.mmdb` — local file, not API   |

---

## 6. Open Source Compliance Checklist

The following files must exist before any public launch milestone:

- [x] `LICENSE` — Apache 2.0 ✅ Created 2026-04-12
- [ ] `README.md` — accurate, prototype-stage draft
- [ ] `CONTRIBUTING.md` — contributor setup and PR process
- [ ] `CODE_OF_CONDUCT.md` — community standards
- [ ] `.env.example` — all required environment variables documented
- [ ] `api/db/migrations/` — migration files numbered and present
- [ ] `api/db/seeds/` — sample Nigerian event data for local dev (no EONET connection needed)
