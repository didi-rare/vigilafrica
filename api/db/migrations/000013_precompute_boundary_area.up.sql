-- Precompute admin boundary area so enrichment stops recomputing it per insert.
--
-- Problem
--   Both branches of trg_enrich_event_location() (000012) order candidate
--   polygons with:
--       ORDER BY ST_Area(geom::geography) ASC
--   ST_Area is therefore recomputed, on the fly, for every candidate row, on
--   every event insert. At 53 ADM1 polygons that is invisible. At continental
--   scale (~750 ADM1 units) it becomes the dominant cost of ingestion.
--
-- Fix
--   Store the area in a GENERATED ALWAYS ... STORED column. Postgres computes
--   it on insert and recomputes it automatically whenever geom changes, so it
--   cannot drift out of sync with the geometry -- unlike a plain column kept in
--   step by a trigger or by the loader remembering to set it.
--
-- Measured via the committed harness scripts/bench-enrichment/bench.sql --
-- PostGIS 15-3.4, 795 ADM1-scale polygons (the real 53 replicated and
-- translated), 2,385 probe points. The harness deliberately measures the SAME
-- workload two ways, because they disagree by ~15% and the flattering one is
-- easy to quote by accident:
--
--   A  LATERAL lookup, isolating the ORDER BY     5,757 ms ->  374 ms   15.4x
--   B  real INSERTs through the real trigger      5,800 ms ->  435 ms   13.3x
--
-- QUOTE B. Method A omits plpgsql overhead and the ADM0 fallback branch, so it
-- overstates the win. An independent reviewer measuring the same change on
-- different hardware got 15.2x (A) and 11.5x (B) -- so the production-realistic
-- figure is a RANGE of roughly 11-13x, not a single number. Earlier revisions
-- of this file claimed 16.2x; that was a method-A measurement quoted as though
-- it were method B, and it was optimistic even for method A.
--
-- Behaviour is preserved, not merely assumed to be: over the same 2,385 probe
-- points both variants assigned an identical (adm_name, country_name) to every
-- single point -- 2,385/2,385, zero differing -- and the stored area equalled
-- ST_Area(geom::geography) exactly for all 797 rows (max abs diff 0). An
-- independent review additionally confirmed 0/48 differences across a hand-built
-- adversarial suite: real shared-border midpoints from ST_Intersection of
-- adjacent state pairs, ADM0-fallback interiors, and points matching nothing.
--
-- !! OPERATIONAL WARNING -- this migration REWRITES THE TABLE. !!
--
-- ADD COLUMN ... GENERATED ALWAYS ... STORED is not a metadata-only change.
-- Postgres physically rewrites admin_boundaries while holding an
-- ACCESS EXCLUSIVE lock -- verified here, not assumed: pg_class.relfilenode
-- changes across the ALTER, and pg_locks reports AccessExclusiveLock granted.
--
-- For that window NOTHING can read or write admin_boundaries, which includes
-- the enrichment trigger firing on every events insert, so concurrent ingestion
-- will block rather than fail.
--
-- At today's 62 rows this is sub-millisecond and harmless. At the ~750-polygon
-- continental scale this migration exists to serve it was ~1.3 s in testing.
-- Anyone adding further generated or non-volatile-default columns to this table
-- AFTER continental boundaries are loaded should expect a real outage window and
-- schedule it against the ingestion cadence.
--
-- Deliberately NO btree index on area_m2. EXPLAIN ANALYZE confirms the planner
-- satisfies ST_Intersects from the GIST index on geom and then sorts the
-- handful of surviving candidates with an in-memory quicksort; it never reads a
-- btree on area_m2. Adding one measured no faster and would be dead weight on
-- every write. (This corrects feature-continental-coverage work item 4, which
-- proposed "precompute area as a stored column and index it".)

ALTER TABLE admin_boundaries
    ADD COLUMN area_m2 double precision
    GENERATED ALWAYS AS (ST_Area(geom::geography)) STORED;

-- Same logic as 000012, ordering by the stored column instead of recomputing.
CREATE OR REPLACE FUNCTION trg_enrich_event_location()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.geom IS NOT NULL THEN
        -- Prefer the most specific administrative level: match a state (ADM1).
        SELECT adm_name, country_name
        INTO NEW.state_name, NEW.country_name
        FROM admin_boundaries
        WHERE adm_level = 1
          AND ST_Intersects(NEW.geom, geom)
        ORDER BY area_m2 ASC
        LIMIT 1;

        -- Fall back to country (ADM0) when no state matched. Labels border
        -- spillover from neighbours we ingest but hold no states for, and
        -- rescues any point that falls in a gap between real ADM1 polygons.
        -- state_name is deliberately left NULL -- we do not invent a state.
        IF NEW.country_name IS NULL THEN
            SELECT country_name
            INTO NEW.country_name
            FROM admin_boundaries
            WHERE adm_level = 0
              AND ST_Intersects(NEW.geom, geom)
            ORDER BY area_m2 ASC
            LIMIT 1;
        END IF;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS enrich_event_location_trigger ON events;
CREATE TRIGGER enrich_event_location_trigger
BEFORE INSERT OR UPDATE OF geom ON events
FOR EACH ROW
EXECUTE FUNCTION trg_enrich_event_location();
