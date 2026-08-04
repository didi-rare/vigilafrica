# Tasks: Precompute Boundary Area for Enrichment

## 1. Migration

- [x] 1.1 Add `000013_precompute_boundary_area.up.sql` with a `GENERATED ALWAYS ... STORED` `area_m2` column on `admin_boundaries`
- [x] 1.2 Reorder **both** trigger branches (ADM1 match and ADM0 fallback) by `area_m2`
- [x] 1.3 Preserve matching semantics exactly — `adm_level` filter, smallest-polygon tie-break, ADM0 fallback, `state_name` left NULL on fallback
- [x] 1.4 Add the down migration, restoring the `000012` trigger **before** dropping the column so no intermediate state is broken
- [x] 1.5 Deliberately omit a btree index on `area_m2` — `EXPLAIN` shows it is never used

## 2. Measurement

- [x] 2.1 Build a 742-polygon continental-scale fixture from real production geometry
- [x] 2.2 Benchmark variant A (current) vs B (stored area) over 2,226 lookups — **5,520 ms → 341 ms, 16.2×**
- [x] 2.3 Confirm the btree index on `area_m2` provides no benefit, via timing and `EXPLAIN ANALYZE`

## 3. Correctness

- [x] 3.1 Compare A vs B assignment-by-assignment, not by count — **2,226/2,226 identical, 0 differing**
- [x] 3.2 Confirm `area_m2` equals `ST_Area(geom::geography)` exactly for all 742 polygons (max diff 0)
- [x] 3.3 Re-enrich all real events through the new trigger — **43/43 identical**
- [x] 3.4 Verify the generated column populates on INSERT and recomputes on `geom` UPDATE
- [x] 3.5 Verify up → down → up round-trip leaves enrichment working and results identical

## 4. Compatibility

- [x] 4.1 Audit every `INSERT INTO admin_boundaries` for explicit column lists (migrations, `cmd/seed`, `generate_boundary_migration.py`)
- [x] 4.2 Confirm no `SELECT *` against `admin_boundaries` anywhere in the tree
- [x] 4.3 Full API test suite passes

## 5. Deferred to the parent proposal

- [ ] 5.1 Geometry simplification at `0.001°` — separate work item
- [ ] 5.2 Re-measure enrichment cost once real continental boundaries are loaded, rather than replicated fixtures
