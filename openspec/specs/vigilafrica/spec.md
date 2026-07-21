# VigilAfrica — Project Specification

**Version**: 1.0
**Status**: ACTIVE — Approved 2026-04-12
**Maintained by**: @didi-rare

## Purpose

VigilAfrica is an open-source African natural event tracker providing real-time situational awareness of natural hazards (floods, wildfires) across Nigeria. The platform ingests data from NASA's Earth Observatory Natural Event Tracker (EONET), enriches it with administrative boundary data, and surfaces it through a public API and interactive map interface — enabling communities, NGOs, and local governments to respond faster to emerging natural threats.

## Requirements

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

---

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

---

### Requirement: Public Health API

The system SHALL expose a public REST API providing health status without any database dependency.

#### Scenario: Health endpoint is always reachable

- **WHEN** a client sends `GET /health`
- **THEN** the API SHALL return HTTP 200 with `{"status":"ok","version":"<semver>"}` within 100ms (p99)
- **AND** the endpoint SHALL respond whether or not the database is available

---

### Requirement: Public Events API

The system SHALL expose a paginated, filterable REST API for accessing enriched natural event data.

#### Scenario: Listing events by category

- **WHEN** a client sends `GET /v1/events?category=floods`
- **THEN** the API SHALL return a 200 response with only flood events in the `data` array
- **AND** the response SHALL include a `meta` block with `total`, `limit`, and `offset` fields

#### Scenario: Empty result is not an error

- **WHEN** a client queries with filters that match no events
- **THEN** the API SHALL return HTTP 200 with `{"data":[],"meta":{"total":0,...}}`
- **AND** SHALL NOT return a 404 or 500 response

---

### Requirement: Situational Context API

The system SHALL resolve the caller's IP address to a geographic location and return relevant open events for that location.

#### Scenario: IP resolves to Nigeria

- **WHEN** a client sends `GET /v1/context` with a Nigerian IP address
- **THEN** the API SHALL return HTTP 200 with a `location` object containing `country_name: "Nigeria"` and the resolved `state_name`
- **AND** an `events` array of open events in that state

#### Scenario: IP cannot be resolved

- **WHEN** the caller's IP cannot be resolved to a known location
- **THEN** the API SHALL return HTTP 200 with `{"location":null,"events":[]}`
- **AND** SHALL NOT return any 4xx or 5xx response

---

### Requirement: Interactive Map Frontend

The system SHALL provide a web-based interactive map using MapLibre GL JS that visualises the current state of natural events across Nigeria.

#### Scenario: Event markers on load

- **WHEN** a user loads the VigilAfrica frontend
- **THEN** the map SHALL display event markers for all open events fetched from `GET /v1/events`
- **AND** markers SHALL be colour-coded by event category (flood vs wildfire)

---

> **Note**: For detailed API schemas, endpoint contracts, data models, and all Architecture Decision Records, see the linked specification documents in this directory.
