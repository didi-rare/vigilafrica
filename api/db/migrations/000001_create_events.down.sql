DROP INDEX IF EXISTS idx_events_geom;
DROP INDEX IF EXISTS idx_events_event_date;
DROP INDEX IF EXISTS idx_events_state_name;
DROP INDEX IF EXISTS idx_events_status;
DROP INDEX IF EXISTS idx_events_category;
DROP TABLE IF EXISTS events;
DROP EXTENSION IF EXISTS postgis;
DROP EXTENSION IF EXISTS "uuid-ossp";
