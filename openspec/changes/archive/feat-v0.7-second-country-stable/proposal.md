---
change_id: feat-v0.7-second-country-stable
status: proposed
created_date: 2026-04-18
author: Claude Code
roadmap_ref: openspec/specs/vigilafrica/roadmap.md §v0.7
---

# Proposal: v0.7 — Second Country Stable

## Why

v0.6 added Ghana as a second country structurally: bounding box ingestion, simplified ADM1 boundary rectangles, `?country=` API filter, and the country onboarding template. But the v0.7 bar is higher — Ghana must deliver the **same quality experience as Nigeria**: raw coordinates → state name in the frontend, measurable enrichment success rate ≥ 85%, observable per-country health, and the system instrumented enough to prove it.

Three deferred items from the v0.6 review (Notes B, C, D) land here:
- `ingestion_runs` has no `country_code` — the `/health` endpoint is blind to which country last succeeded
- `generate_boundary_migration.py` is inline documentation, not a committed tool
- Simplified boundary data is development-quality and needs a documented upgrade path

v0.7 is also the milestone where the frontend stops being Nigeria-only — hardcoded "Nigeria first" copy, missing filter controls, and a map locked to Nigeria's center all need to change before a user from Accra gets the right experience.

## What

1. **Per-country ingestion observability** — `country_code` column added to `ingestion_runs`; `/health` returns `last_ingestion_by_country` map so operators can tell Nigeria from Ghana at a glance.

2. **Enrichment success rate endpoint** — `GET /v1/enrichment-stats` returns per-country totals: events ingested, events enriched to ADM1, success rate %. Satisfies the ≥ 85% documentation criterion and provides a repeatable measurement tool for future countries.

3. **Country + state filter UI** — `EventsDashboard` gains country and state filter dropdowns. Country filter is a static list (Nigeria / Ghana); state filter dynamically populates from the backend `GET /v1/states?country=` endpoint. Both filters compose correctly with category. Map center shifts to the selected country's centroid.

4. **Content decompression** — remove hardcoded "Nigeria first" and "Nigerian administrative boundaries" copy in App.tsx and EventsDashboard.tsx; update prototype banner and milestone list to reflect v0.7 active state.

5. **Ghana country notes file** — `openspec/specs/vigilafrica/ghana-country-notes.md` documents: EONET bbox overlap validation (no overlap confirmed), enrichment edge cases, simplified-vs-HDX boundary gap, and any template deviations.

6. **`scripts/generate_boundary_migration.py`** — commit the inline script from the onboarding template as a runnable tool; add `scripts/README.md`.

## Out of Scope

- Replacing simplified boundary rectangles with full HDX GeoJSON (deferred: needs generate_boundary_migration.py first + a real EONET event sample to validate; production boundary quality is v0.8/post-v1.0 concern per Note D)
- Adding a third country
- New event categories
- UI redesign
