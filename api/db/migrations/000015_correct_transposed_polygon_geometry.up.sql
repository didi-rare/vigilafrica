-- Purpose: correct the two stored events whose geometry arrived from EONET with
--          latitude and longitude reversed.
-- Milestone: task 3.1 of fix-eonet-polygon-transposition.
-- Data quality: production-quality. Operates on real ingested data.
--
-- Why this exists
-- ---------------
-- EONET republishes GDACS polygons with the coordinate pair reversed. The
-- resolver added alongside this migration corrects NEW ingestions by asking
-- GDACS directly, but it never rewrites rows already stored — so without this,
-- production keeps serving a flood in Kwara State, Nigeria that never happened.
-- The event is real; it occurred in Cameroon.
--
-- Verified against GDACS, which is upstream of EONET and therefore the origin
-- rather than a second opinion. For 1104078 the vertex counts match exactly
-- (1311) and only the axis order differs, so these are demonstrably the same
-- polygons with the pairs swapped.
--
-- Authoritative values, read from
--   https://www.gdacs.org/gdacsapi/api/events/geteventdata?eventtype=FL&eventid=<id>
-- on 2026-09-06:
--
--   1104078  centroid [9.414, 4.6027]    country Cameroon (CMR)
--   1104105  centroid [6.0998, 5.5897]   country Nigeria  (NGA)
--
-- Re-derive them with
--   openspec/changes/fix-eonet-polygon-transposition/verify-transposition.mjs
--
-- Why a point rather than the polygon
-- -----------------------------------
-- The GDACS centroid is a single authoritative value needing no episode
-- matching. ⚠️ Episode matching is a real trap: for 1104105 the polygon EONET
-- chose corresponds to GDACS episode 2, while episodes 1, 3 and 4 sit in
-- entirely different places — that event ran 13 Aug to 4 Sep and moved. The
-- centroid is GDACS's own answer for the event as a whole.
--
-- A point also gives these rows coordinates for the first time. The frontend
-- drops null-coordinate events from the map, and polygons never resolved a
-- lon/lat, so these floods could not previously render as markers at all.
--
-- ⚠️ 1104078 will re-enrich to Cameroon, NOT Nigeria
-- --------------------------------------------------
-- Its true point (9.414, 4.6027) still falls inside the Nigeria ingestion bbox
-- (2,4,15,14) because that box overhangs the border, so the event legitimately
-- stays in the dataset. The enrichment trigger will find no Nigerian ADM1 match
-- and fall back to the Cameroon ADM0 outline loaded by 000012, leaving
-- state_name NULL. That is the designed behaviour for border spillover, and it
-- is the honest result: a Cameroonian flood labelled Cameroonian.
--
-- Targeted rather than generalised
-- --------------------------------
-- 000011 generalised its predicate because any point outside every configured
-- bbox is provably a leak. No such predicate exists here: a transposed pair is
-- still a valid coordinate pair, and the correct value cannot be computed in
-- SQL — it has to be fetched from GDACS. So the two known rows are named
-- explicitly, with their provenance recorded above.

BEGIN;

-- Updating geom fires trg_enrich_event_location (BEFORE INSERT OR UPDATE OF
-- geom), which recomputes country_name and state_name from the corrected point.
-- Do not set those columns here; let the trigger own them.
UPDATE events
SET geom      = ST_SetSRID(ST_MakePoint(9.414, 4.6027), 4326),
    geom_type = 'Point',
    longitude = 9.414,
    latitude  = 4.6027
WHERE source_id = 'EONET_22248';

UPDATE events
SET geom      = ST_SetSRID(ST_MakePoint(6.0998, 5.5897), 4326),
    geom_type = 'Point',
    longitude = 6.0998,
    latitude  = 5.5897
WHERE source_id = 'EONET_23208';

COMMIT;
