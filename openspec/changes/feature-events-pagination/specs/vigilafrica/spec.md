## MODIFIED Requirements

### Requirement: Public Events API

The system SHALL expose a paginated, filterable REST API for accessing enriched natural event data.

Listing results SHALL be returned in a **deterministic total order**, so that a client walking the collection page by page observes every matching event exactly once. Ordering SHALL NOT depend on values that change during ingestion without a stable final tiebreaker.

#### Scenario: Listing events by category

- **WHEN** a client sends `GET /v1/events?category=floods`
- **THEN** the API SHALL return a 200 response with only flood events in the `data` array
- **AND** the response SHALL include a `meta` block with `total`, `limit`, and `offset` fields

#### Scenario: Empty result is not an error

- **WHEN** a client queries with filters that match no events
- **THEN** the API SHALL return HTTP 200 with `{"data":[],"meta":{"total":0,...}}`
- **AND** SHALL NOT return a 404 or 500 response

#### Scenario: Paging returns each event exactly once

- **WHEN** a client walks a filtered result set by increasing `offset` in steps of `limit`
- **THEN** every matching event SHALL appear in exactly one page
- **AND** no event SHALL appear in two pages
- **AND** this SHALL hold even when events tie on `event_date`

#### Scenario: Ordering is stable while ingestion is running

- **WHEN** an ingestion run updates events, resetting their `ingested_at`, while a client is paging
- **THEN** the relative order of already-returned events SHALL remain determined by a unique tiebreaker
- **AND** the client SHALL NOT observe a duplicate or a skipped event caused by ties reordering

#### Scenario: Offset beyond the end of the collection

- **WHEN** a client requests an `offset` greater than or equal to `meta.total`
- **THEN** the API SHALL return HTTP 200 with an empty `data` array
- **AND** `meta.total` SHALL still report the true count of matching events

---

### Requirement: Interactive Map Frontend

The system SHALL provide a web-based interactive map using MapLibre GL JS that visualises the current state of natural events.

The frontend SHALL make the size of the result set visible to the user. When the number of matching events exceeds what is displayed, the interface SHALL say so explicitly and provide a means of reaching the remainder. **The system SHALL NOT present a truncated result set as if it were complete.**

#### Scenario: Event markers on load

- **WHEN** a user loads the VigilAfrica frontend
- **THEN** the map SHALL display event markers for the events on the current page of `GET /v1/events`
- **AND** markers SHALL be colour-coded by event category

#### Scenario: Truncated results are disclosed, never silent

- **WHEN** the number of events matching the active filters exceeds the page size
- **THEN** the interface SHALL display the total number of matching events alongside the range currently shown
- **AND** it SHALL offer navigation to the remaining pages
- **AND** it SHALL NOT imply that the visible events are the complete set

#### Scenario: Changing a filter returns to the first page

- **WHEN** a user changes the country, state, or category filter while viewing any page after the first
- **THEN** the interface SHALL request the first page of the new result set
- **AND** it SHALL NOT display an empty page caused by carrying the previous offset forward

#### Scenario: Map and list agree

- **WHEN** a user navigates to a different page of results
- **THEN** the map markers SHALL correspond to the events listed on that same page
