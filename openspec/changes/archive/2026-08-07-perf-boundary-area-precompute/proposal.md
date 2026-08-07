# Proposal: Precompute Boundary Area for Enrichment (perf-boundary-area-precompute)

**Status:** Implemented — work item 4 of [feature-continental-coverage](../../../proposals/feature-continental-coverage.md), extracted and shipped separately because it is independent of that proposal's two blocking decisions.

## Why

Both branches of `trg_enrich_event_location()` (migration `000012`) order candidate polygons with:

```sql
ORDER BY ST_Area(geom::geography) ASC
```

`ST_Area` is therefore recomputed, on the fly, for **every candidate row, on every event insert**. At today's 53 ADM1 polygons that is invisible. At the ~750 ADM1 units continental coverage implies, it becomes the dominant cost of ingestion.

The parent proposal identified this as *"the single highest-leverage change in this proposal, and it is nearly free."* It is also the only work item that depends on **neither** blocking decision (boundary loading strategy, bbox strategy), so it can ship now rather than waiting on them.

## What Changes

Migration `000013` adds a **generated** column and reorders both trigger branches by it:

```sql
ALTER TABLE admin_boundaries
    ADD COLUMN area_m2 double precision
    GENERATED ALWAYS AS (ST_Area(geom::geography)) STORED;
```

### Why generated, not a plain stored column

The parent proposal assumed a plain column. `GENERATED ALWAYS ... STORED` is strictly better and was verified to work on PostGIS 15-3.4: Postgres computes the value on insert and **recomputes it automatically whenever `geom` changes**, so it cannot drift out of sync with the geometry. A plain column would need either a second trigger on `admin_boundaries` or every loader remembering to set it — both are silent-corruption risks in a product whose entire proposition is admin-name-first.

Verified directly: inserting a boundary populates `area_m2` correctly, and updating its `geom` recomputes it.

## Measured, not assumed — and reproducible

Harness committed at [`scripts/bench-enrichment/bench.sql`](../../../../scripts/bench-enrichment/bench.sql). PostGIS 15-3.4, **795 ADM1-scale polygons** (the real 53 replicated and translated), **2,385 probe points**, on a database rebuilt from scratch through **all** migrations — 62 boundary rows (53 ADM1 + 9 ADM0), 804 rows once the fixture is loaded.

⚠️ **An earlier revision of this record reported 55 boundary rows and 797 fixture rows.** Those came from a local database where `000012`'s data section had never been applied, so the **7 neighbour ADM0 rows were missing** and the ADM0 fallback path was under-exercised. Caught by independent review; the figures above are from a fully migrated database.

It deliberately measures the **same workload two ways**, because they disagree by ~15% and the flattering one is easy to quote by accident:

| method | old | new | speedup |
|---|---|---|---|
| A — LATERAL lookup, isolating the `ORDER BY` | 3,607 ms | 243 ms | 14.8× |
| **B — real `INSERT`s through the real trigger** | **3,642 ms** | **277 ms** | **13.2×** |

**Quote B.** Method A omits plpgsql overhead and the ADM0 fallback branch, so it overstates the win.

The harness runs inside a **single transaction ending in `ROLLBACK`**. Method B must swap the trigger to its pre-`000013` form to time the old path, and an error in between would otherwise leave the database running the **old trigger** — a benchmark silently mutating what it exists to measure. Rollback also means the cleanup path is exercised on every run, not only on success, and the script asserts afterwards that no fixture rows survive and the `000013` trigger is in place. Raised by independent review.

### ⚠️ Correction: this proposal previously claimed 16.2×

That figure was a **method-A measurement quoted as though it were method B**, and it was optimistic even for method A. An **independent reviewer** measuring the same change on different hardware got **15.2× (A)** and **11.5× (B)**.

**The production-realistic figure is a range of roughly 11–13×, not a single number.** The win is large and unambiguous; the precision previously implied was not earned.

### Behaviour preservation was tested, not asserted

Identical counts can hide different assignments, so results were compared row by row:

- **2,385 / 2,385** probe points received an **identical** `(adm_name, country_name)` under both variants. **Zero differing.**
- `area_m2` equalled `ST_Area(geom::geography)` **exactly** for all 804 rows — max absolute difference **0**.
- Against real production data (55 boundaries, 43 events), re-enriching every event through the new trigger produced **43 / 43 identical** results, and again after a full down→up round-trip.
- **Independent review** additionally built a 48-point adversarial suite — real shared-border midpoints derived from `ST_Intersection` of adjacent state pairs, ADM0-fallback interiors in Benin/Chad, points matching nothing, and all 37 Nigerian exterior-ring vertices — and found **0/48 differences**, plus 0/2,385 at scale. It also ran the repo's **integration suite** (testcontainers + `golang-migrate` through `000013`), including `enrichment_test.go`, which passed.

## The ordering is now a total order — `id` as final tie-break

Both branches order by `area_m2 ASC, id ASC`, not `area_m2` alone.

**Area is not a total order.** Two intersecting polygons with exactly equal computed area have no defined relative order, so the row `LIMIT 1` returns may change when the sort expression or the query plan changes — which is precisely what this migration does. Without a tie-break, "enrichment resolves identically" was an assumption about the data, not a guarantee.

Demonstrated rather than argued: two overlapping polygons with byte-identical area (`11548563398.86789`), one point inside both, inserted five times — **all five resolved to the same boundary**.

⚠️ **This is a deliberate strengthening relative to `000012`, not a preservation of it.** For equal-area overlaps `000012`'s result was *undefined*; this one is defined. The down migration **keeps** the tie-break for the same reason — reverting the stored column is the point, reverting a correctness fix alongside it is not.

Raised by independent review, which correctly noted the spec delta promised a guarantee the SQL could not deliver.

## ⚠️ Operational: this migration rewrites the table

`ADD COLUMN ... GENERATED ALWAYS ... STORED` is **not** a metadata-only change. Postgres physically rewrites `admin_boundaries` under an **`ACCESS EXCLUSIVE`** lock.

Verified directly rather than assumed: `pg_class.relfilenode` changes across the `ALTER` (23400 → 23404), and `pg_locks` reports `AccessExclusiveLock` granted.

For that window **nothing** can read or write `admin_boundaries` — including the enrichment trigger firing on every `events` insert, so concurrent ingestion **blocks** rather than fails.

At today's 62 rows this is sub-millisecond and harmless. At the ~750-polygon continental scale this migration exists to serve, it measured **~1.3 s**. Anyone adding further generated or non-volatile-default columns to this table *after* continental boundaries land should expect a real outage window and schedule it against the ingestion cadence.

**This was missed in the first revision of this record and surfaced by independent review.**

## ⚠️ Correction to the parent proposal

Work item 4 says *"precompute area as a stored column **and index it**."* **The index half is wrong and is deliberately omitted.**

`EXPLAIN (ANALYZE, BUFFERS)` shows the planner satisfies `ST_Intersects` from the existing GIST index on `geom`, then sorts the handful of surviving candidates with an in-memory quicksort — it **never reads** a btree on `area_m2`. Measured with the index present: no faster. It would be pure write-amplification on every boundary load.

## Out of Scope

- Geometry simplification (parent proposal's separate work item; recommended tolerance `0.001°`).
- Widening `DefaultCountries`, the boundary loader, bbox/pacing strategy — all gated on the parent's two blocking decisions.
- Any change to the enrichment trigger's **matching semantics**. The `adm_level = 1` filter, the smallest-polygon tie-break and the ADM0 fallback are all preserved exactly; only how the area is obtained changes.

## Verification

- [x] Migration applies cleanly to a database holding real production boundary data
- [x] `area_m2` populated for all 55 boundaries; auto-populates on insert and recomputes on `geom` update
- [x] Enrichment output byte-identical on real data (43/43) and at continental scale (2,226/2,226)
- [x] Down migration restores the `000012` trigger, drops the column, and leaves enrichment working; up→down→up round-trips with identical results
- [x] Every existing `INSERT INTO admin_boundaries` uses an explicit column list, so the generated column breaks none of them (checked across migrations, `cmd/seed`, and `scripts/generate_boundary_migration.py`); no `SELECT *` on the table anywhere
- [x] Full API test suite passes
