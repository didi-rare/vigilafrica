# Milestone v0.3 — Localization Engine: Implementation Plan

The objective of v0.3 is to enrich raw coordinate data (`POINT`) from NASA EONET with human-readable Nigerian state labels, build out the full backend REST API for querying those events, and integrate the API into our React frontend to replace the current mock data.

## User Review Required

> [!IMPORTANT]
> **Decisions Finalized**:
> 1. **Go CLI Seed Tool**: We will use a custom `cmd/seed` CLI tool to avoid bloating the git history or golang-migrate buffers.
> 2. **HDX Automation**: The `cmd/seed` tool will automatically download the NGA GeoJSON from the web. This design ensures that as we scale to the remaining 53 African countries, the ingestion is fully automated and repeatable without managing huge manual data folders.
> 3. **Frontend Data Fetching**: Based on the `frontend-developer` skill for optimal production-grade architecture, we will integrate **TanStack Query (React Query)** to handle data fetching. It provides built-in caching, optimistic updates, and robust loading/error states which far outpace a standard `useEffect`.

## Proposed Changes

---

### Backend Data Layer: PostGIS & Seeds

To support location tagging without massive query overhead during reads, we will implement the "Trigger" approach where location boundaries are calculated precisely at ingestion time.

#### [NEW] `api/cmd/seed/main.go`
- A dedicated Go script to automatically download the HDX Nigerian ADM1 boundaries, parse the `geojson`, and insert the state boundaries into a new `admin_boundaries` table (casting to `ST_MultiPolygon(geometry, 4326)`). This script will be architected to easily scale to other African countries later.

#### [NEW] `api/db/migrations/000003_create_admin_boundaries.up.sql`
- Create the `admin_boundaries` table.
- Create a PostgreSQL `FUNCTION` and `TRIGGER` named `trg_enrich_event_location`. 
- Every time a row in `events` undergoes `INSERT` or `UPDATE`, the trigger computes: `SELECT state_name FROM admin_boundaries WHERE ST_Intersects(NEW.geom, geom)` and saves it directly to `events.state_name`.

---

### Backend API: Go Endpoints (F-006, F-007)

We will use the standard Library `http.ServeMux` (Go 1.22+) to route traffic per the `api-contract.md`.

#### [NEW] `api/internal/handlers/events.go`
- Implements `GET /v1/events` supporting pagination `?limit=50&offset=0` and filters `?category=&state=&status=`.
- Implements `GET /v1/events/{id}` for single item retrieval.
- Responses will strictly adhere to the `EventSummary` and `EventDetail` schemas, handling `null` correctly.

#### [MODIFY] `api/internal/database/queries.go`
- Add methods to execute parameterized SQL filtering, e.g., `GetEvents(ctx, filters)` and `GetEventByID(ctx, id)`.

#### [MODIFY] `api/cmd/server/main.go`
- Mount the new endpoints:
  - `mux.Handle("GET /v1/events", eventHandler)`
  - `mux.Handle("GET /v1/events/{id}", eventHandler)`

---

### Frontend: React Integration (F-010, F-016)

#### [NEW] `web/src/api/events.ts`
- Implement a centralized API client hitting `VITE_API_BASE_URL/v1/events` with typings matching the `api-contract.md`.

#### [MODIFY] `web/src/App.tsx` (and related components)
- Strip out the static "Placeholder" demo data.
- Integrate **TanStack Query** (`@tanstack/react-query`) to consume the API. This provides a production-grade caching layer and seamless loading skeleton integration.
- Ensure the Category Filter (Flood vs Wildfire toggle) interacts with React Query to trigger background re-fetches automatically when the state changes.

## Open Questions

- None. Implementation is locked.

## Verification Plan

### Automated Tests
- Run backend unit tests (`go test ./...`) for the new API logic.
- Verify the DB trigger by attempting a raw insert manually and observing if `state_name` automatically populates.

### Manual Verification
- We will fully restart the docker-compose stack. 
- Run the API server, run the seed script, run the ingestor.
- Hit `curl http://localhost:8080/v1/events` and confirm that events strictly within Nigeria have a populated `state_name`, while events outside are `null`.
- Verify the UI visualizes real data directly from the DB.
