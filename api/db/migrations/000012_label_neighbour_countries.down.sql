-- Revert fix-border-event-enrichment.
--
-- Restores the ADM1-only enrichment trigger (000006), removes the neighbour
-- ADM0 rows, and re-enriches so border-spillover events return to NULL country.

BEGIN;

-- 1. Restore the ADM1-only trigger (000006 body).
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

DROP TRIGGER IF EXISTS enrich_event_location_trigger ON events;
CREATE TRIGGER enrich_event_location_trigger
BEFORE INSERT OR UPDATE OF geom ON events
FOR EACH ROW
EXECUTE FUNCTION trg_enrich_event_location();

-- 2. Remove the neighbour ADM0 rows.
DELETE FROM admin_boundaries WHERE country_code IN ('BJ', 'NE', 'TD', 'CM', 'CI', 'BF', 'TG');

-- 3. Re-enrich (border-spillover events revert to NULL country/state).
UPDATE events SET geom = geom WHERE geom IS NOT NULL;

COMMIT;
