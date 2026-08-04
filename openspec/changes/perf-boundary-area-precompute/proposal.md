# Proposal: Precompute Boundary Area for Enrichment (perf-boundary-area-precompute)

**Status:** Implemented — work item 4 of [feature-continental-coverage](../../proposals/feature-continental-coverage.md), extracted and shipped separately because it is independent of that proposal's two blocking decisions.

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

## Measured, not assumed

PostGIS 15-3.4, **742 ADM1-scale polygons** (the real 53 production polygons replicated and translated — the parent proposal's method), **2,226 point lookups** using the production trigger's exact matching logic:

| variant | total | per lookup | |
|---|---|---|---|
| A — production today, `ORDER BY ST_Area(geom::geography)` | 5,520 ms | 2.48 ms | baseline |
| **B — `ORDER BY area_m2`** | **341 ms** | **0.153 ms** | **16.2×** |

Corroborates the parent proposal's independently-measured 18.5×.

### Behaviour preservation was tested, not asserted

Identical counts can hide different assignments, so results were compared row by row:

- **2,226 / 2,226** probe points received an **identical** `(adm_name, country_name)` under both variants. **Zero differing.**
- `area_m2` equalled `ST_Area(geom::geography)` **exactly** for all 742 polygons — max absolute difference **0**.
- Against real production data (55 boundaries, 43 events), re-enriching every event through the new trigger produced **43 / 43 identical** results, and again after a full down→up round-trip.

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
