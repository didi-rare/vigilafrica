---
id: chore-hdx-boundaries
status: proposed
branch: chore/hdx-boundaries
---

# Proposal: Replace Rectangular Boundary Approximations with HDX COD Data (chore-hdx-boundaries)

## Why

[api/db/migrations/000005_admin_boundary_data.up.sql](api/db/migrations/000005_admin_boundary_data.up.sql) seeds all 37 Nigerian and 16 Ghanaian ADM1 regions as **rectangular bounding-box approximations**. The migration header is explicit: *"These are SIMPLIFIED rectangular boundary approximations for development and prototype use. They correctly classify events that are clearly within a state but will be imprecise near borders."*

Events within ~50 km of a state border may enrich to the wrong adjacent state. This was acceptable when ingest volume was low and inspection was manual, but it's the highest-leverage upgrade to enrichment quality available — and the tooling to replace it is already in place ([scripts/generate_boundary_migration.py](scripts/generate_boundary_migration.py)).

Originally captured as Note D in the v0.6 openspec-review (2026-04-18); kept open in memory since then.

## What Changes

1. Download HDX COD ADM1 GeoJSON for both supported countries:
   - Nigeria: <https://data.humdata.org/dataset/cod-ab-nga>
   - Ghana:   <https://data.humdata.org/dataset/cod-ab-gha>
2. Run `scripts/generate_boundary_migration.py` to produce a new migration (likely `00001N_replace_boundary_data.up.sql`) that REPLACEs the rectangular polygons with the real HDX geometries
3. Verify enrichment quality with the success-rate query in [openspec/specs/vigilafrica/country-onboarding-template.md](openspec/specs/vigilafrica/country-onboarding-template.md) §4.3; target ≥85% correct ADM1 attribution
4. Re-run enrichment for the existing event corpus (small migration that re-enriches all events using the new boundaries)

## Out of Scope

- Adding new countries (covered by the country-onboarding template separately)
- ADM2 (LGA) precision — ADM1 only for now
- Boundary versioning / dispute handling
