-- Reproducible measurement for task 1.6 of feature-events-pagination:
-- is a composite index worth adding for the new ListEvents ordering?
--
--   ORDER BY event_date DESC NULLS LAST, id DESC
--
-- ⚠️ The existing `idx_events_event_date ON events(event_date DESC)` CANNOT
-- serve this ordering. Postgres defaults `DESC` to NULLS FIRST, so that index
-- is ordered (event_date DESC NULLS FIRST) while the query asks for NULLS LAST.
-- It also lacks the `id` terminator. The candidate that could serve it is
-- therefore the exact form:
--
--   CREATE INDEX idx_events_event_date_id ON events (event_date DESC NULLS LAST, id DESC)
--
-- Whether the planner USES it is an empirical question at this table size, not
-- a deduction. The `area_m2` work shipped an index EXPLAIN never read; this
-- script exists so the decision is auditable rather than asserted.
--
-- Run:  docker compose start postgres
--       docker exec -i vigilafrica-db psql -U vigilafrica -d vigilafrica -q -v ON_ERROR_STOP=1 \
--         < scripts/bench-events-ordering/bench.sql
--
-- Crash-safety: the whole script runs in ONE transaction and ends with ROLLBACK,
-- so the synthetic rows, the candidate index and the helper functions are all
-- discarded even if it aborts midway. Timings are unaffected — the work still runs.

\set ON_ERROR_STOP on
\timing off

-- 0. Preconditions -----------------------------------------------------------
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'events') THEN
        RAISE EXCEPTION 'events table missing -- run migrations first';
    END IF;
END $$;

BEGIN;

\echo '=== baseline (real rows already in this database) ==='
SELECT count(*) AS real_events,
       count(DISTINCT event_date) AS distinct_event_dates
FROM events;

-- 1. Scale to continental size -----------------------------------------------
-- feature-continental-coverage projects ~3,268 events Africa-wide. Seed up to
-- that total so the planner sees the volume this change is being sized for.
--
-- event_date spreads over 730 days, giving ~4.5 events per date: ties are
-- routine, which is the whole reason the ordering needed a unique terminator.
--
-- geom is left NULL on purpose. trg_enrich_event_location() short-circuits on
-- NULL geom, so the country_name/state_name below survive verbatim and seeding
-- does not pay for spatial enrichment this script is not measuring.
DELETE FROM events WHERE source = 'bench-events-ordering';

INSERT INTO events (source_id, source, title, category, status,
                    country_name, state_name, event_date, ingested_at)
SELECT 'bench-order-' || n,
       'bench-events-ordering',
       'bench fixture ' || n,
       CASE WHEN n % 4 = 0 THEN 'wildfires' ELSE 'floods' END,
       CASE WHEN n % 9 = 0 THEN 'closed' ELSE 'open' END,
       -- Nigeria ~40% of rows, matching its share of the projected continental set.
       (ARRAY['Nigeria','Nigeria','Nigeria','Nigeria',
              'Ghana','Kenya','Ethiopia','Sudan','Chad','Niger'])[1 + (n % 10)],
       'Region ' || (n % 37),
       date_trunc('day', now()) - ((n % 730) || ' days')::interval,
       now()
FROM generate_series(1, GREATEST(0, 3268 - (SELECT count(*)::int FROM events))) n;

ANALYZE events;

\echo '=== seeded scale ==='
SELECT count(*) AS total_events,
       count(DISTINCT event_date) AS distinct_event_dates,
       round(count(*)::numeric / NULLIF(count(DISTINCT event_date), 0), 2) AS avg_rows_per_date,
       count(*) FILTER (WHERE country_name ILIKE 'Nigeria') AS nigeria_rows,
       pg_size_pretty(pg_total_relation_size('events')) AS table_size
FROM events;

-- 2. Helpers -----------------------------------------------------------------

-- A single page is sub-millisecond, so one reading is mostly noise. Take the
-- median of `iters` runs, and report min too so the spread is visible rather
-- than hidden behind one number.
CREATE OR REPLACE FUNCTION bench_ms(q text, iters int)
RETURNS TABLE(median_ms numeric, min_ms numeric) AS $fn$
DECLARE
    t0 timestamptz;
    samples numeric[] := '{}';
    i int;
BEGIN
    -- One untimed warm-up so we measure the query, not the first physical read.
    EXECUTE q;
    FOR i IN 1..iters LOOP
        t0 := clock_timestamp();
        EXECUTE q;
        samples := samples || round((EXTRACT(EPOCH FROM (clock_timestamp() - t0)) * 1000)::numeric, 4);
    END LOOP;
    RETURN QUERY
    -- percentile_cont returns double precision; round(double, int) does not exist.
    SELECT round((percentile_cont(0.5) WITHIN GROUP (ORDER BY s))::numeric, 4),
           round(min(s), 4)
    FROM unnest(samples) s;
END; $fn$ LANGUAGE plpgsql;

-- Asks the planner directly whether a named index appears in the chosen plan.
--
-- pg_stat_user_indexes is NOT usable for this: its counters are flushed at
-- transaction end and the view is held stable within a transaction, so it would
-- report 0 scans no matter what the planner did. Reading the plan is exact and
-- works inside the rolled-back transaction.
CREATE OR REPLACE FUNCTION bench_uses_index(q text, idx text)
RETURNS boolean AS $fn$
DECLARE
    plan json;
BEGIN
    EXECUTE 'EXPLAIN (FORMAT JSON) ' || q INTO plan;
    RETURN plan::text LIKE '%' || idx || '%';
END; $fn$ LANGUAGE plpgsql;

-- The three query shapes ListEvents actually issues. The column list is copied
-- verbatim from queries.go so this measures the real statement, not a proxy.
-- It is written out in full rather than held in a psql variable because psql
-- does not interpolate :vars inside quoted literals.
CREATE TEMP TABLE bench_q(ord int, name text, sql text);
INSERT INTO bench_q VALUES
  (1, 'page 1, no filter',
   'SELECT id, source_id, source, title, category, status, geom_type, latitude, longitude, country_name, state_name, event_date, source_url, ingested_at, enriched_at
      FROM events ORDER BY event_date DESC NULLS LAST, id DESC LIMIT 50 OFFSET 0'),
  (2, 'deep page, offset 3200, no filter',
   'SELECT id, source_id, source, title, category, status, geom_type, latitude, longitude, country_name, state_name, event_date, source_url, ingested_at, enriched_at
      FROM events ORDER BY event_date DESC NULLS LAST, id DESC LIMIT 50 OFFSET 3200'),
  (3, 'page 1, country=Nigeria (the real access pattern)',
   'SELECT id, source_id, source, title, category, status, geom_type, latitude, longitude, country_name, state_name, event_date, source_url, ingested_at, enriched_at
      FROM events WHERE country_name ILIKE ''Nigeria'' ORDER BY event_date DESC NULLS LAST, id DESC LIMIT 50 OFFSET 0');

-- 3. WITHOUT the candidate index ---------------------------------------------
\echo ''
\echo '=== A. existing indexes only (no composite) ==='
CREATE TEMP TABLE bench_a AS
SELECT q.ord, q.name, b.median_ms, b.min_ms
FROM bench_q q, LATERAL bench_ms(q.sql, 40) b;
SELECT name, median_ms, min_ms FROM bench_a ORDER BY ord;

\echo '--- plan (A): page 1, no filter ---'
EXPLAIN (ANALYZE, BUFFERS, COSTS OFF, TIMING OFF, SUMMARY OFF)
SELECT id, source_id, source, title, category, status, geom_type, latitude, longitude,
       country_name, state_name, event_date, source_url, ingested_at, enriched_at
FROM events ORDER BY event_date DESC NULLS LAST, id DESC LIMIT 50 OFFSET 0;

-- 4. WITH the candidate index ------------------------------------------------
CREATE INDEX idx_events_event_date_id ON events (event_date DESC NULLS LAST, id DESC);
ANALYZE events;

\echo ''
\echo '=== B. with idx_events_event_date_id ==='
CREATE TEMP TABLE bench_b AS
SELECT q.ord, q.name, b.median_ms, b.min_ms,
       bench_uses_index(q.sql, 'idx_events_event_date_id') AS index_in_plan
FROM bench_q q, LATERAL bench_ms(q.sql, 40) b;
SELECT name, median_ms, min_ms, index_in_plan FROM bench_b ORDER BY ord;

SELECT pg_size_pretty(pg_relation_size('idx_events_event_date_id')) AS candidate_index_size;

\echo '--- plan (B): page 1, no filter ---'
EXPLAIN (ANALYZE, BUFFERS, COSTS OFF, TIMING OFF, SUMMARY OFF)
SELECT id, source_id, source, title, category, status, geom_type, latitude, longitude,
       country_name, state_name, event_date, source_url, ingested_at, enriched_at
FROM events ORDER BY event_date DESC NULLS LAST, id DESC LIMIT 50 OFFSET 0;

\echo '--- plan (B): deep page, offset 3200 ---'
EXPLAIN (ANALYZE, BUFFERS, COSTS OFF, TIMING OFF, SUMMARY OFF)
SELECT id, source_id, source, title, category, status, geom_type, latitude, longitude,
       country_name, state_name, event_date, source_url, ingested_at, enriched_at
FROM events ORDER BY event_date DESC NULLS LAST, id DESC LIMIT 50 OFFSET 3200;

\echo '--- plan (B): country=Nigeria ---'
EXPLAIN (ANALYZE, BUFFERS, COSTS OFF, TIMING OFF, SUMMARY OFF)
SELECT id, source_id, source, title, category, status, geom_type, latitude, longitude,
       country_name, state_name, event_date, source_url, ingested_at, enriched_at
FROM events WHERE country_name ILIKE 'Nigeria'
ORDER BY event_date DESC NULLS LAST, id DESC LIMIT 50 OFFSET 0;

-- 5. Verdict -----------------------------------------------------------------
\echo ''
\echo '=== C. verdict ==='
SELECT a.name,
       a.median_ms AS without_index_ms,
       b.median_ms AS with_index_ms,
       round(a.median_ms - b.median_ms, 4) AS delta_ms,
       b.index_in_plan,
       CASE
         WHEN NOT b.index_in_plan THEN 'NOT USED -- do not add the index'
         WHEN a.median_ms - b.median_ms < 1 THEN 'used, but saves <1ms -- not worth the write cost'
         ELSE 'used and materially faster -- consider adding'
       END AS verdict
FROM bench_a a JOIN bench_b b USING (ord)
ORDER BY a.ord;

ROLLBACK;

\echo ''
\echo 'Rolled back: synthetic rows, candidate index and helper functions all discarded.'
