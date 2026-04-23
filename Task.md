# v0.8 Pre-Demo Setup

**Branch**: `feat/v0.8-pre-demo-setup`
**Spec**: `openspec/changes/v08-pre-demo-setup/specs/`
**Change record**: `openspec/changes/v08-pre-demo-setup/`

## 1. Seed Data — Ghana Extension

- [x] 1.1 Create `api/db/seeds/sample_events_ghana.sql` with ≥ 3 Ghana events covering ≥ 3 regions, categories Floods + Wildfires, idempotent (`ON CONFLICT DO NOTHING`), relative event dates (`NOW() - INTERVAL '...'`)
- [x] 1.2 Verify Ghana seed events enrich correctly — `state_name` populates for each event after applying seed to a local DB with migration 000005 applied
- [x] 1.3 Add a `docs/seeds/` note in `CONTRIBUTING.md` (or inline comment in seed file) explaining that dates are relative so data stays fresh

## 2. Demo Docker Compose

- [x] 2.1 Create `docker-compose.demo.yml` — services: `demo-db` (postgres:15-postgis/3, named volume `vigil-demo-data`, port 5433) and `demo-api` (same image as prod, `INGEST_INTERVAL_MIN=0` or no ingest service, `DATABASE_URL` pointing to `demo-db`)
- [x] 2.2 Add an init script or compose `healthcheck` + `depends_on` that runs all migrations + both seed files on first boot
- [x] 2.3 Verify: `docker compose -f docker-compose.demo.yml up -d` → `curl localhost:8080/v1/events` returns Nigeria + Ghana events
- [x] 2.4 Verify idempotency: stop + `docker compose -f docker-compose.demo.yml up -d` again → same event count, no duplication errors
- [x] 2.5 Add `vigil-demo-data` volume name to `.gitignore` documentation comment (volume itself is not tracked, just note it exists)

## 3. DEMO.md Documentation

- [x] 3.1 Create `DEMO.md` at repo root with sections: Prerequisites, Start the demo, Access the frontend, Stop the demo, Reset demo data
- [x] 3.2 `DEMO.md` Prerequisites section lists: Docker + Docker Compose, Node.js (for frontend), `git clone` instructions
- [x] 3.3 `DEMO.md` includes a placeholder for the hosted demo URL (`TBD — see project README once deployed`)
- [x] 3.4 Add a `## Demo Environment` section to `CONTRIBUTING.md` with a one-liner and link to `DEMO.md`

## 4. Screenshot

- [x] 4.1 Run demo compose locally, open frontend, ensure Nigeria event markers are visible on the map
- [x] 4.2 Take a screenshot at 1280×800 showing the map view with at least 2 event markers and the filter controls visible
- [x] 4.3 Save as `docs/screenshots/demo.png` (PNG, ≤ 1 MB)
- [x] 4.4 Commit `docs/screenshots/demo.png`

## 5. Demo GIF

- [x] 5.1 Record 25–30 second screen capture: (1) page loads with Nigeria events, (2) click a marker to see popup, (3) switch country filter to Ghana, (4) Ghana events appear with Ghanaian region names
- [x] 5.2 Optimize GIF: target 10 fps, ≤ 3 MB (`gifsicle -O3 --lossy=80` or equivalent)
- [x] 5.3 Save as `docs/screenshots/demo.gif`
- [x] 5.4 Commit `docs/screenshots/demo.gif`

## 6. README Update

- [x] 6.1 Add `## Demo` section to `README.md` (after the "What is VigilAfrica" section) with: embedded `demo.gif`, one-line description, placeholder hosted demo URL
- [x] 6.2 Add `docs/screenshots/` to the `.gitignore` exclusion allowlist if needed (ensure PNGs and GIFs are not gitignored)
- [x] 6.3 Verify README renders correctly on GitHub (image embeds, no broken links)

## 7. Final Verification

- [x] 7.1 Run `docker compose -f docker-compose.demo.yml up -d` from a clean clone — confirm the full flow works without prior setup
- [x] 7.2 Confirm all v0.8 roadmap acceptance criteria are met (checklist in `roadmap.md §v0.8`)
- [x] 7.3 Commit all changes under conventional commit format: `feat(demo): add v0.8 pre-demo setup`

## 8. Graceful EONET Rate Limiting (feature-eonet-rate-limiting)

- [x] 8.1 Implement retry loop in `runIngest` (max 3 retries)
- [x] 8.2 Parse `retry_after` JSON payload for 429/503 status codes and calculate backoff
- [x] 8.3 Implement exponential backoff fallback for missing/invalid `retry_after`
- [x] 8.4 Update React dashboard to present ingestion errors natively from `/health`
- [x] 8.5 Add unit tests for rate-limiting simulation in Go
- [x] 8.6 Commit changes: `feat(ingestor): graceful EONET rate limit retries`

## 9. Backend DB Test Review Remediation (chore-backend-db-tests)

- [x] 9.1 Add tagged database integration test execution to CI
- [x] 9.2 Update OpenSpec verification instructions to include `-tags=integration`
- [x] 9.3 Run `go mod tidy` so `api/go.mod` and `api/go.sum` are stable
- [x] 9.4 Re-run database integration tests and confirm Docker cleanup
