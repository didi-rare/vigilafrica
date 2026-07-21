# Spec: Label neighbour-country border events (fix-border-event-enrichment)

**Status:** Proposed — implementation in `fix/border-event-enrichment`.
**Companion:** [`openspec/proposals/fix-border-event-enrichment.md`](../proposals/fix-border-event-enrichment.md).

## Context

The enrichment trigger `trg_enrich_event_location` ([`api/db/migrations/000006_fix_enrichment_trigger.up.sql`](../../api/db/migrations/000006_fix_enrichment_trigger.up.sql)) matches only `admin_boundaries` rows with `adm_level = 1`, and that table (populated with real HDX polygons by `000010`) holds Nigeria + Ghana only. Events ingested from the neighbour-overhang of the NG/GH ingestion boxes therefore intersect no loaded state and land with `country_name` + `state_name` = NULL — 7 of 43 on staging, all border spillover (Cameroon/Benin/Niger).

## Requirement Delta

The `Requirement: Geospatial Event Enrichment` change lives in the change record, per the `feature-impact-categories` convention:
[`openspec/changes/fix-border-event-enrichment/specs/vigilafrica/spec.md`](../changes/fix-border-event-enrichment/specs/vigilafrica/spec.md). It merges into canonical `openspec/specs/vigilafrica/spec.md` at `/openspec-archive` time.

> **Sentinel gate note:** the gate (`api/cmd/sentinel`) recognises only `openspec/proposals/` and `openspec/changes/` among a PR's *changed* files. The integration test lands in `api/internal/database/` (a critical path), so the change record under `openspec/changes/` is what satisfies the gate.

## Components to Touch

### New files
- `api/db/migrations/000012_label_neighbour_countries.up.sql` / `.down.sql`.

### Modified files
- The enrichment trigger function `trg_enrich_event_location` — `CREATE OR REPLACE` inside `000012` to add the ADM0 fallback (the function is re-defined, not a source file).
- `api/internal/database/` integration test suite — new trigger cases.

No Go source, handler, or ingestor change: ingestion never reads `admin_boundaries` (verified). Labelling is entirely inside the Postgres BEFORE trigger.

## Design Decisions

- **ADM0 fallback, not ADM1-filter removal.** Keep ADM1-first (most specific state wins); only fall back to ADM0 country when no state matched. Preserves NG/GH state labelling exactly; adds country-only labelling for spillover.
- **Real geometry, not rectangles.** Live boundaries are real HDX polygons (`000010`); real country outlines don't overlap, so the fallback is unambiguous. Rectangles would overlap Nigeria's real ADM0 and mislabel.
- **country only; `state_name` stays NULL** for neighbours — honest, since no neighbour states are loaded.
- **Simplified ADM0** (`ST_SimplifyPreserveTopology ~0.01°`) — the fallback only sees points already outside every NG/GH state, so country-level precision suffices and keeps the migration small.
- **7 neighbours** — every country whose territory intersects the NG or GH box: BJ, NE, TD, CM, CI, BF, TG.

## Implementation Plan

Single migration `000012`, `up.sql` in order:
1. `CREATE OR REPLACE FUNCTION trg_enrich_event_location()` — ADM1 lookup (unchanged), then `IF NEW.country_name IS NULL` → ADM0 lookup for `country_name` only. Re-register the trigger (`BEFORE INSERT OR UPDATE OF geom`).
2. Insert 7 neighbour ADM0 rows (`adm_level = 0`, 2-letter code, `ST_Multi(ST_GeomFromGeoJSON(...))`), idempotent (`DELETE ... WHERE country_code IN (...)` then INSERT).
3. Backfill: `UPDATE events SET geom = geom WHERE geom IS NOT NULL;`.

`down.sql`: restore the `000006` function body, delete the 7 neighbour rows, re-run the backfill.

## Acceptance Criteria

- [ ] An event inside the NG/GH ingestion box but outside all loaded ADM1 states gets `country_name` from the intersecting national polygon; `state_name` remains NULL.
- [ ] The ADM0 fallback fires **only** when the ADM1 lookup returns no row — a point inside a real NG/GH state still resolves to that state (ADM1 wins).
- [ ] A point outside every loaded boundary leaves both fields NULL.
- [ ] The 7 currently-NULL staging rows carry `country_name` (Cameroon/Benin/Niger) after the migration's backfill; `state_name` still NULL.
- [ ] No NG/GH event is mislabelled to a neighbour.
- [ ] `admin_boundaries` gains exactly 7 `adm_level = 0` rows (BJ, NE, TD, CM, CI, BF, TG); ingestion behaviour is unchanged (no `DefaultCountries` edit).
- [ ] The migration is reversible (`down.sql` restores the ADM1-only trigger and removes neighbour rows).
- [ ] `scripts/test-api.ps1 -Integration` passes, including the new trigger cases.

## Verification Plan

1. Integration tests (real Postgres + PostGIS, `//go:build integration`, §9.6/§9.7): Cameroon point → Cameroon/NULL; Benin point → Benin/NULL; Lagos point → Nigeria/Lagos (regression, ADM1 wins); outside-all point → NULL/NULL.
2. Post-deploy on staging: `GET /v1/enrichment-stats` — the `null` group is replaced by Cameroon/Benin/Niger groups (country set; `success_rate_pct` still 0 because that metric is state-based — expected); NG + GH stay 100%.
3. `GET /v1/events?limit=200`: the 7 rows now carry `country_name`; `state_name` still null; 0 out-of-bbox; Lagos flood + NG/GH events unchanged.
