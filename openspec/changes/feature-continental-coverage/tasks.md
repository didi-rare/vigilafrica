# feature-continental-coverage

Each implementation PR ticks its tasks here with evidence. Ticking this file in a PR's own diff is also what satisfies the sentinel for that PR (`api/cmd/sentinel`, `isGovernanceRecord`).

## 1. Decisions (maintainer)

- [x] 1.1 P1: where the loader runs. Decided 2026-11-25: API startup goroutine, before the first ingestion, under the `scheduler_locks` lease, with `BOUNDARY_LOADER_ENABLED`. Amended 2026-11-26: at the start of every locked run
- [x] 1.2 P2: when `/ready` answers 503. Decided 2026-11-25: only when every country's last run failed, or a run query failed
- [x] 1.3 P3: South Africa in the first release. Decided 2026-11-25, revised after review: yes (ADM1 file 20 MB)
- [x] 1.4 P4: delete the legacy `api/cmd/seed`. Decided 2026-11-25: yes
- [x] 1.5 P5: NG and GH boxes follow the extent-plus-0.1° rule. Decided 2026-11-25
- [x] 1.6 P6: country outlines for the 9 deferred countries; events matching no African country are not stored. Decided 2026-11-25
- [x] 1.7 P7: a checksum mismatch fails the country loudly; fixed by a manifest update, no mirror. Decided 2026-11-25
- [x] 1.8 Independent review of this record (2026-11-25): 1 blocker and 16 should-fix findings, all applied 2026-11-26
- [x] 1.9 Second independent review (2026-11-26): 2 blockers (skip rule ignored manifest changes other than the checksum; a startup run that finds the lease held skipped the loader), 7 should-fix and nits, all applied 2026-11-26

## 2. Manifest (PR: boundaries manifest and loader)

- [ ] 2.1 Survey: for each GeoJSON country, record the zip URL, the ADM1 member name (not an `_em` variant), end of the reference period, name and P-code fields; list African territories with no entry (at least Western Sahara)
- [x] 2.2 Committed script under `scripts/boundaries/` that builds entries (download, extract member, `member_sha256`, `bbox`, fields). Done (PR: boundaries manifest and loader): `scripts/boundaries/boundary_manifest.py build --iso3 NGA --code NG --name Nigeria` (any country: HDX `package_show` for `cod-ab-<iso3>`, the `<iso3>_admin_boundaries.geojson.zip` resource, the `<iso3>_admin1.geojson` member, never `_em`). Vintage is the end of HDX `dataset_date`; bbox is the member extent plus 0.1°, rounded outward to 2 decimals; name candidates copied from `generate_boundary_migration.py` `ADM1_NAME_CANDIDATES`, P-code candidates start with `adm1_pcode`. Offline tests: `python -I scripts/boundaries/test_boundary_manifest.py`, 15 passed. Run 2026-11-26 for NG and GH: 37 and 16 features
- [x] 2.3 Committed checksum script that re-verifies every entry (run on the PR). Done: `python -I scripts/boundaries/boundary_manifest.py verify` re-downloads every entry and checks `member_sha256`; run 2026-11-26: `2/2 entries verified` (GH `3ad6d140…871b`, NG `e6fe0153…6a70`)
- [x] 2.4 `api/internal/boundaries/manifest.json` with NG and GH only, embedded with `go:embed`. Done: `api/internal/boundaries/manifest.json` (GH, NG; one country per line), embedded in `manifest.go` with `go:embed`. Boxes: NG `[2.56, 4.17, 14.78, 14.0]`, GH `[-3.37, 4.64, 1.31, 11.28]` (outward rounding; spec §1's figures are nearest-rounded)
- [x] 2.5 Offline validation test (unique alpha-2 codes, `member_sha256` and `bbox` on every entry, required fields, no `_em` member, boxes inside Africa). Done: `TestEmbeddedManifestIsValid` runs `Parse` on the embedded file (every spec §1 rule, plus https URLs, YYYY-MM-DD vintage, sorted codes, unknown keys refused); `TestParseRejectsInvalidManifests` covers 22 invalid cases. Africa rectangle `[-26, -47.5, 64, 38]` (`AfricaRect`)

## 3. Loader (same PR)

- [x] 3.1 Migration `000017_boundary_sources` (table with `entry_sha256` and the `failed_*` columns, `admin_boundaries.adm_pcode`; up and down). Done: `api/db/migrations/000017_boundary_sources.{up,down}.sql`; `TestMigration000017RoundTripAndReplay` (down, up twice, constraints) passes against PostGIS 15-3.4
- [x] 3.2 Loader per spec §2: skip when the entry hash matches and `reenriched_at` is set; retry a failed entry only after `BOUNDARY_RETRY_HOURS`; size ceiling; member extraction; name overrides; `ST_CollectionExtract` before `ST_Multi`. Done: `api/internal/boundaries/loader.go`, `parse.go`, `config.go`; skip on entry hash + `reenriched_at`, retry back-off, size ceiling on archive and member, only the named member decompressed, CRS and geometry-type checks, overrides, P-codes. Unit tests with an httptest TLS server and a fake store; SQL in `api/internal/database/boundaries.go`
- [x] 3.3 Boundary swap, ADM0 union, re-enrichment and provenance in one transaction, with a statement timeout under the lease TTL. Done: `BoundaryLoadTx` (`database/boundaries.go`): delete levels 0 and 1, insert `ST_Multi(ST_CollectionExtract(ST_MakeValid(...), 3))`, ADM0 union, `UPDATE events SET geom = geom` on events in the box or still carrying a deleted row's country name, upsert `boundary_sources`, all in one transaction with `set_config('statement_timeout', …, true)` and a context deadline of `BOUNDARY_TX_TIMEOUT` (default 2m). Null-country count logged after commit
- [x] 3.4 Run at the start of every locked scheduler run, before ingestion, under the lease; `BOUNDARY_LOADER_ENABLED=false` in dev and CI; release the lock with a fresh context; stop the run when a renewal fails. Done: `runLocked` in `scheduler.go` runs the loader before ingestion in every locked run; renewal = `TryAcquireSchedulerLock` with the same holder, refused or failed renewal stops the run; release on `context.WithoutCancel` + 10 s. `BOUNDARY_LOADER_ENABLED` defaults true; `false` in `.env.example` and `docker-compose.yml` (dev); staging and prod compose pass it through with default `true`. CI starts no API process (`ci-cd.yml` runs only `go test` and `go build`), so it has no env to set Fixed after the independent review (P1): `main` closed the pool right after `srv.Shutdown` while the scheduler goroutine had not yet released the lease, so a stopped container kept it for a whole TTL. `StartScheduler` now returns a done channel that closes after the release, and `main` waits on it (within the 15 s shutdown timeout) before `repo.Close()`. `TestStartSchedulerDoneClosesAfterLeaseRelease` fails with the wait reverted ("lease released 0 times by the time done closed") and passes with it. Real container, 2026-11-26, `docker stop` during the first boundary download: with the fix the `scheduler_locks` row is gone and nothing is recorded in `boundary_sources`; the pre-fix binary (f4e067d) left the row held (`<container>:1`, `locked_until` in the future)
- [x] 3.5 `/health` `boundaries` object, and `HealthResponse.boundaries` in `openapi.yaml`. Done: `boundaries` object (`loaded`, `failed`, `manifest_sha256`; `null` when unreadable) in `handlers/health.go`, `HealthResponse.boundaries` + `BoundaryStatus` in `openapi.yaml`, `npm run sync:openapi`, `api-contract.md` §2. `TestReadinessIgnoresFailedBoundaries` proves `/ready` ignores it
- [x] 3.6 Tests: wrong checksum rejected; zero-feature file rejected; second run skips; one country's failure does not stop the others; a crash before commit leaves old boundaries; a changed override with the same member reloads; lease held at boot, loader runs on the next tick; a failed entry is not downloaded again within the retry window. Done. Unit (`internal/boundaries`): `TestLoaderRejectsWrongChecksum`, `TestLoaderRejectsUnusableFiles` (zero features, no name, non-polygon, projected CRS), `TestLoaderLoadsAndSecondRunSkips`, `TestLoaderOneCountryFailureDoesNotStopOthers`, `TestLoaderReloadsWhenOverrideChangesWithSameMember`, `TestLoaderDefersFailedEntryUntilRetryWindow`, `TestLoaderStopsWhenLeaseRenewalFails`; (`internal/ingestor`): `TestLeaseHeldAtBootLoaderRunsOnNextTick`, `TestLockReleasedOnFreshContextAfterShutdown`, `TestLockedRunStopsWhenRenewalFails`. Integration (PostGIS): `TestBoundaryLoaderEndToEnd` (load, skip, override reload, crash injected at re-enrichment leaves the old boundaries and event names). The crash assertion was re-broken on purpose (swap committed before re-enrichment in a second transaction): it failed at the old-boundaries check, then passed with the fix restored Added after review: `TestStartSchedulerDoneClosesAfterLeaseRelease`, `TestStartSchedulerDisabledReturnsClosedChannel`, `TestLoaderRedirectPolicy` (https-only redirects, at most 5 hops), and a stale-failure case in `TestStatusCountsLoadedAndCurrentFailures`
- [ ] 3.7 NG/GH equivalence: committed before/after script; unit counts 37/16 and stored-event names unchanged on staging. Script done: `scripts/boundaries/equivalence.sh` (snapshot/compare; exits 1 on any change). Local run 2026-11-26 against PostGIS 15-3.4 with 89 real NG/GH events ingested from EONET plus 16,625 probe events from `scripts/boundaries/equivalence-probes.sql` (13,231 + 3,264 points on a 0.1° grid over the old NG and GH boxes, and 130 polygons; local databases only): `RESULT: EQUIVALENT`, NG 37 → 37 and GH 16 → 16 ADM1, 16,714 events unchanged; a hand-edited snapshot made it report `CHANGED` and exit 1. The staging run is still to do after merge
- [x] 3.8 P4: remove `api/cmd/seed` and its mentions in `docs/standards/developers-go.md`. Done: `api/cmd/seed` deleted; `developers-go.md` §1.1, §2.2 and §8.1 no longer list it (other mentions are in archived change records only)

## 4. Countries (PR: enable the first release)

- [ ] 4.1 Add the remaining GeoJSON countries (`adm1`, enabled) and the 9 deferred countries (`adm0`) to the manifest
- [ ] 4.2 Load on staging; `/health` `boundaries.failed` empty; peak memory (including the archive) and time per country from a committed `scripts/bench-boundary-load/` script; longest per-country transaction under 2.5 minutes
- [ ] 4.3 Re-run the equivalence script after all countries load; list and explain every event that changed country
- [ ] 4.4 Spot-check fixture: one coordinate per enabled country, expected names from ISO 3166-2 with aliases
- [ ] 4.5 Name overrides for source errors found by the spot check (at least ZAF "Nothern Cape")

## 5. Ingestion (PR: continental ingestion)

- [ ] 5.1 Country list from enabled `adm1` manifest entries whose ADM1 rows are loaded. **Before "never loaded means not ingested" ships** (review of the loader PR, 2026-11-26): split the loader's retry policy. Transient download errors (timeout, HTTP 5xx, connection reset) get a short back-off; deterministic ones (checksum mismatch, parse or validation errors) keep `BOUNDARY_RETRY_HOURS`. Today every failure waits the full window, which would keep a fresh database's country unserved for a day after one HDX blip
- [ ] 5.2 `cmd/ingest` uses the scheduler's run function (lease and pacing). Note (review of the loader PR): `cmd/ingest` currently writes without the lease, so events it inserts while a boundary load transaction is open are named against the old boundaries and are not re-enriched by that load
- [ ] 5.3 Pause between countries (`INGEST_COUNTRY_PAUSE_MS`)
- [ ] 5.4 Lease renewal between countries; a failed renewal stops the run
- [ ] 5.5 GDACS budget per country instead of per run, with a per-country deadline below the lease TTL
- [ ] 5.6 Rotate the starting country each tick
- [ ] 5.7 One failure email and one degraded email per run, each listing countries; `alert_sent_at` set on every included degraded run; sent at run end, including a run stopped by shutdown
- [ ] 5.8 P2 readiness rule over ingested countries (no runs yet means ready), dropping the global last-run `note()`; `openapi.yaml:50` updated; tests
- [ ] 5.9 P6: skip events whose geometry intersects no ADM0 row; `events_skipped_no_country` per run
- [ ] 5.10 P6: committed script, run after task 4.2: count stored events with no country by cause (outside Africa, territory in no file, at sea, sliver), record the counts, then remove them; take ADM0 from the dataset's `admin0` member if slivers are found
- [ ] 5.11 24-hour staging measurement with a committed `scripts/bench-continental-ingest/` script; GDACS budget values cite it; degraded country-runs under 10%
- [ ] 5.12 Density scripts read their boxes from the manifest

## 6. API (PR: continental API, merges before the web PR)

- [ ] 6.1 Contract first: `openapi.yaml` (replace the four country enums), `npm run sync:openapi`, `api-contract.md` (including line 188's error message)
- [ ] 6.2 `GET /v1/countries` with `label_point` from `ST_PointOnSurface`
- [ ] 6.3 Country validation from the manifest; new `errUnknownCountry` message
- [ ] 6.4 Step 1: `/v1/states` with no `country` still answers, marked deprecated in the contract
- [ ] 6.5 GeoIP reader returns the ISO country code alongside the name
- [ ] 6.6 Step 2, a release after the web change is live in production: `/v1/states` without `country` answers 400, marked as a breaking change

## 7. Web (PR: continental interface)

- [ ] 7.1 Countries from `/v1/countries`; remove `SUPPORTED_COUNTRIES` and `COUNTRY_CENTERS`; offer only `loaded: true`; merge after the API's `/v1/countries` is live in the target environment
- [ ] 7.2 Searchable, keyboard-accessible country picker
- [ ] 7.3 State picker disabled until a country is chosen; `fetchStates` and `fetchEvents` always send `country` with `state`
- [ ] 7.4 Continental default map view; selected country framed by its `bbox`
- [ ] 7.5 Boundary source and vintage shown for the selected country
- [ ] 7.6 Hazard-neutral copy: `index.html`, `App.tsx`, partner page lead (digest flood-only, across covered countries), `llms.txt` text in `vite.config.ts:139`
- [ ] 7.7 Regenerate `web/public/social-card.png` without "NG·GH"
- [ ] 7.8 Tests; pixel evidence for CSS changes

## 8. Other two-country places (any PR above)

- [ ] 8.1 `README.md:56`, `:67`
- [ ] 8.2 `country-onboarding-template.md` §1.1–1.5, §2.1–2.3 and §3.1 rewritten for the manifest process; §2.3's no-overlap rule dropped (P5)
- [ ] 8.3 `tests/performance/load-test.js:27-28`: add a third country

## 9. Release

- [ ] 9.1 Phase 2 early stop check recorded on the release PR
- [ ] 9.2 Independent review recorded on each PR above
- [ ] 9.3 Promote to staging; verify `/health`, `/ready`, `boundaries`, per-country ingestion, map and console
- [ ] 9.4 Record how many stale-open wildfires appear on the first page of the default list
- [ ] 9.5 Promote to production; same checks
- [ ] 9.6 Archive this change (`/openspec-archive`)
