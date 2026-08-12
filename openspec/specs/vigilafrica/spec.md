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

Status coverage SHALL be obtained with **two separate upstream requests per
country per run** — an unwindowed `status=open` query and a `status=closed`
query bounded to a recent-days window. The EONET v3 API accepts only `open`,
`closed`, or `all` for `status` and silently degrades an unrecognised value to
open-only, so the two concerns cannot be expressed in a single request.
Long-lived events (wildfires can stay open for years) SHALL NOT be constrained
by a days window, while recently-closed events — floods typically close within
~48 hours — SHALL be captured by the windowed closed query.

#### Scenario: Scheduled event polling

- **WHEN** the ingestor worker runs on its configured interval (default 60 minutes)
- **THEN** it SHALL fetch events from EONET filtered to each configured country bounding box
- **AND** the open-status request SHALL carry no days window, so long-burning events are never dropped
- **AND** store each event with its ingested timestamp and source identifier

#### Scenario: Recently-closed events are ingested

- **WHEN** the ingestor polls a configured country
- **THEN** it SHALL issue a second request for events with `status=closed`, bounded to a recent-days window (30 days)
- **AND** it SHALL union those results with the open-query results
- **AND** an event returned by both requests SHALL be persisted once, de-duplicated on its source identifier
- **AND** the fetched-event count SHALL be treated as records seen upstream, not a distinct-event count

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

The smallest-matching-polygon tie-break SHALL be resolved using a **persisted**
boundary area rather than recomputing it per candidate row on every insert. The
persisted area SHALL be maintained by the database itself so that it cannot
diverge from the geometry it describes.

Candidate ordering SHALL be a **total order**: area alone does not distinguish
polygons of equal area, so a stable unique identifier SHALL be applied as the
final ordering term. Without it the selected boundary is undefined whenever two
intersecting candidates have equal area, and no preservation guarantee can hold.

This is a performance and integrity change only: it SHALL NOT alter which
boundary any event resolves to, except that cases previously left undefined by
equal areas SHALL become deterministic.

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

#### Scenario: Persisted boundary area stays consistent with its geometry

- **WHEN** an administrative boundary row is inserted
- **THEN** the database SHALL populate its persisted area without the caller supplying it
- **AND WHEN** that row's `geom` is subsequently updated
- **THEN** the database SHALL recompute the persisted area automatically
- **AND** the persisted area SHALL equal the area computed directly from the geometry

#### Scenario: Equal-area candidates resolve deterministically

- **WHEN** an event's geometry intersects two or more boundaries of exactly equal area
- **THEN** the enricher SHALL select the same boundary every time
- **AND** the selection SHALL NOT depend on the query plan or on physical row order

#### Scenario: Precomputing area does not change enrichment results

- **WHEN** the same set of event coordinates is enriched before and after the area is persisted
- **THEN** every event SHALL resolve to an identical `state_name` and `country_name`
- **AND** the change SHALL be reversible, restoring the prior behaviour without data loss

### Requirement: Public Health API

The system SHALL expose a public REST API providing health status without any database dependency.

#### Scenario: Health endpoint is always reachable

- **WHEN** a client sends `GET /health`
- **THEN** the API SHALL return HTTP 200 with `{"status":"ok","version":"<semver>"}` within 100ms (p99)
- **AND** the endpoint SHALL respond whether or not the database is available

---

### Requirement: Public Events API

The system SHALL expose a paginated, filterable REST API for accessing enriched natural event data.

Listing results SHALL be returned in a **deterministic total order** terminating in a unique immutable identifier. Ordering SHALL NOT include a column that **every** ingestion cycle rewrites regardless of whether the event changed.

⚠️ **Precisely what this does and does not require.** It requires that re-ingesting an event whose content is unchanged does not reorder results — the routine case, since every tick re-upserts every live event. It does **not** claim exactly-once paging under arbitrary writes. A row can still move when:

- its `event_date` is revised (a NULL date becoming known, or an upstream correction) — the upsert does write this column;
- a genuinely new event sorts ahead of the client's current page;
- re-ingestion changes a **filtered** column (`category`, `status`, or the geometry behind `country_name`/`state_name`), so the row leaves or joins the filtered set ahead of the client's offset.

These are properties of offset pagination over a live table, not of the ordering. Removing them would require keyset pagination or a consistent snapshot, neither of which is in scope.

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

#### Scenario: Re-ingesting UNCHANGED events does not reorder results

- **WHEN** an ingestion run re-upserts events that are already stored and whose content is unchanged, updating only their ingestion timestamp
- **THEN** the order of results SHALL be unchanged
- **AND** a client paging through the collection SHALL NOT see a duplicate or a skipped event as a result
- **AND** this SHALL hold because the ingestion timestamp is not part of the ordering, not merely because ties are broken

#### Scenario: The residual limits are acknowledged, not concealed

- **WHEN** a genuinely new event is stored that sorts ahead of a client's current page
- **OR** an existing event's `event_date` is revised by re-ingestion
- **OR** re-ingestion changes an event's `category`, `status`, or resolved location so that it leaves or joins the client's filtered set ahead of the current offset
- **THEN** later rows may shift by one position, and the client may see one event twice or miss one
- **AND** each of these limitations SHALL be documented rather than claimed to be solved

#### Scenario: Offset beyond the end of the collection

- **WHEN** a client requests an `offset` greater than or equal to `meta.total`
- **THEN** the API SHALL return HTTP 200 with an empty `data` array
- **AND** `meta.total` SHALL still report the true count of matching events

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

