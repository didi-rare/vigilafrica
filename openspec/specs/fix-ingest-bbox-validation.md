# Spec: Ingest Bounding-Box Containment Guard (fix-ingest-bbox-validation)

**Status:** Proposed — implementation in `fix/ingest-bbox-validation`.
**Companion:** [`openspec/proposals/fix-ingest-bbox-validation.md`](../proposals/fix-ingest-bbox-validation.md) (root-cause evidence, rationale, out-of-scope, origin).

## Context

A Wakulla, **Florida, USA** wildfire (`EONET_20263`, `30.05 / -84.57`) reached the
**production** database of an Africa-scoped product, surfaced on the public feed,
and was re-ingested every hour with an empty `country_name`.

Root cause, verified against the live EONET API: **EONET's server-side `bbox`
filter leaks.** Querying the Nigeria box returns EONET_20263 even though its only
geometry point is in Florida — reproduced with both the code's coordinate order
and EONET's documented order, so it is not an argument-order defect. The ingest
loop then upserts whatever EONET returns, performing **no client-side coordinate
validation**. The normalizer is correct; the bbox argument ordering is cosmetic.

This also violates an existing written requirement:
`openspec/specs/vigilafrica/spec.md` → `Requirement: Natural Event Ingestion`
already states events SHALL be *"filtered to the Nigeria bounding box"*. The
requirement assumed upstream filtering was authoritative. This change restores
compliance and sharpens the requirement to say containment is enforced locally.

## MODIFIED Requirements

> Delta only. Merged into `openspec/specs/vigilafrica/spec.md` at
> `/openspec-archive` time, per the `feature-impact-categories` convention — the
> canonical spec is not edited by this change.

### Requirement: Natural Event Ingestion

The system SHALL ingest natural event data from the NASA EONET API v3 on a
scheduled interval and persist enriched events in a PostgreSQL/PostGIS database.
Bounding-box containment SHALL be enforced **client-side** by the ingestor;
upstream `bbox` filtering is treated as a hint, not a guarantee.

#### Scenario: Event outside the country bounding box is rejected

- **WHEN** the upstream source returns an event whose resolved point falls outside the queried country's bounding box
- **THEN** the ingestor SHALL NOT persist that event
- **AND** it SHALL log the skip with the country, source_id, and coordinates
- **AND** it SHALL count the skip separately from other skip reasons

#### Scenario: Event with no resolvable point is not rejected on containment grounds

- **WHEN** an event's geometry yields no point coordinates (e.g. Polygon)
- **THEN** the ingestor SHALL persist it rather than drop unverifiable data
- **AND** it SHALL count such events and report the count once per run
- **AND** it SHALL emit per-event detail, including the geometry type, at Debug level

## Components to Touch

### New files

None. The guard is contained entirely within the existing `ingestor` package —
no reusable point-in-box helper exists anywhere in `api/` (verified), so one is
added locally rather than introducing a geo package for a single predicate.

### Modified files

1. `api/internal/ingestor/eonet.go`
   - New unexported `withinBBox(bbox [4]float64, lon, lat float64) bool`, placed
     beside `CountryConfig`/`DefaultCountries` whose `BBox` it consumes.
     Bounds **inclusive** (matching EONET's own `bbox` semantics); no sign
     assumptions — Ghana's `min_lon` is `-3.5`.
   - Containment guard in the `runIngest` loop, between the `geoJSON == ""` skip
     and `repo.UpsertEvent`, mirroring the existing skip pattern (`continue`, no
     error returned — an out-of-bbox event is not a run failure).
   - `IngestResult` gains `EventsSkippedBBox int` and `EventsUnverifiedGeom int`,
     both surfaced in the `ingestion: run complete` and `ingestion: run failed`
     log lines.
2. `api/internal/ingestor/eonet_test.go` — see Verification Plan.

**Coverage:** `runIngest` is the only path reaching `UpsertEvent`; both
`api/cmd/ingest/main.go` and `scheduler.go` funnel through `Ingest` → `runIngest`,
so no other call site requires a guard.

## Implementation Plan

1. Add `withinBBox` + the guard + the `EventsSkippedBBox` counter.
2. Correct the existing `okBody` test fixture, whose point `[0.0, 0.0]` sits
   **outside** `testCountry`'s bbox `[2.0, 4.0, 15.0, 14.0]`. No current test
   asserts `EventsStored`, so none break today — but the fixture is semantically
   wrong and would silently poison any future storage assertion.
3. Add a `recordingRepo` (embeds `*mockRepo`, records upserts) so tests can
   assert *which* events survived, not merely how many.
4. Add the tests in Verification Plan.

## Design Decisions

- **Unverifiable geometry is stored, not dropped.** The normalizer resolves
  `lon`/`lat` only for `Point` and leaves them nil for `Polygon`. Rejecting
  nil-coordinate events would silently discard legitimate flood polygons — a
  behaviour change well beyond this bug. They are stored **and counted**, because
  there is no evidence EONET leaks only Point geometries and a silent
  pass-through would recreate the original blind spot. The count is reported once
  per run rather than once per event: a per-event line would be noise at `Info`,
  and `Debug` is invisible in production (`LOG_LEVEL` defaults to `info`), which
  would defeat the purpose of recording it. Per-event detail remains at `Debug`
  for local diagnosis.
- **A dedicated counter, not the fetched−stored delta.** Three other `continue`
  paths (normalize failure, empty geometry, upsert failure) already consume that
  delta, so it cannot distinguish an upstream bbox leak from a broken normalizer
  or a down database.
- **Inclusive bounds**, matching EONET's `bbox` query semantics, so an event
  exactly on a border is ingested rather than lost.

## Acceptance Criteria

- [ ] An event whose resolved point lies outside the queried country's bbox is **not** upserted.
- [ ] The skip emits a `Warn` with `country`, `source_id`, `lon`, `lat`.
- [ ] `IngestResult.EventsSkippedBBox` counts such skips and appears in the run-complete log.
- [ ] An event with unresolvable coordinates (Polygon) **is** upserted, is counted in `EventsUnverifiedGeom`, and does not count as a bbox skip.
- [ ] The unverified-geometry count appears in the run-complete log; per-event `geom_type` detail is emitted at `Debug`.
- [ ] Boundary coordinates are treated as inside (inclusive).
- [ ] Negative longitudes (Ghana) are handled correctly.
- [ ] Legitimate cross-border events genuinely inside the queried box (Cameroon/Benin/Niger within Nigeria's bbox) are still ingested — the guard must not over-reject.
- [ ] No error is returned for a skipped event; the run still completes successfully.
- [ ] Existing ingestor tests pass unchanged.

## Verification Plan

1. **Unit — `TestWithinBBox`**: table-driven (§9.2/§9.3, `tt := tt` per §9.9) covering inside, west/east/north/south of box, the real Florida coordinates, both inclusive corners, negative-longitude Ghana cases, and real Cameroon/Benin border coordinates that fall inside Nigeria's bbox and must **not** be rejected (the guard drops out-of-box events, not out-of-country ones).
2. **Integration — `TestRunIngest_SkipsEventOutsideCountryBBox`**: httptest response containing one in-bbox Nigerian event plus the real `EONET_20263` Florida point. Asserts `EventsFetched == 2`, `EventsStored == 1`, `EventsSkippedBBox == 1`, and that only `EONET_NG_IN` was upserted.
3. **Integration — `TestRunIngest_StoresEventWithUnverifiableGeometry`**: a Polygon event outside the bbox is stored, counts as `EventsUnverifiedGeom == 1`, and does **not** count as a bbox skip.
4. Stdlib `testing` + `httptest` only (§9.4 — testify is not a dependency and would require an ADR; §9.11 — no wall-clock dependence).
5. Full suite via Docker: `scripts/test-api.ps1` (native `go test` is AppLocker-blocked on the maintainer's machine).
6. **Post-deploy (staging, then production):** watch one ingest cycle for `ingestion: skipping event outside country bbox` naming `EONET_20263` and a run-complete line with `events_skipped_bbox >= 1`; then confirm the out-of-bbox audit query returns zero rows:

```sql
SELECT count(*) FROM events WHERE NOT (
  (longitude BETWEEN 2.0 AND 15.0 AND latitude BETWEEN 4.0 AND 14.0) OR
  (longitude BETWEEN -3.5 AND 1.2 AND latitude BETWEEN 4.5 AND 11.2));
```

7. **One-time cleanup after the guard is live in production** — generalised rather than targeting the single known offender, since the audit may reveal other latent violations. Run the `SELECT count(*)` form first to see what would be removed:

```sql
DELETE FROM events WHERE NOT (
  (longitude BETWEEN 2.0 AND 15.0 AND latitude BETWEEN 4.0 AND 14.0) OR
  (longitude BETWEEN -3.5 AND 1.2 AND latitude BETWEEN 4.5 AND 11.2));
```
