# Spec: Continental Coverage, First Release (feature-continental-coverage)

**Companion:** [`proposal.md`](proposal.md). Scoping evidence: [`openspec/proposals/feature-continental-coverage.md`](../../proposals/feature-continental-coverage.md). Line references are to `origin/development` at `79288c1` (merged 2026-11-19), checked on 2026-11-25 and 2026-11-26; re-check before editing.

## Facts this spec is built on

- **HDX layout.** Each HDX COD-AB dataset publishes **one zip per country holding every admin level**, not a separate ADM1 file. Nigeria: `nga_admin_boundaries.geojson.zip`, "administrative level 0-3 boundaries", containing `nga_admin0/1/2/3.geojson` plus `_em` variants and other layers (checked 2026-11-25/26). South Africa's 89 MB is its whole zip (93,548,039 bytes); its ADM1 member is 20.0 MB uncompressed. The scoping proposal's ~22 MB mean is also all-level zips. All five datasets the review queried (NGA, GHA, KEN, ZAF, BWA) show `metadata_modified` 2026-11-23, so HDX refreshes datasets.
- **Boundaries.** `admin_boundaries` (`000002`) holds `country_code`, `country_name`, `adm_level` (0–2), `adm_name`, `geom MultiPolygon 4326`, plus a generated `area_m2` (`000013`). Nigeria (37) and Ghana (16) ADM1 rows and their ADM0 unions come from migration `000010` (HDX COD-AB), keyed by **alpha-2** codes (`NG`, `GH`). Migration `000012` adds simplified geoBoundaries ADM0 outlines for seven neighbours (BF, BJ, CI, CM, NE, TD, TG) as a country-name fallback.
- **Enrichment.** The trigger fires `BEFORE INSERT OR UPDATE OF geom ON events` (`000013:82-118`): ADM1 match first (smallest `area_m2` wins), ADM0 fallback second. Re-enrichment is `UPDATE events SET geom = geom WHERE …`, the method `000010` and `000012` used.
- **Country lists, four copies:** `DefaultCountries` (`api/internal/ingestor/eonet.go:84`), `countryCodeToName` (`api/internal/handlers/country.go:14`), `SUPPORTED_COUNTRIES` / `COUNTRY_CENTERS` (`web/src/components/EventsDashboard.tsx:22-28`), and the `openapi.yaml` enums `[Nigeria, Ghana]` / `[NG, GH]` (lines 86, 95, 218, 226). `api-contract.md:188` quotes the current error message.
- **Scheduler.** Countries run sequentially (`runAllCountries`, `scheduler.go:121`) under a `scheduler_locks` lease with a **5-minute TTL** (`scheduler.go:83`); the "~3 minute worst case" rationale is at `scheduler.go:75-77` and `gdacs.go:628-629`, and the "proper fix" of renewing the lease is named at `scheduler.go:80-82`. Calling `TryAcquireSchedulerLock` with the same holder extends the lease (`queries.go:514-535`). On boot the scheduler runs one ingestion immediately (`scheduler.go:50-53`).
- **GDACS budget.** One `RunBudget` per run caps GDACS work at **60 requests** and a **90 s wall-clock deadline fixed at run start** (`gdacs.go:617`, `:630`, `:654-660`). After the deadline a lookup fails as `resolveUpstream` (`gdacs.go:693-699`), which makes that country's run `degraded` (`eonet.go:186-190`).
- **Readiness.** `/ready` answers 503 if `anyFailure || queryFailed` (`handlers/health.go:129`). `note()` is fed both the global last run (`health.go:103-110`) and each country's last run, so one country's failure is enough. `openapi.yaml:50` documents this rule.
- **Alerts.** Failures email on every failed run, per country (`scheduler.go:138-151`); only `degraded` runs are deduplicated per streak via `alert_sent_at`. `SendIngestFailure` takes one run (`alert/resend.go:93`).
- **Filters.** `buildEventFilterClause` already ANDs `country` and `state` (`queries.go:45-51`). The gap is the web app: `fetchStates()` is called with no country when none is selected (`EventsDashboard.tsx:305-309`, `api/events.ts:165-169`), and `fetchEvents` sends `state` without `country` when no country is selected (`api/events.ts:78-79`). `/v1/states` does not require `country` (`openapi.yaml:211`, "optionally filtered by country").
- **Deploy.** `deploy/vigil-deploy` is only the SSH forced-command filter; `deploy/vigil-deploy-run` runs `docker compose` with `docker-compose.staging.yml` or `docker-compose.prod.yml` from the checked-out commit. One API container runs per environment (fixed `container_name`). The demo stack runs with ingestion off (`docker-compose.demo.yml:35`, `INGEST_INTERVAL_MIN=0`).
- **GeoIP.** The reader checks `record.Country.IsoCode` but returns only the English name (`geoip/reader.go:53-58`). There is no `.mmdb` in the repo.
- **Context.** The Nigeria default centre is already gone; only the env-gated, tested `DevForceLagos` remains (`handlers/context.go:55-75`).
- **Digest.** `BuildTodayDigest` filters floods by date with **no country** (`digest/digest.go:62-71`), so it widens to every covered country automatically.
- **Legacy seed.** `api/cmd/seed` downloads simplified geoBoundaries Nigeria and inserts it as `NGA`. Only `cmd/server` is built into the image; nothing runs it.
- **`cmd/ingest`** loops over `DefaultCountries` with one budget but no lock and no pacing (`cmd/ingest/main.go:44-46`).

## 1. The manifest

`api/internal/boundaries/manifest.json`, embedded with `go:embed`, is the single source of truth for covered countries.

| field | meaning |
|---|---|
| `code` | ISO 3166-1 alpha-2, matching `admin_boundaries.country_code` |
| `name` | canonical English name, written to `country_name` and returned by the API |
| `levels` | `adm1` (named states) or `adm0` (country outline only, for naming events in deferred countries) |
| `source` | `hdx-cod-ab` or `geoboundaries` (`adm0` entries only) |
| `archive_url` | the zip (or file) to download |
| `member` | the file inside the zip to load, for example `nga_admin1.geojson`; never an `_em` variant |
| `member_sha256` | checksum of the extracted member, so a refresh of other levels does not break loading |
| `vintage` | the dataset's reference date as published by the source |
| `name_field` | the ADM1 name property (for example `adm1_name`) |
| `pcode_field` | the ADM1 P-code property, stored as a stable key |
| `name_overrides` | map of source name → corrected name, for errors in the source (for example ZAF "Nothern Cape") |
| `bbox` | `[min_lon, min_lat, max_lon, max_lat]`: the member's extent plus 0.1°, for every country including NG and GH (decided 2026-11-25) |
| `enabled` | ingested and offered; `false` keeps loaded boundaries for naming but stops ingesting and offering the country |

- **Built by a committed script** (`scripts/boundaries/`) that downloads each archive, extracts the member, computes `member_sha256` and `bbox`, detects `name_field` and `pcode_field` (candidate list in `scripts/generate_boundary_migration.py:29-33`), and writes the entry. A country change is a one-line reviewable diff.
- **Consumers:** the ingester's country list, the API's country validation, `cmd/ingest`, the density scripts' boxes. The web app reads `/v1/countries`, never the file.
- **Validation test (CI, offline):** codes unique and alpha-2; every `adm1` entry has `member`, `member_sha256`, `name_field`, `bbox`; no `member` ends in `_em.geojson`; boxes inside the Africa rectangle.
- **Checksum script (on the PR, needs network):** re-downloads every entry and verifies `member_sha256`.
- **Box change for NG and GH (decided 2026-11-25):** the rule gives NG `[2.57, 4.17, 14.78, 13.99]` against today's `[2.0, 4.0, 15.0, 14.0]`, and GH `[-3.36, 4.64, 1.30, 11.27]` against `[-3.5, 4.5, 1.2, 11.2]` (review computation from `000010` and the current HDX files). Accepted: the neighbouring countries now have their own boxes.

## 2. Boundary loader

**Where it runs (P1, decided 2026-11-25):** inside the API process, in the scheduler's startup goroutine, **before** the initial ingestion and under the existing `scheduler_locks` lease, so the loader and ingestion never overlap and no new lock is needed. HTTP serving starts first, so boot never waits on HDX. `BOUNDARY_LOADER_ENABLED` (default `true`; `false` in the demo compose file, dev and CI) switches it off. The alternative considered was a one-shot compose service with `depends_on: service_completed_successfully` in the versioned compose files, which also needs no host change.

**New migration `000017_boundary_sources`:**

- `boundary_sources`: one row per loaded country: `country_code` (PK), `country_name`, `levels`, `source`, `archive_url`, `member`, `member_sha256`, `vintage`, `unit_count`, `loaded_at`, `reenriched_at`.
- `admin_boundaries.adm_pcode TEXT NULL`.

**Per manifest entry, in manifest order:**

1. Skip if `boundary_sources.member_sha256` equals the manifest's **and** `reenriched_at` is set.
2. Download the archive; reject if larger than `BOUNDARY_MAX_ARCHIVE_BYTES` (default 128 MiB, above ZAF's 94 MB). Extract `member` only. **A checksum mismatch fails that country loudly** (error log, `/health` `boundaries.failed`), and the fix is to update the manifest after review; there is no self-hosted mirror (decided 2026-11-25).
3. Parse features using `name_field` and `pcode_field`; apply `name_overrides`; reject a file with zero features or a feature with no name.
4. **In one transaction:**
   - delete the country's rows at the levels being loaded, including a `000012` simplified ADM0 outline;
   - insert ADM1 rows with `ST_Multi(ST_CollectionExtract(ST_MakeValid(geom), 3))` so an invalid ring cannot yield a non-polygon;
   - insert the ADM0 row as `ST_Multi(ST_Union(...))` of the ADM1 rows (for `adm0` entries, the outline itself);
   - re-enrich stored events inside the country's `bbox` with `UPDATE events SET geom = geom WHERE …`;
   - upsert `boundary_sources` with `reenriched_at`.

   Doing the re-enrichment inside the transaction means a crash cannot leave the new boundaries committed with stale event names.
5. Pause `BOUNDARY_COUNTRY_PAUSE_MS` before the next entry and renew the lease.

**Deferred countries (decided 2026-11-25):** the 9 shapefile-only countries get `adm0` entries (country outline only), so events there are still named by country.

**Events outside every covered country (decided 2026-11-25):** events whose point matches no ADM0 row are not stored. The ingester checks this before insert and counts them in a new `events_skipped_no_country`, like `EventsSkippedBBox`. A one-off cleanup counts stored events with `country_name IS NULL`, records the count on the PR, then removes them.

**Nigeria and Ghana are in the manifest from the first PR.** Loading them replaces the `000010` rows with the same HDX source. Acceptance: unit counts unchanged (37, 16), and every stored NG/GH event's `country_name` and `state_name` unchanged (committed before/after script). **The same script is re-run after all countries load (task 4.2)**, and every event that changes country is listed, because the smallest-area match now competes across borders drawn from different sources.

**Memory and time:** only the ADM1 member is parsed. South Africa's ADM1 member is 20.0 MB, so it is in the first release (P3, revised 2026-11-25). Peak memory and wall time per country are measured by a committed script (`scripts/bench-boundary-load/`).

**Observability:** `/health` gains a `boundaries` object: `loaded` count, `failed` codes, manifest checksum. A failed country does not affect `/ready`. `HealthResponse` in `openapi.yaml` gains the object.

## 3. Ingestion

- **Country list from the manifest** (`adm1` entries with `enabled: true`), replacing the literal `DefaultCountries`. `cmd/ingest` uses the same run function as the scheduler, with the lease and the pacing.
- **Pacing:** `INGEST_COUNTRY_PAUSE_MS` (default 1000) between countries. About 90 EONET requests per hourly tick at 45 countries (2 per country, `eonet.go:342-358`).
- **Lease renewal:** renew the `scheduler_locks` lease between countries instead of raising the 5-minute TTL. A crashed run still releases within one TTL.
- **GDACS budget per country:** a fresh request allowance and deadline for each country's GDACS work, replacing the single per-run deadline (which would leave late countries with no lookups). The old reason for a short per-run deadline, protecting the lease, no longer applies once the lease is renewed. Values are set from a 24-hour staging measurement by a committed script (`scripts/bench-continental-ingest/`): run time, EONET requests and GDACS requests per country and per run.
- **Rotate the starting country** each tick.
- **Alerts, one per run:**
  - Failures: one email per run listing every country whose run failed (today: one per failed country per run).
  - Degraded: one email per run listing countries for which `degradedAlertAction` returns `send`; after a successful send, every included run gets `alert_sent_at`; countries returning `carry` keep today's behaviour and are not listed.
  - `SendIngestFailure` and the degraded sender take a list of runs.
- **Readiness (P2, decided 2026-11-25):** `/ready` answers 503 only when every country's last run failed or a run query failed. The global last-run `note()` (`health.go:103-110`) is dropped from the readiness decision. `/health` still reports `degraded` when any country failed or degraded. `openapi.yaml:50` is updated. Both deploy smoke checks already accept `degraded` (`deploy-staging.yml:58-60`, `deploy-production.yml:94-96`).

## 4. API

Contract first: `openspec/specs/vigilafrica/openapi.yaml`, then `npm run sync:openapi`, then `api-contract.md`.

- **`GET /v1/countries`:** enabled `adm1` countries with `code`, `name`, `bbox`, `label_point` (`ST_PointOnSurface` of the ADM0 row; `null` until loaded), `source`, `vintage`, `loaded`.
- **Country validation from the manifest.** The four enums become plain strings documented against `/v1/countries`; `errUnknownCountry` points to `/v1/countries`; `api-contract.md:188` updated.
- **`/v1/states` requires `country`.** This is a **breaking contract change**: the dashboard's default view calls it with no country today. It ships in the same release as the web change in §5 and is marked breaking in the contract.
- **GeoIP carries the ISO code:** the reader returns `CountryCode` (from `record.Country.IsoCode`) alongside the name, so Phase 3 matches countries by code, not by name. This replaces the MaxMind name table, which cannot be built without a `.mmdb`.

## 5. Web

- **Countries from `/v1/countries`**, replacing `SUPPORTED_COUNTRIES` and `COUNTRY_CENTERS`.
- **Searchable country picker** for 45+ entries; keyboard accessible; selection stays in the URL.
- **State picker disabled until a country is chosen**; `fetchStates` and `fetchEvents` always send `country` with `state`.
- **Default map view:** the continent when no country is selected and no location resolves; the selected country's `bbox` otherwise.
- **Provenance:** for a selected country, its boundary source and vintage.
- **Copy:** hazard-neutral `web/index.html` metadata and `App.tsx`; the partner page lead per D4, still describing the digest as flood-only and stating it covers every covered country; the `llms.txt` text in `web/vite.config.ts:139` ("Nigeria and Ghana are live"). `SITEMAP_ROUTES` (`vite.config.ts:84`) has no descriptions and needs no change. Regenerate `web/public/social-card.png`, which shows "NG·GH".
- **CSS changes** carry pixel evidence (`scripts/bench-dashboard-cls/`). New colours are tokens.

## 6. Other two-country places

- `README.md:56`, `:67` (product description). `README.md:28` describes the demo's curated seed data and stays.
- `openspec/specs/vigilafrica/country-onboarding-template.md` §2.2: replace the manual `DefaultCountries` step with the manifest process.
- `tests/performance/load-test.js:27-28`: add a country beyond NG and GH.
- **Digest:** it widens to every covered country with no code change. Under D4 the endpoint is left alone; the widening is a consequence of coverage, not a change to its shape. The partner page says so.

## Acceptance criteria

- [ ] No country list remains outside the manifest: grep for `SUPPORTED_COUNTRIES`, `COUNTRY_CENTERS`, `countryCodeToName`, literal `DefaultCountries` entries and `enum: [Nigeria` / `enum: [NG` returns only manifest-derived code.
- [ ] The checksum script verifies every entry's `member_sha256` (run on the PR); a wrong checksum is rejected in a unit test.
- [ ] A second start with an unchanged manifest downloads nothing (staging log shows every country skipped).
- [ ] A crash injected between the boundary write and re-enrichment leaves the old boundaries in place (test).
- [ ] NG and GH unit counts and stored-event names are unchanged by the first load; after all countries load, the same script lists every event that changed country, and each change is explained on the PR.
- [ ] On staging, all enabled countries and all `adm0` entries are loaded and `/health` `boundaries.failed` is empty.
- [ ] Spot check: one coordinate per enabled country resolves to the expected ADM1 name, with expected names taken from ISO 3166-2 subdivision names (an independent source), aliases listed in the fixture.
- [ ] No stored event has `country_name IS NULL` after the cleanup; `events_skipped_no_country` is reported per run.
- [ ] Over 24 hours on staging at full country count, fewer than 10% of country-runs are `degraded` (measured by the committed script); run time and request counts are recorded on the PR.
- [ ] One failing country does not make `/ready` answer 503 (test), and a run with several failing countries sends one email (test).
- [ ] `/v1/countries`, the `/v1/states` breaking change, the readiness rule, `HealthResponse.boundaries` and the country-validation message are in `openapi.yaml`, the synced handlers copy and `api-contract.md`.
- [ ] The country picker works for every enabled country with keyboard only; the state picker is disabled until a country is chosen; the map frames the continent by default.
- [ ] No copy says "Nigeria and Ghana live" or shows "NG·GH"; the partner page describes the digest as flood-only across covered countries.
- [ ] `go vet ./...` and the Go suite pass (`scripts/test-api.ps1` on the Windows host; `-race` in CI); `npm run lint`, `type-check`, `lint:styles`, `test`, `build` pass in `web/`.
- [ ] Every number that drives a decision here (load time and memory, GDACS budget, degraded share, null-country count) comes from a committed `scripts/bench-*` script.
- [ ] The Phase 2 early stop check is recorded on the release PR before release.
- [ ] An independent review is recorded on each PR.

## Out of scope

The near-me panel and the display window (Phase 3), new categories, digest content changes, simplification, ADM2, named states for the 9 shapefile-only countries.
