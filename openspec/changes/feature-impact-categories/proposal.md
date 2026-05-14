# Proposal: Impact Category Expansion (feature-impact-categories)

**Status:** Proposed — v1.0 production launched; v1.1.0 was a release-please CI infra release, so this proposal will land in the next functional release.

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

v1.1 expands the supported category set from:

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

- `natural-event-ingestion`: EONET polling requests all v1.1 supported
  categories.
- `event-api`: Category filters and validation accept the v1.1 supported set.
- `event-map-ui`: Marker, badge, and filter rendering supports four categories
  without collapsing every non-flood category into wildfire styling.
- `seed-data`: Demo/local seed data includes representative v1.1 events for
  Nigeria and Ghana.

## Out of Scope

- No v1.0 launch-gate changes.
- No secondary data oracle; NASA EONET remains the only upstream source.
- No `severeStorms` or `drought` implementation in v1.1. Those are reserved
  for v1.2.
- No user accounts, subscriptions, SMS, push notifications, or alert routing.
- No generic "all NASA categories" support.

## User Impact

Users will see a broader, more meaningful set of natural hazards after v1.0:
landslides and temperature extremes can appear in the API, filters, event cards,
detail views, and map markers with category-specific labels and styling. If live
EONET volume is sparse in a supported country, curated seed/demo records will
still let reviewers understand the intended experience.
