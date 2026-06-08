# Spec: Secondary Data Oracle — GDACS (feature-secondary-oracle)

**Status:** Proposed — phased implementation begins after v1.3.
**Companion:** [`openspec/proposals/feature-secondary-oracle.md`](../proposals/feature-secondary-oracle.md) (rationale, design decisions, risks, open questions).

## Context

EONET is reliable but is a single point of failure and a single source of truth
(R-01, 2026-05-27 business/market review). This spec details the components a
GDACS secondary oracle would touch. The internal `models.Event` is already
source-neutral (`Source` is a plain `string`, defaulting to `'eonet'`), so the
work is: add a source abstraction, a GDACS fetch+normalize path, a per-source
dedup key, a cross-source correlation pass, and per-source observability — without
changing EONET's behaviour.

## Components to Touch

### New files
1. `api/internal/ingestor/source.go` — the `Source` interface (`Name()`,
   `Fetch(ctx, CountryConfig) ([]models.Event, []string, error)`).
2. `api/internal/ingestor/eonet_source.go` — `EONETSource` wrapping the existing
   `eonet.go` fetch + `normalizer.Normalize`, passing `"eonet"` explicitly.
3. `api/internal/ingestor/gdacs.go` — `GDACSSource`: fetch from
   `https://www.gdacs.org/gdacsapi/api/events/geteventlist/SEARCH` (GeoJSON),
   filter `FL,WF` + bbox, paginate, isolated retry/backoff.
4. `api/internal/normalizer/gdacs_normalizer.go` — `RawGDACSFeature` →
   `models.Event` (+ GeoJSON); `FL→floods`, `WF→wildfires`;
   `SourceID="<eventtype>-<eventid>"`; capture alert level into `raw_payload`.
5. `api/internal/database/correlation.go` — the correlation pass (link
   same-category, spatially+temporally proximate events across sources).
6. `api/db/migrations/0000XX_secondary_oracle.{up,down}.sql` — see Schema.

### Modified files
1. `api/internal/normalizer/normalizer.go` — `Normalize` gains a `source string`
   param; remove the hardcoded `Source: "eonet"`.
2. `api/internal/ingestor/scheduler.go` — iterate registered sources × countries;
   per-source interval (`GDACS_INGEST_INTERVAL_MIN`) + per-source advisory lock;
   write `ingestion_runs.source`.
3. `api/internal/database/db.go` — `UpsertEvent` `ON CONFLICT (source, source_id)`;
   `CreateIngestionRun` records `source`; `ListEvents`/`GetNearbyEvents` collapse
   to canonical + add `sources`/`source_count`; add a `GetLastIngestionRunBySource`.
4. `api/internal/handlers/*` (health) — add `last_ingestion_by_source`.
5. `api/cmd/server/main.go` — register EONET + GDACS sources; read
   `GDACS_INGEST_INTERVAL_MIN`.
6. `openspec/specs/vigilafrica/api-contract.md` + `openspec/specs/vigilafrica/openapi.yaml`
   (then `npm run sync:openapi`) — additive `sources`/`source_count` fields.

## Schema

```sql
-- up
ALTER TABLE events DROP CONSTRAINT events_source_id_key;
ALTER TABLE events ADD CONSTRAINT events_source_source_id_key UNIQUE (source, source_id);
ALTER TABLE events ADD COLUMN correlated_event_id UUID NULL
    REFERENCES events(id) ON DELETE SET NULL;
CREATE INDEX idx_events_correlated ON events(correlated_event_id);
ALTER TABLE ingestion_runs ADD COLUMN source TEXT NOT NULL DEFAULT 'eonet';
CREATE INDEX idx_ingestion_runs_source_started ON ingestion_runs(source, started_at DESC);
```

`UNIQUE (source, source_id)` is a superset of the old `UNIQUE (source_id)` for the
existing all-`eonet` data → safe, no backfill. The `down` migration restores
`UNIQUE (source_id)` and drops the added columns/indexes.

## Config (new env vars)

- `GDACS_INGEST_INTERVAL_MIN` (default 60; `0` disables GDACS → exact current
  behaviour).
- `CORRELATION_RADIUS_KM` (default ~25), `CORRELATION_WINDOW_HOURS` (default ~72)
  — correlation thresholds, conservative, overridable.

## Implementation Plan (phased — see proposal for rationale)

1. **Phase 1 — Plumbing, no behaviour change:** `Source` interface; `EONETSource`;
   `Normalize(source)`; migration (compound unique + `ingestion_runs.source` +
   `correlated_event_id`); `UpsertEvent` conflict-target change. EONET-only.
2. **Phase 2 — GDACS ingest:** `gdacs.go` + `gdacs_normalizer.go`; per-source
   scheduling + runs. Both sources ingest (interim duplicates acceptable).
3. **Phase 3 — Correlation + canonical API:** `correlation.go`; `sources` /
   `source_count` in API; default list collapses correlated duplicates.
4. **Phase 4 — Observability:** per-source `/health`; one-vs-all-stale alerting.

Each phase is its own PR with its own OpenSpec record (ADR-010 Sentinel gate now
requires it for `api/internal/`, `api/cmd/` changes).

## Acceptance Criteria

- [ ] EONET ingestion behaviour is unchanged behind the `Source` interface
      (existing ingestor/normalizer tests pass without modification).
- [ ] Migration applies with no row loss on all-EONET data; `down` restores the
      original `UNIQUE (source_id)`.
- [ ] `events` dedups on `(source, source_id)`; an EONET and a GDACS event sharing
      a raw ID coexist as two rows.
- [ ] GDACS FL/WF events inside the NG/GH bboxes are ingested, mapped, and stored
      with `source='gdacs'`, `source_id='<type>-<id>'`.
- [ ] Correlation links same-category cross-source events within
      `CORRELATION_RADIUS_KM` + `CORRELATION_WINDOW_HOURS`; idempotent; never
      merges across categories or within a single source.
- [ ] Default event API returns one canonical entry per cluster with
      `sources`/`source_count`; secondary rows retained + retrievable; OpenAPI
      updated and CI `Check OpenAPI spec in sync` passes.
- [ ] `GDACS_INGEST_INTERVAL_MIN=0` reproduces exact current behaviour.
- [ ] `/health` reports `last_ingestion_by_source`.

## Verification Plan

- [ ] **Unit:** GDACS normalizer (FL/WF mapping, ID formatting, bbox filter,
      alert-level capture); correlation predicate (table-driven: same/different
      category, in/out of spatial+temporal thresholds, idempotency); EONET
      normalizer unchanged.
- [ ] **Integration (testcontainers-go via `scripts/test-api.ps1 -Integration`):**
      seed both sources incl. one overlapping wildfire → two source rows, one
      canonical `source_count=2`; assert `(source, source_id)` dedup and per-source
      `ingestion_runs`.
- [ ] **Resilience:** GDACS 5xx/outage → EONET ingest + run status unaffected;
      `/health` shows only GDACS stale.
- [ ] **Migration:** up on an all-EONET snapshot → no row loss; `down` restores
      `UNIQUE (source_id)`.
- [ ] `go vet ./...`, `go test ./...` (Docker runner), and `npm run build` green.
