# VigilAfrica — API Contract Specification

**Version**: 1.0
**Status**: LOCKED — Approved 2026-04-12
**Maintained by**: @didi-rare

> **Governance rule**: All API changes (new endpoints, new query params, schema changes, behaviour changes) must update this document FIRST. Any implementation that diverges from this contract is drift and will be caught by OpenSpec validation. Agents must not add endpoints or fields not defined here.

---

## 1. Global Conventions

### Base URLs

| Environment | URL                                  |
|-------------|--------------------------------------|
| Local dev   | `http://localhost:8080`              |
| Staging     | `https://api-staging.vigilafrica.org` |
| Production  | `https://api.vigilafrica.org`        |

### Content Type

- All requests with a body: `Content-Type: application/json`
- All responses: `Content-Type: application/json`

### Error Response Format

All error responses use the same shape, regardless of status code:

```json
{
  "error": "<human-readable error description>"
}
```

Stack traces, internal error messages, database errors, and file paths are **never** exposed in API responses.

### Timestamp Format

All timestamps are ISO 8601 in UTC: `2026-04-12T13:00:00Z`

### UUID Format

All IDs are UUID v4: `550e8400-e29b-41d4-a716-446655440000`

### Pagination

Paginated endpoints use query params `?limit=<n>&offset=<n>`.

| Param    | Default | Min | Max | Description              |
|----------|---------|-----|-----|--------------------------|
| `limit`  | 50      | 1   | 200 | Results per page         |
| `offset` | 0       | 0   | —   | Number of results to skip |

Paginated responses always include a `meta` block:

```json
{
  "data": [...],
  "meta": {
    "total": 142,
    "limit": 50,
    "offset": 0
  }
}
```

### CORS

CORS is enabled and the allowed `Origin` is set via the `CORS_ORIGIN` environment variable. In production this is the Vercel deployment domain. In local dev, `CORS_ORIGIN=*` is acceptable.

### Null Handling

- Fields that may be absent use `null` in the JSON response — fields are **never omitted** from the response shape
- Array fields are always `[]` (empty array) when empty — never `null`

---

## 2. Endpoint: GET /health

**Feature**: F-001
**Milestone**: v0.1
**Auth**: None

### Description

Returns the health status of the API service. No database dependency — this endpoint must respond even if the database is unavailable.

### Request

```
GET /health
```

No query parameters. No request body.

### Response: 200 OK

```json
{
  "status": "ok",
  "version": "0.1.0"
}
```

| Field     | Type   | Description                                         |
|-----------|--------|-----------------------------------------------------|
| `status`  | string | Always `"ok"` when the API is running               |
| `version` | string | Semantic version, injected at build time via ldflags |

### Performance Contract

Response time: **< 100ms** (p99)

---

## 3. Endpoint: GET /v1/events

**Feature**: F-006
**Milestone**: v0.3
**Auth**: None

### Description

Returns a paginated list of enriched natural events. Supports filtering by category, state, and status.

### Request

```
GET /v1/events[?category=<value>][&state=<value>][&status=<value>][&limit=<n>][&offset=<n>]
```

### Query Parameters

| Param      | Type    | Allowed Values            | Default      | Description                              |
|------------|---------|---------------------------|--------------|------------------------------------------|
| `category` | string  | `floods`, `wildfires`     | — (all)      | Filter by event category                 |
| `state`    | string  | Any Nigerian state name    | — (all)      | Filter by enriched state name (case-insensitive) |
| `status`   | string  | `open`, `closed`          | `open`       | Filter by event status                   |
| `limit`    | integer | 1–200                     | 50           | Maximum results per page                 |
| `offset`   | integer | ≥ 0                       | 0            | Pagination offset                         |

### Response: 200 OK

```json
{
  "data": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "source_id": "EONET_5678",
      "source": "eonet",
      "title": "Flooding in Benue State",
      "category": "floods",
      "status": "open",
      "geometry_type": "Point",
      "latitude": 7.33,
      "longitude": 8.13,
      "country_name": "Nigeria",
      "state_name": "Benue",
      "event_date": "2026-04-10T00:00:00Z",
      "ingested_at": "2026-04-12T08:00:00Z"
    }
  ],
  "meta": {
    "total": 1,
    "limit": 50,
    "offset": 0
  }
}
```

### EventSummary Schema

| Field           | Type            | Nullable | Description                                              |
|-----------------|-----------------|----------|----------------------------------------------------------|
| `id`            | UUID string     | No       | Internal VigilAfrica event UUID                          |
| `source_id`     | string          | No       | Original EONET event ID (e.g., `EONET_5678`)            |
| `source`        | string          | No       | Data source — always `"eonet"` in MVP                   |
| `title`         | string          | No       | Human-readable event title from EONET                   |
| `category`      | string          | No       | `"floods"` or `"wildfires"`                             |
| `status`        | string          | No       | `"open"` or `"closed"`                                  |
| `geometry_type` | string          | Yes      | `"Point"` or `"Polygon"`. `null` if no geometry         |
| `latitude`      | number          | Yes      | Centroid latitude. `null` for non-Point or no geometry  |
| `longitude`     | number          | Yes      | Centroid longitude. `null` for non-Point or no geometry |
| `country_name`  | string          | Yes      | `"Nigeria"` or `null` if not yet enriched               |
| `state_name`    | string          | Yes      | Nigerian state name or `null` if outside Nigeria        |
| `event_date`    | ISO 8601 string | Yes      | Original event timestamp from EONET                     |
| `ingested_at`   | ISO 8601 string | No       | When VigilAfrica ingested this event                    |

### Response: 400 Bad Request

Returned for invalid query parameter values:

```json
{
  "error": "invalid category: 'earthquake'. valid values: floods, wildfires"
}
```

```json
{
  "error": "invalid limit: must be between 1 and 200"
}
```

### Empty Result

```json
{
  "data": [],
  "meta": {
    "total": 0,
    "limit": 50,
    "offset": 0
  }
}
```

The `data` field is **always an array**, never `null`.

---

## 4. Endpoint: GET /v1/events/:id

**Feature**: F-007
**Milestone**: v0.3
**Auth**: None

### Description

Returns the full detail of a single event by its internal UUID.

### Request

```
GET /v1/events/:id
```

| Path Param | Type        | Description              |
|------------|-------------|--------------------------|
| `id`       | UUID string | Internal VigilAfrica UUID |

### Response: 200 OK

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "source_id": "EONET_5678",
  "source": "eonet",
  "title": "Flooding in Benue State",
  "category": "floods",
  "status": "open",
  "geometry_type": "Point",
  "latitude": 7.33,
  "longitude": 8.13,
  "country_name": "Nigeria",
  "state_name": "Benue",
  "event_date": "2026-04-10T00:00:00Z",
  "ingested_at": "2026-04-12T08:00:00Z",
  "enriched_at": "2026-04-12T08:01:00Z",
  "source_url": "https://eonet.gsfc.nasa.gov/api/v3/events/EONET_5678"
}
```

### EventDetail Schema

All fields from EventSummary (§3), plus:

| Field        | Type            | Nullable | Description                                      |
|--------------|-----------------|----------|--------------------------------------------------|
| `enriched_at` | ISO 8601 string | Yes      | When spatial enrichment was run. `null` if not yet enriched |
| `source_url`  | string          | Yes      | Direct link to the original EONET event page     |

### Response: 404 Not Found

```json
{
  "error": "event not found"
}
```

### Response: 400 Bad Request

```json
{
  "error": "invalid event id: must be a valid UUID"
}
```

---

## 5. Endpoint: GET /v1/context

**Feature**: F-008
**Milestone**: v0.4
**Auth**: None

### Description

Returns the caller's detected location (via MaxMind GeoLite2 local file — no external API call) and a list of open events in their resolved country and state. Enables the "What is happening near me?" experience.

### Request

```
GET /v1/context
```

No query parameters. No request body. Caller IP is read from:
1. `X-Forwarded-For` header (first IP — from Vercel proxy)
2. Fallback: `RemoteAddr` from the HTTP request

### Response: 200 OK — Location Resolved

```json
{
  "location": {
    "country_code": "NG",
    "country_name": "Nigeria",
    "state_name": "Benue"
  },
  "events": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "title": "Flooding in Benue State",
      "category": "floods",
      "status": "open",
      "state_name": "Benue",
      "country_name": "Nigeria",
      "event_date": "2026-04-10T00:00:00Z"
    }
  ]
}
```

### ContextLocation Schema

| Field          | Type   | Description                                   |
|----------------|--------|-----------------------------------------------|
| `country_code` | string | ISO 3166-1 alpha-2 country code (e.g., `"NG"`) |
| `country_name` | string | English country name (e.g., `"Nigeria"`)       |
| `state_name`   | string | English state/subdivision name. May be `null` if only country resolved |

### ContextEvent Schema (simplified EventSummary)

| Field          | Type            | Description                    |
|----------------|-----------------|--------------------------------|
| `id`           | UUID string     | Internal event UUID             |
| `title`        | string          | Event title                     |
| `category`     | string          | `"floods"` or `"wildfires"`   |
| `status`       | string          | `"open"` or `"closed"`        |
| `state_name`   | string \| null  | Nigerian state name             |
| `country_name` | string \| null  | Country name                    |
| `event_date`   | ISO 8601 string | Event timestamp                 |

### Response: 200 OK — Location Not Resolved

```json
{
  "location": null,
  "events": []
}
```

> **Critical behaviour**: This endpoint **always returns HTTP 200**. Location resolution failure is a graceful degradation, not an error. A `4xx` or `5xx` response must never be returned for a failed IP lookup.

### Performance Contract

Response time: **< 200ms** (p99). The GeoIP lookup is a local file read — no network calls permitted.

---

## 6. HTTP Status Code Reference

| Code | Meaning              | When Used                                               |
|------|----------------------|---------------------------------------------------------|
| 200  | OK                   | Successful response (including empty results)           |
| 400  | Bad Request          | Invalid query param value, non-UUID path param          |
| 404  | Not Found            | Event ID does not exist in the database                 |
| 405  | Method Not Allowed   | Wrong HTTP method (e.g., POST to a GET endpoint)        |
| 500  | Internal Server Error| Unrecoverable error (database down, panic recovered)    |

### 500 Response

```json
{
  "error": "internal server error"
}
```

Stack traces and internal details are **never** included in the 500 response.

---

## 7. Environment Variables

All configuration is via environment variables. No hardcoded values in source code.

| Variable                  | Required | Default                     | Description                                                     |
|---------------------------|----------|-----------------------------|-----------------------------------------------------------------|
| `DATABASE_URL`            | Yes      | —                           | PostgreSQL DSN: `postgres://user:pass@host:5432/dbname`         |
| `GEOIP_DB_PATH`           | Yes      | `/data/GeoLite2-City.mmdb`  | Absolute path to MaxMind GeoLite2-City `.mmdb` file             |
| `API_PORT`                | No       | `8080`                      | HTTP server port                                                |
| `CORS_ORIGIN`             | No       | `*`                         | Allowed CORS origin (set to Vercel domain in production)        |
| `INGEST_INTERVAL_MINUTES` | No       | `60`                        | Polling interval for scheduled ingestion (F-012)                |
| `RATE_LIMIT_RPM`          | No       | `60`                        | API rate limit in requests per minute (v0.5+)                   |
| `LOG_LEVEL`               | No       | `info`                      | Structured log level: `debug`, `info`, `warn`, `error`          |
| `APP_VERSION`             | No       | `dev`                       | Injected at build time via `-ldflags "-X main.version=0.1.0"`  |
| `VITE_API_BASE_URL`       | No       | `http://localhost:8080`     | Frontend env var — API base URL for `fetch()` calls            |
