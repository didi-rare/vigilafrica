-- Reproducible benchmark for the enrichment tie-break optimisation (migration 000013).
--
-- Measures the SAME workload two ways, because the two disagree by ~30% and the
-- optimistic one is easy to quote by accident:
--
--   METHOD A -- LATERAL lookup. Isolates the ORDER BY. Flattering: it skips
--              plpgsql overhead and the ADM0 fallback branch.
--   METHOD B -- real INSERTs through the real trigger. Production-realistic and
--              the number that should be quoted.
--
-- Self-cleaning: synthetic boundaries use country_code 'ZZ' and synthetic events
-- use source 'bench-enrichment'; both are removed at the end, and the production
-- trigger is restored.
--
-- Run:  docker compose start postgres
--       docker exec -i vigilafrica-db psql -U vigilafrica -d vigilafrica -q -v ON_ERROR_STOP=1 < scripts/bench-enrichment/bench.sql

\set ON_ERROR_STOP on
\timing off

-- 0. Preconditions -----------------------------------------------------------
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'admin_boundaries' AND column_name = 'area_m2'
    ) THEN
        RAISE EXCEPTION 'migration 000013 is not applied -- nothing to compare';
    END IF;
END $$;

-- 1. Scale admin_boundaries to continental size ------------------------------
-- 53 real ADM1 rows + 14 translated copies each = 795, approximating the ~750
-- ADM1 units Africa-wide that this optimisation exists to serve.
DELETE FROM admin_boundaries WHERE country_code = 'ZZ';

INSERT INTO admin_boundaries (country_code, country_name, adm_level, adm_name, geom)
SELECT 'ZZ', 'Benchland', 1, b.adm_name || '-' || k,
       ST_Translate(b.geom, (k % 7) * 12.0, ((k / 7)::int) * 9.0)::geometry(MultiPolygon, 4326)
FROM admin_boundaries b, generate_series(1, 14) k
WHERE b.adm_level = 1 AND b.country_code <> 'ZZ';

ANALYZE admin_boundaries;
SELECT count(*) FILTER (WHERE adm_level = 1) AS adm1_polygons,
       pg_size_pretty(pg_total_relation_size('admin_boundaries')) AS table_size
FROM admin_boundaries;

-- 2. Probe points ------------------------------------------------------------
DROP TABLE IF EXISTS bench_probe;
-- NOTE: row_number() must be applied AFTER ST_Dump expands, not alongside it.
-- Set-returning functions in a target list expand after window functions are
-- evaluated, so numbering in the same SELECT yields one n per INPUT row --
-- 795 distinct values repeated 3x -- which collides on the unique source_id
-- later. The subquery forces the dump to complete first.
CREATE TABLE bench_probe AS
SELECT row_number() OVER () AS n, s.geom
FROM (
    SELECT (ST_Dump(ST_GeneratePoints(geom, 3))).geom::geometry(Point, 4326) AS geom
    FROM admin_boundaries WHERE adm_level = 1
) s;
CREATE INDEX ON bench_probe USING GIST(geom);
ANALYZE bench_probe;
SELECT count(*) AS probe_points FROM bench_probe;

-- Warm the cache so we time the query, not the first physical read.
SELECT count(*) FROM admin_boundaries;

-- 3. METHOD A -- LATERAL lookup ---------------------------------------------
\echo '--- METHOD A: LATERAL lookup (isolates ORDER BY; flattering) ---'
\timing on
\echo 'A1  OLD  ORDER BY ST_Area(geom::geography)'
SELECT count(m.adm_name) AS matched FROM bench_probe p
CROSS JOIN LATERAL (
    SELECT adm_name FROM admin_boundaries b
    WHERE b.adm_level = 1 AND ST_Intersects(p.geom, b.geom)
    ORDER BY ST_Area(b.geom::geography) ASC LIMIT 1) m;

\echo 'A2  NEW  ORDER BY area_m2'
SELECT count(m.adm_name) AS matched FROM bench_probe p
CROSS JOIN LATERAL (
    SELECT adm_name FROM admin_boundaries b
    WHERE b.adm_level = 1 AND ST_Intersects(p.geom, b.geom)
    ORDER BY b.area_m2 ASC LIMIT 1) m;
\timing off

-- 4. METHOD B -- real INSERTs through the real trigger -----------------------
-- Includes plpgsql overhead and the ADM0 fallback branch. This is the number to quote.
DELETE FROM events WHERE source = 'bench-enrichment';

-- Restore the pre-000013 trigger body.
CREATE OR REPLACE FUNCTION trg_enrich_event_location()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.geom IS NOT NULL THEN
        SELECT adm_name, country_name INTO NEW.state_name, NEW.country_name
        FROM admin_boundaries WHERE adm_level = 1 AND ST_Intersects(NEW.geom, geom)
        ORDER BY ST_Area(geom::geography) ASC LIMIT 1;
        IF NEW.country_name IS NULL THEN
            SELECT country_name INTO NEW.country_name
            FROM admin_boundaries WHERE adm_level = 0 AND ST_Intersects(NEW.geom, geom)
            ORDER BY ST_Area(geom::geography) ASC LIMIT 1;
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

\echo '--- METHOD B: INSERT through the real trigger (production-realistic) ---'
\timing on
\echo 'B1  OLD  ORDER BY ST_Area(geom::geography)'
INSERT INTO events (source_id, source, title, category, status, geom, geom_type, latitude, longitude, event_date)
SELECT 'bench-old-' || n, 'bench-enrichment', 'bench', 'floods', 'open',
       geom, 'Point', ST_Y(geom), ST_X(geom), now()
FROM bench_probe;
\timing off

-- Restore the 000013 trigger body.
CREATE OR REPLACE FUNCTION trg_enrich_event_location()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.geom IS NOT NULL THEN
        SELECT adm_name, country_name INTO NEW.state_name, NEW.country_name
        FROM admin_boundaries WHERE adm_level = 1 AND ST_Intersects(NEW.geom, geom)
        ORDER BY area_m2 ASC LIMIT 1;
        IF NEW.country_name IS NULL THEN
            SELECT country_name INTO NEW.country_name
            FROM admin_boundaries WHERE adm_level = 0 AND ST_Intersects(NEW.geom, geom)
            ORDER BY area_m2 ASC LIMIT 1;
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

\timing on
\echo 'B2  NEW  ORDER BY area_m2'
INSERT INTO events (source_id, source, title, category, status, geom, geom_type, latitude, longitude, event_date)
SELECT 'bench-new-' || n, 'bench-enrichment', 'bench', 'floods', 'open',
       geom, 'Point', ST_Y(geom), ST_X(geom), now()
FROM bench_probe;
\timing off

-- 5. Correctness -- did the two variants label every event identically? ------
\echo '--- CORRECTNESS: identical labelling required, not just identical counts ---'
SELECT count(*) AS compared,
       count(*) FILTER (WHERE o.state_name   IS NOT DISTINCT FROM n.state_name
                          AND o.country_name IS NOT DISTINCT FROM n.country_name) AS identical,
       count(*) FILTER (WHERE o.state_name   IS DISTINCT FROM n.state_name
                           OR o.country_name IS DISTINCT FROM n.country_name) AS differing
FROM events o
JOIN events n
  ON n.source_id = 'bench-new-' || replace(o.source_id, 'bench-old-', '')
WHERE o.source_id LIKE 'bench-old-%';

-- Stored area must equal the directly computed area, exactly.
SELECT count(*) AS polygons,
       count(*) FILTER (WHERE area_m2 = ST_Area(geom::geography)) AS exact,
       max(abs(area_m2 - ST_Area(geom::geography))) AS max_abs_diff
FROM admin_boundaries;

-- 6. Cleanup -----------------------------------------------------------------
DELETE FROM events WHERE source = 'bench-enrichment';
DELETE FROM admin_boundaries WHERE country_code = 'ZZ';
DROP TABLE IF EXISTS bench_probe;
ANALYZE admin_boundaries;
\echo '--- cleaned up; production trigger (000013 form) left in place ---'
