---
change_id: feat-v0.7-second-country-stable
status: proposed
created_date: 2026-04-18
author: Claude Code
roadmap_ref: openspec/specs/vigilafrica/roadmap.md §v0.7
deferred_from: project_v06_deferred_notes.md Notes B, C, D
---

# Technical Design: v0.7 — Second Country Stable

## Scope Boundary

All changes are confined to:
- `api/db/migrations/000007_*`
- `api/internal/database/`
- `api/internal/handlers/`
- `api/internal/ingestor/`
- `api/internal/models/`
- `web/src/api/events.ts`
- `web/src/components/EventsDashboard.tsx`
- `web/src/App.tsx`
- `scripts/`
- `openspec/specs/vigilafrica/ghana-country-notes.md`

No changes to: routing, auth, alerter, normalizer, PostGIS schema.

---

## 1. Database — Per-Country ingestion_runs (Note B)

### Migration `000007_add_country_to_ingestion_runs.up.sql`

```sql
ALTER TABLE ingestion_runs
  ADD COLUMN IF NOT EXISTS country_code TEXT NOT NULL DEFAULT 'NG';

CREATE INDEX IF NOT EXISTS idx_ingestion_runs_country_started
  ON ingestion_runs (country_code, started_at DESC);
```

`DEFAULT 'NG'` backfills historical rows as Nigeria (all runs prior to this migration were Nigeria-only).

### `api/internal/models/ingestion_run.go` — extend `IngestionRun`

Add `CountryCode string` field.

### `api/internal/database/queries.go` — updated signatures

```go
// CreateIngestionRun now accepts countryCode.
func (r *pgRepo) CreateIngestionRun(ctx context.Context, startedAt time.Time, countryCode string) (int64, error)

// GetLastIngestionRunByCountry returns the most recent run for a specific country.
func (r *pgRepo) GetLastIngestionRunByCountry(ctx context.Context, countryCode string) (*models.IngestionRun, error)

// GetLastIngestionRunAllCountries returns one row per country (most recent).
// Used by /health to build the last_ingestion_by_country map.
func (r *pgRepo) GetLastIngestionRunAllCountries(ctx context.Context) (map[string]*models.IngestionRun, error)
```

`GetLastIngestionRunAllCountries` uses a `DISTINCT ON (country_code) ORDER BY country_code, started_at DESC` query.

### Repository interface (`api/internal/database/repository.go`)

Add new signatures to the `Repository` interface; retain `GetLastIngestionRun` as a deprecated alias pointing to the Nigeria-scoped query to preserve /health backward compat during transition.

### `api/internal/ingestor/eonet.go` — pass country code

`Ingest(ctx, repo, country)` passes `country.Code` to `repo.CreateIngestionRun`.

### `api/internal/handlers/health.go` — updated response

```json
{
  "status": "ok",
  "version": "0.7.0",
  "last_ingestion": { ... },
  "last_ingestion_by_country": {
    "NG": { "status": "success", "completed_at": "...", "events_stored": 12 },
    "GH": { "status": "success", "completed_at": "...", "events_stored": 3 }
  }
}
```

`last_ingestion` (global, for backward compat) stays unchanged. `last_ingestion_by_country` is additive.

---

## 2. Backend — Enrichment Stats Endpoint

### New endpoint: `GET /v1/enrichment-stats`

**Query (database/queries.go):**

```sql
SELECT
  country_name,
  COUNT(*)                                                   AS total_events,
  COUNT(*) FILTER (WHERE state_name IS NOT NULL)             AS enriched_events,
  ROUND(
    COUNT(*) FILTER (WHERE state_name IS NOT NULL)::numeric /
    NULLIF(COUNT(*), 0) * 100, 1
  )                                                          AS success_rate_pct
FROM events
GROUP BY country_name
ORDER BY country_name;
```

**Response shape:**

```json
{
  "stats": [
    { "country_name": "Ghana",   "total_events": 8,  "enriched_events": 7,  "success_rate_pct": 87.5 },
    { "country_name": "Nigeria", "total_events": 45, "enriched_events": 42, "success_rate_pct": 93.3 },
    { "country_name": null,      "total_events": 2,  "enriched_events": 0,  "success_rate_pct": 0.0 }
  ]
}
```

`country_name: null` row covers events that fell outside all boundaries — a border/edge-case diagnostic signal.

**Handler:** `api/internal/handlers/enrichment_stats.go`  
**Route registration:** `GET /v1/enrichment-stats` added to router in `api/cmd/server/main.go`  
**No auth required** — read-only diagnostic endpoint, rate-limited by existing middleware.

**Repository method:**

```go
type EnrichmentStat struct {
  CountryName     *string
  TotalEvents     int
  EnrichedEvents  int
  SuccessRatePct  float64
}

func (r *pgRepo) GetEnrichmentStats(ctx context.Context) ([]EnrichmentStat, error)
```

---

## 3. Backend — States List Endpoint

The frontend country+state filter needs to know which states exist per country without hardcoding them.

### New endpoint: `GET /v1/states?country=Ghana`

**Query:**

```sql
SELECT DISTINCT state_name
FROM events
WHERE country_name ILIKE $1
  AND state_name IS NOT NULL
ORDER BY state_name;
```

**Response:**

```json
{ "states": ["Ashanti", "Greater Accra", "Northern", "Volta"] }
```

**Handler:** `api/internal/handlers/states.go`  
Accepts optional `?country=` param; returns all distinct states across all countries if omitted.

---

## 4. Frontend — Country + State Filter Controls

### `web/src/api/events.ts` — new fetchStates function

```ts
export async function fetchStates(country?: string): Promise<string[]>
```

### `web/src/components/EventsDashboard.tsx` — filter state

Add local React state:

```ts
const [selectedCountry, setSelectedCountry] = useState<string>('')
const [selectedCategory, setSelectedCategory] = useState<EventCategory | ''>('')
const [selectedState, setSelectedState]    = useState<string>('')
```

**Country filter:** static dropdown — `[All Countries, Nigeria, Ghana]`. No API call needed.

**State filter:** query `fetchStates(selectedCountry)` on country change, populate dropdown. Reset state when country changes.

**Events query key:** `['events', selectedCountry, selectedCategory, selectedState]` — reactive to all three.

**Map center:** per-country centroid map:

```ts
const COUNTRY_CENTERS: Record<string, [number, number]> = {
  'Nigeria': [8.6753, 9.082],
  'Ghana':   [-1.0232, 7.9465],
}
```

If no country selected, use IP geolocation result from context API (existing behavior).

### `web/src/components/EventsDashboard.tsx` — text fixes

- Line 93 subtitle: `"tagged with Nigerian administrative boundaries"` → `"tagged with African administrative boundaries"`

### `web/src/App.tsx` — content fixes

| Location | From | To |
|---|---|---|
| Prototype banner (line 79) | `v0.4 complete · v0.5 Operational Prototype active` | `v0.6 complete · v0.7 Second Country Stable active` |
| Hero desc (line 121) | `"Open-source. Nigeria first."` | `"Open-source. Nigeria and Ghana live."` |
| Poll step desc (line 51) | `"filtered to Nigeria"` | `"filtered to Nigeria and Ghana"` |
| Status card (line 204) | `"The current focus (**v0.5**) is the Operational Prototype"` | `"v0.7 Second Country Stable is active — Ghana enrichment validation in progress."` |

---

## 5. Documentation

### `openspec/specs/vigilafrica/ghana-country-notes.md`

Covers:
- Tier classification: Ghana is **Tier 2** (moderate EONET frequency, 16 post-2019 ADM1 regions, good HDX data)
- EONET bbox overlap validation: Ghana `[-3.5, 4.5, 1.2, 11.2]` vs Nigeria `[2.0, 4.0, 15.0, 14.0]` — max_lon Ghana (1.2) < min_lon Nigeria (2.0), zero overlap confirmed
- Simplified boundary limitations: rectangular approximations cover ≥ 85% of EONET events (point geometries well within state centroids); border ambiguity within ~50km of shared boundaries (Ghana-Togo, Ghana-Burkina Faso)
- Deviations from onboarding template: none — Ghana followed all phases in v0.6
- Upgrade path: replace simplified rectangles with HDX COD GeoJSON using `scripts/generate_boundary_migration.py` (see scripts/README.md)

### `scripts/generate_boundary_migration.py`

Python script (lifted from onboarding template §1.4) that:
1. Accepts a GeoJSON file path + country code + country name
2. Outputs a `000NNN_boundary_<country_code>.up.sql` migration file with real polygon data
3. Wraps output in the same idempotent `DO $$ BEGIN IF NOT EXISTS` pattern used in 000005

### `scripts/README.md`

One-page index: what each script does and when to use it.

---

## 6. Files Created or Modified

### New files
- `api/db/migrations/000007_add_country_to_ingestion_runs.up.sql`
- `api/internal/handlers/enrichment_stats.go`
- `api/internal/handlers/states.go`
- `openspec/specs/vigilafrica/ghana-country-notes.md`
- `scripts/generate_boundary_migration.py`
- `scripts/README.md`

### Modified files
- `api/internal/database/queries.go` — new query methods
- `api/internal/database/repository.go` — extended interface
- `api/internal/models/ingestion_run.go` — add CountryCode field
- `api/internal/ingestor/eonet.go` — pass country.Code to CreateIngestionRun
- `api/internal/handlers/health.go` — add last_ingestion_by_country
- `api/cmd/server/main.go` — register new routes
- `web/src/api/events.ts` — add fetchStates
- `web/src/components/EventsDashboard.tsx` — filter controls + text fix
- `web/src/App.tsx` — copy updates

---

## 7. Acceptance Criteria

All must pass before this change is merged:

| # | Criterion | How to verify |
|---|---|---|
| AC-01 | Ghana events display `state_name` in the frontend (not raw coordinates) | Ingest + check event cards in browser |
| AC-02 | `GET /v1/enrichment-stats` returns ≥ 85% success_rate_pct for Ghana | API call after ingestion |
| AC-03 | `GET /v1/enrichment-stats` returns ≥ 85% success_rate_pct for Nigeria | API call after ingestion |
| AC-04 | `GET /health` returns `last_ingestion_by_country` with both NG and GH entries | API call after one scheduler tick |
| AC-05 | Country filter dropdown shows Nigeria and Ghana; selecting one limits event list | Browser UI test |
| AC-06 | State filter populates correctly after selecting Ghana (Ashanti, Greater Accra, etc.) | Browser UI test |
| AC-07 | `GET /v1/events?country=Nigeria` returns only Nigerian events | curl/API test |
| AC-08 | `GET /v1/events?country=Ghana` returns only Ghanaian events | curl/API test |
| AC-09 | Map center shifts to Ghana centroid when Ghana country filter is selected | Browser UI test |
| AC-10 | Prototype banner reads "v0.6 complete · v0.7 Second Country Stable active" | Browser visual check |
| AC-11 | `scripts/generate_boundary_migration.py` runs without error on a sample GeoJSON | `python scripts/generate_boundary_migration.py --help` |
| AC-12 | `ghana-country-notes.md` exists and documents bbox overlap validation | File review |
| AC-13 | `ingestion_runs` rows written after this migration include `country_code` | DB query after ingest |

---

## 8. Expert Skills Needed for Implementation

| Domain | Skill | Why |
|---|---|---|
| Go backend | `golang-pro` | Migration, repository interface extension, new handlers |
| PostgreSQL | `database-migrations-sql-migrations` | `ALTER TABLE`, `DISTINCT ON`, enrichment stats query |
| React | `react-best-practices` | Filter dropdowns, reactive query key, state resets |
| API design | `api-design-principles` | Enrichment stats + states endpoint shape |
| Python scripting | `python-pro` | generate_boundary_migration.py |

---

## 9. Traceability

| Roadmap criterion | Design section |
|---|---|
| Ghana enrichment "before/after" proof | §4 (filter UI shows state_name), §3 (states endpoint) |
| Enrichment success rate ≥ 85% documented | §2 (/v1/enrichment-stats) |
| Border/edge cases documented | §5 (ghana-country-notes.md) |
| Deviations from template recorded | §5 (ghana-country-notes.md) |
| EONET bbox overlap validation | §5 (ghana-country-notes.md) |
| Frontend state/province filter works for Ghana | §4 (EventsDashboard filter controls) |
| API `?country=` returns correct results for both | AC-07, AC-08 (already implemented in v0.6) |
