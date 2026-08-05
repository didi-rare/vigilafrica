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

## ✅ HDX COD survey — RESOLVED 2026-08-02 (was open question #1)

Surveyed all **54 African states** against the HDX CKAN API (`package_show?id=cod-ab-{iso3}`), the same source `000010` already uses:

| finding | result |
|---|---|
| **COD-AB dataset exists** | **54 / 54 (100%)** |
| Has a GeoJSON resource | **45 / 54 (83%)** |
| Data vintage 2024 or newer | **50 / 54** (35× 2025, 12× 2024, 3× 2026) |
| Vintage older than 2023 | **4** — MDG **2010**, TZA 2018, BWA 2021, DJI 2022 |

**The plan's central assumption holds.** Boundary data exists for every African country.

**The 9 without GeoJSON — a clean, self-explaining pattern.** They are `BWA, DJI, GAB, LSO, MDG, MUS, RWA, SYC, TZA`. That set is **exactly identical** to the set of records HDX has not refreshed since 2026 (metadata last touched 2020–2023; the other 45 were all touched in 2026). HDX is adding GeoJSON exports as it refreshes records — so this gap is shrinking on its own, and it doubles as a *least-maintained* flag. **All 9 publish SHP**, so they are convertible (`ogr2ogr`), not blocked.

⚠️ **Madagascar's boundaries are from 2010** — 15 years stale. For a safety-adjacent product that needs an explicit call: use with a provenance warning, source elsewhere, or exclude.

### This settles Decision 1 empirically

Sampled actual resource sizes from HDX (8 countries):

| | Nigeria | Ghana | Togo | Kenya | Ethiopia | DR Congo | Cabo Verde | **South Africa** |
|---|---|---|---|---|---|---|---|---|
| GeoJSON | 10.9 MB | 3.3 MB | 1.8 MB | 6.8 MB | 22.7 MB | 24.3 MB | 18.6 MB | **89.2 MB** |

Mean **22 MB**; naive 54-country projection **≈ 1.2 GB** of source GeoJSON. (These files carry *all* admin levels — we need only ADM1 — but the order of magnitude is decisive.)

**Option (a) "keep migrations, one per country" is now ruled out on evidence, not preference.** Recommendation (b), the seed loader, stands and is no longer a judgement call.

### 🔬 MEASURED 2026-08-03 — simplification experiment (run against real data in PostGIS 3.4)

Loaded the actual production boundaries (`000002` + `000010`) into a throwaway PostGIS 15-3.4 container, generated **10,600 ground-truth points** (200 random inside each of the 53 ADM1 units, of which **1,133 lie within 2 km of a border** — where simplification does its damage), and swept `ST_SimplifyPreserveTopology` using **the production trigger's exact matching logic** (`ST_Intersects` + `ORDER BY ST_Area ASC LIMIT 1`).

| tolerance | vertices kept | size | misassigned | misassigned **near border** | **unassigned** |
|---|---|---|---|---|---|
| none (baseline) | 100% | 984 kB | 0% | 0% | **0%** |
| **0.001° (~110 m)** | **45.1%** | **445 kB** | **0.066%** | **0.618%** | **0.000%** |
| 0.005° (~550 m) | 16.1% | 161 kB | 0.396% | 3.707% | 0.170% |
| 0.01° (~1.1 km) | 9.6% | 96 kB | 0.811% | 7.590% | 0.245% |
| 0.02° (~2.2 km) | 5.5% | 57 kB | 1.594% | 14.916% | 0.519% |
| 0.05° (~5.5 km) | 2.6% | 27 kB | 3.585% | 28.067% | 1.509% |

**Recommendation: 0.001°.** It halves vertex count at 0.066% error and — decisively — **zero unassigned points**.

⚠️ **Watch the `unassigned` column, not the error rate.** From 0.005° upward, points begin matching *no polygon at all*, so enrichment silently yields `NULL state_name`. For a product whose entire proposition is admin-name-first, a **silent enrichment failure is worse than a slightly-wrong name**. That column, not accuracy, is what rules out the aggressive tolerances.

### ✏️ Corrections to this proposal's own earlier estimates

- ~~"plausibly 10–50× on storage"~~ — **wrong.** Measured **2.2×** at the safe tolerance. Simplification is a modest win, not a transformative one.
- ~~"tens of MB of geometry… dominated by outliers"~~ — **overstated.** Scaled to **742 ADM1 polygons** (real geometry, replicated and translated) the table is **13 MB / 879k vertices**. That is nothing for Postgres. **Geometry volume is not a real constraint.**
- The **~1.2 GB figure is *source download*** — zipped GeoJSON containing *all* admin levels. The ADM1-only subset that lands in the database is ~13 MB. These were being conflated.
- Decision 1's case therefore rests on **repo hygiene** (~33 MB of migration SQL at 750 units), **not** database capacity. Still a valid argument for the loader — a narrower one.

### 🔬 MEASURED — enrichment performance at continental scale (work item 4)

Same 742-polygon table, 2,000 lookups, production trigger logic:

| variant | total | per lookup | |
|---|---|---|---|
| A — current production (`ORDER BY ST_Area(geom::geography)`) | 4,811 ms | 2.41 ms | baseline |
| **B — precomputed `area_m2` column** | **260 ms** | 0.13 ms | **18.5× faster** |
| C — B + 0.001° simplification | 144 ms | 0.07 ms | 33× faster |

**Work item 4 is the single highest-leverage change in this proposal, and it is nearly free** — add a stored column, change one `ORDER BY`. The bottleneck is `ST_Area` being recomputed per candidate row on every insert, **not** geometry size. Simplification contributes a further 1.8× on top, but ~95% of the win is the column.

Neither variant is catastrophic in absolute terms (2,000 events ≈ 4.8 s even unoptimised), so **performance was never going to block this** — but an 18× win for a stored column is worth taking regardless.

### New requirement this survey surfaced: geometry simplification

Nigeria's **37 ADM1 units** occupy **2.3 MB** as full-resolution WKT (`000010`) ≈ 62 KB/unit. Extrapolating to ~700–800 African ADM1 units gives **tens of MB of geometry**, dominated by outliers like South Africa. Complexity varies ~50× between countries (Togo 1.8 MB vs South Africa 89.2 MB), so any mean-based estimate is unreliable — plan for a range.

For our use — *point-in-polygon to name a state* — full coastline resolution is unnecessary. **Simplification tolerance becomes an explicit design parameter**, plausibly worth 10–50× on storage and index size with no practical accuracy loss. There is precedent in the codebase: `000012` already loaded *"geoBoundaries gbOpen ADM0 (simplified), further reduced"*.

⚠️ But simplification must not be applied blindly: over-simplifying shared borders creates **gaps or overlaps between adjacent states**, which would make the enrichment trigger mis-assign or drop points. Use topology-preserving simplification (`ST_SimplifyPreserveTopology` or a shared-edge-aware tool like mapshaper), and validate against known coordinates before and after.

### Consequent scope additions

- **The generator takes GeoJSON only.** Either add SHP input or add an `ogr2ogr` pre-step for the 9 countries.
- **Per-country vintage must be recorded and surfaced** (work item 9), given the 2010–2026 spread.
- **Simplification + validation** is a work item in its own right, not a detail.

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

1. ~~**Is HDX COD ADM1 available and current for all ~54 countries?**~~ **✅ ANSWERED 2026-08-02 — yes, 54/54.** See the survey section above. Residual sub-questions: what to do about **Madagascar (2010 boundaries)**, and whether to add SHP support to the generator or an `ogr2ogr` pre-step for the 9 non-GeoJSON countries.
2. **What simplification tolerance?** New, and now the biggest open technical question — it drives storage, index size and enrichment latency, and mis-set it breaks border adjacency. Needs an accuracy/size experiment against known coordinates.
3. **How many ADM1 units are there in Africa, exactly?** Still estimated at ~700–800, unverified. Requires counting features in the downloaded files. Determines the real geometry volume alongside (2).
4. What are EONET's published rate limits? Decision 2's pacing depends on it.
5. Do we widen to all of Africa at once, or in tranches (e.g. West Africa first)? Tranches de-risk the loader and the simplification tuning, and give an earlier checkpoint. **South Africa alone is 89 MB — a natural candidate to sequence late.**
6. Does "VigilAfrica" still position as Nigeria/Ghana-focused with continental data, or reposition entirely? Affects item 8 and the partnership narrative.

## Origin

Data-density investigation, 2026-08-02, prompted by a monetisation question. The measurement inverted the assumed problem: partnerships and distribution were believed to be the constraint on having a usable product; they are not. Content is, and content is a scope decision the project controls unilaterally. Full measurement record and the disproven "satellites miss local events" hypothesis in [[project-flood-data-source-gap]].
