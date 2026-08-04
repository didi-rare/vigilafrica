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
-- Measured 2026-08-04, PostGIS 15-3.4, 742 ADM1-scale polygons (the real 53
-- production polygons replicated and translated), 2,226 point lookups using the
-- production trigger's exact matching logic:
--
--     ORDER BY ST_Area(geom::geography)   5,520 ms   2.48  ms/lookup
--     ORDER BY area_m2 (this migration)     341 ms   0.153 ms/lookup   16.2x
--
-- Behaviour is preserved, not merely assumed to be: over the same 2,226 probe
-- points both variants assigned an identical (adm_name, country_name) to every
-- single point -- 2,226/2,226, zero differing -- and the stored area equalled
-- ST_Area(geom::geography) exactly for all 742 polygons (max abs diff 0).
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
