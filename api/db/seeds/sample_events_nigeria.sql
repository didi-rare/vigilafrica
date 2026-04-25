-- VigilAfrica — Sample Events Seed Dataset
-- Purpose: Local development and demo seeding (no EONET connection required)
-- Coverage: Nigeria — Floods + Wildfires (v0.5 scope)
-- All coordinates are within Nigeria's bounding box: Lat 4.0–14.0, Long 2.0–15.0
-- Geometry is stored as GeoJSON Points; PostGIS trigger enriches state_name automatically
--
-- Usage:
--   psql $DATABASE_URL -f api/db/seeds/sample_events_nigeria.sql
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
    'EONET_SEED_FLOOD_001', 'eonet',
    'Flooding along Benue River, Benue State',
    'floods', 'open',
    ST_GeomFromGeoJSON('{"type":"Point","coordinates":[8.13, 7.33]}'),
    'Point', 7.33, 8.13,
    '2026-04-10 09:00:00+00', 'https://eonet.gsfc.nasa.gov/api/v3/events/EONET_SEED_FLOOD_001',
    '{"id":"EONET_SEED_FLOOD_001","title":"Flooding along Benue River","seed":true}'::jsonb
),
(
    'EONET_SEED_FLOOD_002', 'eonet',
    'Severe flooding in Lokoja, Kogi State',
    'floods', 'open',
    ST_GeomFromGeoJSON('{"type":"Point","coordinates":[6.74, 7.80]}'),
    'Point', 7.80, 6.74,
    '2026-04-11 14:00:00+00', 'https://eonet.gsfc.nasa.gov/api/v3/events/EONET_SEED_FLOOD_002',
    '{"id":"EONET_SEED_FLOOD_002","title":"Severe flooding in Lokoja","seed":true}'::jsonb
),
(
    'EONET_SEED_FLOOD_003', 'eonet',
    'Flash flooding in Ibadan, Oyo State',
    'floods', 'closed',
    ST_GeomFromGeoJSON('{"type":"Point","coordinates":[3.90, 7.38]}'),
    'Point', 7.38, 3.90,
    '2026-03-28 07:30:00+00', 'https://eonet.gsfc.nasa.gov/api/v3/events/EONET_SEED_FLOOD_003',
    '{"id":"EONET_SEED_FLOOD_003","title":"Flash flooding in Ibadan","seed":true}'::jsonb
),
(
    'EONET_SEED_FLOOD_004', 'eonet',
    'Niger Delta flooding, Delta State',
    'floods', 'open',
    ST_GeomFromGeoJSON('{"type":"Point","coordinates":[5.89, 5.50]}'),
    'Point', 5.50, 5.89,
    '2026-04-12 11:00:00+00', 'https://eonet.gsfc.nasa.gov/api/v3/events/EONET_SEED_FLOOD_004',
    '{"id":"EONET_SEED_FLOOD_004","title":"Niger Delta flooding","seed":true}'::jsonb
),
(
    'EONET_SEED_FLOOD_005', 'eonet',
    'Flood inundation near Makurdi, Benue State',
    'floods', 'closed',
    ST_GeomFromGeoJSON('{"type":"Point","coordinates":[8.52, 7.74]}'),
    'Point', 7.74, 8.52,
    '2026-03-15 06:00:00+00', 'https://eonet.gsfc.nasa.gov/api/v3/events/EONET_SEED_FLOOD_005',
    '{"id":"EONET_SEED_FLOOD_005","title":"Flood inundation near Makurdi","seed":true}'::jsonb
),
(
    'EONET_SEED_FLOOD_006', 'eonet',
    'Coastal flooding in Warri, Delta State',
    'floods', 'open',
    ST_GeomFromGeoJSON('{"type":"Point","coordinates":[5.75, 5.52]}'),
    'Point', 5.52, 5.75,
    '2026-04-13 10:00:00+00', 'https://eonet.gsfc.nasa.gov/api/v3/events/EONET_SEED_FLOOD_006',
    '{"id":"EONET_SEED_FLOOD_006","title":"Coastal flooding in Warri","seed":true}'::jsonb
),
(
    'EONET_SEED_FLOOD_007', 'eonet',
    'Flooding in Maiduguri, Borno State',
    'floods', 'open',
    ST_GeomFromGeoJSON('{"type":"Point","coordinates":[13.16, 11.85]}'),
    'Point', 11.85, 13.16,
    '2026-04-09 08:00:00+00', 'https://eonet.gsfc.nasa.gov/api/v3/events/EONET_SEED_FLOOD_007',
    '{"id":"EONET_SEED_FLOOD_007","title":"Flooding in Maiduguri","seed":true}'::jsonb
),
(
    'EONET_SEED_FLOOD_008', 'eonet',
    'River flooding near Onitsha, Anambra State',
    'floods', 'closed',
    ST_GeomFromGeoJSON('{"type":"Point","coordinates":[6.78, 6.14]}'),
    'Point', 6.14, 6.78,
    '2026-03-20 13:00:00+00', 'https://eonet.gsfc.nasa.gov/api/v3/events/EONET_SEED_FLOOD_008',
    '{"id":"EONET_SEED_FLOOD_008","title":"River flooding near Onitsha","seed":true}'::jsonb
),

-- ── Wildfires ────────────────────────────────────────────────────────────────

(
    'EONET_SEED_FIRE_001', 'eonet',
    'Wildfire in Plateau State highlands',
    'wildfires', 'open',
    ST_GeomFromGeoJSON('{"type":"Point","coordinates":[8.89, 9.22]}'),
    'Point', 9.22, 8.89,
    '2026-04-08 15:30:00+00', 'https://eonet.gsfc.nasa.gov/api/v3/events/EONET_SEED_FIRE_001',
    '{"id":"EONET_SEED_FIRE_001","title":"Wildfire in Plateau State highlands","seed":true}'::jsonb
),
(
    'EONET_SEED_FIRE_002', 'eonet',
    'Savanna fire near Kano, Kano State',
    'wildfires', 'closed',
    ST_GeomFromGeoJSON('{"type":"Point","coordinates":[8.52, 12.00]}'),
    'Point', 12.00, 8.52,
    '2026-03-22 11:00:00+00', 'https://eonet.gsfc.nasa.gov/api/v3/events/EONET_SEED_FIRE_002',
    '{"id":"EONET_SEED_FIRE_002","title":"Savanna fire near Kano","seed":true}'::jsonb
),
(
    'EONET_SEED_FIRE_003', 'eonet',
    'Bushfire in Kaduna State',
    'wildfires', 'open',
    ST_GeomFromGeoJSON('{"type":"Point","coordinates":[7.44, 10.52]}'),
    'Point', 10.52, 7.44,
    '2026-04-14 09:00:00+00', 'https://eonet.gsfc.nasa.gov/api/v3/events/EONET_SEED_FIRE_003',
    '{"id":"EONET_SEED_FIRE_003","title":"Bushfire in Kaduna State","seed":true}'::jsonb
),
(
    'EONET_SEED_FIRE_004', 'eonet',
    'Grassland fire near Yola, Adamawa State',
    'wildfires', 'closed',
    ST_GeomFromGeoJSON('{"type":"Point","coordinates":[12.46, 9.20]}'),
    'Point', 9.20, 12.46,
    '2026-03-30 16:00:00+00', 'https://eonet.gsfc.nasa.gov/api/v3/events/EONET_SEED_FIRE_004',
    '{"id":"EONET_SEED_FIRE_004","title":"Grassland fire near Yola","seed":true}'::jsonb
),
(
    'EONET_SEED_FIRE_005', 'eonet',
    'Wildfire in Taraba State',
    'wildfires', 'open',
    ST_GeomFromGeoJSON('{"type":"Point","coordinates":[11.37, 8.89]}'),
    'Point', 8.89, 11.37,
    '2026-04-15 12:00:00+00', 'https://eonet.gsfc.nasa.gov/api/v3/events/EONET_SEED_FIRE_005',
    '{"id":"EONET_SEED_FIRE_005","title":"Wildfire in Taraba State","seed":true}'::jsonb
),
(
    'EONET_SEED_FIRE_006', 'eonet',
    'Dry season fire near Sokoto, Sokoto State',
    'wildfires', 'closed',
    ST_GeomFromGeoJSON('{"type":"Point","coordinates":[5.23, 13.06]}'),
    'Point', 13.06, 5.23,
    '2026-03-10 14:00:00+00', 'https://eonet.gsfc.nasa.gov/api/v3/events/EONET_SEED_FIRE_006',
    '{"id":"EONET_SEED_FIRE_006","title":"Dry season fire near Sokoto","seed":true}'::jsonb
)

ON CONFLICT (source_id) DO NOTHING;
