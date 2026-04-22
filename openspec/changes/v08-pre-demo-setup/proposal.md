## Why

v0.7 closed the second-country (Ghana) implementation — the prototype now works end-to-end for two African countries. Before v1.0 is attempted, the roadmap requires a stable, curated demo environment that any NGO contact, journalist, or potential contributor can access from a single URL without setup or relying on live EONET availability.

## What Changes

- **New Demo Docker Compose** — `docker-compose.demo.yml` wiring a separate Postgres container, pre-seeded with curated Nigeria + Ghana event data; live ingestion disabled
- **Extended seed dataset** — `api/db/seeds/sample_events_nigeria.sql` extended with Ghana events (covering ≥ 3 Ghanaian regions, ≥ 2 event types); safe to run idempotently
- **`DEMO.md`** — step-by-step instructions for standing up the demo environment locally; linked from `CONTRIBUTING.md`
- **Screenshot** — at least one representative screenshot of the running app committed to `docs/screenshots/`
- **30-second demo GIF** — animated walkthrough committed to `docs/screenshots/` and embedded in `README.md`

## Capabilities

### New Capabilities

- `demo-environment`: Isolated, seeded demo deployment separate from production — own Docker Compose, own database, own frontend preview URL

### Modified Capabilities

- `seed-data`: Existing Nigeria-only seed is extended to cover Ghana events (spec-level change: seed must cover all supported countries, not just Nigeria)

## Impact

- **Files added**: `docker-compose.demo.yml`, `DEMO.md`, `docs/screenshots/demo.png`, `docs/screenshots/demo.gif`
- **Files modified**: `api/db/seeds/sample_events_nigeria.sql` (extended with Ghana data), `README.md` (GIF embed + demo URL link), `CONTRIBUTING.md` (link to DEMO.md)
- **No API or schema changes** — demo uses existing endpoints with seeded data
- **No frontend code changes** — demo points existing frontend build at the demo API base URL via `VITE_API_BASE_URL`
- **No new dependencies**
