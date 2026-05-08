-- Fix enrichment trigger for multi-country correctness.
--
-- Problem with the original trigger (000003):
--   SELECT adm_name, country_name FROM admin_boundaries
--   WHERE ST_Intersects(NEW.geom, geom) LIMIT 1
--
--   With both ADM0 and ADM1 rows for multiple countries, this could match:
--   (a) An ADM0 national boundary instead of an ADM1 state boundary, causing
--       state_name to be set to the country name (e.g., "Nigeria" as state_name).
--   (b) The wrong country's boundary if bounding boxes are adjacent.
--
-- Fix:
--   1. Filter to adm_level = 1 only — ensures we always get a state-level match.
--   2. ORDER BY ST_Area(geom::geography) ASC — prefer the smallest matching
--      polygon (most specific state) when simplified rectangles overlap.
--   3. country_name is set from the boundary record's country_name column,
--      which is always correct for the matched ADM1 row.

CREATE OR REPLACE FUNCTION trg_enrich_event_location()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.geom IS NOT NULL THEN
        SELECT adm_name, country_name
        INTO NEW.state_name, NEW.country_name
        FROM admin_boundaries
        WHERE adm_level = 1
          AND ST_Intersects(NEW.geom, geom)
        ORDER BY ST_Area(geom::geography) ASC
        LIMIT 1;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger registration is unchanged — CREATE OR REPLACE on the function is sufficient.
-- Re-registering the trigger here for completeness; DROP IF EXISTS prevents duplication.
DROP TRIGGER IF EXISTS enrich_event_location_trigger ON events;
CREATE TRIGGER enrich_event_location_trigger
BEFORE INSERT OR UPDATE OF geom ON events
FOR EACH ROW
EXECUTE FUNCTION trg_enrich_event_location();
