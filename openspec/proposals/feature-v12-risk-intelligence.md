# Proposal: v1.2 Risk Intelligence Categories (feature-v12-risk-intelligence)

**Status:** Deferred to v1.2 (after v1.1 impact category expansion)

## Why

After v1.1 adds `landslides` and `tempExtremes`, VigilAfrica should continue
expanding toward categories with stronger public-safety and humanitarian value.
NASA EONET exposes `severeStorms` and `drought`, both of which are relevant to
African risk awareness but should not be bundled into v1.1.

`severeStorms` fits the existing event-map model relatively well, while
`drought` is slower-moving and may require different UX language, freshness
expectations, and contextual framing.

Reference: https://eonet.gsfc.nasa.gov/api/v3/categories

## What Changes

v1.2 is expected to add:

- `severeStorms`
- `drought`

The v1.2 implementation should reuse the category registry introduced by v1.1
and extend ingestion, API validation, database constraints, frontend
presentation, and seed/demo data for these two categories.

## Out of Scope

- No v1.0 quality-gate work.
- No v1.1 `landslides` or `tempExtremes` implementation details beyond reusing
  the resulting category registry.
- No secondary data oracle.
- No real-time user alert subscriptions.
- No category-specific drought analytics unless a separate v1.2 design approves
  that scope.

## User Impact

v1.2 would broaden VigilAfrica from acute event awareness into a more useful
risk-intelligence surface: severe storms for urgent weather hazards, and
drought for slower humanitarian/agricultural risk signals.
