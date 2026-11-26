-- Migration: 000017_boundary_sources (down)
-- Purpose: drop the loader's provenance table and admin_boundaries.adm_pcode.
-- Milestone: feature-continental-coverage.
-- Data quality: lossy for provenance only. Boundaries the loader wrote stay in
--   admin_boundaries (minus their P-codes), and events keep their names: for
--   Nigeria and Ghana the rows come from the same HDX source as 000010. The
--   next up migration starts with no provenance, so the loader reloads every
--   country once.

ALTER TABLE admin_boundaries DROP COLUMN IF EXISTS adm_pcode;

DROP TABLE IF EXISTS boundary_sources;
