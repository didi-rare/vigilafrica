-- Probe events for the boundary-load equivalence check (feature-continental-
-- coverage task 3.7; scripts/boundaries/equivalence.sh).
--
-- Real stored events only test the places events happened to fall. This adds
-- a dense, deterministic set through the real enrichment trigger, so the
-- before/after comparison also covers state borders, coasts and the strips
-- between the old and new boxes:
--   - points on a 0.1-degree grid over both OLD ingestion boxes
--     (DefaultCountries in api/internal/ingestor/eonet.go): NG [2.0, 4.0, 15.0,
--     14.0] gives 131 x 101 = 13,231 points, GH [-3.5, 4.5, 1.2, 11.2] gives
--     48 x 68 = 3,264 points;
--   - 130 polygons of 0.3 x 0.3 degrees on a 1-degree grid over the NG box,
--     because the trigger matches on the full geometry, not a point.
-- 16,625 rows in all, source_id prefixed EQV_.
--
-- FOR A LOCAL OR THROWAWAY DATABASE ONLY. Never run it against staging or
-- production: the probes are fake events and would be served by the API.
--
-- Order of use (local):
--   1. migrate a fresh PostGIS database (start the API with
--      BOUNDARY_LOADER_ENABLED=false, then stop it);
--   2. psql ... -f scripts/boundaries/equivalence-probes.sql
--   3. scripts/boundaries/equivalence.sh snapshot before.tsv
--   4. start the API with BOUNDARY_LOADER_ENABLED=true, wait for
--      "boundary load complete";
--   5. scripts/boundaries/equivalence.sh snapshot after.tsv
--      scripts/boundaries/equivalence.sh compare before.tsv after.tsv
--   6. remove the probes: DELETE FROM events WHERE source_id LIKE 'EQV\_%';
--
-- Idempotent: ON CONFLICT DO NOTHING, so a second run adds nothing.

INSERT INTO events (source_id, source, title, category, status, geom, geom_type, latitude, longitude)
SELECT format('EQV_%s_%s_%s', box, x, y), 'eonet', 'equivalence probe', 'floods', 'open',
       ST_SetSRID(ST_MakePoint(x, y), 4326), 'Point', y, x
FROM (
  SELECT 'NG' AS box, round(x::numeric, 2)::float8 AS x, round(y::numeric, 2)::float8 AS y
  FROM generate_series(2.0, 15.0, 0.1) AS x, generate_series(4.0, 14.0, 0.1) AS y
  UNION ALL
  SELECT 'GH', round(x::numeric, 2)::float8, round(y::numeric, 2)::float8
  FROM generate_series(-3.5, 1.2, 0.1) AS x, generate_series(4.5, 11.2, 0.1) AS y
) g
ON CONFLICT (source_id) DO NOTHING;

INSERT INTO events (source_id, source, title, category, status, geom, geom_type)
SELECT format('EQV_POLY_%s_%s', x, y), 'eonet', 'equivalence polygon probe', 'floods', 'open',
       ST_MakeEnvelope(x, y, x + 0.3, y + 0.3, 4326), 'Polygon'
FROM generate_series(2.0, 14.5, 1.0) AS x, generate_series(4.0, 13.5, 1.0) AS y
ON CONFLICT (source_id) DO NOTHING;

SELECT count(*) AS probes, count(*) FILTER (WHERE country_name IS NOT NULL) AS named
FROM events WHERE source_id LIKE 'EQV\_%';
