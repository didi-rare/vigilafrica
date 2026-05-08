# VigilAfrica — Project Specification

**Version**: 1.0
**Status**: ACTIVE — Approved 2026-04-12
**Maintained by**: @didi-rare

## Purpose

VigilAfrica is an open-source African natural event tracker providing real-time situational awareness of natural hazards (floods, wildfires) across Nigeria. The platform ingests data from NASA's Earth Observatory Natural Event Tracker (EONET), enriches it with administrative boundary data, and surfaces it through a public API and interactive map interface — enabling communities, NGOs, and local governments to respond faster to emerging natural threats.

## Requirements

### Requirement: Natural Event Ingestion

The system SHALL ingest natural event data from the NASA EONET API v3 on a scheduled interval and persist enriched events in a PostgreSQL/PostGIS database.

#### Scenario: Scheduled event polling

- **WHEN** the ingestor worker runs on its configured interval (default 60 minutes)
- **THEN** it SHALL fetch open events from EONET filtered to the Nigeria bounding box (Lat 4.0–14.0, Long 2.0–15.0)
- **AND** store each event with its ingested timestamp and source identifier

---

### Requirement: Geospatial Event Enrichment

The system SHALL enrich raw EONET event coordinates with Nigerian administrative boundary data (state name) using PostGIS spatial queries.

#### Scenario: Point event enrichment

- **WHEN** a new event with a Point geometry is ingested
- **THEN** the enricher SHALL perform a PostGIS `ST_Intersects` query against Nigeria ADM1 boundaries
- **AND** set the `state_name` and `country_name` fields on the event record

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
