## MODIFIED Requirements

### Requirement: Situational Context API

The system SHALL resolve a geographic location for the caller and return nearby events for that location, **regardless of event status**.

⚠️ **The previous wording promised "open events" and was false.** `GetNearbyEvents` filters on geometry and distance only — it has never had a status predicate, so closed events were always returned. The requirement is corrected to match the implementation rather than the implementation narrowed to match it: closed-event ingestion was added deliberately (a recently-closed flood near you is still situational awareness), `status` is present on every event so clients can distinguish, and filtering here would silently remove data callers see today.

⚠️ **Amended from "resolve the caller's IP address".** That is no longer the only path: a caller may state a location explicitly, and the IP path is now bound by the trust policy below. The previous wording described behaviour the system no longer has.

Location SHALL be determined by this precedence, and the response SHALL report which step produced it:

1. explicit `lat`/`lng` supplied by the caller
2. a development environment override
3. the resolved client IP

The system SHALL honour forwarded headers (`X-Forwarded-For`, `X-Real-IP`) **only** when the immediate peer is a configured trusted proxy — on this endpoint as well as on rate limiting. There SHALL be exactly one client-IP resolution path in the service.

An explicit coordinate that is unparseable or out of range SHALL be rejected. It SHALL NOT fall back to IP geolocation, because that would make a typo indistinguishable from a working query.

#### Scenario: IP resolves to a known location

- **WHEN** a client sends `GET /v1/context` from an address that resolves
- **THEN** the API SHALL return HTTP 200 with a `location` object
- **AND** `location_source` SHALL be `ip`

#### Scenario: A forged forwarded header from an untrusted peer is ignored

- **WHEN** a request carrying `X-Forwarded-For` or `X-Real-IP` arrives from a peer that is not a configured trusted proxy
- **THEN** the system SHALL use the peer address and disregard the headers
- **AND** the returned location SHALL NOT reflect the forged address
- **AND** this SHALL hold identically for rate limiting and for geolocation

#### Scenario: A trusted proxy is still believed

- **WHEN** a request carrying `X-Forwarded-For` arrives from a configured trusted proxy
- **THEN** the system SHALL resolve to the first entry in that header
- **AND** the reverse-proxy deployment SHALL continue to geolocate real clients correctly

#### Scenario: An explicit location takes precedence

- **WHEN** a client sends `GET /v1/context?lat=6.5244&lng=3.3792`
- **THEN** the API SHALL return context for those coordinates rather than for the caller's IP
- **AND** `location_source` SHALL be `explicit`
- **AND** `country` and `state` MAY be empty, because the system holds no point-to-administrative-name resolver and SHALL NOT guess

#### Scenario: An invalid coordinate is rejected, not silently ignored

- **WHEN** a client supplies a `lat` or `lng` that is unparseable, out of range, or missing its pair
- **THEN** the API SHALL return HTTP 400 naming the offending parameter
- **AND** it SHALL NOT fall back to IP geolocation

#### Scenario: Nearby events are not filtered by status

- **WHEN** the area around the resolved location contains both open and closed events
- **THEN** the API SHALL return both, ordered by distance
- **AND** each event SHALL carry its `status` so the caller can distinguish them

#### Scenario: Location cannot be resolved

- **WHEN** the caller's IP cannot be resolved to a known location, or no geolocation database is available
- **THEN** the API SHALL return HTTP 200 with a null `location`
- **AND** `location_source` SHALL be `unavailable`
- **AND** SHALL NOT return any 4xx or 5xx response
