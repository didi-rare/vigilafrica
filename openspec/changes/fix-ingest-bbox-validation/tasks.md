# Tasks: fix-ingest-bbox-validation

Proposal: [`openspec/proposals/fix-ingest-bbox-validation.md`](../../proposals/fix-ingest-bbox-validation.md)
Spec: [`openspec/specs/fix-ingest-bbox-validation.md`](../../specs/fix-ingest-bbox-validation.md)

## Implementation

- [x] Add `withinBBox(bbox, lon, lat)` to `api/internal/ingestor/eonet.go` — inclusive bounds, no longitude sign assumptions (Ghana `min_lon = -3.5`).
- [x] Add the containment guard to the `runIngest` loop, between the empty-geometry skip and `repo.UpsertEvent`.
- [x] Skip out-of-bbox events via `continue` (not an error — an upstream leak is not a run failure); log at `Warn` with country/source_id/lon/lat.
- [x] Add `IngestResult.EventsSkippedBBox`, surfaced in the run-complete and run-failed logs.
- [x] Store events whose containment cannot be verified (Polygon → nil lon/lat) rather than dropping them; count them in `IngestResult.EventsUnverifiedGeom` and report once per run, with per-event detail at `Debug`.

## Tests

- [x] `TestWithinBBox` — table-driven: inside, all four out-of-box directions, real Florida coordinates, both inclusive corners, negative-longitude Ghana, and real Cameroon/Benin border coordinates that must NOT be rejected.
- [x] `TestRunIngest_SkipsEventOutsideCountryBBox` — asserts fetched 2 / stored 1 / skipped 1 and that only the in-bbox event was upserted.
- [x] `TestRunIngest_StoresEventWithUnverifiableGeometry` — polygon stored, counted, not a bbox skip.
- [x] Add `recordingRepo` so tests can assert *which* events were upserted.
- [x] Correct the `okBody` fixture, whose point `[0.0, 0.0]` sat outside the test bbox.
- [x] Full suite green via `scripts/test-api.ps1`; `go vet` clean.

## Deploy

- [ ] Merge to `development`.
- [ ] Propagate to `main` (staging), verify one ingest cycle skips `EONET_20263`.
- [ ] Propagate to `release` + version tag (API code — prod deploy is tag-gated).
- [ ] Run the generalised out-of-bbox `DELETE` once the guard is live in production (a `DELETE` before then is futile — re-ingested hourly).
- [ ] `/openspec-archive` — merges this delta into `openspec/specs/vigilafrica/spec.md`.

## Known limitation (follow-up)

- [ ] `EventsSkippedBBox` / `EventsUnverifiedGeom` are log-only. `CompleteIngestionRun` takes only `fetched`/`stored`, so the counters do not persist to `ingestion_runs` and do not appear in `/health`. Persisting them needs a migration plus a signature change — deliberately out of scope for this fix.
