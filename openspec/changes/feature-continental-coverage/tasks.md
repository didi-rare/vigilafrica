# feature-continental-coverage

Each implementation PR ticks its tasks here with evidence. Ticking this file in a PR's own diff is also what satisfies the sentinel for that PR (`api/cmd/sentinel`, `isGovernanceRecord`).

## 1. Decisions (maintainer)

- [x] 1.1 P1: where the loader runs. Decided 2026-11-25: API startup, background, advisory lock
- [x] 1.2 P2: when `/ready` answers 503. Decided 2026-11-25: only when every country's last run failed, or the query failed
- [x] 1.3 P3: South Africa in the first release. Decided 2026-11-25: only if its staging load check passes
- [x] 1.4 P4: delete the legacy `api/cmd/seed`. Decided 2026-11-25: yes

## 2. Manifest (PR: boundaries manifest and loader)

- [ ] 2.1 Survey: for each GeoJSON country, confirm HDX publishes an ADM1-level GeoJSON resource and record its URL and vintage
- [ ] 2.2 Committed script under `scripts/boundaries/` that builds manifest entries (download, `sha256`, `bbox`, `name_field`)
- [ ] 2.3 `api/internal/boundaries/manifest.json` with NG and GH only, embedded with `go:embed`
- [ ] 2.4 Manifest validation test (unique alpha-2 codes, checksums present, boxes inside Africa)

## 3. Loader (same PR)

- [ ] 3.1 Migration `000017_boundary_sources` (up and down)
- [ ] 3.2 Loader per spec §2: skip unchanged, checksum and size checks, one transaction per country, ADM0 as ADM1 union, provenance row
- [ ] 3.3 Re-enrich the country's stored events after its load
- [ ] 3.4 Run it per decision P1
- [ ] 3.5 `/health` `boundaries` object
- [ ] 3.6 Tests: wrong checksum rejected; zero-feature file rejected; second run skips; one country's failure does not stop the others
- [ ] 3.7 NG/GH equivalence: committed before/after script; unit counts 37/16 and stored-event names unchanged on staging
- [ ] 3.8 P4: remove `api/cmd/seed` and its mentions in `docs/standards/developers-go.md`

## 4. Countries (PR: enable the first release)

- [ ] 4.1 Add the remaining GeoJSON countries to the manifest, enabled
- [ ] 4.2 Load on staging; `/health` `boundaries` shows zero failures; peak memory and time per country recorded
- [ ] 4.3 P3: South Africa staging check recorded; included or moved to the shapefile tranche
- [ ] 4.4 Spot-check fixture: one known coordinate per country resolves to the expected ADM1 name

## 5. Ingestion (PR: continental ingestion)

- [ ] 5.1 Country list from enabled manifest entries
- [ ] 5.2 Pause between countries (`INGEST_COUNTRY_PAUSE_MS`)
- [ ] 5.3 Scheduler lock renewal during the run
- [ ] 5.4 Rotate the starting country each tick
- [ ] 5.5 One alert per run listing affected countries
- [ ] 5.6 P2 readiness rule, with tests
- [ ] 5.7 24-hour staging measurement: run time, EONET and GDACS requests per run; GDACS budget set from it
- [ ] 5.8 Density scripts read their boxes from the manifest

## 6. API (PR: continental API)

- [ ] 6.1 Contract first: `openapi.yaml`, `npm run sync:openapi`, `api-contract.md`
- [ ] 6.2 `GET /v1/countries`
- [ ] 6.3 Country validation from the manifest; new `errUnknownCountry` message
- [ ] 6.4 State qualification by country
- [ ] 6.5 `/v1/states` requires `country`
- [ ] 6.6 Remove or confine the Nigeria literals in `handlers/context.go`
- [ ] 6.7 MaxMind-to-manifest country-name test table

## 7. Web (PR: continental interface)

- [ ] 7.1 Countries from `/v1/countries`; remove `SUPPORTED_COUNTRIES` and `COUNTRY_CENTERS`
- [ ] 7.2 Searchable, keyboard-accessible country picker
- [ ] 7.3 Continental default map view; selected country framed by its `bbox`
- [ ] 7.4 Boundary source and vintage shown for the selected country
- [ ] 7.5 Hazard-neutral copy: `index.html`, `App.tsx`, partner page lead (digest still flood-only), `llms.txt`, `SITEMAP_ROUTES`
- [ ] 7.6 Tests; pixel evidence for CSS changes

## 8. Release

- [ ] 8.1 Phase 2 early stop check recorded on the release PR
- [ ] 8.2 Independent review recorded on each PR above
- [ ] 8.3 Promote to staging; verify `/health`, `/ready`, `boundaries`, per-country ingestion, map and console
- [ ] 8.4 Record how many stale-open wildfires appear on the first page of the default list
- [ ] 8.5 Promote to production; same checks
- [ ] 8.6 Archive this change (`/openspec-archive`)
