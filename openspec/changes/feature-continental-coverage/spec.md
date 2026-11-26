# Spec: Continental Coverage, First Release (feature-continental-coverage)

**Companion:** [`proposal.md`](proposal.md). Scoping evidence: [`openspec/proposals/feature-continental-coverage.md`](../../proposals/feature-continental-coverage.md). Line references are to `origin/development` at `79288c1` (merged 2026-11-19), checked on 2026-11-25 and 2026-11-26; re-check before editing.

## Facts this spec is built on

- **HDX layout.** Each HDX COD-AB dataset publishes **one zip per country holding every admin level**, not a separate ADM1 file. Nigeria: `nga_admin_boundaries.geojson.zip`, "administrative level 0-3 boundaries", containing `nga_admin0/1/2/3.geojson` plus `_em` variants and other layers (checked 2026-11-25/26). South Africa's 89 MB is its whole zip (93,548,039 bytes); its ADM1 member is 20.0 MB uncompressed. The scoping proposal's ~22 MB mean is also all-level zips. All five datasets the review queried (NGA, GHA, KEN, ZAF, BWA) show `metadata_modified` 2026-11-23, so HDX refreshes datasets. The second review (2026-11-26) found the same layout, `<iso3>_admin_boundaries.geojson.zip` holding `<iso3>_admin1.geojson`, for NGA, KEN, ETH, EGY, MAR, ZAF and COD; `_em` variants exist only for NGA and COD. HDX publishes the reference period as a date range (Nigeria: 2019-04-17 to 2025-10-30). Morocco's COD-AB stops at 27.662°N in both `mar_admin0` and `mar_admin1`, so Western Sahara is in no country file.
- **Boundaries.** `admin_boundaries` (`000002`) holds `country_code`, `country_name`, `adm_level` (0–2), `adm_name`, `geom MultiPolygon 4326`, plus a generated `area_m2` (`000013`). Nigeria (37) and Ghana (16) ADM1 rows and their ADM0 unions come from migration `000010` (HDX COD-AB), keyed by **alpha-2** codes (`NG`, `GH`). Migration `000012` adds simplified geoBoundaries ADM0 outlines for seven neighbours (BF, BJ, CI, CM, NE, TD, TG) as a country-name fallback.
- **Enrichment.** The trigger fires `BEFORE INSERT OR UPDATE OF geom ON events` (`000013:82-118`): ADM1 match first (smallest `area_m2` wins), ADM0 fallback second. Matching is `ST_Intersects` on the event's full geometry (`000013:91`, `:104`); the ingester's box check on polygons compares envelopes (`eonet.go:629`). Re-enrichment is `UPDATE events SET geom = geom WHERE …`, the method `000010` and `000012` used. The `000010` ADM0 rows are unions of the ADM1 rows (`000010:137-143`), so a gap between ADM1 polygons is a gap in ADM0 too.
- **Country lists, four copies:** `DefaultCountries` (`api/internal/ingestor/eonet.go:84`), `countryCodeToName` (`api/internal/handlers/country.go:14`), `SUPPORTED_COUNTRIES` / `COUNTRY_CENTERS` (`web/src/components/EventsDashboard.tsx:22-28`), and the `openapi.yaml` enums `[Nigeria, Ghana]` / `[NG, GH]` (lines 86, 95, 218, 226). `api-contract.md:188` quotes the current error message.
- **Scheduler.** Countries run sequentially (`runAllCountries`, `scheduler.go:121`) under a `scheduler_locks` lease with a **5-minute TTL** (`scheduler.go:83`); the "~3 minute worst case" rationale is at `scheduler.go:75-77` and `gdacs.go:628-629`, and the "proper fix" of renewing the lease is named at `scheduler.go:80-82`. Calling `TryAcquireSchedulerLock` with the same holder extends the lease (`queries.go:514-535`). On boot the scheduler runs one ingestion immediately (`scheduler.go:50-53`). A run that finds the lease held is skipped (`scheduler.go:98-101`). The lease is released with the shutdown context (`scheduler.go:102-106`), which is already cancelled after SIGTERM, so a stopped container's lease stays until it expires. The holder is `hostname:pid` (`scheduler.go:111-117`), and neither compose file sets a hostname. The whole scheduler is off when `INGEST_INTERVAL_MIN=0` (`scheduler.go:32-37`). Migrations run in `NewRepository` before the scheduler starts (`database/db.go:85`). All file references to `scheduler.go` mean `api/internal/ingestor/scheduler.go`.
- **GDACS budget.** One `RunBudget` per run caps GDACS work at **60 requests** and a **90 s wall-clock deadline fixed at run start** (`gdacs.go:617`, `:630`, `:654-660`). After the deadline a lookup fails as `resolveUpstream` (`gdacs.go:693-699`), which makes that country's run `degraded` (`eonet.go:186-190`).
- **Readiness.** `/ready` answers 503 if `anyFailure || queryFailed` (`handlers/health.go:129`). `note()` is fed both the global last run (`health.go:103-110`) and each country's last run, so one country's failure is enough. `openapi.yaml:50` documents this rule. The per-country map holds the last run of every country that has ever run (`queries.go:435-442`), and each is noted (`health.go:119-124`).
- **Alerts.** Failures email on every failed run, per country (`scheduler.go:138-151`); only `degraded` runs are deduplicated per streak via `alert_sent_at`. `SendIngestFailure` takes one run (`alert/resend.go:93`) and sends both kinds of email (`scheduler.go:149`, `:239`).
- **Filters.** `buildEventFilterClause` already ANDs `country` and `state` (`queries.go:45-51`). The gap is the web app: `fetchStates()` is called with no country when none is selected (`EventsDashboard.tsx:305-309`, `api/events.ts:165-169`), and `fetchEvents` sends `state` without `country` when no country is selected (`api/events.ts:79-80`). `/v1/states` does not require `country` (`openapi.yaml:211`, "optionally filtered by country").
- **Deploy.** `deploy/vigil-deploy` is only the SSH forced-command filter; `deploy/vigil-deploy-run` runs `docker compose` with `docker-compose.staging.yml` or `docker-compose.prod.yml` from the checked-out commit. One API container runs per environment (fixed `container_name`). The production container has a read-only root with `/tmp` on tmpfs (`docker-compose.prod.yml:69-71`). The web app deploys separately, through Vercel's GitHub integration (`web/vercel.json`), so the API and the web app never switch at the same moment. The demo stack runs with ingestion off (`docker-compose.demo.yml:35`, `INGEST_INTERVAL_MIN=0`).
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
| `archive_url` | the zip, or a plain GeoJSON file for a geoBoundaries outline, to download |
| `member` | the file inside the zip to load, for example `nga_admin1.geojson`; never an `_em` variant; omitted when `archive_url` is a plain file |
| `member_sha256` | checksum of the extracted member (of the downloaded file when there is no member), so a refresh of other levels does not break loading; required on every entry |
| `vintage` | the end of the source's reference period (HDX publishes a range; Nigeria's ends 2025-10-30) |
| `name_field` | the ADM1 name property (for example `adm1_name`) |
| `pcode_field` | the ADM1 P-code property, stored as a stable key |
| `name_overrides` | map of source name → corrected name, for errors in the source (for example ZAF "Nothern Cape") |
| `bbox` | `[min_lon, min_lat, max_lon, max_lat]`: the member's extent plus 0.1°, for every country including NG and GH (decided 2026-11-25) |
| `enabled` | ingested and offered; `false` keeps loaded boundaries for naming but stops ingesting and offering the country |

- **Built by a committed script** (`scripts/boundaries/`) that downloads each archive, extracts the member, computes `member_sha256` and `bbox`, detects `name_field` and `pcode_field` (name candidates from `scripts/generate_boundary_migration.py:29-33`; that script has no P-code list, so the new script adds one, starting with `adm1_pcode`), and writes the entry. A country change is a one-line reviewable diff.
- **Consumers:** the ingester's country list, the API's country validation, `cmd/ingest`, the density scripts' boxes. The web app reads `/v1/countries`, never the file.
- **Validation test (CI, offline):** codes unique and alpha-2; every entry has `archive_url`, `member_sha256`, `vintage` and `bbox`; every `adm1` entry has `member` and `name_field`; no `member` ends in `_em.geojson`; boxes inside the Africa rectangle.
- **Checksum script (on the PR, needs network):** re-downloads every entry and verifies `member_sha256`.
- **Box change for NG and GH (decided 2026-11-25):** the rule gives NG `[2.57, 4.17, 14.78, 13.99]` against today's `[2.0, 4.0, 15.0, 14.0]`, and GH `[-3.36, 4.64, 1.30, 11.27]` against `[-3.5, 4.5, 1.2, 11.2]` (review computation from `000010` and the current HDX files). Accepted: the strip between the old and new edges lies outside the country's own file, so an event there belongs to a neighbour and is fetched through that neighbour's box. Task 4.3's script confirms nothing is lost.

## 2. Boundary loader

**Where it runs (P1, decided 2026-11-25, amended 2026-11-26):** inside the API process, at the start of **every** locked scheduler run (the startup run and each tick), **before** ingestion and under the same `scheduler_locks` lease, so the loader and ingestion never overlap and no new lock is needed.

- **Why every run, not only startup:** a startup run that finds the lease held is skipped. A redeploy during a run leaves the old container's lease in place for up to one TTL, and the new container has a new holder (**Likely:** Docker's default hostname is the container ID). With a startup-only loader, that deploy would load no boundaries until the next restart. When nothing has changed, the loader only reads `boundary_sources`.
- **Lease handling:** the lock is released with a fresh short-lived context, so a stopped container frees the lease at once. If a lease renewal fails or returns false, the run stops.
- **Boot:** `StartScheduler` returns as soon as it has launched its goroutine, so boot never waits on HDX.
- **Switches:** because the loader lives in the scheduler, `INGEST_INTERVAL_MIN=0` (the demo stack) turns it off too. `BOUNDARY_LOADER_ENABLED` (default `true`; `false` in dev and CI) lets ingestion run without downloading boundaries.

The alternative considered was a one-shot compose service with `depends_on: service_completed_successfully` in the versioned compose files, which also needs no host change.

**New migration `000017_boundary_sources`:**

- `boundary_sources`: one row per manifest country the loader has tried: `country_code` (PK), `country_name`, `levels`, `source`, `archive_url`, `member`, `member_sha256`, `entry_sha256` (the hash of the manifest entry that was loaded), `vintage`, `unit_count`, `loaded_at`, `reenriched_at`; and for a failed attempt `failed_entry_sha256`, `failed_at`, `last_error`.
- `admin_boundaries.adm_pcode TEXT NULL`.

**Per manifest entry, in manifest order:**

1. Skip if `boundary_sources.entry_sha256` equals the SHA-256 of the canonicalised manifest entry (keys sorted; `enabled` excluded, since it changes nothing stored) **and** `reenriched_at` is set. Any change to the entry (a name override, a field, the box, the member) reloads the country; an unchanged entry downloads nothing. Also skip, without downloading, a country whose `failed_entry_sha256` equals the current entry hash and whose `failed_at` is less than `BOUNDARY_RETRY_HOURS` (default 24) ago, so a country that failed its checksum (P7) is not downloaded again every tick.
2. Download the archive; reject if larger than `BOUNDARY_MAX_ARCHIVE_BYTES` (default 128 MiB, above ZAF's 94 MB). Extract `member` only. The archive is held in memory or on the tmpfs `/tmp` (the production root is read-only), so it counts toward peak memory. **A checksum mismatch fails that country loudly** (error log, `/health` `boundaries.failed`, the `failed_*` columns), and the fix is to update the manifest after review; there is no self-hosted mirror (decided 2026-11-25).
3. Parse features using `name_field` and `pcode_field`; apply `name_overrides`; reject a file with zero features or a feature with no name.
4. **In one transaction**, with `SET LOCAL statement_timeout` from `BOUNDARY_TX_TIMEOUT` (default 2 minutes, under the 5-minute lease TTL):
   - delete the country's rows at the levels being loaded, including a `000012` simplified ADM0 outline;
   - insert ADM1 rows with `ST_Multi(ST_CollectionExtract(ST_MakeValid(geom), 3))` so an invalid ring cannot yield a non-polygon;
   - insert the ADM0 row as `ST_Multi(ST_Union(...))` of the ADM1 rows (for `adm0` entries, the outline itself);
   - re-enrich stored events inside the country's `bbox` with `UPDATE events SET geom = geom WHERE …`;
   - upsert `boundary_sources` with `entry_sha256` and `reenriched_at`, clearing the `failed_*` columns.

   Doing the re-enrichment inside the transaction means a crash cannot leave the new boundaries committed with stale event names. After commit, the loader logs how many events inside the box were left with `country_name IS NULL`.
5. Pause `BOUNDARY_COUNTRY_PAUSE_MS` before the next entry and renew the lease; stop the run if the renewal fails.

**Deferred countries (decided 2026-11-25):** the 9 shapefile-only countries get `adm0` entries (country outline only), so events there are still named by country.

**Events outside every covered country (decided 2026-11-25):** an event whose geometry intersects no loaded ADM0 row is not stored. This is the trigger's own `ST_Intersects` test on the full geometry, so a flood polygon that reaches land is kept even when its centroid is at sea. The ingester checks this before insert and counts the skipped events in a new `events_skipped_no_country`, like `EventsSkippedBBox`.

The rule drops more than events outside Africa. Each of these is counted before the cleanup (task 5.10):

- **Territory in no file:** Western Sahara (Morocco's file stops at 27.662°N). The survey (task 2.1) lists any other African territory with no entry.
- **Points at sea** inside a country's box.
- **Slivers between states:** a gap between ADM1 polygons is also a gap in the ADM0 union, so the trigger's ADM0 fallback cannot rescue it. If the count shows sliver drops, the ADM0 row comes from the dataset's own `admin0` member instead of the union (an `adm0_member` field is added to the manifest then).

**The null-country cleanup is a committed script, not a migration.** Migrations run before the loader, so a migration would delete events before the new boundaries could name them. The script runs once after every enabled country has loaded and re-enriched (task 4.2). It counts stored events with `country_name IS NULL` by cause, records the counts on the PR, then removes them. A later boundary refresh can leave new null-country events; the loader logs that count, and the script is re-run when it is not zero.

**Nigeria and Ghana are in the manifest from the first PR.** Loading them replaces the `000010` rows with the same HDX source. Acceptance: unit counts unchanged (37, 16), and every stored NG/GH event's `country_name` and `state_name` unchanged (committed before/after script). **The same script is re-run after all countries load (task 4.3)**, and every event that changes country is listed, because the smallest-area match now competes across borders drawn from different sources.

**Memory and time:** only the ADM1 member is parsed. South Africa's ADM1 member is 20.0 MB, so it is in the first release (P3, revised 2026-11-25). Peak memory (including the archive) and wall time per country are measured by a committed script (`scripts/bench-boundary-load/`). The longest per-country load transaction must stay under half the lease TTL (2.5 minutes).

**Observability:** `/health` gains a `boundaries` object: `loaded` count, `failed` codes, manifest checksum. A failed country does not affect `/ready`. `HealthResponse` in `openapi.yaml` gains the object.

## 3. Ingestion

- **Country list from the manifest** (`adm1` entries with `enabled: true` whose ADM1 rows are present in `admin_boundaries`), replacing the literal `DefaultCountries`. An enabled country that has never loaded (for example a checksum failure on a fresh database, P7) is neither ingested nor offered until it loads, and is listed in `/health` `boundaries.failed`; otherwise every event there would be dropped as matching no country. A country already loaded keeps being ingested on its existing boundaries while a refresh fails. `cmd/ingest` uses the same run function as the scheduler, with the lease and the pacing.
- **Pacing:** `INGEST_COUNTRY_PAUSE_MS` (default 1000) between countries. About 90 EONET requests per hourly tick at 45 countries (2 per country, `eonet.go:342-358`).
- **Lease renewal:** renew the `scheduler_locks` lease between countries instead of raising the 5-minute TTL. A crashed run still releases within one TTL. A failed or refused renewal stops the run.
- **GDACS budget per country:** a fresh request allowance and deadline for each country's GDACS work, replacing the single per-run deadline (which would leave late countries with no lookups). The old reason for a short per-run deadline, protecting the lease, no longer applies once the lease is renewed between countries; the per-country deadline is set below the TTL so no single country can outlive the lease. Values are set from a 24-hour staging measurement by a committed script (`scripts/bench-continental-ingest/`): run time, EONET requests and GDACS requests per country and per run.
- **Rotate the starting country** each tick.
- **Alerts, at most two emails per run:**
  - Failures: one email per run listing every country whose run failed (today: one per failed country per run).
  - Degraded: one email per run listing countries for which `degradedAlertAction` returns `send`; after a successful send, every included run gets `alert_sent_at`; countries returning `carry` keep today's behaviour and are not listed.
  - `SendIngestFailure`, which sends both kinds today, takes a list of runs.
  - Both emails are sent when the run ends, including a run stopped by shutdown (with a fresh short-lived context, as for the lock release).
- **Readiness (P2, decided 2026-11-25):** `/ready` answers 503 only when every country's last run failed or a run query failed. The global last-run `note()` (`health.go:103-110`) is dropped from the readiness decision. Both rules are evaluated over the countries that are ingested (§3), so a country disabled after a failure no longer counts. With none of them run yet (a fresh database), `/ready` answers 200; a country that has never run does not count as failed. `/health` still reports `degraded` when any such country failed or degraded. `openapi.yaml:50` is updated. Both deploy smoke checks already accept `degraded` (`deploy-staging.yml:58-60`, `deploy-production.yml:94-96`).

## 4. API

Contract first: `openspec/specs/vigilafrica/openapi.yaml`, then `npm run sync:openapi`, then `api-contract.md`.

- **`GET /v1/countries`:** enabled `adm1` countries (including any not yet loaded) with `code`, `name`, `bbox`, `label_point` (`ST_PointOnSurface` of the ADM0 row; `null` until loaded), `source`, `vintage`, `loaded`.
- **Country validation from the manifest.** The four enums become plain strings documented against `/v1/countries`; `errUnknownCountry` points to `/v1/countries`; `api-contract.md:188` updated.
- **`/v1/states` requires `country`, in two steps.** The web app deploys through Vercel and the API over SSH, so they never switch together.
  - **Step 1 (this change):** the API adds `/v1/countries` and still accepts `/v1/states` with no country, marked deprecated in the contract. The web change merges only after that API is live in the environment the merge deploys to, and from then on always sends `country`.
  - **Step 2 (a later release, once the web change is live in production):** `/v1/states` without `country` answers 400, marked as a **breaking change**.
  - Step 2 first would empty the current dashboard's state list, because `fetchStates` throws on a non-OK answer (`api/events.ts:171-173`). The web change first would leave its picker with no countries (404 from the old API).
- **GeoIP carries the ISO code:** the reader returns `CountryCode` (from `record.Country.IsoCode`) alongside the name, so Phase 3 matches countries by code, not by name. This replaces the MaxMind name table, which cannot be built without a `.mmdb`.

## 5. Web

- **Countries from `/v1/countries`**, replacing `SUPPORTED_COUNTRIES` and `COUNTRY_CENTERS`; only `loaded: true` countries are offered. Merges after §4 step 1 is live.
- **Searchable country picker** for 45+ entries; keyboard accessible; selection stays in the URL.
- **State picker disabled until a country is chosen**; `fetchStates` and `fetchEvents` always send `country` with `state`.
- **Default map view:** the continent when no country is selected and no location resolves; the selected country's `bbox` otherwise.
- **Provenance:** for a selected country, its boundary source and vintage.
- **Copy:** hazard-neutral `web/index.html` metadata and `App.tsx`; the partner page lead per D4, still describing the digest as flood-only and stating it covers every covered country; the `llms.txt` text in `web/vite.config.ts:139` ("Nigeria and Ghana are live"). `SITEMAP_ROUTES` (`vite.config.ts:84`) has no descriptions and needs no change. Regenerate `web/public/social-card.png`, which shows "NG·GH".
- **CSS changes** carry pixel evidence (`scripts/bench-dashboard-cls/`). New colours are tokens.

## 6. Other two-country places

- `README.md:56`, `:67` (product description). `README.md:28` describes the demo's curated seed data and stays.
- `openspec/specs/vigilafrica/country-onboarding-template.md`: §1.1–1.5, §2.1–2.3 and §3.1 (manual download, migration and `DefaultCountries` steps) are replaced by the manifest process. §2.3's "no bounding-box overlap" rule is dropped, since P5 accepts overlapping boxes.
- `tests/performance/load-test.js:27-28`: add a country beyond NG and GH.
- **Digest:** it widens to every covered country with no code change. Under D4 the endpoint is left alone; the widening is a consequence of coverage, not a change to its shape. The partner page says so.

## Acceptance criteria

- [ ] No country list remains outside the manifest: grep for `SUPPORTED_COUNTRIES`, `COUNTRY_CENTERS`, `countryCodeToName`, literal `DefaultCountries` entries and `enum: [Nigeria` / `enum: [NG` returns only manifest-derived code.
- [ ] The checksum script verifies every entry's `member_sha256` (run on the PR); a wrong checksum is rejected in a unit test.
- [ ] A second start with an unchanged manifest downloads nothing (staging log shows every country skipped); changing a `name_overrides` entry with the same member reloads that country (test); a failed entry is not downloaded again within `BOUNDARY_RETRY_HOURS` (test).
- [ ] With the lease held at boot, the loader runs on the next tick (test); a stopped container frees the lease (test); a failed renewal stops the run (test).
- [ ] A crash injected between the boundary write and re-enrichment leaves the old boundaries in place (test).
- [ ] NG and GH unit counts and stored-event names are unchanged by the first load; after all countries load, the same script lists every event that changed country, and each change is explained on the PR.
- [ ] On staging, all enabled countries and all `adm0` entries are loaded and `/health` `boundaries.failed` is empty; the longest per-country load transaction is under 2.5 minutes (`scripts/bench-boundary-load/`).
- [ ] An enabled country with no loaded boundaries is not ingested and not offered (test).
- [ ] Spot check: one coordinate per enabled country resolves to the expected ADM1 name, with expected names taken from ISO 3166-2 subdivision names (an independent source), aliases listed in the fixture.
- [ ] Before the cleanup, the null-country counts by cause (outside Africa, territory in no file, at sea, sliver) are recorded on the PR; after it, no stored event has `country_name IS NULL`; `events_skipped_no_country` is reported per run.
- [ ] Over 24 hours on staging at full country count, fewer than 10% of country-runs are `degraded` (measured by the committed script); run time and request counts are recorded on the PR.
- [ ] One failing country does not make `/ready` answer 503 (test); a fresh database with no runs is ready, and a disabled country is ignored (test); a run with several failing countries sends one failure email (test).
- [ ] `/v1/countries`, the `/v1/states` deprecation (step 1) and later breaking change (step 2), the readiness rule, `HealthResponse.boundaries` and the country-validation message are in `openapi.yaml`, the synced handlers copy and `api-contract.md`.
- [ ] The country picker works for every enabled country with keyboard only; the state picker is disabled until a country is chosen; the map frames the continent by default.
- [ ] No copy says "Nigeria and Ghana live" or shows "NG·GH"; the partner page describes the digest as flood-only across covered countries.
- [ ] `go vet ./...` and the Go suite pass (`scripts/test-api.ps1` on the Windows host; `-race` in CI); `npm run lint`, `type-check`, `lint:styles`, `test`, `build` pass in `web/`.
- [ ] Every number that drives a decision here (load time and memory, GDACS budget, degraded share, null-country count) comes from a committed `scripts/bench-*` script.
- [ ] The Phase 2 early stop check is recorded on the release PR before release.
- [ ] An independent review is recorded on each PR.

## Out of scope

The near-me panel and the display window (Phase 3), new categories, digest content changes, simplification, ADM2, named states for the 9 shapefile-only countries.
