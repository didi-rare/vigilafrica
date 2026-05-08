# Milestone v0.3 — Localization Engine Tasks

- [x] Create `admin_boundaries` DB table and trigger function for `events` (`api/db/migrations/000003_create_admin_boundaries.up.sql`).
- [x] Implement Go CLI script `api/cmd/seed/main.go` to download and ingest Nigeria ADM1 boundaries.
- [x] Read and parse GeoJSON inside `cmd/seed`.
- [x] Update UPSERT mechanism or Trigger to tag new events with proper state constraints if present.
- [x] Build `api/internal/handlers/events.go` standard library handlers.
- [x] Add `GET /v1/events` routing support with parameters caching to queries.
- [x] Add `GET /v1/events/{id}` for single retrieval.
- [x] Add frontend typings (`web/src/api/events.ts`).
- [x] Set up frontend `TanStack Query` (`React Query`) provider.
- [x] Wire up dashboard to real API endpoints replacing mock data.
