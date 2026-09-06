-- Reverting this migration is deliberately a no-op. No-op down migrations have
-- precedent here: 000010 and 000011 do the same.
--
-- The "before" state is the transposed geometry EONET supplied — a Cameroonian
-- flood stored inside Kwara State, Nigeria, and a northern-Nigerian flood stored
-- in Adamawa. Restoring it would mean deliberately reinstating data proven wrong
-- against the upstream source, and the original rings are not recoverable from
-- this file in any case.
--
-- If these rows genuinely need to be reset, delete them and let ingestion
-- restore them through the GDACS resolver:
--
--   DELETE FROM events WHERE source_id IN ('EONET_22248', 'EONET_23208');
--
-- ⚠️ That only restores events still inside the 30-day closed-event window
-- (closedEventWindowDays in eonet.go). GDACS 1104078 closed on 2026-08-06 and is
-- already outside it, so deleting that row removes it permanently.

SELECT 1;
