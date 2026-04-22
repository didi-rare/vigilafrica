-- VigilAfrica — Sample Events Seed Dataset (Ghana Extension)
-- Purpose: Local development and demo seeding 
-- Coverage: Ghana — Floods + Wildfires
-- Geometry is stored as GeoJSON Points; PostGIS trigger enriches state_name automatically
-- Event dates are relative to keep the demo data looking fresh on boot.
--
-- Usage:
--   psql $DATABASE_URL -f api/db/seeds/sample_events_ghana.sql
--
-- Safe to run multiple times — uses ON CONFLICT (source_id) DO NOTHING

INSERT INTO events (
    source_id, source, title, category, status,
    geom, geom_type, latitude, longitude,
    event_date, source_url, raw_payload
)
VALUES

-- ── Floods ───────────────────────────────────────────────────────────────────

(
    'EONET_SEED_GH_FLOOD_001', 'eonet',
    'Coastal flooding in Accra',
    'floods', 'open',
    ST_GeomFromGeoJSON('{"type":"Point","coordinates":[-0.20, 5.55]}'),
    'Point', 5.55, -0.20,
    NOW() - INTERVAL '1 day', 'https://eonet.gsfc.nasa.gov/api/v3/events/EONET_SEED_GH_FLOOD_001',
    '{"id":"EONET_SEED_GH_FLOOD_001","title":"Coastal flooding in Accra","seed":true}'::jsonb
),
(
    'EONET_SEED_GH_FLOOD_002', 'eonet',
    'Heavy rains in Kumasi',
    'floods', 'open',
    ST_GeomFromGeoJSON('{"type":"Point","coordinates":[-1.62, 6.68]}'),
    'Point', 6.68, -1.62,
    NOW() - INTERVAL '3 days', 'https://eonet.gsfc.nasa.gov/api/v3/events/EONET_SEED_GH_FLOOD_002',
    '{"id":"EONET_SEED_GH_FLOOD_002","title":"Heavy rains in Kumasi","seed":true}'::jsonb
),

-- ── Wildfires ────────────────────────────────────────────────────────────────

(
    'EONET_SEED_GH_FIRE_001', 'eonet',
    'Savanna fire near Tamale',
    'wildfires', 'open',
    ST_GeomFromGeoJSON('{"type":"Point","coordinates":[-0.85, 9.40]}'),
    'Point', 9.40, -0.85,
    NOW() - INTERVAL '2 days', 'https://eonet.gsfc.nasa.gov/api/v3/events/EONET_SEED_GH_FIRE_001',
    '{"id":"EONET_SEED_GH_FIRE_001","title":"Savanna fire near Tamale","seed":true}'::jsonb
)

ON CONFLICT (source_id) DO NOTHING;
