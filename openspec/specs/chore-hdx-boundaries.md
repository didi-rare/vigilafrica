---
id: chore-hdx-boundaries
status: proposed
branch: chore/hdx-boundaries
---

# Spec: Replace Rectangular Boundary Approximations with HDX COD Data (chore-hdx-boundaries)

## Context

Migration `000005_admin_boundary_data.up.sql` seeded NG (36 states + FCT) and GH (16 regions) as **rectangular bounding-box approximations**. The migration's own header is explicit: "These are SIMPLIFIED rectangular boundary approximations for development and prototype use. They correctly classify events that are clearly within a state but will be imprecise near borders."

Events within ~50 km of a state border may enrich to the wrong adjacent state. This was an accepted v0.6 limitation, originally captured as Note D in the v0.6 openspec-review (2026-04-18) and kept open in memory since. This spec replaces those rectangles with real HDX COD polygons.

Companion: [openspec/proposals/chore-hdx-boundaries.md](openspec/proposals/chore-hdx-boundaries.md).

## Decision Log

| # | Decision | Alternatives | Why |
|---|---|---|---|
| D1 | Single REPLACE migration (`000010_replace_boundary_data.up.sql`) that DELETEs the 000005 rows then INSERTs the HDX rows | Two migrations — one DELETE, one INSERT | Atomic. The combined migration runs inside a single `BEGIN…COMMIT` so a partial failure rolls back — no chance of a window where boundaries are missing |
| D2 | Use HDX COD ADM1 v01 GeoJSON, downloaded from the public CKAN API | GADM, OpenStreetMap, Natural Earth | HDX COD is the OCHA-curated humanitarian gold standard. Aligns with `country-onboarding-template.md` §0.2 which already mandates HDX. The CKAN API gives a direct, stable download URL — no scraping |
| D3 | Compute ADM0 by `ST_Union` of the ADM1 polygons | Download ADM0 separately and store it | The HDX zip contains both, but the union is geometrically guaranteed to match the ADM1 outer rings. Storing a separately-sourced ADM0 risks 1-pixel gaps that the enrichment trigger would surface as "in country but in no state" anomalies |
| D4 | Re-enrich existing events via `UPDATE events SET geom = geom WHERE geom IS NOT NULL` at the end of the migration | Skip re-enrichment; let new events get the new boundaries while old events keep the old enrichment | Skipping leaves the events table in a mixed state — old events still attributed via rectangles, new events via HDX. The whole point of the upgrade is to fix wrong-state attribution; leaving old events wrong is a bug. The trigger's `BEFORE UPDATE OF geom` fires on any geom-mentioning UPDATE, even when `geom = geom` |
| D5 | `down.sql` is documented as no-op (irreversible) per developers-go.md §11.2 | Restore the rectangular polygons in `down.sql` | Going back to rectangles is a deliberate accuracy downgrade. If a rollback is needed, `000005` is in version control and can be re-applied manually. Header of the down.sql documents the manual recovery path |
| D6 | Boundaries stored as full-resolution POLYGON/MULTIPOLYGON (no `ST_Simplify`) | Simplify with tolerance ~0.001° (~100 m) to shrink the migration file | Full resolution is what the user gets from HDX; PostGIS handles arbitrary polygon sizes; the existing GIST index on `geom` keeps queries fast. The migration is 2.3MB — acceptable for a one-time replace. Simplification can be revisited if production query latency degrades |
| D7 | Touch the existing generator script `scripts/generate_boundary_migration.py` to fix two bugs found while running it | Hand-write the SQL | The script existed but had never been exercised end-to-end. Fixed: (a) `detect_property` returned the value instead of the key (`adm1_name` value `'Abia'` was being used as a column name); (b) HDX 2025 COD format uses lowercase `adm1_name` which wasn't in the candidate list. Both fixes are documented and harmless for any future country onboarding |
| D8 | Do not split NG and GH into separate migrations | Two files (000010_NG, 000011_GH) | Atomic replacement requires they live in one transaction. Per §11.6 each migration should be "one logical change" — replacing the boundary data IS the one logical change; the two countries are just the scope of that change |

## Components to Touch

### New files

| File | Purpose |
|---|---|
| `api/db/migrations/000010_replace_boundary_data.up.sql` | The combined DELETE + INSERT(NG ADM1) + INSERT(GH ADM1) + INSERT(NG ADM0) + INSERT(GH ADM0) + re-enrichment migration. 2.3MB |
| `api/db/migrations/000010_replace_boundary_data.down.sql` | Documented no-op (per §11.2). Header explains the manual recovery procedure if a rollback is ever needed |

### Modified files

| File | Change |
|---|---|
| `scripts/generate_boundary_migration.py` | Fix `detect_property` to return the property key, not its value. Add `adm1_name`/`adm0_name` to the candidate lists for HDX 2025 COD format |
| [openspec/proposals/chore-hdx-boundaries.md](openspec/proposals/chore-hdx-boundaries.md) | Update `branch:` frontmatter from `tbd` to `chore/hdx-boundaries` |

### Untouched

- `api/db/migrations/000005_admin_boundary_data.up.sql` — stays in version control as the rollback target. Not edited (per §11.7 — never edit a merged migration)
- `api/db/migrations/000006_fix_enrichment_trigger.up.sql` — the trigger logic is correct already; this migration only updates the data it reads
- Go application code — the existing handlers and ingestor consume `admin_boundaries` via the trigger, which sees no API surface change
- Frontend — no UI implications beyond the values that flow through `country_name` / `state_name` becoming more accurate

## Behaviour Contract

- **B1** — After this migration applies, `SELECT COUNT(*) FROM admin_boundaries WHERE country_code='NG' AND adm_level=1` MUST equal 37 (36 states + FCT, per HDX v01)
- **B2** — `SELECT COUNT(*) FROM admin_boundaries WHERE country_code='GH' AND adm_level=1` MUST equal 16 (per HDX v01)
- **B3** — `SELECT COUNT(*) FROM admin_boundaries WHERE adm_level=0` MUST include exactly one row each for NG and GH, computed via `ST_Union` of the corresponding ADM1 rows
- **B4** — A point inside the canonical centroid of any state MUST enrich to the correct `(country_name, state_name)`. Spot checks documented in the Verification Plan:
  - Abuja `(7.49, 9.07)` → `(Nigeria, Federal Capital Territory)`
  - Accra `(-0.187, 5.604)` → `(Ghana, Greater Accra)`
  - Kumasi `(-1.625, 6.688)` → `(Ghana, Ashanti)`
  - Lagos `(3.379, 6.524)` → `(Nigeria, Lagos)`
- **B5** — All existing events with non-null `geom` MUST be re-enriched against the new boundaries. The `UPDATE events SET geom = geom WHERE geom IS NOT NULL` at the end of the migration fires the `BEFORE UPDATE OF geom` trigger on every such row
- **B6** — Enrichment success rate (events where `state_name IS NOT NULL` divided by total events with non-null geom) MUST be **≥ 85%** for Tier 1 (Nigeria) and **≥ 70%** for Tier 2 (Ghana) per `country-onboarding-template.md` tier targets. Measured via the `/v1/enrichment-stats` endpoint
- **B7** — No events MAY be lost or duplicated. `SELECT COUNT(*) FROM events` before and after this migration MUST be identical
- **B8** — The migration MUST be atomic: any failure during apply rolls back the whole change. Implemented via the `BEGIN…COMMIT` wrapper
- **B9** — The migration MUST NOT alter `admin_boundaries` rows for any country other than NG and GH. The DELETE clause is scoped to `country_code IN ('NG', 'GH')`

## Phase 1 — Generate the Migration

- [x] Patch `scripts/generate_boundary_migration.py` to fix `detect_property` + add `adm1_name`/`adm0_name` candidates
- [x] Download HDX COD ADM1 GeoJSON for both countries via CKAN API:
  - `nga_admin_boundaries.geojson.zip` (11.4 MB) → `nga_admin1.geojson` (1.6 MB, 37 features)
  - `gha_admin_boundaries.geojson.zip` (3.4 MB) → `gha_admin1.geojson` (1 MB, 16 features)
- [x] Run the generator to produce per-country SQL
- [x] Combine into one migration with DELETE + INSERTs + ADM0 union + re-enrichment

## Phase 2 — Verify Locally

- [ ] `docker compose up -d postgres` — start a clean Postgres + PostGIS container
- [ ] Apply all migrations via the API server's startup (`cd api && go run ./cmd/server/` with `INGEST_INTERVAL_MIN=0`)
- [ ] `psql $DATABASE_URL -c "SELECT country_code, adm_level, COUNT(*) FROM admin_boundaries GROUP BY 1,2 ORDER BY 1,2"` → expect:
  - `NG | 0 | 1`
  - `NG | 1 | 37`
  - `GH | 0 | 1`
  - `GH | 1 | 16`
- [ ] Insert each of the four B4 spot-check coordinates as a test event; verify enrichment matches the expected (country, state) pair; clean up
- [ ] Confirm `SELECT COUNT(*) FROM events` is unchanged before and after the migration (B7)

## Phase 3 — Verify on Staging

- [ ] Open PR to `development`; merge → promote `development → main` → `Deploy Staging` workflow rebuilds + reapplies migrations against the staging Postgres
- [ ] `curl https://api.staging.vigilafrica.org/v1/enrichment-stats | jq` → verify `success_rate_pct` is ≥ 85 for Nigeria, ≥ 70 for Ghana (B6)
- [ ] Spot-check 3-5 events on `staging.vigilafrica.org/events/<id>` whose previous attribution was suspect; confirm the post-HDX state name looks plausible
- [ ] Check `api.staging.vigilafrica.org/v1/events?country=Nigeria&limit=200` and visually scan that no event has a state in the wrong country (e.g. a Ghanaian state listed under Nigeria)

## Acceptance Criteria

- [ ] Migration file `000010_replace_boundary_data.up.sql` applies cleanly on a fresh Postgres + PostGIS 15 container (B1, B2, B3)
- [ ] All 4 B4 centroid spot-checks pass locally
- [ ] `go test ./...` from `api/` still green (no Go code changed)
- [ ] `go vet ./...` clean; `go build ./...` clean
- [ ] Staging deploy completes successfully and `/v1/enrichment-stats` shows ≥ 85% NG / ≥ 70% GH (B6)
- [ ] PR description includes the four spot-check results from Phase 2 + the staging enrichment-stats output

## Out of Scope (reaffirmed)

- Adding new countries (covered by `country-onboarding-template.md` separately)
- ADM2 (LGA) precision — ADM1 only for now
- Boundary versioning / dispute handling (HDX v01 is the chosen snapshot; future updates are a separate migration)
- `ST_Simplify` optimisation of the polygons (D6 — revisit only if query latency degrades)
- Updating the staging banner / frontend copy to reflect the boundary upgrade — silent improvement

## Risks

- **R1 — Migration is too slow to apply in production**: 2.3MB SQL + 53 polygon INSERTs + a full `UPDATE events` re-enrichment. In practice the production events table has ~hundreds of rows today, so re-enrichment is fast (< 1s). Mitigation: pre-emptively `EXPLAIN` the UPDATE during Phase 2 to confirm; if event volume ever grows to millions, the re-enrichment step should be split into a separate migration with batching
- **R2 — HDX polygon is invalid in some way (self-intersection, ring direction)**: PostGIS would reject the INSERT and roll back the transaction. Mitigation: the structural sanity check in `chore-hdx-boundaries` verified all 53 polygons start with `POLYGON(` or `MULTIPOLYGON(` and have balanced parens; PostGIS validation is final
- **R3 — Re-enrichment produces a WORSE outcome for some specific event**: e.g. an event near a state border gets attributed to a different state under the new boundaries. **This is intended** — the new attribution is correct per HDX; the old attribution was approximate. The acceptance criterion measures aggregate success rate, not per-event diff
- **R4 — Enrichment success rate drops**: theoretically possible if HDX polygons leave gaps that rectangles covered (e.g. small islands or disputed territories). Mitigation: the `ST_Union` ADM0 computation acts as a country-level fallback, but the trigger explicitly filters `adm_level = 1`. If B6 fails on staging, the remediation is to widen the trigger logic to fall back to ADM0 when no ADM1 matches — a follow-up spec
- **R5 — Generator script change breaks future country onboarding**: D7 modifies a script that's used to onboard new countries. Mitigation: the change is purely additive (added candidates to a list; fixed a real bug). Documented in the spec and the script's own header

## Verification Plan

1. **Pre-commit** (already done):
   - Structural sanity check via Python: counts of inserts, parens balance, WKT openers all valid
   - `git diff --stat` shows the expected file set
2. **Local Postgres** (Phase 2 above) — to be run after `docker compose up -d`
3. **Staging deploy** (Phase 3) — the dev → main promotion triggers the staging migration apply against a Postgres that already has the 000005 rectangles; this exercises the DELETE-then-INSERT flow end-to-end
4. **Production roll** — handled by the normal release workflow once staging is verified
