# Spec: Continental Coverage, First Release (feature-continental-coverage)

**Companion:** [`proposal.md`](proposal.md). Scoping evidence: [`openspec/proposals/feature-continental-coverage.md`](../../proposals/feature-continental-coverage.md). Line references are to `origin/development` at `79288c1` (2026-11-25); re-check before editing.

## Facts this spec is built on

- **Boundaries.** `admin_boundaries` (`000002`) holds `country_code`, `country_name`, `adm_level` (0–2), `adm_name`, `geom MultiPolygon 4326`, plus a generated `area_m2` (`000013`). Nigeria (37) and Ghana (16) ADM1 rows and their ADM0 unions come from migration `000010` (HDX COD-AB), keyed by **alpha-2** codes (`NG`, `GH`). Migration `000012` adds simplified geoBoundaries ADM0 outlines for seven neighbours (BF, BJ, CI, CM, NE, TD, TG) as a country-name fallback.
- **Enrichment.** The trigger fires `BEFORE INSERT OR UPDATE OF geom ON events` (`000012`): ADM1 match first (smallest area wins), ADM0 fallback second. Re-enrichment is `UPDATE events SET geom = geom WHERE …`, the method `000012` already used.
- **Country lists, three copies:** `DefaultCountries` (`api/internal/ingestor/eonet.go:84`), `countryCodeToName` (`api/internal/handlers/country.go:14`), and `SUPPORTED_COUNTRIES` / `COUNTRY_CENTERS` (`web/src/components/EventsDashboard.tsx:22-28`).
- **Scheduler.** Countries run sequentially (`runAllCountries`, `scheduler.go:121`) under a lock with a **5-minute TTL** (`scheduler.go:83`) "sized for a documented ~3 minute worst case" at two countries. One `RunBudget` caps GDACS work per run at **60 requests and 90 s** (`gdacs.go`, `maxGDACSRequestsPerRun`, `gdacsRunBudgetDuration`).
- **Readiness.** `/ready` answers 503 if **any** country's last run is `failure`, or a query fails (`handlers/health.go`, `anyFailure || queryFailed`).
- **Filters.** `state` is matched by `ILIKE` on `state_name` alone (`queries.go`, `buildEventFilterClause`), so it is ambiguous across countries.
- **Legacy seed.** `api/cmd/seed` downloads simplified geoBoundaries Nigeria and inserts it as `NGA`. No workflow or deploy step runs it.

## 1. The manifest

`api/internal/boundaries/manifest.json`, embedded with `go:embed`, is the single source of truth for covered countries.

Each entry:

| field | meaning |
|---|---|
| `code` | ISO 3166-1 alpha-2, matching `admin_boundaries.country_code` |
| `name` | canonical English name, written to `country_name` and returned by the API |
| `source` | `hdx-cod-ab` (the only value in this release) |
| `source_url` | the ADM1 GeoJSON resource URL |
| `vintage` | the dataset's reference date as published by HDX |
| `name_field` | the ADM1 name property in the file (for example `ADM1_EN`) |
| `sha256` | checksum of the downloaded file |
| `bbox` | `[min_lon, min_lat, max_lon, max_lat]`: the file's extent plus 0.1°, the rule `DefaultCountries` documents today |
| `enabled` | whether the country is ingested and offered |

- **Built by a committed script** (`scripts/boundaries/`) that downloads each file, computes `sha256` and `bbox`, detects `name_field` using the candidate list in `scripts/generate_boundary_migration.py`, and writes the entry. A country change is then a one-line reviewable diff.
- **Consumers:** the ingester builds its country list from enabled entries; the API's country validation reads it; the density scripts read it for their mirrored boxes. The web app reads `/v1/countries`, never the file.
- **Validation test:** codes unique and alpha-2, every enabled entry has a `sha256` and a `bbox` inside the Africa rectangle, NG and GH boxes equal today's `DefaultCountries` values or differ only by the documented rule (any difference recorded on the PR).

## 2. Boundary loader

**Where it runs: decision P1.** Recommended: inside the API process at startup, after migrations, in a background goroutine under a Postgres advisory lock, so a slow or failed HDX download never blocks boot. The alternative is a one-shot binary run by the host's deploy script, which is installed by hand (`deploy/vigil-deploy`).

**New migration `000017_boundary_sources`:** one row per loaded country: `country_code` (PK), `country_name`, `source`, `source_url`, `vintage`, `sha256`, `adm1_count`, `loaded_at`.

**Per country, in manifest order:**

1. Skip if `boundary_sources.sha256` equals the manifest's.
2. Download; reject if the size exceeds a configured ceiling or the checksum differs (log, mark the country `boundaries_failed`, continue with the next).
3. Parse ADM1 features using `name_field`; reject a file with zero features or a missing name on any feature.
4. In one transaction: delete the country's ADM1 rows and its ADM0 row (including a `000012` simplified outline), insert the ADM1 rows at full resolution with `ST_Multi(ST_MakeValid(…))`, insert the ADM0 row as `ST_Union` of the ADM1 rows, upsert `boundary_sources`.
5. Re-enrich stored events inside the country's `bbox` with `UPDATE events SET geom = geom WHERE …`, so names follow the new boundaries.

**Nigeria and Ghana are in the manifest from the first PR.** Loading them replaces the `000010` rows with the same HDX source and records provenance. Acceptance: unit counts unchanged (37, 16), and every stored NG/GH event's `country_name` and `state_name` unchanged (before/after diff committed as a script).

**Memory and time:** files are parsed whole. Mean source size is about 22 MB (scoping proposal, 8-country sample); South Africa's 89 MB is decision P3. Record peak memory and wall time per country on staging.

**Observability:** `/health` gains a `boundaries` object: countries loaded, countries failed (with codes), manifest checksum. A failed country does not affect `/ready`.

## 3. Ingestion

- **Country list from the manifest** (enabled entries), replacing the literal `DefaultCountries`.
- **Pacing:** a pause between countries, `INGEST_COUNTRY_PAUSE_MS` (default 1000). About 90 EONET requests per hourly tick at 45 countries (2 per country).
- **Lock renewal:** extend the scheduler lock periodically during the run (the "proper fix" the comment at `scheduler.go:81` names) instead of raising the 5-minute TTL. A crashed run still releases within one TTL.
- **GDACS budget:** measure actual GDACS requests and wall time per run on staging for at least 24 hours at full country count, then set `maxGDACSRequestsPerRun` and `gdacsRunBudgetDuration` from the measurement, with the duration kept below the lock renewal interval. Rotate the starting country each tick so exhaustion does not always fall on the same countries.
- **Alerts:** one failure or degraded alert per run, listing affected countries, instead of one per country. The existing per-streak deduplication (`alert_sent_at`) keeps working per country.
- **Readiness: decision P2.** Recommended: `/ready` answers 503 only when every country's last run failed or the run query failed. `/health` still reports each country, and `degraded` overall when any failed. Both deploy smoke checks already accept `degraded`.

## 4. API

Contract first: `openspec/specs/vigilafrica/openapi.yaml`, then `npm run sync:openapi`, then `api-contract.md`.

- **`GET /v1/countries`:** enabled countries with `code`, `name`, `bbox`, `centroid` (from the ADM0 row), `source`, `vintage`, `loaded` (boolean, from `boundary_sources`).
- **Country validation from the manifest.** `errUnknownCountry` no longer lists values; it points to `/v1/countries`.
- **State qualification:** when `country` and `state` are both given, `state` matches within that country only. `state` alone stays accepted for compatibility and may match several countries; the API contract says so. The web app always sends both.
- **`/v1/states`:** confirm it requires `country` (`GetDistinctStatesByCountry`); if it does not, require it.
- **Context defaults:** remove the Nigeria literals in `handlers/context.go` (scoping work item 6), or confine them to the development override path with a test.

## 5. Web

- **Countries from `/v1/countries`**, replacing `SUPPORTED_COUNTRIES` and `COUNTRY_CENTERS`.
- **Searchable country picker** for 45+ entries; keyboard accessible; the selection stays in the URL as today.
- **Default map view:** the continent when no country is selected and no location resolves; the selected country's `bbox` otherwise.
- **Provenance:** when a country is selected, show its boundary source and vintage (scoping work item 9).
- **Copy:** hazard-neutral landing and metadata text (`web/index.html`, `App.tsx`); partner page lead per D4, with the digest still described as flood-only; `llms.txt` and `SITEMAP_ROUTES` descriptions in `web/vite.config.ts`. "Nigeria and Ghana live" appears nowhere.
- **CSS changes** carry pixel evidence (`scripts/bench-dashboard-cls/`). New colours are tokens.

## 6. Country-name matching (for Phase 3)

A committed test table maps each enabled country's MaxMind English name to its manifest `name`. Mismatches (for example "Ivory Coast" or "DR Congo") are listed with the chosen mapping. This change only adds the table; Phase 3 uses it.

## Acceptance criteria

- [ ] No country list remains outside the manifest: grep for `SUPPORTED_COUNTRIES`, `COUNTRY_CENTERS`, `countryCodeToName` and literal `DefaultCountries` entries returns only manifest-derived code.
- [ ] Every enabled manifest entry's file verifies against its `sha256`; a deliberately wrong checksum is rejected in a test.
- [ ] A second deploy with an unchanged manifest downloads nothing (staging log shows every country skipped).
- [ ] NG and GH unit counts and stored-event names are unchanged by the first load (committed before/after script).
- [ ] On staging, all enabled countries are loaded and `/health` `boundaries` shows zero failures.
- [ ] Spot check: one known coordinate per enabled country resolves to the expected ADM1 name (committed fixture; at least the 10 countries with the most floods in `floods-by-title.mjs` hand-checked).
- [ ] A full scheduled run at full country count completes on staging; run time, EONET requests and GDACS requests per run are recorded on the PR, and the GDACS budget values cite that measurement.
- [ ] One failing country does not make `/ready` answer 503 (test), and produces one alert per run, not per country (test).
- [ ] `/v1/countries`, the state qualification and the country-validation message are in `openapi.yaml`, the synced handlers copy and `api-contract.md`.
- [ ] The country picker works for every enabled country with keyboard only; the map frames the continent by default.
- [ ] No copy says "Nigeria and Ghana live"; partner page still describes the digest as flood-only.
- [ ] `go vet ./...` and the Go suite pass (`scripts/test-api.ps1` on the Windows host; `-race` in CI); `npm run lint`, `type-check`, `lint:styles`, `test`, `build` pass in `web/`.
- [ ] The Phase 2 early stop check is recorded on the release PR before release.
- [ ] An independent review is recorded on each PR.

## Out of scope

The near-me panel and display window (Phase 3), new categories, digest changes, simplification, ADM2, the 9 shapefile-only countries.
