---
id: chore-enrichment-regression-tests
status: in-progress
branch: claude/chore-web-audit-leftovers-xt5v6i
spec: ../specs/chore-enrichment-regression-tests.md
---

# Proposal: Codify the Adversarial Enrichment Cases as CI Regression Tests (chore-enrichment-regression-tests)

## Why

Every location label the product shows — "Lagos, Nigeria", "Cameroon", nothing — comes from one
trigger, `trg_enrich_event_location()` (migration `000013`). Its semantics are small and precise:

1. an event's `geom` is matched against ADM1 polygons; the **smallest intersecting polygon wins**,
   with `id` as the final tie-breaker so the ordering is total;
2. if no ADM1 matches, the **smallest intersecting ADM0 polygon** supplies the country and
   `state_name` is left NULL — we do not invent a state;
3. the trigger fires `BEFORE INSERT OR UPDATE OF geom`, so moving an event re-labels it, and a
   label from the old position must not survive a move it no longer fits.

When `000013` replaced the on-the-fly `ST_Area` with a stored column, the independent reviewer
proved semantics were preserved with a hand-built adversarial suite: real shared-border midpoints
from `ST_Intersection` of adjacent states, ADM0-fallback interiors, points matching nothing, and
all 37 vertices of Nigeria's exterior ring — **0/48 differences**. That suite was run once, by hand,
and discarded. The archived change's task 3.8 and the deferred-work register's **B2** both say the
same thing: none of the tie-break, shared-border or geometry-update cases are covered by CI, so a
future change to the matching logic would not be caught. The register calls it the highest-value
item in its section, "because it protects the enrichment correctness that the whole product's
location labelling rests on."

What CI covers today is `TestEnrichmentTrigger_ADM0Fallback`: six fixed points (three neighbour
interiors, Lagos, Borno, open ocean). None sits on a border, none exercises the tie-break, and
none moves. A rewrite of the trigger that dropped the `id` tie-breaker, that preferred the
*largest* polygon, or that left a stale `state_name` behind on a geometry update would pass it.

## What Changes

One new integration test file, `api/internal/database/enrichment_adversarial_test.go`, in the
existing `database_test` package under the existing `//go:build integration` tag, using the
existing testcontainers PostGIS harness. Four groups:

1. **Shared-border vertices.** For a sample of adjacent ADM1 pairs — found by the test itself with
   `ST_Touches`, not hard-coded — take a vertex that both rings share, insert an event there, and
   assert: both states intersect the point (so the case is a real contest, not a near-miss), the
   assigned state is the **smaller** one by `ST_Area(geom::geography)` computed independently of the
   trigger's stored column, and the assignment is stable across repeated inserts.
2. **Exact-area tie-break.** Two synthetic ADM1 polygons with byte-identical geometry (so
   `area_m2` is equal to the last bit) in a synthetic country placed in open ocean, inserted in a
   known order. The event at their shared interior must resolve to the **lower `id`**. Inserting
   them in the opposite order must flip the winner. This is the only way to prove the `id`
   terminator is load-bearing; real boundaries never tie.
3. **Geometry update.** One event upserted through the real `UpsertEvent` path at Lagos, then moved
   to Kano, then to a Cameroon interior, then to open ocean. Each move must re-label exactly:
   Kano/Nigeria; NULL/Cameroon (the stale "Kano" must be cleared, not retained); NULL/NULL. Then a
   metadata-only update through `UpdateEventMetadata` must **not** touch the labels, because the
   trigger is `UPDATE OF geom` and a title change is not a move.
4. **Exterior-ring safety net.** Every vertex of Nigeria's ADM0 exterior ring resolves to a
   non-NULL country. These are the hardest points the ADM0 fallback has to catch; the reviewer's
   suite walked all 37 by hand, and this walks however many there are now.

Each group is table-driven, uses `t.Run` names that say what the case is, and leaves nothing
behind: synthetic boundaries and all inserted events are removed in `t.Cleanup`, because the suite
shares one database and never truncates (the pagination tests document the same constraint).

## Not changing

- The trigger, the migrations, the repository. This change adds tests only. If a test fails
  today, that is a finding to report, not a test to adjust — the semantics above are the ones
  `000013` states in its own header.
- The six-point `TestEnrichmentTrigger_ADM0Fallback` stays as it is; the new file complements it.

## Impact

- **Affected:** one new test file under `api/internal/database/`; the sentinel gate applies and
  this proposal is the record.
- **CI time:** a few dozen extra inserts against an already-running container; negligible.
- **Risk:** none at runtime. The one real risk is a test that is flaky on topology — a "shared"
  vertex that PostGIS does not consider inside both rings. The test guards against that by asserting
  the two-candidate precondition explicitly and failing loudly if the data no longer supports it,
  rather than skipping.

## Constraint of the authoring environment

Docker is not available in the session writing these tests, so they cannot be run locally here.
CI runs `go test -tags=integration ./internal/database/` on every push, and that run is the
verification. The spec's acceptance criteria are written so that the CI log is the evidence, and a
locally-reproducible path (`scripts/test-api.ps1 -Integration`, or `go test -tags=integration` on
any machine with Docker) is named for anyone who wants to run them by hand.

## Verification

- [x] `go vet ./...` and `go test -race ./...` clean (unit suite, run here under both go1.26.0 and
      go1.26.9); `go vet -tags=integration ./internal/database/` clean
- [x] `go test -tags=integration ./internal/database/` green in CI on the PR head (`eac10d9`),
      with all four new groups and the cleanup proof listed in the `-v` output: 12/12 shared-border
      pairs (119 contests discovered), both tie-break orders, all six geometry-update steps, 196 of
      the ring's 6,253 vertices with zero NULL countries, zero leaked fixtures
- [x] Mutation check, recorded in `Task.md` §7 — **reasoned against the SQL, not executed**: Docker
      is absent from the authoring session and the record says so
- [x] Register item B2 closed with a pointer to the test file

## What the first real run taught

The first CI run that reached the integration step (`baaa48b`) failed 6 of the 12 shared-border
pairs, and the trigger was right every time: the test built the event's GeoJSON with `%f`, which
rounds to six decimals, so a vertex shared by two rings moved by up to half a microdegree into one
polygon's interior. The trigger then evaluated a one-candidate point while the oracle used the
exact vertex, and they disagreed exactly when the rounding fell into the larger polygon. Fixed with
full-precision formatting, plus a precondition that the **stored** geometry still intersects two
states, so the same class of fault would now fail with its cause named. Recorded because the
reviewer's original hand-built suite, run through an interactive SQL session, could never have hit
this: it is a defect of the *Go-to-GeoJSON* path, and only exists once the probe goes through the
real repository.

Before any of that could run, CI was red on `development` itself: Go 1.26.9 shipped 11 standard-library
advisories reachable from our code and govulncheck under the pinned 1.26.6 failed every branch. The
pin bump (`baaa48b`) is carried in this PR, the same way `00a673c` carried the previous one.

## Origin

Deferred-work register item **B2**, which carries forward task 3.8 of
[`2026-08-07-perf-boundary-area-precompute`](../changes/archive/2026-08-07-perf-boundary-area-precompute/tasks.md)
(#211). Picked up 2026-10-09 after the web-audit leftovers closed (#283).
