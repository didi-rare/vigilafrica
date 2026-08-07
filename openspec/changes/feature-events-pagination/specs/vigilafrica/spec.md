## MODIFIED Requirements

### Requirement: Public Events API

The system SHALL expose a paginated, filterable REST API for accessing enriched natural event data.

Listing results SHALL be returned in a **deterministic total order** built only from values that are stable under ingestion. Ordering SHALL NOT include a column that routine ingestion rewrites, and SHALL terminate in a unique immutable identifier.

⚠️ This requires that re-ingesting existing events does not reorder results. It does **not** claim exactly-once paging under arbitrary concurrent writes: a genuinely new event can still shift later rows by one position. Removing that too would require keyset pagination or a consistent snapshot, neither of which is in scope.

#### Scenario: Listing events by category

- **WHEN** a client sends `GET /v1/events?category=floods`
- **THEN** the API SHALL return a 200 response with only flood events in the `data` array
- **AND** the response SHALL include a `meta` block with `total`, `limit`, and `offset` fields

#### Scenario: Empty result is not an error

- **WHEN** a client queries with filters that match no events
- **THEN** the API SHALL return HTTP 200 with `{"data":[],"meta":{"total":0,...}}`
- **AND** SHALL NOT return a 404 or 500 response

#### Scenario: Paging a static result set returns each event exactly once

- **WHEN** a client walks a filtered result set by increasing `offset` in steps of `limit`, with no events added in the meantime
- **THEN** every matching event SHALL appear in exactly one page
- **AND** no event SHALL appear in two pages
- **AND** this SHALL hold even when events tie on `event_date`

#### Scenario: Re-ingesting existing events does not reorder results

- **WHEN** an ingestion run re-upserts events that are already stored, updating their ingestion timestamp
- **THEN** the order of results SHALL be unchanged
- **AND** a client paging through the collection SHALL NOT see a duplicate or a skipped event as a result
- **AND** this SHALL hold because the ingestion timestamp is not part of the ordering, not merely because ties are broken

#### Scenario: The residual limit is acknowledged, not concealed

- **WHEN** a genuinely new event is stored that sorts ahead of a client's current page
- **THEN** later rows may shift by one position, and the client may see one event twice or miss one
- **AND** this limitation SHALL be documented rather than claimed to be solved

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
