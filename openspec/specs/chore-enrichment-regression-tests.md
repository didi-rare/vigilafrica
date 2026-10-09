---
id: chore-enrichment-regression-tests
status: in-progress
proposal: ../proposals/chore-enrichment-regression-tests.md
branch: claude/chore-web-audit-leftovers-xt5v6i
---

# Spec: Adversarial Enrichment Regression Tests

Technical spec for [`chore-enrichment-regression-tests`](../proposals/chore-enrichment-regression-tests.md).
Tests only; no production code changes.

## Components touched

| file | change |
|---|---|
| `api/internal/database/enrichment_adversarial_test.go` | **new** — four test functions, `//go:build integration`, `package database_test` |
| `openspec/proposals/chore-deferred-work-register.md` | B2 marked closed with a pointer to the file |
| `Task.md` | task list with evidence |

Nothing under `api/db/migrations/` or the non-test files in `api/internal/database/` changes.

## The semantics under test (from `000013_precompute_boundary_area.up.sql`)

```sql
SELECT adm_name, country_name INTO NEW.state_name, NEW.country_name
FROM admin_boundaries WHERE adm_level = 1 AND ST_Intersects(NEW.geom, geom)
ORDER BY area_m2 ASC, id ASC LIMIT 1;
IF NEW.country_name IS NULL THEN
  SELECT country_name INTO NEW.country_name
  FROM admin_boundaries WHERE adm_level = 0 AND ST_Intersects(NEW.geom, geom)
  ORDER BY area_m2 ASC, id ASC LIMIT 1;
END IF;
-- trigger: BEFORE INSERT OR UPDATE OF geom
```

Three facts the tests must pin, each with the mutation that would break it:

| fact | breaking mutation | group that catches it |
|---|---|---|
| smallest intersecting ADM1 wins | `ASC` → `DESC`, or `area_m2` → anything not area-ordered | 1 (shared border) |
| `id` makes the order total | drop `, id ASC` | 2 (exact tie) |
| a move re-labels, and clears what no longer fits | `UPDATE OF geom` → `INSERT` only; or `SELECT … INTO` replaced by a conditional assignment that skips NULLs | 3 (geometry update) |
| ADM0 fallback catches every border point | drop the fallback block | 4 (exterior ring) and the existing six-point test |

## Harness

- `testRepo` (shared `database.Repository`) for everything that goes through the real repository
  path: `UpsertEvent`, `UpdateEventMetadata`, `ListEvents`.
- A raw `pgx.Connect(ctx, testDSN)` for the three things the repository does not expose and should
  not: discovering adjacent boundary pairs, inserting synthetic boundaries, and computing the
  independent oracle (`ST_Area(geom::geography)`, which is deliberately *not* the stored `area_m2`
  column the trigger reads — if the generated column ever drifted, this cross-check would show it).
  Same pattern as `migration_transposition_test.go`.
- Helpers reused from `enrichment_test.go`: `findEventBySourceID`, `assertOptString`, `ptrStr`,
  `ptrF64`. New helpers stay unexported in the new file.
- Isolation: the suite shares one database and never truncates. Every event this file inserts
  carries a `source_id` prefixed `ADV_`, every synthetic boundary carries `country_code = 'ZZ'`,
  and `t.Cleanup` deletes both by those keys. Synthetic geometry sits at lon −30, lat −30 (South
  Atlantic), where no real boundary and no other test's event can be.

## Group 1 — shared-border vertices (`TestEnrichment_SharedBorderPicksSmallerState`)

Discovery query (run once, result is the table). Pairs are same-country ADM1 polygons that
`ST_Intersects`, and the probe point is the first point of the intersection of their boundaries:

```sql
WITH pairs AS (
  SELECT a.id AS aid, b.id AS bid, a.adm_name AS a_name, b.adm_name AS b_name, a.country_name,
         (ST_DumpPoints(ST_Intersection(ST_Boundary(a.geom), ST_Boundary(b.geom)))).geom AS p
  FROM admin_boundaries a JOIN admin_boundaries b
    ON a.adm_level = 1 AND b.adm_level = 1 AND a.country_code = b.country_code AND a.id < b.id
   AND ST_Intersects(a.geom, b.geom)
), first_point AS (
  SELECT DISTINCT ON (aid, bid) aid, bid, a_name, b_name, country_name, p FROM pairs ORDER BY aid, bid
)
SELECT a_name, b_name, country_name, ST_X(p), ST_Y(p),
       (SELECT count(*) FROM admin_boundaries c WHERE c.adm_level = 1 AND ST_Intersects(p, c.geom)) AS candidates
FROM first_point ORDER BY aid, bid;
```

⚠️ Changed from the first draft, which used `ST_Touches`: real HDX polygons may overlap or gap by
floating-point amounts, and `ST_Touches` is false the moment two interiors meet, so a fixture
that is topologically "adjacent" to a human could yield zero pairs. `ST_Intersects` catches
touching and overlapping alike; a point where the two boundaries meet lies on both rings either
way. `ST_DumpPoints` rather than `ST_PointN` because the intersection may be a point, a line or a
collection depending on the pair.

The two-candidate precondition is computed **in the query** (`candidates`), and pairs whose point
PostGIS does not consider inside at least two ADM1 polygons are excluded rather than asserted on.
What is asserted is that **≥ 8** real contests exist; fewer means the fixture or the query changed,
and the test fails rather than skips. The first 12 contests run, deterministically ordered. For
each:

1. Oracle: the `adm_name` with the smallest `ST_Area(geom::geography)` among the intersecting
   ADM1 polygons, with `id` as a secondary key. Tripoints (a third, smaller state also touching
   the point) are logged, not special-cased: the oracle names the real smallest candidate.
2. Insert an event at the point through `UpsertEvent`; assert `state_name` equals the oracle and
   `country_name` equals the pair's country.
3. Upsert the same event again unchanged; assert the label is unchanged (stability).

The test name includes both state names.

## Group 2 — exact-area tie-break (`TestEnrichment_EqualAreaTieBreaksOnLowestID`)

Two runs, as subtests, each inserting two synthetic ADM1 rows with **identical** geometry:

```sql
INSERT INTO admin_boundaries (country_code, country_name, adm_level, adm_name, geom)
VALUES ('ZZ', 'Testland', 1, $1, ST_Multi(ST_SetSRID(ST_MakeEnvelope(-30.5, -30.5, -29.5, -29.5), 4326)))
RETURNING id;
```

- Subtest "alpha inserted first": insert `Tie Alpha` then `Tie Beta`; event at (−30, −30) must
  resolve to `Tie Alpha` / `Testland`.
- Subtest "beta inserted first": clean up, insert `Tie Beta` then `Tie Alpha`; the same point must
  now resolve to `Tie Beta`.

Identical geometry gives identical `area_m2` to the bit, so `area_m2 ASC` is a genuine tie and only
`id ASC` decides. The two subtests together prove the decider is `id` and not insertion luck,
name order, or plan order. Each subtest asserts `area_m2` of the two rows is exactly equal before
inserting the event, so the tie precondition is checked, not assumed.

An ADM0 row for `ZZ` is **not** inserted: the point matches ADM1 directly, and keeping the synthetic
footprint to two rows limits what cleanup can miss.

## Group 3 — geometry update (`TestEnrichment_GeometryUpdateRelabelsAndClears`)

One event, `ADV_MOVER`, driven through the real repository:

| step | via | geom | expect `state_name` | expect `country_name` |
|---|---|---|---|---|
| insert | `UpsertEvent` | Lagos (3.3941795, 6.4550575) | Lagos | Nigeria |
| move | `UpsertEvent` | Kano interior (8.5167, 12.0) | Kano | Nigeria |
| move | `UpsertEvent` | Cameroon interior (11.601622, 5.707452) | **NULL** | Cameroon |
| move | `UpsertEvent` | open ocean (0, 0) | NULL | NULL |
| move back | `UpsertEvent` | Lagos | Lagos | Nigeria |
| title only | `UpdateEventMetadata` | unchanged | Lagos | Nigeria |

The Cameroon row is the one that matters: the trigger's `SELECT … INTO` must set `state_name` to
NULL when the ADM1 query returns no row (plpgsql semantics), and a rewrite that assigned only on
match would leave "Kano" on an event in Cameroon. The last row pins that a metadata-only update does
not re-run enrichment (the trigger is `UPDATE OF geom`); it uses the exported
`UpdateEventMetadata` with a changed title and asserts the labels are untouched. The Kano
coordinate is checked against the fixture by a precondition query (`ST_Intersects` with the Kano
ADM1 row) so a wrong constant fails loudly rather than mislabelling the expectation.

## Group 4 — exterior-ring safety net (`TestEnrichment_NigeriaExteriorRingAlwaysLabelled`)

```sql
SELECT (dp).path[2] AS n, ST_X((dp).geom), ST_Y((dp).geom)
FROM (SELECT ST_DumpPoints(ST_ExteriorRing(ST_GeometryN(geom, 1))) AS dp
      FROM admin_boundaries WHERE country_code = 'NG' AND adm_level = 0) q;
```

The NG ADM0 row is `ST_Multi(ST_Union(geom))` of the states (migration `000010`), so its exterior
ring has far more than the 37 vertices the reviewer's hand-built suite walked; the test takes the
largest polygon of the multipolygon, samples its ring evenly to at most 200 probes, and asserts the
ring holds ≥ 30 vertices so a silently empty or degenerate ring cannot pass. For every probe, insert
an event and assert `country_name` is **non-NULL**. `state_name` is not asserted: ADM1 coverage at the national border is allowed to have
gaps, and that is precisely what the ADM0 fallback exists for. Events are inserted in one loop with
`source_id` `ADV_NGRING_<n>` and deleted together.

## Cleanup

```go
t.Cleanup(func() {
    _, _ = conn.Exec(ctx, `DELETE FROM events WHERE source_id LIKE 'ADV_%'`)
    _, _ = conn.Exec(ctx, `DELETE FROM admin_boundaries WHERE country_code = 'ZZ'`)
})
```

Registered in each test function (not once per file), so a single failing function still cleans
up after itself; §9.10 prefers `t.Cleanup` for exactly this.

## Standards

- §9.1 co-located `_test.go`, black-box `database_test` package.
- §9.2/§9.3 table-driven with descriptive `t.Run` names; §9.9 `tt := tt` capture.
- §9.4 stdlib `testing` only; `t.Fatalf` for preconditions and `t.Errorf` for assertions.
- §9.6/§9.7 real PostGIS via testcontainers, `//go:build integration`, run as the existing
  separate CI step.
- §9.11 no wall-clock, no network beyond the container the harness already starts.
- §5.3 every value passed as `$N` (the discovery queries take no user input, but the inserts do).

## Acceptance criteria

1. `go vet ./...` and `go test -race ./...` pass locally (the new file is excluded by its build tag,
   so this proves it does not break the unit build; `go vet -tags=integration ./internal/database/`
   additionally proves it compiles).
2. CI's "Run Database Integration Tests" step is green on the PR head and its `-v` log lists all
   four new test functions with their subtests.
3. Group 1 finds ≥ 8 adjacent pairs whose boundary point intersects ≥ 2 states (fewer means the
   discovery query or fixture changed — fail, do not skip), and runs the first 12.
4. Group 2's two subtests resolve to different winners.
5. Group 3 asserts all six rows, including the NULL `state_name` in Cameroon.
6. Group 4 covers ≥ 30 vertices with no NULL country.
7. After the suite, `SELECT count(*) FROM events WHERE source_id LIKE 'ADV_%'` and
   `… FROM admin_boundaries WHERE country_code = 'ZZ'` are both 0 — asserted by a final test that
   runs last in the file (Go runs tests in source order within a package), so cleanup is proven,
   not trusted.
8. Register B2 is marked closed with the file path; Task.md records the mutation check per the
   proposal.
