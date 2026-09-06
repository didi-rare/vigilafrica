-- Reverting this migration is deliberately a no-op.
--
-- The "before" state is the transposed geometry EONET supplied: a Cameroonian
-- flood stored inside Kwara State, Nigeria. Restoring it would mean deliberately
-- reinstating data we have proven wrong against the upstream source, and the
-- original polygons are not recoverable from this file in any case — 1311 and 28
-- vertices are not something to inline for the sake of symmetry.
--
-- If these rows genuinely need to be reset, delete them and let ingestion
-- restore them through the GDACS resolver:
--
--   DELETE FROM events WHERE source_id IN ('EONET_22248', 'EONET_23208');
--
-- ⚠️ That only restores events still inside the 30-day closed-event window
-- (closedEventWindowDays in eonet.go). 1104078 closed on 2026-08-06 and is
-- already outside it, so deleting that row removes it permanently.

SELECT 1;
