# Proposal: v1.3 Risk Intelligence Categories (feature-v13-risk-intelligence)

**Status:** Proposed — v1.3. Lands **second** in the v1.3 cycle, after the
companion [feature-impact-categories](../changes/feature-impact-categories/proposal.md)
proposal introduces the category registry. Both ship before the v1.3.0 tag.

## Why

After [feature-impact-categories](../changes/feature-impact-categories/proposal.md)
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
