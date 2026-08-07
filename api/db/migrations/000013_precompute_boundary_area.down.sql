-- Revert to recomputing ST_Area per candidate row (the 000012 trigger), then
-- drop the generated column.
--
-- NOT a byte-for-byte revert to 000012, deliberately: the `id` tie-breaker is
-- kept. Area alone is not a total order, so 000012's behaviour for equal-area
-- overlapping polygons was undefined. Reverting the stored column is the point
-- of this migration; reverting a correctness fix with it is not, and would
-- leave the rollback path with a known non-determinism. The tie-break costs
-- nothing and no later migration depends on 000012's exact function body.
--
-- Order matters: the function must stop referencing area_m2 before the column
-- is dropped, otherwise the trigger breaks for the window between the two
-- statements. Restoring the function first keeps every intermediate state valid.

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
        ORDER BY ST_Area(geom::geography) ASC, id ASC
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
            ORDER BY ST_Area(geom::geography) ASC, id ASC
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

ALTER TABLE admin_boundaries DROP COLUMN IF EXISTS area_m2;
