## MODIFIED Requirements

> Delta only. Merged into `openspec/specs/vigilafrica/spec.md` at
> `/openspec-archive` time, per the `feature-impact-categories` convention — the
> canonical spec is not edited by this change.

### Requirement: Geospatial Event Enrichment

The system SHALL enrich raw event coordinates with administrative boundary data
using PostGIS spatial queries. Enrichment SHALL prefer the most specific
administrative level available (state/ADM1) and SHALL fall back to country/ADM0
when no state boundary matches, so that events ingested from the neighbour
overhang of a country's bounding box are labelled by country even when no
state-level boundary is loaded for that country.

#### Scenario: Point event enriched with state

- **WHEN** a new event with a Point geometry falls inside a loaded ADM1 (state) boundary
- **THEN** the enricher SHALL set both `state_name` and `country_name` from that boundary
- **AND** it SHALL prefer the smallest matching ADM1 polygon when several overlap

#### Scenario: Border-spillover event enriched with country only

- **WHEN** a Point event falls inside a country's ingestion bounding box but outside every loaded ADM1 boundary, yet inside a loaded ADM0 (national) boundary
- **THEN** the enricher SHALL set `country_name` from the ADM0 boundary
- **AND** it SHALL leave `state_name` NULL rather than inventing a state

#### Scenario: Event outside all loaded boundaries

- **WHEN** a Point event falls outside every loaded ADM1 and ADM0 boundary
- **THEN** the enricher SHALL leave both `state_name` and `country_name` NULL

#### Scenario: ADM0 fallback never overrides a state match

- **WHEN** a Point event falls inside a loaded ADM1 state
- **THEN** the ADM0 fallback SHALL NOT run
- **AND** the event SHALL retain the state's `country_name`, never a neighbour's
