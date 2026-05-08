---
change_id: feat-v0.6-country-expansion
status: in_progress
created_date: 2026-04-17
author: Claude Code
spec_ref: openspec/specs/roadmap-v05-operational-prototype.md
adr_refs: ADR-009
---

# Change: v0.6 Country Expansion Model

## What This Change Implements

v0.6 milestone — country expansion model. All changes are confined to
`api/internal/*`, `api/cmd/*`, `api/db/*`, `openspec/specs/vigilafrica/`,
and `web/src/api/`. No UI component changes.

## Files Created or Modified

### New files
- `api/db/migrations/000005_admin_boundary_data.up.sql` — Nigeria + Ghana ADM0/ADM1 seed data (fixes pre-existing gap; Nigeria boundaries were never committed)
- `api/db/migrations/000006_fix_enrichment_trigger.up.sql` — replace LIMIT 1 trigger with adm_level=1 filter + area-preference ordering, preventing ADM0/ADM1 ambiguity in multi-country enrichment
- `openspec/specs/vigilafrica/country-onboarding-template.md` — canonical template for adding any future country

### Modified files
- `api/internal/ingestor/eonet.go` — extract `CountryConfig` struct; replace hardcoded Nigeria bbox with per-country parameterised fetch; `Ingest()` accepts a `CountryConfig`
- `api/internal/ingestor/scheduler.go` — iterate over `ingestor.DefaultCountries` (Nigeria + Ghana) on each scheduled tick
- `api/internal/database/queries.go` — add `Country string` to `EventFilters`; update `ListEvents` WHERE clause
- `api/internal/handlers/events.go` — parse `?country=` query parameter
- `web/src/api/events.ts` — add optional `country?: string` parameter to `fetchEvents()`

## Traceability

| Feature / Requirement | Spec ref | ADR ref |
|---|---|---|
| Country Onboarding Template | roadmap.md §v0.6 | — |
| Tier classification criteria | roadmap.md §v0.6 | — |
| Boundary dataset standards | roadmap.md §v0.6 | ADR-009 |
| Enrichment validation rules | roadmap.md §v0.6 | — |
| Fallback logic for border events | roadmap.md §v0.6 | — |
| Ghana as second country proof | roadmap.md §v0.6 | — |
| Fix enrichment trigger ambiguity | Pre-existing bug | — |
| Nigeria boundary data (pre-existing gap) | roadmap.md §v0.3 | — |
