-- Reproducible simplification-tolerance sweep for feature-continental-coverage.
--
-- Question: how aggressively can ADM1 boundary geometry be simplified before
-- enrichment starts naming the wrong state -- or worse, no state at all?
--
-- Committed because the original sweep was run in a throwaway container with no
-- script or data snapshot, so independent review could not reproduce any of it.
--
-- Method mirrors production exactly: ST_Intersects against admin_boundaries
-- filtered to adm_level = 1, tie-broken by smallest area, LIMIT 1. The BASELINE
-- assignment is that same matcher run on UNSIMPLIFIED geometry -- not "the
-- polygon the point was generated in" -- so overlapping polygons are handled the
-- way production handles them.
--
-- Reads only; creates and drops its own bench_simp_* tables. Does not touch
-- admin_boundaries or events.
--
-- Run:  docker compose start postgres
--       docker exec -i vigilafrica-db psql -U vigilafrica -d vigilafrica -q -v ON_ERROR_STOP=1 \
--         < scripts/bench-simplification/sweep.sql

\set ON_ERROR_STOP on
\timing off

DROP TABLE IF EXISTS bench_simp_pts, bench_simp_res, bench_simp_edges;

-- 1. Ground-truth points: 200 inside each real ADM1 unit --------------------
CREATE TABLE bench_simp_pts AS
SELECT row_number() OVER () AS n, s.geom
FROM (
    SELECT (ST_Dump(ST_GeneratePoints(geom, 200))).geom::geometry(Point, 4326) AS geom
    FROM admin_boundaries WHERE adm_level = 1 AND country_code <> 'ZZ'
) s;
CREATE INDEX ON bench_simp_pts USING GIST(geom);

-- Baseline assignment + "is this point near a border?" flag.
-- Near-border is where simplification does its damage, so it is reported
-- separately; a headline error rate averaged over interior points hides it.
ALTER TABLE bench_simp_pts ADD COLUMN truth_name TEXT;
ALTER TABLE bench_simp_pts ADD COLUMN near_border BOOLEAN;

UPDATE bench_simp_pts p SET truth_name = (
    SELECT b.adm_name FROM admin_boundaries b
    WHERE b.adm_level = 1 AND b.country_code <> 'ZZ' AND ST_Intersects(p.geom, b.geom)
    ORDER BY b.area_m2 ASC LIMIT 1);

-- Boundary linework is materialised into an INDEXED geography column first.
-- Calling ST_DWithin against ST_Boundary(b.geom)::geography inline recomputes
-- the boundary per candidate row and cannot use an index -- that form did not
-- finish in 10 minutes here. This form completes in seconds.
DROP TABLE IF EXISTS bench_simp_edges;
CREATE TABLE bench_simp_edges AS
SELECT adm_name, ST_Boundary(geom)::geography AS edge
FROM admin_boundaries WHERE adm_level = 1 AND country_code <> 'ZZ';
CREATE INDEX ON bench_simp_edges USING GIST(edge);
ANALYZE bench_simp_edges;

UPDATE bench_simp_pts p SET near_border = EXISTS (
    SELECT 1 FROM bench_simp_edges e
    WHERE ST_DWithin(e.edge, p.geom::geography, 2000));

ANALYZE bench_simp_pts;
SELECT count(*) AS ground_truth_points,
       count(*) FILTER (WHERE near_border) AS within_2km_of_a_border,
       count(*) FILTER (WHERE truth_name IS NULL) AS baseline_unassigned
FROM bench_simp_pts;

-- 2. Sweep -------------------------------------------------------------------
CREATE TABLE bench_simp_res (
    tolerance     DOUBLE PRECISION,
    vertices      BIGINT,
    bytes         BIGINT,
    misassigned   BIGINT,
    mis_border    BIGINT,
    unassigned    BIGINT
);

DO $$
DECLARE
    tol   DOUBLE PRECISION;
    tols  DOUBLE PRECISION[] := ARRAY[0, 0.001, 0.005, 0.01, 0.02, 0.05];
BEGIN
    FOREACH tol IN ARRAY tols LOOP
        DROP TABLE IF EXISTS bench_simp_geom;
        -- tolerance 0 means "no simplification" -- the baseline row.
        EXECUTE format($f$
            CREATE TABLE bench_simp_geom AS
            SELECT adm_name,
                   CASE WHEN %1$L::double precision = 0 THEN geom
                        ELSE ST_Multi(ST_SimplifyPreserveTopology(geom, %1$L::double precision))
                   END::geometry(MultiPolygon,4326) AS geom
            FROM admin_boundaries WHERE adm_level = 1 AND country_code <> 'ZZ'
        $f$, tol);
        CREATE INDEX ON bench_simp_geom USING GIST(geom);
        -- Store the area rather than recomputing it in the ORDER BY. This is the
        -- same fix migration 000013 applies to production, and it matters here
        -- for the same reason: recomputing ST_Area per candidate row across
        -- 10,600 points x 6 tolerances made this sweep too slow to finish.
        -- An earlier revision of this script did exactly that and had to be
        -- abandoned mid-run.
        ALTER TABLE bench_simp_geom ADD COLUMN area_m2 double precision
            GENERATED ALWAYS AS (ST_Area(geom::geography)) STORED;
        ANALYZE bench_simp_geom;

        INSERT INTO bench_simp_res
        SELECT tol,
               (SELECT sum(ST_NPoints(geom)) FROM bench_simp_geom),
               (SELECT sum(pg_column_size(geom)) FROM bench_simp_geom),
               count(*) FILTER (WHERE m.name IS NOT NULL AND m.name IS DISTINCT FROM p.truth_name),
               count(*) FILTER (WHERE m.name IS NOT NULL AND m.name IS DISTINCT FROM p.truth_name AND p.near_border),
               count(*) FILTER (WHERE m.name IS NULL)
        FROM bench_simp_pts p
        CROSS JOIN LATERAL (
            SELECT (SELECT g.adm_name FROM bench_simp_geom g
                    WHERE ST_Intersects(p.geom, g.geom)
                    ORDER BY g.area_m2 ASC LIMIT 1) AS name
        ) m;

        RAISE NOTICE 'tolerance % done', tol;
    END LOOP;
    DROP TABLE IF EXISTS bench_simp_geom;
END $$;

-- 3. Report ------------------------------------------------------------------
\echo '--- simplification sweep (percentages are of all ground-truth points) ---'
SELECT
    CASE WHEN r.tolerance = 0 THEN 'none (baseline)'
         ELSE r.tolerance::text || ' deg' END                                  AS tolerance,
    round(100.0 * r.vertices / NULLIF(base.vertices, 0), 1) || '%'             AS vertices_kept,
    pg_size_pretty(r.bytes)                                                     AS size,
    round(100.0 * r.misassigned / t.total, 3) || '%'                            AS misassigned,
    round(100.0 * r.mis_border  / NULLIF(t.border, 0), 3) || '%'                AS misassigned_near_border,
    round(100.0 * r.unassigned  / t.total, 3) || '%'                            AS unassigned
FROM bench_simp_res r
CROSS JOIN (SELECT count(*) AS total, count(*) FILTER (WHERE near_border) AS border FROM bench_simp_pts) t
CROSS JOIN (SELECT vertices FROM bench_simp_res WHERE tolerance = 0) base
ORDER BY r.tolerance;

\echo ''
\echo 'WATCH THE unassigned COLUMN, NOT THE ERROR RATE. A point matching NO polygon'
\echo 'yields NULL state_name -- a silent enrichment failure, which is worse than a'
\echo 'slightly-wrong name in an admin-name-first product.'

DROP TABLE IF EXISTS bench_simp_pts, bench_simp_res, bench_simp_edges;
