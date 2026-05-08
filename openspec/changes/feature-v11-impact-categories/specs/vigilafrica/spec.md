## MODIFIED Requirements

### Requirement: Natural Event Ingestion

The system SHALL ingest natural event data from the NASA EONET API v3 on a
scheduled interval and persist enriched events in a PostgreSQL/PostGIS database.
The supported v1.1 event categories SHALL be `floods`, `wildfires`,
`landslides`, and `tempExtremes`.

#### Scenario: Scheduled event polling includes v1.1 categories

- **WHEN** the ingestor worker runs on its configured interval
- **THEN** it SHALL fetch open and closed events from EONET filtered to each supported country bounding box
- **AND** it SHALL request the categories `floods,wildfires,landslides,tempExtremes`
- **AND** it SHALL store each supported event with its ingested timestamp and source identifier

#### Scenario: Unsupported EONET category is not silently reclassified

- **WHEN** an EONET payload contains a category outside the supported v1.1 category set
- **THEN** the system SHALL skip or reject that event deliberately
- **AND** it SHALL NOT default the event category to `floods` or `wildfires`
- **AND** the behavior SHALL be covered by automated tests

### Requirement: Public Event API

The system SHALL expose a paginated, filterable REST API for accessing enriched
natural event data. Category filters SHALL accept only the supported v1.1
category set: `floods`, `wildfires`, `landslides`, and `tempExtremes`.

#### Scenario: Listing landslide events by category

- **WHEN** a client sends `GET /v1/events?category=landslides`
- **THEN** the API SHALL return a 200 response with only landslide events in the `data` array
- **AND** the response SHALL include a `meta` block with `total`, `limit`, and `offset` fields

#### Scenario: Listing temperature extreme events by category

- **WHEN** a client sends `GET /v1/events?category=tempExtremes`
- **THEN** the API SHALL return a 200 response with only temperature extreme events in the `data` array
- **AND** the response SHALL include a `meta` block with `total`, `limit`, and `offset` fields

#### Scenario: Rejecting unsupported category filters

- **WHEN** a client sends `GET /v1/events?category=earthquakes`
- **THEN** the API SHALL return a 400 response
- **AND** the error message SHALL list the valid category values

### Requirement: Frontend Event Map

The system SHALL provide an interactive frontend map that displays localized
event markers and category filters for all supported v1.1 event categories.

#### Scenario: Category filter renders all v1.1 categories

- **WHEN** a user loads the VigilAfrica frontend
- **THEN** the category filter SHALL include Floods, Wildfires, Landslides, and Temperature Extremes
- **AND** choosing any category SHALL request the matching API `category` value

#### Scenario: Markers and badges are category-specific

- **WHEN** the frontend renders events across the v1.1 supported category set
- **THEN** event cards, detail views, and map markers SHALL use category-specific labels and visual variants
- **AND** landslide and temperature extreme events SHALL NOT render as wildfire fallback styling
