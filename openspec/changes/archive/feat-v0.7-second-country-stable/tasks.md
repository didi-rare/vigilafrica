---
change_id: feat-v0.7-second-country-stable
status: in_progress
---

# Tasks: v0.7 — Second Country Stable

## Group A — Database & Go Backend

- [x] **A1** Write `000007_add_country_to_ingestion_runs.up.sql` — `ALTER TABLE ingestion_runs ADD COLUMN country_code TEXT NOT NULL DEFAULT 'NG'` + index
- [x] **A2** Add `CountryCode string` to `models.IngestionRun`
- [x] **A3** Update `CreateIngestionRun(ctx, startedAt, countryCode string)` — add param, update INSERT
- [x] **A4** Add `GetLastIngestionRunAllCountries(ctx)` — `DISTINCT ON (country_code)` query returning `map[string]*models.IngestionRun`
- [x] **A5** Add `GetEnrichmentStats(ctx)` query + `EnrichmentStat` model
- [x] **A6** Add `GetDistinctStatesByCountry(ctx, country string)` query
- [x] **A7** Update `Repository` interface with all new signatures
- [x] **A8** Update `ingestor/eonet.go` — pass `country.Code` to `CreateIngestionRun`
- [x] **A9** Update `handlers/health.go` — call `GetLastIngestionRunAllCountries`, add `last_ingestion_by_country` to response; bump version to "0.7.0"
- [x] **A10** Write `handlers/enrichment_stats.go` — `GET /v1/enrichment-stats`
- [x] **A11** Write `handlers/states.go` — `GET /v1/states?country=`
- [x] **A12** Register new routes in `api/cmd/server/main.go`

## Group B — Frontend

- [x] **B1** Add `fetchStates(country?: string): Promise<string[]>` to `web/src/api/events.ts`
- [x] **B2** Add country + state + category filter state + controls to `EventsDashboard.tsx`
- [x] **B3** Wire filter state to `useQuery` key and `fetchEvents` params
- [x] **B4** Add `COUNTRY_CENTERS` map; update `mapCenter` logic to use selected country or IP geolocation
- [x] **B5** Fix subtitle: "Nigerian administrative boundaries" → "African administrative boundaries"
- [x] **B6** Fix `App.tsx` copy: prototype banner, hero desc, Poll step desc, status card body; update `milestones.json`

## Group C — Documentation & Scripts

- [x] **C1** Write `openspec/specs/vigilafrica/ghana-country-notes.md`
- [x] **C2** Write `scripts/generate_boundary_migration.py`
- [x] **C3** Write `scripts/README.md`

## Execution Order

```
A1 → A2 → A3, A4, A5, A6 (parallel) → A7 → A8, A9, A10, A11 (parallel) → A12
B1 → B2 → B3, B4 (parallel) → B5, B6 (parallel)
C1, C2, C3 (parallel, independent)
```
