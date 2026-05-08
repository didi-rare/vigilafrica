-- Restore the original enrichment trigger function from migration 000003
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
