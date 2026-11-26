-- Migration: 000017_boundary_sources
-- Purpose: provenance for the boundary loader, and a stable key on admin units.
-- Milestone: feature-continental-coverage (spec §2, task 3.1).
-- Data quality: schema only; no boundary or event rows are written here. The
--   loader (api/internal/boundaries) fills both at run time, after migrations.
--
-- boundary_sources holds one row per manifest country the loader has tried.
-- The first group of columns describes the LAST SUCCESSFUL load and is NULL
-- until one happens; the failed_* columns describe the LAST FAILED attempt and
-- are cleared by the next success. A country whose first attempt failed (for
-- example a checksum mismatch on a fresh database, decision P7) therefore has
-- a row with only the failed_* columns set.
--
--   entry_sha256         SHA-256 of the canonicalised manifest entry that was
--                        loaded (keys sorted, `enabled` excluded). The loader
--                        skips a country only when this equals the current
--                        entry's hash AND reenriched_at is set.
--   reenriched_at        when stored events in the country's box were
--                        re-enriched against the new boundaries; set in the
--                        same transaction as the boundary swap.
--   failed_entry_sha256  hash of the entry whose load last failed; with
--                        failed_at it stops a failing entry being downloaded
--                        again before BOUNDARY_RETRY_HOURS have passed.
--
-- admin_boundaries.adm_pcode: the source's P-code for the unit. Nullable, no
-- default, so ADD COLUMN is a catalogue-only change: it does NOT rewrite the
-- table (contrast 000013's generated column).
--
-- Replayable per developers-go.md §11.4: IF NOT EXISTS throughout.

CREATE TABLE IF NOT EXISTS boundary_sources (
    country_code        TEXT        PRIMARY KEY CHECK (country_code ~ '^[A-Z]{2}$'),
    country_name        TEXT,
    levels              TEXT        CHECK (levels IN ('adm1', 'adm0')),
    source              TEXT        CHECK (source IN ('hdx-cod-ab', 'geoboundaries')),
    archive_url         TEXT,
    member              TEXT,
    member_sha256       TEXT,
    entry_sha256        TEXT,
    vintage             DATE,
    unit_count          INTEGER     CHECK (unit_count > 0),
    loaded_at           TIMESTAMPTZ,
    reenriched_at       TIMESTAMPTZ,
    failed_entry_sha256 TEXT,
    failed_at           TIMESTAMPTZ,
    last_error          TEXT,
    -- A row records a load, a failed attempt, or both.
    CONSTRAINT boundary_sources_load_or_failure
        CHECK (entry_sha256 IS NOT NULL OR failed_entry_sha256 IS NOT NULL),
    -- A recorded load carries its whole provenance.
    CONSTRAINT boundary_sources_load_complete
        CHECK (entry_sha256 IS NULL OR (
            country_name IS NOT NULL AND levels IS NOT NULL AND source IS NOT NULL
            AND archive_url IS NOT NULL AND member_sha256 IS NOT NULL AND vintage IS NOT NULL
            AND unit_count IS NOT NULL AND loaded_at IS NOT NULL)),
    -- A recorded failure carries when it happened.
    CONSTRAINT boundary_sources_failure_complete
        CHECK ((failed_entry_sha256 IS NULL) = (failed_at IS NULL))
);

ALTER TABLE admin_boundaries ADD COLUMN IF NOT EXISTS adm_pcode TEXT;
