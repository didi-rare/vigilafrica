## MODIFIED Requirements

> Delta only. Merged into `openspec/specs/vigilafrica/spec.md` at
> `/openspec-archive` time, per the `feature-impact-categories` convention — the
> canonical spec is not edited by this change.

### Requirement: Natural Event Ingestion

The system SHALL ingest natural event data from the NASA EONET API v3 on a
scheduled interval and persist enriched events in a PostgreSQL/PostGIS database.

Bounding-box containment SHALL be enforced **client-side** by the ingestor.
Upstream `bbox` filtering is treated as a hint, not a guarantee: EONET has been
observed returning events wholly outside the requested box (a Wakulla, Florida
wildfire against the Nigeria bounding box), which then reached production with
no country attribution.

#### Scenario: Scheduled event polling

- **WHEN** the ingestor worker runs on its configured interval (default 60 minutes)
- **THEN** it SHALL fetch open events from EONET filtered to each configured country bounding box
- **AND** store each event with its ingested timestamp and source identifier

#### Scenario: Event outside the country bounding box is rejected

- **WHEN** the upstream source returns an event whose resolved point falls outside the queried country's bounding box
- **THEN** the ingestor SHALL NOT persist that event
- **AND** it SHALL log the skip with the country, source_id, and coordinates
- **AND** it SHALL count the skip separately from other skip reasons

#### Scenario: Event inside the bounding box but outside the named country is retained

- **WHEN** an event falls inside the queried country's bounding box but belongs to a neighbouring country (the boxes legitimately overlap borders)
- **THEN** the ingestor SHALL persist it
- **AND** containment SHALL be judged against the bounding box, never against the country name

#### Scenario: Event with no resolvable point is not rejected on containment grounds

- **WHEN** an event's geometry yields no point coordinates (e.g. Polygon)
- **THEN** the ingestor SHALL persist it rather than drop unverifiable data
- **AND** it SHALL count such events and report the count once per run
- **AND** it SHALL emit per-event detail, including the geometry type, at Debug level
