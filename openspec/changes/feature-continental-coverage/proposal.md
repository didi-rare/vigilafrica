# Change: Continental Coverage, First Release (feature-continental-coverage)

**Status:** Proposed (4/47 tasks: decisions made). Phase 1 of [`feature-place-first-direction`](../../proposals/feature-place-first-direction.md).
**Scoping record:** [`openspec/proposals/feature-continental-coverage.md`](../../proposals/feature-continental-coverage.md) holds the measurements, the HDX survey and the two blocking decisions. This change record turns them into buildable work.

## Why

VigilAfrica names events in two countries. ADR-017 (2026-11-19) makes the product continental, and the scoping proposal measured why it matters: about twelve times as many African floods are in the feed as in Nigeria and Ghana, and floods continue somewhere on the continent in every month on record. Continental coverage is what changes what a visitor sees, so it is built before the validation gate (Phase 2 of the direction).

## What changes

1. **One checked-in country manifest becomes the only list of covered countries.** Today the list exists three times and is kept in sync by hand: `DefaultCountries` in the ingester, `countryCodeToName` in the API, and `SUPPORTED_COUNTRIES`/`COUNTRY_CENTERS` in the web app. All three are replaced by the manifest (decided 2026-11-19: Decision 1, a seed loader driven by a checked-in manifest with checksums).
2. **A boundary loader** reads the manifest, downloads each country's HDX COD-AB ADM1 file, refuses it if its checksum differs, replaces that country's boundaries in one transaction, records source and vintage, and re-names the country's stored events. Countries whose checksum has not changed are skipped, so a deploy downloads nothing unless the manifest changed.
3. **Ingestion polls one box per country, paced** (decided 2026-11-19: Decision 2), with the scheduler lock renewed during the run and the GDACS budget sized from measurement.
4. **Per-country failure stops being whole-service failure.** `/ready` currently answers 503 when any one country's last run failed; at 45 countries one feed hiccup would fail the deploy gate. Alerts are aggregated per run instead of one email per country.
5. **The API and web app become continental:** a `/v1/countries` endpoint, a searchable country picker, state names qualified by country, a continental default map view, visible boundary provenance, and hazard-neutral copy.

## Scope

- **First release: the countries whose HDX COD-AB ADM1 boundaries are published as GeoJSON — up to 45.** South Africa's 89 MB file is loaded last and only after a staging time and memory check (decision P3).
- **Deferred to the shapefile tranche:** the 9 shapefile-only countries (BWA, DJI, GAB, LSO, MDG, MUS, RWA, SYC, TZA), including Madagascar (decided 2026-11-19).
- Boundaries load at full resolution; no simplification (scoping proposal, open question 2).

## Decisions (made by the maintainer on 2026-11-25, each as recommended)

| # | Question | Decision |
|---|---|---|
| P1 | Where does the loader run? | Inside the API process at startup, after migrations, in the background under a Postgres advisory lock. Boot never waits on HDX, and no change to the host's deploy script is needed. |
| P2 | When does `/ready` answer 503? | Only when every country's last run failed, or the ingestion-run query failed. A single country's failure shows as `degraded` in `/health`. |
| P3 | South Africa in the first release? | Yes if its load passes a staging check (time and peak memory recorded on the PR); otherwise it moves to the shapefile tranche. |
| P4 | The legacy `api/cmd/seed` binary? | Delete it. It loads simplified geoBoundaries Nigeria under the code `NGA`, which would duplicate Nigeria's rows (stored as `NG`) if anyone ran it. |

## Not in this change

- The near-me panel and the 30-day display window (Phase 3 of the direction).
- New hazard categories (D3), digest changes (D4), simplification, ADM2.
- The 9 shapefile-only countries.

## Risks

- **HDX availability at load time.** Mitigated by P1 (boot does not depend on it) and by skipping unchanged countries.
- **GDACS budget exhaustion at 45 countries.** **Likely**, not measured: more countries means more flood polygons to resolve, and the budget (60 requests, 90 s per run) was sized for two. Exhaustion marks later countries `degraded`. The spec requires measuring it on staging before release.
- **Stale-open wildfires multiply.** The list is newest first so they sink; the display window arrives in Phase 3. Their count on the first page is recorded after release.
- **Country-name matching.** Event country names come from the manifest; IP geolocation names come from MaxMind. They must match for the Phase 3 country rung; this change adds the test table.
