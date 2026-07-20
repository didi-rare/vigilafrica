---
id: feature-secondary-oracle
status: proposed
branch: tbd
---

# Proposal: Secondary Data Oracle — GDACS (feature-secondary-oracle)

> **Design proposal — no implementation in this change.** This document promotes
> the long-deferred secondary-oracle idea from a backlog stub to a concrete,
> reviewable design. It is the Day-6 deliverable of the 2026-05-29
> partnership-readiness sprint. It defines *what we would build and why*, grounded
> in the current ingestion pipeline, and is scoped for a phased implementation
> **after** v1.3. It changes no application code.

## Why

VigilAfrica ingests every event from a **single upstream source — NASA EONET**
(`api/internal/ingestor/eonet.go`, polling `https://eonet.gsfc.nasa.gov/api/v3/events`).
That is a single point of failure on the most load-bearing part of the system:

- **Availability risk.** If EONET has an extended outage or changes its API
  shape, VigilAfrica silently stops getting new events. The `/health` endpoint
  would report stale ingestion, but there is no fallback — the product goes dark.
- **Credibility risk (grant + partnership).** The 2026-05-27 business/market
  review logged "single-oracle dependency" as **R-01**. A funder or DRR partner
  (NRCS, Code for Africa, Bezos Earth Fund) doing technical due diligence will
  ask "what happens when your one source fails?" and "how do you know an event is
  real?" Today the honest answer is "we don't have an answer." A second,
  independent authority is the credible answer.

The **Global Disaster Alert and Coordination System (GDACS)** — a joint
UN/European Commission framework — is the natural second oracle: it is an
independent, authoritative, free, global feed that already covers our two
categories (floods, wildfires). Adding it converts a fragile single-source
pipeline into a resilient multi-source one **and** unlocks a corroboration
signal we can surface to users and funders: *"confirmed by N independent
sources."*

## Goals

1. **Availability:** if either source is down, the other keeps the product live;
   ingestion degrades gracefully to single-source instead of going dark.
2. **Corroboration:** when both sources report the same physical event, surface
   it as confirmed by multiple authorities — a confidence signal, not a duplicate.
3. **Source fidelity:** never destroy or overwrite one source's data with
   another's; every upstream record stays independently auditable.
4. **Additive + reversible:** EONET behaviour is unchanged; GDACS can be disabled
   by config and the system falls back to exactly today's behaviour.

## Non-Goals (Out of Scope)

- **Implementation.** This is a design doc; no Go code, no migration ships here.
- **New event categories.** GDACS also carries earthquakes, cyclones, volcanoes,
  droughts; we map only `FL → floods` and `WF → wildfires` to match the existing
  category set (`CHECK (category IN ('floods','wildfires'))`). Expanding
  categories is a separate roadmap decision.
- **A third+ source.** The design generalises to N sources, but only GDACS is in
  scope.
- **User-facing subscription/notification changes.** The daily digest and alerts
  consume whatever is in the `events` table; they need no change beyond
  optionally noting multi-source confirmation later.
- **Replacing EONET.** EONET stays the primary; GDACS is additive.

## Background — the current single-source pipeline

Grounding for the design (verbatim from the codebase as of this proposal):

| Concern | Today (EONET-only) |
| --- | --- |
| Fetch | `ingestor.Ingest(ctx, repo, country CountryConfig)` in `eonet.go`; bbox + `category=floods,wildfires` query; 5 MB / 256 KB caps; 429/503 + transient retry/backoff |
| Normalize | `normalizer.Normalize(raw RawEONETEvent, rawPayload []byte) (models.Event, geoJSON string, error)` — **hardcodes `Source: "eonet"`** |
| Model | `models.Event{ SourceID, Source, Title, Category, Status, GeomType, Latitude, Longitude, EventDate, SourceURL, RawPayload, ... }` — `Source` is a plain string |
| Store / dedup | `repo.UpsertEvent(ctx, e, geoJSON)` → `INSERT … ON CONFLICT (source_id) DO UPDATE …` |
| Schema | `events.source_id TEXT NOT NULL UNIQUE`; `source TEXT NOT NULL DEFAULT 'eonet'`; PostGIS `geom geometry(Geometry,4326)` + GIST index |
| Schedule | `ingestor.StartScheduler(...)`; `time.Ticker`, `INGEST_INTERVAL_MIN` (default 60); Postgres advisory lock `"ingestion-scheduler"`; per-country `ingestion_runs` |
| Health | `/health` reports `last_ingestion` + `last_ingestion_by_country` |

Three properties of the current design are EONET-specific and must change to admit
a second source (details under Design):

1. **`events.source_id` is `UNIQUE` on its own** — a GDACS event whose ID collides
   with an EONET ID would be wrongly treated as the same row.
2. **`normalizer.Normalize` hardcodes `Source: "eonet"`** — every event would be
   mislabelled.
3. **The ingestor/normalizer are EONET-shaped** — no source abstraction.

## Design

### 1. Source abstraction

Introduce a small interface so the scheduler is source-agnostic. Each source owns
its own fetch + parse; the shared model, upsert, and run-tracking stay common.

```go
// api/internal/ingestor/source.go (new)
type Source interface {
    Name() string                                               // "eonet" | "gdacs"
    Fetch(ctx context.Context, country CountryConfig) ([]models.Event, []string, error)
    // returns normalized events + their GeoJSON strings (parallel slices),
    // with Event.Source already set to Name()
}
```

- `EONETSource` wraps the existing `eonet.go` fetch + `normalizer.Normalize`,
  passing `"eonet"` explicitly (removing the hardcode).
- `GDACSSource` is new (below).
- `normalizer.Normalize` gains a `source string` parameter (or each source calls
  its own normalizer); EONET keeps `RawEONETEvent`, GDACS adds `RawGDACSFeature`.

This is the only refactor of existing code; it is behaviour-preserving for EONET.

### 2. GDACS source

- **Endpoint:** `https://www.gdacs.org/gdacsapi/api/events/geteventlist/SEARCH`,
  GeoJSON `FeatureCollection`. Filter by `eventlist=FL,WF`, alert level, and a
  date window; paginate via `pagenumber` (max 100 records/page). A GeoRSS feed
  (`/xml/rss.xml`) exists as a fallback parser if the JSON API is unavailable.
- **Geographic filter:** GDACS SEARCH is global; filter to our `CountryConfig`
  bounding boxes (`NG`, `GH`) client-side using the feature centroid (same boxes
  EONET already uses), so coverage stays aligned across sources.
- **Category mapping:** `FL → floods`, `WF → wildfires`. Ignore `EQ/TC/VO/DR`.
- **Identity:** GDACS keys events by `eventtype` + `eventid` (+ `episodeid` for
  updates). `Event.SourceID = "<eventtype>-<eventid>"` (e.g. `WF-1012345`);
  `Event.Source = "gdacs"`.
- **Severity:** GDACS alert level (Green/Orange/Red) is captured into
  `raw_payload` now and optionally promoted to a typed `alert_level` column later
  (see Open Questions) — useful for prioritising the digest.
- **Resilience:** GDACS gets its own retry/backoff and its own failure handling;
  a GDACS outage must not fail the EONET run (and vice-versa).

### 3. Schema changes (one additive migration)

```sql
-- events: dedup must be per-source, not global
ALTER TABLE events DROP CONSTRAINT events_source_id_key;
ALTER TABLE events ADD CONSTRAINT events_source_source_id_key UNIQUE (source, source_id);

-- correlation link (nullable self-reference; see §4)
ALTER TABLE events ADD COLUMN correlated_event_id UUID NULL
    REFERENCES events(id) ON DELETE SET NULL;
CREATE INDEX idx_events_correlated ON events(correlated_event_id);

-- per-source ingestion tracking (parallels the v0.7 country_code addition)
ALTER TABLE ingestion_runs ADD COLUMN source TEXT NOT NULL DEFAULT 'eonet';
CREATE INDEX idx_ingestion_runs_source_started ON ingestion_runs(source, started_at DESC);
```

The compound `UNIQUE (source, source_id)` is a **superset** of the old constraint
for existing data (every current row is `source='eonet'`), so the migration is
safe with no backfill. `UpsertEvent`'s `ON CONFLICT` target changes to
`(source, source_id)`.

### 4. Cross-source correlation (the core decision)

The same physical wildfire can appear in **both** feeds with different IDs,
slightly different coordinates, and slightly different timestamps. Naively storing
both double-counts events and puts two markers on the map for one fire; naively
merging destroys source fidelity.

**Decision: keep both source rows; link them with a correlation pass; expose a
canonical, "confirmed-by" view.** We never delete or overwrite a source's row.

- A periodic **correlation pass** (after each ingest, or on its own ticker) links
  candidate rows from *different* sources that represent the same event:
  - **same `category`**, AND
  - **spatial proximity** — centroids within a threshold (e.g. `ST_DWithin` ≤ ~25 km),
    AND
  - **temporal proximity** — `event_date` within a window (e.g. ≤ 72 h).
- Among a correlated cluster, one row is the **canonical** record (deterministic
  rule — e.g. prefer EONET, tiebreak earliest `event_date`); the others set
  `correlated_event_id → canonical.id`.
- Thresholds are conservative to bias toward *not* merging (a missed correlation
  shows two markers — annoying; a false merge hides a real distinct event —
  dangerous). Thresholds are config, not hardcoded.

**API surface:** `ListEvents`/`GetNearbyEvents` return one entry per *canonical*
event by default, annotated with `sources: ["eonet","gdacs"]` and `source_count`.
Secondary (correlated) rows are hidden from the default list but remain queryable
(e.g. `?include=all`) and are never destroyed. This is an **additive** OpenAPI
change (`api-contract.md` / `openapi.yaml`): new optional fields, no breaking
change.

This is what turns redundancy into a **feature**: an event "confirmed by 2
independent international sources" is a credibility signal we can show users and
cite to funders — directly answering R-01.

**Alternative considered — merge-on-write (collapse into one row):** rejected. It
destroys source fidelity, makes "which source is authoritative" ambiguous, and
makes re-correlation impossible when one source later updates an episode.

### 5. Scheduling & resilience

- The scheduler iterates **registered sources × countries**. Each source can have
  its own interval (`INGEST_INTERVAL_MIN` for EONET stays; add
  `GDACS_INGEST_INTERVAL_MIN`, default 60). GDACS can be disabled by setting its
  interval to 0 → exact current behaviour.
- Each (source, country) run is tracked independently in `ingestion_runs.source`,
  so a GDACS failure is isolated and visible without affecting EONET's run status.
- The existing advisory-lock pattern extends to a per-source lock name.

### 6. Observability

- `/health` gains `last_ingestion_by_source` alongside the existing
  `last_ingestion_by_country`, so an operator sees per-source freshness at a
  glance.
- Staleness alerting (Resend) distinguishes **"one source stale"** (degraded,
  warn) from **"all sources stale"** (outage, page). The daily digest can later
  note multi-source confirmation per event.

## Capabilities

### New Capabilities

- `secondary-oracle-ingestion`: A second independent disaster feed (GDACS) ingests
  alongside EONET, with per-source scheduling, resilience, and run tracking.
- `cross-source-correlation`: Events reporting the same physical incident across
  sources are linked and surfaced as a single "confirmed by N sources" record
  without losing per-source data.

### Modified Capabilities

- `event-ingestion`: refactored behind a `Source` interface; `Normalize` becomes
  source-parameterised; `UpsertEvent` dedups on `(source, source_id)`.
- `health-observability`: `/health` reports per-source ingestion freshness.
- `event-api`: list/detail responses gain additive `sources` / `source_count`
  fields (OpenAPI updated).

## Phased Rollout (for the eventual implementation)

Sequenced so each phase is independently reviewable and shippable, and so EONET is
never at risk:

- **Phase 1 — Plumbing (no behaviour change).** `Source` interface; refactor EONET
  behind it; `Normalize(source)`; migration for compound unique +
  `ingestion_runs.source` + `correlated_event_id`. EONET-only still; everything
  green.
- **Phase 2 — GDACS ingest.** `GDACSSource` (fetch FL/WF, bbox filter, map +
  normalize), `GDACS_INGEST_INTERVAL_MIN`, per-source runs. Both sources ingest;
  duplicates may appear (acceptable, interim).
- **Phase 3 — Correlation + canonical API.** Correlation pass; `sources` /
  `source_count` in the API; collapse duplicates in the default list. This is the
  credibility feature.
- **Phase 4 — Observability.** Per-source `/health`; one-vs-all-stale alerting;
  optional digest "confirmed by" note.

Each phase is its own PR with its own OpenSpec record (the Sentinel gate now
requires it for `api/internal/`, `api/cmd/` changes — see ADR-010).

## Acceptance Criteria (for the implementation, not this doc)

- [ ] A `Source` interface exists; EONET is implemented behind it with **no change
      to EONET ingestion behaviour** (existing tests pass unchanged).
- [ ] `events` dedups on `(source, source_id)`; the migration applies cleanly with
      no backfill and no row loss on the existing EONET data.
- [ ] `GDACSSource` ingests FL/WF events within the NG/GH bounding boxes, mapped to
      `floods`/`wildfires`, with `source='gdacs'` and `source_id='<type>-<id>'`.
- [ ] A correlation pass links same-category events from different sources within
      the configured spatial+temporal thresholds, setting `correlated_event_id`;
      it is idempotent and never merges across categories.
- [ ] The event API returns one canonical entry per correlated cluster by default,
      annotated with `sources` + `source_count`; secondary rows are retained and
      retrievable; OpenAPI is updated (additive).
- [ ] Per-source scheduling works; GDACS disabled (interval 0) reproduces exact
      current behaviour.
- [ ] `/health` reports per-source ingestion freshness; staleness alerting
      distinguishes one-source-stale from all-stale.

## Risks

- **R1 — False correlation merges hide a real distinct event.** Mitigation:
  conservative, configurable thresholds biased toward *not* merging; raw per-source
  rows are never destroyed and stay queryable, so a bad merge is reversible.
- **R2 — GDACS API instability / rate limits / format drift.** Mitigation:
  isolated retry/backoff; a GDACS failure never fails the EONET run; GeoRSS
  fallback parser; the system degrades to single-source, which is exactly today.
- **R3 — Migration on the unique constraint.** Mitigation: the compound key is a
  superset for existing all-EONET data; no dedup/backfill needed; reversible
  `down` migration restores `UNIQUE(source_id)`.
- **R4 — Interim double-counting (Phase 2 before Phase 3).** Mitigation: phases
  are short; analytics/digest can note "correlation pending" or Phase 3 can land
  close behind Phase 2; documented as a known interim state.
- **R5 — Coordinate precision mismatch** (GDACS points vs EONET points/polygons).
  Mitigation: correlate on centroids via PostGIS; thresholds absorb minor offset.

## Open Questions (for the maintainer)

1. **Correlation thresholds:** starting values for spatial (~25 km?) and temporal
   (~72 h?) windows, and per-category overrides (wildfires move; floods linger)?
2. **Canonical-source preference:** prefer EONET as canonical, or prefer the
   earliest report regardless of source?
3. **API default:** collapse correlated duplicates by default (recommended) or
   keep one-row-per-source and make collapse opt-in?
4. **`alert_level` column:** add the GDACS Green/Orange/Red severity as a typed
   column in Phase 2, or keep it in `raw_payload` until a consumer needs it?
5. **Correlation cadence:** run inline after each ingest, or on its own ticker?

## Verification Plan (for the implementation)

- **Unit:** per-source normalizers (EONET unchanged; GDACS FL/WF mapping, ID
  formatting, bbox filter); correlation predicate (table-driven: same/different
  category, within/outside spatial + temporal thresholds, idempotency).
- **Integration (testcontainers-go, via `scripts/test-api.ps1`):** seed both
  sources incl. a known overlapping wildfire; assert two source rows persist, one
  canonical with `source_count=2`; assert `(source, source_id)` dedup; assert
  per-source `ingestion_runs`.
- **Resilience:** simulate a GDACS 5xx/outage; assert EONET ingest + run status
  unaffected and `/health` shows GDACS stale only.
- **Migration:** apply up on a snapshot of all-EONET data → no row loss; `down`
  restores `UNIQUE(source_id)`.

## Origin

Day-6 deliverable of the 2026-05-29 partnership-readiness sprint. Promotes
`feature-secondary-oracle` from a deferred post-launch stub to a concrete,
phased, reviewable design — closing the **R-01 single-oracle** gap from the
2026-05-27 business/market review for grant + partnership due diligence. Grounded
in the current EONET pipeline (`api/internal/ingestor`, `normalizer`, `database`,
`db/migrations`). No implementation ships in this change; Phase 1 begins after
v1.3.
