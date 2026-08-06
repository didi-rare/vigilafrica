# Proposal: Impact Category Expansion (feature-impact-categories)

> ## ⚠️ 2026-08-03 — KEEP THE REGISTRY, DROP THE CATEGORIES. Do not implement as written.
>
> Measured against EONET (`status=all`, 2024-10-01 → 2026-08-03), **both categories this proposal adds return zero events — worldwide**:
>
> | category | NG+GH | Africa | **GLOBAL** |
> |---|---|---|---|
> | `landslides` | 0 | 0 | **0** |
> | `tempExtremes` | 0 | 0 | **0** |
>
> Both are **valid** EONET category IDs (verified against `/api/v3/categories`) — EONET publishes the definitions and no longer populates them. Implementing them means shipping UI, API validation, DB constraints, ingestion and seed data for **nothing**.
>
> **What survives:** the **category registry** is genuinely useful groundwork and a real prerequisite for any future expansion. Keep it.
> **What does not:** the specific category choices. Re-decide them *after* continental coverage, from measured counts.
>
> The companion `feature-v13-risk-intelligence` (`severeStorms`, `drought`) was **archived 2026-08-03** for the same reason — `drought` is also 0 globally.
>
> ⚠️ **Lesson:** three proposals were written against categories producing zero events and none checked the counts. **Measure a category's real event volume before proposing it.** Event density is a *geography-scope* problem — the two categories already ingested yield 291 events for NG+GH and ≥2,000 Africa-wide over the same window. See `feature-continental-coverage`.
>
> **Status caveat:** this change is 0/32 tasks and dated **2026-05-05** (verified against `.openspec.yaml` and the file's first commit `60d0b9e`; an earlier revision said 2026-05-29, conflating it with the unrelated partnership-readiness sprint). Its viability question is now answered; whether to revive the registry half is a maintainer decision.

**Status:** Proposed — re-targeted to v1.3. v1.1.0 was a release-please CI infra release and v1.2.0 was the post-v1.1 audit roll-up; neither shipped new EONET categories. This proposal was to land **first** in the v1.3 cycle (introducing the category registry), with `feature-v13-risk-intelligence` landing second on top of it. ⚠️ **That sequencing is void:** the companion was **archived 2026-08-03 as non-viable** (commit `73ebb48`) and now lives at `openspec/archive/proposal-feature-v13-risk-intelligence.md`. Neither ships as written.

## Why

VigilAfrica v1.0 proves the public launch path with two NASA EONET categories:
`floods` and `wildfires`. That is enough for the launch quality gate, but it
does not fully answer the product-impact concern for communities, civic
responders, journalists, and NGOs. The next production-facing feature should
increase real-world hazard coverage without turning the system into a broad,
unvalidated disaster taxonomy.

NASA EONET currently exposes `landslides` and `tempExtremes` as natural event
categories. Adding these two categories gives VigilAfrica a stronger public
safety surface while staying close to the current event-map model.

Reference: https://eonet.gsfc.nasa.gov/api/v3/categories

## What Changes

⚠️ **VOID — see the banner at the top of this file.** Both categories below return **zero events worldwide**, so this expansion is not implemented as written. The text is retained to show what was proposed and why it was dropped.

v1.3 was to expand the supported category set from:

- `floods`
- `wildfires`

to:

- `floods`
- `wildfires`
- `landslides`
- `tempExtremes`

The change updates ingestion, normalization, storage constraints, API
validation, frontend filtering, event presentation, demo seed data, and
OpenSpec documentation so the new categories are first-class supported
categories rather than unknown values falling through existing flood/fire
branches.

## Capabilities

### New Capabilities

- `landslide-events`: Ingest, store, filter, and display NASA EONET landslide
  events for supported countries.
- `temperature-extreme-events`: Ingest, store, filter, and display NASA EONET
  temperature extreme events for supported countries.

### Modified Capabilities

- `natural-event-ingestion`: EONET polling requests all categories supported by
  this proposal (and is extended further by `feature-v13-risk-intelligence`).
- `event-api`: Category filters and validation accept the expanded supported set.
- `event-map-ui`: Marker, badge, and filter rendering supports four categories
  without collapsing every non-flood category into wildfire styling.
- `seed-data`: Demo/local seed data includes representative `landslides` and
  `tempExtremes` events for Nigeria and Ghana.

## Out of Scope

- No v1.0 launch-gate changes.
- No secondary data oracle; NASA EONET remains the only upstream source.
- No `severeStorms` or `drought` implementation in this proposal. These were
  the scope of the companion proposal `feature-v13-risk-intelligence`, which
  was **archived 2026-08-03 as non-viable** — `drought` returns **0 events
  globally**, the same defect that stalled this proposal. There is no live
  companion; do not plan against one.
- No user accounts, subscriptions, SMS, push notifications, or alert routing.
- No generic "all NASA categories" support.

## User Impact

⚠️ **VOID.** No user-visible change follows from this proposal as written — both categories are empty at source, so there is nothing to show.
Retained for the record:

~~Users will see a broader, more meaningful set of natural hazards once v1.3
ships: landslides and temperature extremes can appear in the API, filters,
event cards, detail views, and map markers with category-specific labels and
styling. If live EONET volume is sparse in a supported country, curated
seed/demo records will still let reviewers understand the intended experience.
~~`severeStorms` and `drought` join in the same v1.3 release via the companion
risk-intelligence proposal.~~ **Void** — that companion was archived as
non-viable on 2026-08-03, and the two categories above are empty at source.
