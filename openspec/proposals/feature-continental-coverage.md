---
id: feature-continental-coverage
status: proposed
branch: tbd
---

# Proposal: Widen Ingestion From Two Countries to Africa (feature-continental-coverage)

> **Scoping proposal — no implementation in this change.** It records what was measured, what the code already supports, and what widening actually costs. Two decisions inside need a maintainer call before anyone writes code.

## Why

Measured 2026-08-02 against the live feeds, same 22-month window (2024-10-01 → 2026-08-02):

| source | method | Nigeria + Ghana | Africa-wide |
|---|---|---|---|
| **EONET** (already ingested) | satellite detection | **43 events, 1 flood** | **≥2,000 events, 80 floods** |
| GDACS | model + impact threshold | **0** | ~150 / 13 months |
| ReliefWeb `/v2/disasters` | human declaration | **3** (2 floods) | — |

**We discard ~97% of our own feed at the bounding box.** The same source, polled by the same code, carries ~46× more events if the box opens to the continent.

Critically, the sparsity is **real, not a detection artifact**. ReliefWeb is reports-based rather than satellite-limited and is the control that rules this out: it records **3 declared disasters** for both countries in 22 months, and only **64 since 1986**. Nigeria and Ghana do not generate a dense stream of discrete, nameable hazard events. **No additional data source fixes this** — see [[project-flood-data-source-gap]] for the full triangulation.

There are exactly two levers on density: widen the geography (this proposal), or drop below disaster granularity to local reports (a separate, larger architectural change). This one is cheap and has no external dependency.

⚠️ **Scope honestly: this makes the product non-empty. It does not make it locally relevant.** Of the ~2,000 Africa-wide events, **1,917 are wildfires**. The urban-pluvial flooding that matters most to Lagos residents remains invisible to every global source. Do not let this proposal be mistaken for a fix to that.

## What the code already supports (better than expected)

Investigated against the tree, not assumed:

- **Enrichment is fully generic.** `trg_enrich_event_location()` (migration `000006`) does `ST_Intersects` against `admin_boundaries` filtered to `adm_level = 1`, tie-broken by `ORDER BY ST_Area(geom::geography) ASC`, with an ADM0 fallback (`000012`) for border spillover. **It contains no country-specific logic.** Scaling enrichment is loading rows, not changing code.
- **A boundary generator already exists** — `scripts/generate_boundary_migration.py` converts an HDX COD ADM1 GeoJSON into migration SQL, auto-detecting the name property across seven known HDX column conventions. It is unit-tested (`generate_boundary_migration_test.py`).
- **Ingestion is already country-parameterised** — `DefaultCountries []CountryConfig` in `eonet.go:83`, and the scheduler picks up new entries automatically (`runAllCountries`, `scheduler.go:121`).
- **The bbox containment guard is generic** — `withinBBox` (`eonet.go:94`) validates every event against the requested box.

The architecture was built for this. The cost is concentrated in **data volume** and **two design decisions**, not in rewriting the pipeline.

## Decision 1 — how are boundaries loaded? (blocking)

`000010_replace_boundary_data.up.sql` is **2.3 MB** for **53 ADM1 units** (Nigeria 37, Ghana 16). Africa has roughly **700–800 ADM1 units** (**estimate — needs per-country verification against HDX availability**). Naive extrapolation: **~30–40 MB of migration SQL**.

That breaks the migration model. The existing file already carries the admission:

> *"HOW TO REVIEW / VERIFY THIS 2.3MB FILE: regenerate it from HDX rather than reading the WKT line-by-line."*

At 40 MB that is not reviewable, not diffable, and bloats every clone and CI checkout permanently.

**Options:**

- **(a) Keep migrations, one per country.** ~54 files, ~40 MB total. Consistent with today; unreviewable at scale; permanent repo weight.
- **(b) Seed-data loader.** Migrations own *schema*; a separate idempotent loader owns *boundary data*, pulling from HDX/geoBoundaries at deploy or via a one-shot job. Repo stays small; adds a deploy step and a network dependency; needs its own integrity check.
- **(c) Hybrid.** Ship simplified geometries in-repo (bounded size) and offer full-resolution as an optional loader.

**Recommendation: (b).** Boundary data is reference data with its own release cadence, not schema. Bundling it into migrations conflates the two and is already uncomfortable at 2.3 MB.

## Decision 2 — one Africa bbox, or 54 country bboxes? (blocking)

Today: 2 countries × 2 requests (open + closed) = **4 requests per tick**, iterated **sequentially with no inter-country pacing** (`scheduler.go:121`). There is retry/backoff and `RetryAfter` handling *within* a country fetch, but nothing between countries.

At 54 countries that becomes **108 sequential requests per tick**.

**Option A — one Africa-wide bbox.** 2 requests per tick. Simplest, fastest, no rate-limit exposure.
  - ✗ Breaks per-country `ingestion_runs` tracking (`country_code` column, `/health.last_ingestion_by_country`, `/v1/enrichment-stats`).
  - ✗ `withinBBox` becomes a continent-level guard — much weaker as a data-quality check. The Florida-leak defect it was written for (`000011`, `fix-ingest-bbox-validation`) would be caught far later.
  - ✗ Country attribution then rests *entirely* on enrichment. Today the bbox is an independent cross-check.

**Option B — keep per-country boxes.** Preserves all per-country observability and the containment guard.
  - ✗ 108 requests/tick; needs inter-country pacing and a rate-limit strategy EONET's terms must be checked against.
  - ✗ Ingestion cycle wall-time grows ~27×.

**Recommendation: B, with pacing added.** The per-country ingestion-run records and the bbox guard are real quality controls that have already caught one production defect. Losing them to save requests is a bad trade for a data-integrity-sensitive product. But the pacing work is genuine and must be scoped in, not assumed.

## Work items

| # | Item | Notes |
|---|---|---|
| 1 | Boundary acquisition for ~54 countries | HDX COD where available, geoBoundaries fallback. Availability/vintage **varies by country** — must be surveyed, not assumed. |
| 2 | Implement Decision 1 | Loader or migrations |
| 3 | Extend `DefaultCountries` + Decision 2 pacing | Mechanical once decided |
| 4 | Enrichment performance | `ORDER BY ST_Area(geom::geography)` is computed per insert. With ~800 polygons instead of 53, precompute area as a stored column and index it. **Measure before and after.** |
| 5 | **ADM1 name disambiguation** | ~800 names across 54 countries **will** collide (multiple "Central", "Northern", "Eastern"). `/v1/events?state=` filters by `ILIKE` on `state_name` alone (`queries.go:50`) — ambiguous across countries. Needs country qualification in the API and UI. |
| 6 | API de-hardcoding | `handlers/country.go:15-16` (NG/GH map), its `errUnknownCountry` message, `handlers/context.go:35,53-54` (Nigeria defaults). |
| 7 | Frontend | `EventsDashboard.tsx:20` `SUPPORTED_COUNTRIES`, `:24-25` `COUNTRY_CENTERS`. A 54-entry dropdown needs a different UX than 2 — grouping or search. |
| 8 | Copy + metadata | "Nigeria and Ghana live" appears in `App.tsx` (×4), `ForPartners.tsx`, `index.html` meta description, and the generated `llms.txt`. |
| 9 | Provenance | Record per-country source + vintage. Mixed HDX/geoBoundaries provenance must be visible, not silent — this is a safety-adjacent product. |

## ⚠️ Coupling: this makes the deferred performance work mandatory

The two performance proposals are currently **held pending traffic** ([[project-traction-data-gap]]). Continental coverage removes that justification and turns them into prerequisites:

- `div.events-list` already stacks to **~14,000px at tablet width with 43 events**. At ~2,000 it is unusable — and the CLS reservation work just shipped (#193/#198) is sized against today's dashboard height.
- The map renders a marker per event. 2,000 markers on a mid-range Android over Slow 4G is a different problem from 43.
- `/v1/events` caps `limit` at **200**, so pagination stops being optional.

**`perf-mobile-first-render` must ship with or before continental coverage**, not after. Widening the feed without it converts a fast, empty site into a slow, full one.

## Out of Scope

- Sub-disaster/local-report ingestion (news, geocoding, confidence model) — the *other* density lever, and a much larger architectural change.
- `feature-secondary-oracle` (GDACS). Measured at **0 events for NG/GH**; only becomes meaningful *after* this proposal. See its own stop-block.
- New hazard categories. Orthogonal.
- Any change to the enrichment trigger's matching semantics.

## Open questions

1. **Is HDX COD ADM1 actually available and current for all ~54 countries?** The whole plan rests on this and it is unverified. Survey before committing.
2. What are EONET's published rate limits? Decision 2's pacing depends on it.
3. Do we widen to all of Africa at once, or in tranches (e.g. West Africa first)? Tranches de-risk items 1 and 4 and give an earlier checkpoint.
4. Does "VigilAfrica" still position as Nigeria/Ghana-focused with continental data, or reposition entirely? Affects item 8 and the partnership narrative.

## Origin

Data-density investigation, 2026-08-02, prompted by a monetisation question. The measurement inverted the assumed problem: partnerships and distribution were believed to be the constraint on having a usable product; they are not. Content is, and content is a scope decision the project controls unilaterally. Full measurement record and the disproven "satellites miss local events" hypothesis in [[project-flood-data-source-gap]].
