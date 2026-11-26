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
- [ ] 2.2 Committed script under `scripts/boundaries/` that builds entries (download, extract member, `member_sha256`, `bbox`, fields)
- [ ] 2.3 Committed checksum script that re-verifies every entry (run on the PR)
- [ ] 2.4 `api/internal/boundaries/manifest.json` with NG and GH only, embedded with `go:embed`
- [ ] 2.5 Offline validation test (unique alpha-2 codes, `member_sha256` and `bbox` on every entry, required fields, no `_em` member, boxes inside Africa)

## 3. Loader (same PR)

- [ ] 3.1 Migration `000017_boundary_sources` (table with `entry_sha256` and the `failed_*` columns, `admin_boundaries.adm_pcode`; up and down)
- [ ] 3.2 Loader per spec §2: skip when the entry hash matches and `reenriched_at` is set; retry a failed entry only after `BOUNDARY_RETRY_HOURS`; size ceiling; member extraction; name overrides; `ST_CollectionExtract` before `ST_Multi`
- [ ] 3.3 Boundary swap, ADM0 union, re-enrichment and provenance in one transaction, with a statement timeout under the lease TTL
- [ ] 3.4 Run at the start of every locked scheduler run, before ingestion, under the lease; `BOUNDARY_LOADER_ENABLED=false` in dev and CI; release the lock with a fresh context; stop the run when a renewal fails
- [ ] 3.5 `/health` `boundaries` object, and `HealthResponse.boundaries` in `openapi.yaml`
- [ ] 3.6 Tests: wrong checksum rejected; zero-feature file rejected; second run skips; one country's failure does not stop the others; a crash before commit leaves old boundaries; a changed override with the same member reloads; lease held at boot, loader runs on the next tick; a failed entry is not downloaded again within the retry window
- [ ] 3.7 NG/GH equivalence: committed before/after script; unit counts 37/16 and stored-event names unchanged on staging
- [ ] 3.8 P4: remove `api/cmd/seed` and its mentions in `docs/standards/developers-go.md`

## 4. Countries (PR: enable the first release)

- [ ] 4.1 Add the remaining GeoJSON countries (`adm1`, enabled) and the 9 deferred countries (`adm0`) to the manifest
- [ ] 4.2 Load on staging; `/health` `boundaries.failed` empty; peak memory (including the archive) and time per country from a committed `scripts/bench-boundary-load/` script; longest per-country transaction under 2.5 minutes
- [ ] 4.3 Re-run the equivalence script after all countries load; list and explain every event that changed country
- [ ] 4.4 Spot-check fixture: one coordinate per enabled country, expected names from ISO 3166-2 with aliases
- [ ] 4.5 Name overrides for source errors found by the spot check (at least ZAF "Nothern Cape")

## 5. Ingestion (PR: continental ingestion)

- [ ] 5.1 Country list from enabled `adm1` manifest entries whose ADM1 rows are loaded
- [ ] 5.2 `cmd/ingest` uses the scheduler's run function (lease and pacing)
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
