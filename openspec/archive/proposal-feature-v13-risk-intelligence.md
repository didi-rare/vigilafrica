---
id: feature-v13-risk-intelligence
status: archived
branch: n/a — never implemented
archived_on: 2026-08-03
archived_reason: >
  Not viable. Measured 2026-08-03: both proposed categories yield effectively
  no events. `drought` returns 0 events GLOBALLY over 22 months; `severeStorms`
  returns 0 for Nigeria+Ghana and 3 Africa-wide. Both are valid EONET category
  IDs — EONET publishes the definitions but no longer populates them.
---

> ## 🗄️ ARCHIVED 2026-08-03 — NOT VIABLE, DO NOT IMPLEMENT
>
> Measured against EONET (`status=all`, 2024-10-01 → 2026-08-03), this proposal's two categories carry effectively no data:
>
> | category | NG+GH | Africa | **GLOBAL** |
> |---|---|---|---|
> | `drought` | 0 | 0 | **0** |
> | `severeStorms` | **0** | 3 | 164 |
>
> Both are **valid** EONET category IDs (verified against `/api/v3/categories`) — EONET publishes the definitions and no longer populates `drought` at all, while `severeStorms` simply does not occur in this geography. Implementing either would add UI, API validation, DB constraints and ingestion for **zero events**.
>
> The companion `feature-impact-categories` is affected the same way: its `landslides` and `tempExtremes` are **also 0 globally**. Its *category registry* remains useful; its *category choices* do not.
>
> ⚠️ **Lesson recorded:** three proposals were authored against EONET categories that produce zero events, and none of them checked the counts first. **Measure a category's actual event volume before proposing it.** Same error class as assuming GDACS would fix event density — it returns 0 for Nigeria and Ghana. See `project-flood-data-source-gap` and `feature-continental-coverage`.
>
> Real event density is a **geography-scope** problem, not a category problem: the two categories already ingested yield 291 events for NG+GH and ≥2,000 Africa-wide over the same window.

# Proposal: v1.3 Risk Intelligence Categories (feature-v13-risk-intelligence)

**Status:** Proposed — v1.3. Lands **second** in the v1.3 cycle, after the
companion [feature-impact-categories](../changes/archive/2026-08-07-feature-impact-categories/proposal.md)
proposal introduces the category registry. Both ship before the v1.3.0 tag.

## Why

After [feature-impact-categories](../changes/archive/2026-08-07-feature-impact-categories/proposal.md)
adds `landslides` and `tempExtremes` (and the supporting category registry),
VigilAfrica should continue expanding toward categories with stronger
public-safety and humanitarian value. NASA EONET exposes `severeStorms` and
`drought`, both of which are relevant to African risk awareness but are split
into this companion proposal rather than bundled into the registry-introducing
one — `drought` in particular has different UX expectations and benefits from
its own review surface.

`severeStorms` fits the existing event-map model relatively well, while
`drought` is slower-moving and may require different UX language, freshness
expectations, and contextual framing.

Reference: https://eonet.gsfc.nasa.gov/api/v3/categories

## What Changes

This proposal (landing in v1.3 alongside `feature-impact-categories`) adds:

- `severeStorms`
- `drought`

The implementation reuses the category registry introduced by
`feature-impact-categories` and extends ingestion, API validation, database
constraints, frontend presentation, and seed/demo data for these two
categories.

## Out of Scope

- No v1.0 quality-gate work.
- No `landslides` or `tempExtremes` implementation details — those are scoped
  to the companion `feature-impact-categories` proposal; this proposal only
  reuses the registry it introduces.
- No secondary data oracle.
- No real-time user alert subscriptions.
- No category-specific drought analytics unless a separate v1.3 design
  approves that scope.

## User Impact

v1.3 will broaden VigilAfrica from acute event awareness into a more useful
risk-intelligence surface: severe storms for urgent weather hazards, and
drought for slower humanitarian/agricultural risk signals.
