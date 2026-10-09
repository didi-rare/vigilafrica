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
- [x] 2.6 **First CI run (`baaa48b`) failed 6 of 12 pairs, and it was the
      test's defect, not the trigger's.** Discovery found 119 adjacent pairs,
      all 119 real two-candidate contests. The failures all had the trigger
      naming the *larger* state. Cause: `advUpsertPoint` built the GeoJSON
      with `%f`, which rounds to six decimals; a vertex shared by two rings,
      moved by up to half a microdegree, lands inside one polygon's interior,
      so the trigger evaluated a one-candidate point while the oracle used
      the exact vertex — and they disagreed precisely when the rounding fell
      into the larger polygon. Fixed with `strconv.FormatFloat(v, 'f', -1,
      64)` (proven to round-trip bit-exact where `%f` does not), and a new
      precondition asserts the **stored** event geometry still intersects ≥2
      ADM1 polygons, so a precision loss fails with its cause named instead
      of as a "wrong" label.
- [ ] 2.5 CI: all twelve subtests pass — **pending the re-run**

## 3. Group 2 — exact-area tie-break (`TestEnrichment_EqualAreaTieBreaksOnLowestID`)

- [x] 3.1 Two `ZZ`/`Testland` ADM1 rows, identical `ST_MakeEnvelope` geometry at
      (−30, −30); precondition asserts `count(DISTINCT area_m2) = 1`.
- [x] 3.2 Two subtests with opposite insertion order; each expects the
      first-inserted (lowest `id`) name. Reset between subtests.
- [x] 3.3 CI (`baaa48b`): both subtests pass — `alpha inserted first wins`
      and `beta inserted first wins` both green, so the `id` terminator is
      what decides the tie.

## 4. Group 3 — geometry update (`TestEnrichment_GeometryUpdateRelabelsAndClears`)

- [x] 4.1 Lagos → Kano → Cameroon → ocean → Lagos through `UpsertEvent`; Kano
      constant (8.5167, 12.0022) checked against the Kano ADM1 polygon as a
      precondition.
- [x] 4.2 The Cameroon step asserts `state_name` is NULL (stale "Kano" cleared).
- [x] 4.3 `UpdateEventMetadata` with a changed title asserts labels untouched
      (`UPDATE OF geom` does not fire on a title change).
- [x] 4.4 CI (`baaa48b`): all six steps pass, including the Cameroon step's
      NULL `state_name` and the metadata-only update leaving labels alone.

## 5. Group 4 — exterior ring (`TestEnrichment_NigeriaExteriorRingAlwaysLabelled`)

- [x] 5.1 Largest polygon of the NG ADM0 multipolygon (it is `ST_Union` of the
      states, so the ring has many vertices), `ST_DumpPoints` of its exterior
      ring, even sampling to ≤ 200 probes, assert ≥ 30 vertices exist.
- [x] 5.2 Raw inserts (the trigger is what is under test; path is irrelevant),
      every NOT NULL column supplied (`source_id`, `source`, `title`,
      `category`, `status`); assert zero rows with NULL `country_name`, with up
      to five offending coordinates in the failure message.
- [x] 5.3 CI (`baaa48b`): passes — the ring has **6,253** vertices (the
      reviewer's 37 was a far simpler boundary), 196 probed at step 32, zero
      with a NULL country.

## 6. Cleanup

- [x] 6.1 `advCleanup` registered per test function: deletes `ADV\_%` events and
      `ZZ` boundaries.
- [x] 6.2 `TestEnrichment_ZZCleanupLeftNothing` runs last in the file and asserts
      both counts are 0.
- [x] 6.3 CI (`baaa48b`): passes — zero `ADV_` events and zero `ZZ`
      boundaries left after the four groups, including after group 1's
      failures, which is the case per-function cleanup exists for.

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

## 7a. CI was red before the tests could run — fix ported into this PR

- [x] 7a.1 First CI run on the PR (`b3db8cf`) failed in **Run Go Vulnerability
      Check**, before the integration step: govulncheck under the pinned Go
      1.26.6 reports 11 stdlib advisories reachable from our code
      (GO-2026-6599…6611, published 2026-10-08). `build-and-test` is red on
      `development`'s own head (`8e88161`) with the same failure — **not this
      PR's**, and the integration tests this PR exists to run never executed.
- [x] 7a.2 Same shape as the 2026-08 batch, fixed the same way as `00a673c`:
      every one of the 11 is "Fixed in 1.26.9" (read from the `x/vulndb` OSV
      records via the Go proxy, since vuln.go.dev is blocked from this
      session). Bumped the three pins together: `ci-cd.yml` and
      `openspec-verify.yml` `go-version` 1.26.6 → 1.26.9, `api/Dockerfile`
      `GO_IMAGE` digest → `c95332c2…`, the `golang:1.26-alpine` index as of
      2026-10-08 18:04 UTC. Verified by content via the Docker Hub tags API:
      that index and the `1.26.9-alpine` index (`cdfd4fe2…`) reference the
      **same** amd64 image manifest, `397ecc64…`, and the same arm64 one, so
      the pinned bytes are the 1.26.9 build whichever tag names them.
      ⚠️ The previous bump additionally read `GOLANG_VERSION` from the image
      config blob; that read was attempted four times here and rate-limited
      (HTTP 429, anonymous registry pulls) every time. Not completed — the
      shared-manifest evidence above is what this pin rests on.
- [x] 7a.3 Local, under `GOTOOLCHAIN=go1.26.9` (go1.26.9 linux/amd64):
      `go vet ./...` clean, `go vet -tags=integration ./internal/database/`
      clean, `go test -race ./...` green across all packages, `go mod tidy`
      leaves go.mod/go.sum unchanged (CI's tidy-diff step will agree);
      `scripts/check-image-pins.js` passes on the new digest.
- [x] 7a.4 govulncheck could not be re-run here (vulnerability DB host
      blocked); CI's step on `baaa48b` is the proof: **"No vulnerabilities
      found."** The unit suite, tidy-diff, image-pin and deploy-wiring steps
      all passed on the new toolchain, and the integration step ran for the
      first time on this PR.

## 8. Records

- [x] 8.1 Register B2 marked closed with a pointer to the file
- [ ] 8.2 Proposal verification boxes ticked from the CI log, status
      `in-progress` → ready for `/openspec-review`
