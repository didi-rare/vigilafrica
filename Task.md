# chore-enrichment-regression-tests

**Branch:** `claude/chore-web-audit-leftovers-xt5v6i`
**Proposal:** [openspec/proposals/chore-enrichment-regression-tests.md](openspec/proposals/chore-enrichment-regression-tests.md)
**Spec:** [openspec/specs/chore-enrichment-regression-tests.md](openspec/specs/chore-enrichment-regression-tests.md)
**Origin:** deferred-work register item B2, carrying task 3.8 of
`2026-08-07-perf-boundary-area-precompute` (#211)

⚠️ **Docker is not available in the authoring session.** The integration
suite cannot run here; CI's "Run Database Integration Tests" step on the PR is
the execution. Everything below marked *reasoned* was checked against the SQL
by reading, not by running, and says so.

## 1. The file

- [x] 1.1 `api/internal/database/enrichment_adversarial_test.go`, `//go:build
      integration`, `package database_test`, reusing `testRepo`, `testDSN`,
      `assertOptString`, `ptrStr`, `ptrF64`. Raw `pgx` connection for
      discovery, synthetic rows and oracles (the `migration_transposition_test`
      pattern).
- [x] 1.2 `go vet ./...` clean; `go vet -tags=integration ./internal/database/`
      clean (proves the tagged file compiles); `go test -race ./...` green.
      `gofmt -l` reports only the pre-existing `enrichment_test.go`, untouched.

## 2. Group 1 — shared-border vertices (`TestEnrichment_SharedBorderPicksSmallerState`)

- [x] 2.1 Pairs discovered from the fixture (`ST_Intersects` on same-country
      ADM1 pairs, first point of the intersection of their boundaries), not
      hard-coded. Changed from the spec's `ST_Touches`: real HDX polygons may
      overlap or gap by floating-point amounts and `ST_Touches` is false the
      moment interiors meet; `ST_Intersects` plus the ≥2-candidate filter is the
      robust form of the same idea.
- [x] 2.2 The ≥2-candidate check is done in the discovery query and pairs that
      fail it are excluded; what is asserted is that **≥ 8** real contests
      exist (fail, not skip). Runs the first 12.
- [x] 2.3 Oracle is `ORDER BY ST_Area(geom::geography) ASC, id ASC` — the
      pre-000013 expression, independent of the stored `area_m2` the trigger
      reads. Tripoints are logged, not special-cased: the oracle names the real
      smallest candidate.
- [x] 2.4 Stability: re-upsert at the same point, same label.
- [ ] 2.5 CI: subtests listed in the `-v` log — **pending the PR run**

## 3. Group 2 — exact-area tie-break (`TestEnrichment_EqualAreaTieBreaksOnLowestID`)

- [x] 3.1 Two `ZZ`/`Testland` ADM1 rows, identical `ST_MakeEnvelope` geometry at
      (−30, −30); precondition asserts `count(DISTINCT area_m2) = 1`.
- [x] 3.2 Two subtests with opposite insertion order; each expects the
      first-inserted (lowest `id`) name. Reset between subtests.
- [ ] 3.3 CI: both subtests pass — **pending the PR run**

## 4. Group 3 — geometry update (`TestEnrichment_GeometryUpdateRelabelsAndClears`)

- [x] 4.1 Lagos → Kano → Cameroon → ocean → Lagos through `UpsertEvent`; Kano
      constant (8.5167, 12.0022) checked against the Kano ADM1 polygon as a
      precondition.
- [x] 4.2 The Cameroon step asserts `state_name` is NULL (stale "Kano" cleared).
- [x] 4.3 `UpdateEventMetadata` with a changed title asserts labels untouched
      (`UPDATE OF geom` does not fire on a title change).
- [ ] 4.4 CI: all six steps pass — **pending the PR run**

## 5. Group 4 — exterior ring (`TestEnrichment_NigeriaExteriorRingAlwaysLabelled`)

- [x] 5.1 Largest polygon of the NG ADM0 multipolygon (it is `ST_Union` of the
      states, so the ring has many vertices), `ST_DumpPoints` of its exterior
      ring, even sampling to ≤ 200 probes, assert ≥ 30 vertices exist.
- [x] 5.2 Raw inserts (the trigger is what is under test; path is irrelevant),
      every NOT NULL column supplied (`source_id`, `source`, `title`,
      `category`, `status`); assert zero rows with NULL `country_name`, with up
      to five offending coordinates in the failure message.
- [ ] 5.3 CI: passes — **pending the PR run**

## 6. Cleanup

- [x] 6.1 `advCleanup` registered per test function: deletes `ADV\_%` events and
      `ZZ` boundaries.
- [x] 6.2 `TestEnrichment_ZZCleanupLeftNothing` runs last in the file and asserts
      both counts are 0.
- [ ] 6.3 CI: passes — **pending the PR run**

## 7. Mutation check (reasoned against the SQL — Docker absent)

| mutation of `trg_enrich_event_location()` | group that fails | why |
|---|---|---|
| `ORDER BY area_m2 ASC` → `DESC` | 1 | the oracle still names the smallest; the trigger now returns the largest; any contest where the two differ fails, and all twelve do unless the candidates tie exactly, which real states never do |
| drop `, id ASC` | 2 | with equal `area_m2` the plan returns whichever row it visits first — the same row in both subtests, so one of the two opposite-order expectations fails; it cannot satisfy both |
| `BEFORE INSERT OR UPDATE OF geom` → `BEFORE INSERT` | 3 | the move to Kano leaves "Lagos" on the row; step 2 fails |
| assign `state_name` only when the ADM1 query matches | 3 | the move to Cameroon leaves "Kano"; step 3's NULL assertion fails |
| remove the ADM0 fallback block | 3 and 4 | Cameroon step gets NULL country; every border vertex outside ADM1 coverage gets NULL country |

Executed version of this table: anyone with Docker can replay a mutated
trigger with `psql -f` against the test container and run
`go test -tags=integration -run TestEnrichment_ ./internal/database/`.

## 8. Records

- [x] 8.1 Register B2 marked closed with a pointer to the file
- [ ] 8.2 Proposal verification boxes ticked from the CI log, status
      `in-progress` → ready for `/openspec-review`
