-- Create the function to enrich event geometry with administrative boundaries
CREATE OR REPLACE FUNCTION trg_enrich_event_location()
RETURNS TRIGGER AS $$
BEGIN
    -- Only run spatial query if we have a valid geometry point
    IF NEW.geom IS NOT NULL THEN
        SELECT adm_name, country_name
        INTO NEW.state_name, NEW.country_name
        FROM admin_boundaries
        WHERE ST_Intersects(NEW.geom, geom)
        LIMIT 1;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create the trigger that fires on INSERT or UPDATE
DROP TRIGGER IF EXISTS enrich_event_location_trigger ON events;
CREATE TRIGGER enrich_event_location_trigger
BEFORE INSERT OR UPDATE OF geom ON events
FOR EACH ROW
EXECUTE FUNCTION trg_enrich_event_location();

-- Note to reverse/down script:
-- DROP TRIGGER enrich_event_location_trigger ON events;
-- DROP FUNCTION trg_enrich_event_location();
