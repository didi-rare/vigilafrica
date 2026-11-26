# Change: Continental Coverage, First Release (feature-continental-coverage)

**Status:** Proposed (9/62 tasks: decisions and two reviews done). Phase 1 of [`feature-place-first-direction`](../../proposals/feature-place-first-direction.md).
**Scoping record:** [`openspec/proposals/feature-continental-coverage.md`](../../proposals/feature-continental-coverage.md) holds the measurements, the HDX survey and the two blocking decisions. This change record turns them into buildable work.

## Why

VigilAfrica names events in two countries. ADR-017 (2026-11-19) makes the product continental, and the scoping proposal measured why it matters: roughly twelve times as many African floods are in the feed as in Nigeria and Ghana, and floods continue somewhere on the continent in every month on record. Continental coverage is what changes what a visitor sees, so it is built before the validation gate (Phase 2 of the direction).

## What changes

1. **One checked-in country manifest becomes the only list of covered countries.** Today the list exists four times: `DefaultCountries` in the ingester, `countryCodeToName` in the API, `SUPPORTED_COUNTRIES`/`COUNTRY_CENTERS` in the web app, and enums in the OpenAPI contract. All four are replaced by the manifest (decided 2026-11-19: Decision 1, a seed loader driven by a checked-in manifest with checksums).
2. **A boundary loader** downloads each country's HDX COD-AB zip, which holds every admin level, extracts the ADM1 file, refuses it if its checksum differs, and in one transaction replaces the country's boundaries, re-names its stored events and records source and vintage. A country whose manifest entry is unchanged is skipped, so a deploy downloads nothing unless the manifest changed. The loader runs at the start of every scheduler run, so a start that finds the lock held only delays it by one tick.
3. **Ingestion polls one box per country, paced** (decided 2026-11-19: Decision 2), with the scheduler lease renewed between countries and the GDACS budget made per country and sized from measurement.
4. **Per-country failure stops being whole-service failure.** `/ready` currently answers 503 when any one country's last run failed; at 45 countries one feed hiccup would fail the deploy gate. Alerts become at most two emails per run (failed, degraded) instead of one per country.
5. **Every stored event is named by a country.** The 9 deferred countries get country outlines, and events whose geometry touches no covered country are no longer stored. An enabled country whose boundaries have never loaded is not ingested until they load.
6. **The API and web app become continental:** `/v1/countries`, a searchable country picker, a state picker that waits for a country, a continental default map view, visible boundary provenance, and hazard-neutral copy. `/v1/states` stops accepting a missing country one release after the web change, because the web app and the API deploy separately.

## Scope

- **First release: the countries whose HDX COD-AB boundaries are published as GeoJSON, up to 45, South Africa included.** South Africa's 89 MB is its whole zip; its ADM1 file is 20 MB.
- **Deferred to the shapefile tranche for named states:** the 9 shapefile-only countries (BWA, DJI, GAB, LSO, MDG, MUS, RWA, SYC, TZA), including Madagascar (decided 2026-11-19). They get country outlines in this release.
- Boundaries load at full resolution; no simplification (scoping proposal, open question 2).

## Decisions

Made by the maintainer, each as recommended.

| # | Question | Decision |
|---|---|---|
| P1 | Where does the loader run? | 2026-11-25: inside the API, in the scheduler's startup goroutine before the first ingestion, under the existing `scheduler_locks` lease, with an off-switch for demo, dev and CI. Boot never waits on HDX; no host change. Amended 2026-11-26 after the second review: it runs at the start of every locked run, not only at startup, because a startup run that finds the lease held is skipped. |
| P2 | When does `/ready` answer 503? | 2026-11-25: only when every country's last run failed, or a run query failed. A single country's failure shows as `degraded` in `/health`. |
| P3 | South Africa in the first release? | 2026-11-25, revised the same day after review: yes. Its ADM1 file is 20 MB, not 89 MB. |
| P4 | The legacy `api/cmd/seed` binary? | 2026-11-25: delete it. It loads simplified Nigeria under the code `NGA`; stored rows use `NG`. |
| P5 | Boxes for Nigeria and Ghana | 2026-11-25: use the same rule as every country (file extent plus 0.1°). Nigeria's box narrows by about half a degree on the west; that strip is outside Nigeria's file, so events there are fetched through the neighbours' boxes. |
| P6 | Events outside every covered country | 2026-11-25: load country outlines for the 9 deferred countries, and stop storing events that match no African country. This also drops events in territory no file covers (Western Sahara), points at sea and slivers between states; each is counted before the cleanup. |
| P7 | A source file that no longer matches its checksum | 2026-11-25: fail that country loudly and fix it by updating the manifest after review. No self-hosted mirror. |

## Not in this change

- The near-me panel and the 30-day display window (Phase 3 of the direction).
- New hazard categories (D3), changes to the digest's content or shape (D4), simplification, ADM2.
- Named states for the 9 shapefile-only countries.

## Risks

- **HDX availability and refreshes.** Boot does not depend on HDX (P1), unchanged countries are skipped, and checksums are taken on the extracted ADM1 file so a refresh of other levels does not break loading. A changed ADM1 file fails loudly (P7); a fresh database cannot load that country until the manifest is updated, and does not ingest it meanwhile. A failed country is retried once a day.
- **GDACS budget at 45 countries.** One per-run deadline would starve late countries; the budget becomes per country, and the share of degraded runs over 24 hours is an acceptance criterion.
- **The lease.** A long country load could outlive the 5-minute lease. Each load transaction has a statement timeout under the TTL, and a failed renewal stops the run.
- **Deploy order.** The web app (Vercel) and the API (SSH) deploy separately, so `/v1/states` changes in two steps (spec §4).
- **Events changing country.** The smallest-area match now competes across borders from different sources. The before/after script runs again after all countries load and every change is explained.
- **Source name errors.** HDX names contain mistakes (South Africa's "Nothern Cape"); the manifest carries name overrides and the spot check uses ISO 3166-2 names, an independent source.
- **The digest widens.** It has no country filter, so it covers every covered country with no code change. That follows D4 (the endpoint is left alone); the partner page says so.
- **Stale-open wildfires multiply.** The list is newest first so they sink; the display window arrives in Phase 3. Their count on the first page is recorded after release.
