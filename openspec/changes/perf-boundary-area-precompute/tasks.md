# Tasks: Precompute Boundary Area for Enrichment

## 1. Migration

- [x] 1.1 Add `000013_precompute_boundary_area.up.sql` with a `GENERATED ALWAYS ... STORED` `area_m2` column on `admin_boundaries`
- [x] 1.2 Reorder **both** trigger branches (ADM1 match and ADM0 fallback) by `area_m2`
- [x] 1.3 Preserve matching semantics exactly — `adm_level` filter, smallest-polygon tie-break, ADM0 fallback, `state_name` left NULL on fallback
- [x] 1.4 Add the down migration, restoring the `000012` trigger **before** dropping the column so no intermediate state is broken
- [x] 1.5 Deliberately omit a btree index on `area_m2` — `EXPLAIN` shows it is never used

## 2. Measurement

- [x] 2.1 Build a continental-scale fixture (795 polygons) from real production geometry
- [x] 2.2 Benchmark old vs new over 2,385 probe points — **A (LATERAL) 15.4×, B (INSERT through trigger) 13.3×**
- [x] 2.3 Confirm the btree index on `area_m2` provides no benefit, via timing and `EXPLAIN ANALYZE`
- [x] 2.4 **Commit the harness** (`scripts/bench-enrichment/bench.sql`) so the numbers are auditable rather than asserted — raised by independent review, which noted the original claim had no reproducible artifact
- [x] 2.5 Correct the headline from **16.2×** to the **11–13×** production-realistic range; the original was a method-A figure quoted as method B

## 2a. Operational risk (surfaced by independent review, missed originally)

- [x] 2a.1 Verify that `ADD COLUMN ... GENERATED STORED` rewrites the table — `pg_class.relfilenode` changes across the `ALTER`
- [x] 2a.2 Verify the lock level — `pg_locks` reports `AccessExclusiveLock` granted
- [x] 2a.3 Document the blocking window in both the migration and this record, including that concurrent ingestion blocks rather than fails
- [ ] 2a.4 Re-assess before adding any further generated/non-volatile-default column to `admin_boundaries` **after** continental boundaries land — ~1.3 s at 795 rows is a real outage window at scale

## 3. Correctness

- [x] 3.1 Compare old vs new assignment-by-assignment, not by count — **2,385/2,385 identical, 0 differing**
- [x] 3.2 Confirm `area_m2` equals `ST_Area(geom::geography)` exactly for all 797 rows (max diff 0)
- [x] 3.6 Independent adversarial suite — shared-border midpoints via `ST_Intersection`, ADM0-fallback interiors, points matching nothing, all 37 NG exterior-ring vertices: **0/48 differences**
- [x] 3.7 Run the **integration** suite, not just unit tests — `scripts/test-api.ps1 -Integration` exercises `enrichment_test.go` against a real migrated database. *(Missed in the author's own verification; run by the reviewer.)*
- [ ] 3.8 Codify the adversarial cases as regression tests — none of the tie-break, shared-border or geometry-update cases are currently covered by CI, so a future change to the matching logic would not be caught. **Pre-existing gap, not introduced here.**
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
