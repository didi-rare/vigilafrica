# Tasks: fix-border-event-enrichment

Proposal: [`../../proposals/fix-border-event-enrichment.md`](../../proposals/fix-border-event-enrichment.md)
Spec: [`../../specs/fix-border-event-enrichment.md`](../../specs/fix-border-event-enrichment.md)

## Boundary data

- [ ] Fetch ADM0 GeoJSON for the 7 neighbours from geoBoundaries gbOpen (ISO3: BEN, NER, TCD, CMR, CIV, BFA, TGO).
- [ ] Simplify (`ST_SimplifyPreserveTopology ~0.01°`) and embed as `ST_Multi(ST_GeomFromGeoJSON(...))`; keep the migration small.

## Migration `000012_label_neighbour_countries`

- [ ] `up.sql` step 1: `CREATE OR REPLACE FUNCTION trg_enrich_event_location()` — ADM1 first, then ADM0 country fallback when `country_name IS NULL`. Re-register trigger `BEFORE INSERT OR UPDATE OF geom`.
- [ ] `up.sql` step 2: insert 7 neighbour ADM0 rows (`adm_level = 0`, 2-letter codes), idempotent (`DELETE ... WHERE country_code IN (...)` then INSERT).
- [ ] `up.sql` step 3: backfill `UPDATE events SET geom = geom WHERE geom IS NOT NULL;`.
- [ ] `down.sql`: restore the `000006` (ADM1-only) function body, delete the 7 rows, re-run the backfill.

## Tests

- [ ] Integration (real Postgres+PostGIS, `//go:build integration`): Cameroon → Cameroon/NULL; Benin → Benin/NULL; Lagos → Nigeria/Lagos (regression); outside-all → NULL/NULL.
- [ ] `scripts/test-api.ps1 -Integration` green.

## Ship

- [ ] PR to `development`; `/openspec-review`; merge.
- [ ] Propagate to `main` (staging); verify `/v1/enrichment-stats` (null group gone; NG/GH still 100%) and the 7 rows now carry `country_name`.
- [ ] Carry to `release` in the next production cut.
- [ ] `/openspec-archive fix-border-event-enrichment` once live — merges the delta into canonical spec.
