---
id: feature-continental-coverage
status: proposed
branch: tbd
---

# Proposal: Widen Ingestion From Two Countries to Africa (feature-continental-coverage)

> **Scoping proposal — no implementation in this change.** It records what was measured, what the code already supports, and what widening actually costs. Two decisions inside need a maintainer call before anyone writes code.

## Why

> ### ⚠️ CORRECTED 2026-08-04 after independent review — the original figures in this section were wrong
>
> This proposal was opened claiming **43 events for NG+GH** and a **~46×** multiplier. Independent review found that contradicted a sibling proposal, and re-measurement confirmed it. A first attempt at correction was **also wrong** (159 events) because its bounding box started at 2.6°E — **Ghana spans −3.5 to 1.2°E and was excluded entirely.**
>
> **The multiplier is ~11×, not ~46×.** The case for widening survives, at roughly a quarter of the strength originally claimed.
>
> Measurement is now a committed script — [`scripts/eonet-density/measure.mjs`](../../scripts/eonet-density/measure.mjs) — which **mirrors** the production bounding boxes from `eonet.go` (copied, not imported — they must be kept in sync by hand) and applies the same `withinBBox` guard, rather than hand-rolling a box. Re-run it rather than trusting any number quoted here.
>
> ⚠️ **This measures feed AVAILABILITY at a geography, not what production currently stores.** Production polls unwindowed `status=open` plus `status=closed&days=30`; the script queries `status=all` across the full window, so the stored count is lower than 290. The **ratio** is what this proposal rests on and both sides are measured identically, so it holds — but do not read 290 as a database count.
>
> ⚠️ **The Africa box is a rectangle, not the continent.** It admits parts of southern Europe and the Middle East, so the Africa-wide figure is an **over**-count. An earlier note called that "the conservative direction" — backwards: overstating Africa **inflates** the multiplier and flatters the case for widening. Treat **11.3× as an upper bound**.

Measured 2026-08-04, same 22-month window (2024-10-01 → 2026-08-04), production bboxes + guard:

| source | method | Nigeria + Ghana | Africa-wide |
|---|---|---|---|
| **EONET** (already ingested) | satellite detection | **290 events, 9 floods** *(NG 174, GH 116)* | **3,268 events, 159 floods** |
| GDACS | model + impact threshold | **0** | ~150 / 13 months ⚠️ *see stop-block caveat* |
| ReliefWeb `/v2/disasters` | human declaration | **3** (2 floods) | — |

**We discard ~91% of our own feed at the bounding box.** The same source, polled by the same code, carries **11.3× more events** — and **17.7× more floods** — if the box opens to the continent.

*The flood multiplier being higher than the event multiplier is the genuinely encouraging part: widening is disproportionately good for the hazard this product is actually about.*

Critically, the sparsity is **real, not a detection artifact**. ReliefWeb is reports-based rather than satellite-limited and is the control that rules this out: it records **3 declared disasters** for both countries in 22 months, and only **64 since 1986**. Nigeria and Ghana do not generate a dense stream of discrete, nameable hazard events. **No additional data source fixes this** — see [[project-flood-data-source-gap]] for the full triangulation.

There are exactly two levers on density: widen the geography (this proposal), or drop below disaster granularity to local reports (a separate, larger architectural change). This one is cheap and has no external dependency.

⚠️ **Scope honestly: this makes the product non-empty. It does not make it locally relevant.** Of the **3,268** Africa-wide events, **3,109 are wildfires — 95.1%**. The urban-pluvial flooding that matters most to Lagos residents remains invisible to every global source. Do not let this proposal be mistaken for a fix to that.

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

### 🔬 MEASURED — simplification experiment, RE-RUN AND REPRODUCED 2026-08-04

The original sweep (2026-08-03) was run in a throwaway container with **no committed script**, so independent review could not reproduce any of it. It has now been re-run from a committed harness — [`scripts/bench-simplification/sweep.sql`](../../scripts/bench-simplification/sweep.sql).

**10,600 ground-truth points** (200 inside each of the 53 ADM1 units; **1,081** within 2 km of a border — where simplification does its damage), swept through **the production matcher** (`ST_Intersects` + smallest-area tie-break + `LIMIT 1`). The baseline assignment is that same matcher on *unsimplified* geometry, so overlapping polygons behave as they do in production.

| tolerance | vertices kept | size | misassigned | misassigned **near border** | **unassigned** |
|---|---|---|---|---|---|
| **none (baseline)** | 100% | 828 kB | **0.000%** | **0.000%** | **0.000%** |
| 0.001° (~110 m) | 45.1% | 388 kB | 0.057% | 0.538% | **0.038%** |
| 0.005° (~550 m) | 16.1% | 155 kB | 0.349% | 3.315% | 0.085% |
| 0.01° (~1.1 km) | 9.6% | 97 kB | 0.462% | 4.391% | 0.245% |
| 0.02° (~2.2 km) | 5.5% | 57 kB | 0.925% | 8.781% | 0.472% |
| 0.05° (~5.5 km) | 2.6% | 28 kB | 1.915% | 16.219% | 1.340% |

*Fixed seed (`ST_GeneratePoints(geom, 200, 42)`); two consecutive runs are byte-identical.*

### ⚠️ RECOMMENDATION CHANGED — do not simplify

Earlier revisions recommended `0.001°` on the grounds that it *"halves vertex count at 0.066% error and — decisively — **zero unassigned points**."*

**The zero was a sampling artefact.** The sweep was unseeded, so every run drew a different point set. Three samples gave three different answers for that cell (0.000%, 0.009%, 0.038%). Seeded, `0.001°` yields **0.038% unassigned — about 4 points in 10,600 that match no polygon at all.**

Only the **vertices kept** column is deterministic; it is pure geometry and matched across every run. The error columns never did, and they are the ones the decision rested on.

**That removes the argument for simplifying, because this proposal already established the thing simplification was meant to buy is not needed:**

- Geometry volume is **not a constraint** — 795 ADM1 polygons measure **13 MB**. (See the corrections below.)
- The saving at `0.001°` is **~2.1×** — 828 kB → 388 kB. Trivial against a database that holds far more elsewhere.
- Enrichment speed is **not** the motivation either: the 11–13× win came from the stored-area column, not from simplification.

So the trade is: save ~440 kB, and accept a **non-zero rate of silent enrichment failure**. This proposal's own rule says that is the wrong side of the trade — *"a silent enrichment failure is worse than a slightly-wrong name."* At baseline both error columns are exactly **0.000%**, and it costs nothing we need.

**Recommendation: load boundaries at full resolution.** Revisit only if geometry volume becomes a measured constraint, in which case `0.001°` remains the best non-zero tolerance by a clear margin — roughly 6× lower misassignment and 2× lower unassigned than `0.005°`.

⚠️ **Watch the `unassigned` column, not the error rate.** At **every** non-zero tolerance — including `0.001°` — some points match *no polygon at all*, so enrichment silently yields `NULL state_name`. An earlier revision claimed this only began at `0.005°`; that was the unseeded sample talking. For a product whose entire proposition is admin-name-first, a **silent enrichment failure is worse than a slightly-wrong name**. That column, not accuracy, is what rules out the aggressive tolerances.

### ✏️ Corrections to this proposal's own earlier estimates

- ~~"plausibly 10–50× on storage"~~ — **wrong.** Measured **2.2×** at the safe tolerance. Simplification is a modest win, not a transformative one.
- ~~"tens of MB of geometry… dominated by outliers"~~ — **overstated.** Scaled to **742 ADM1 polygons** (real geometry, replicated and translated) the table is **13 MB / 879k vertices**. That is nothing for Postgres. **Geometry volume is not a real constraint.**
- The **~1.2 GB figure is *source download*** — zipped GeoJSON containing *all* admin levels. The ADM1-only subset that lands in the database is ~13 MB. These were being conflated.
- Decision 1's case therefore rests on **repo hygiene** (~33 MB of migration SQL at 750 units), **not** database capacity. Still a valid argument for the loader — a narrower one.

### 🔬 MEASURED — enrichment performance at continental scale (work item 4)

> ### ⚠️ SUPERSEDED — these figures were unauditable and the headline was overstated
>
> Independent review found this section had **no committed script or data snapshot**, so none of it could be re-run. Work item 4 has since **shipped** as `perf-boundary-area-precompute` (migration `000013`) with a committed harness — [`scripts/bench-enrichment/bench.sql`](../../scripts/bench-enrichment/bench.sql). Prefer that; the table below is retained only as a record of what was originally claimed.
>
> **The corrected speedup is a range of ~11–13×, not 18.5×.** The original number came from a `LATERAL` lookup that omits plpgsql overhead and the ADM0 fallback branch. Measuring `INSERT`s through the real trigger gives **13.3×** on one machine and **11.5×** on a reviewer's. The isolated-lookup method gives ~15×.
>
> Variant C (simplification on top) has **not** been re-measured against the committed harness and remains unaudited — treat the 33× as unsupported.

Original claim, retained for the record — same 742-polygon table, 2,000 lookups:

| variant | total | per lookup | |
|---|---|---|---|
| A — production at the time (`ORDER BY ST_Area(geom::geography)`) | 4,811 ms | 2.41 ms | baseline |
| ~~B — precomputed `area_m2` column~~ | ~~260 ms~~ | ~~0.13 ms~~ | ~~**18.5×**~~ → **see correction above** |
| ~~C — B + 0.001° simplification~~ | ~~144 ms~~ | ~~0.07 ms~~ | ~~33×~~ → **unaudited** |

**Work item 4 remains the highest-leverage change in this proposal and it is nearly free** — a stored column and one `ORDER BY`. The bottleneck is `ST_Area` being recomputed per candidate row on every insert, **not** geometry size. That conclusion is unchanged; only its magnitude was overstated.

Nothing here is catastrophic in absolute terms (2,000 events ≈ 4.8 s even unoptimised), so **performance was never going to block this** — an 11–13× win for a stored column is simply worth taking.

### New requirement this survey surfaced: geometry simplification

⚠️ **Corrected 2026-08-04:** that 2.3 MB file holds **53** ADM1 units (Nigeria 37 + Ghana 16), not Nigeria's 37 alone, so the real figure is **≈ 43 KB/unit**, not 62. The 62 came from dividing the whole file by Nigeria's share. The ~30–40 MB extrapolation in Decision 1 was computed from the correct 43 KB/unit and is unaffected. Extrapolating to ~700–800 African ADM1 units still gives tens of MB of geometry, dominated by outliers like South Africa. Complexity varies ~50× between countries (Togo 1.8 MB vs South Africa 89.2 MB), so any mean-based estimate is unreliable — plan for a range.

For our use — *point-in-polygon to name a state* — full coastline resolution looked unnecessary. ⚠️ **Superseded — see the recommendation change above: do not simplify.** An earlier draft of this sentence claimed simplification was *"plausibly worth 10–50× on storage and index size with no practical accuracy loss"*. Both halves are retracted. The measured saving at `0.001°` is **~2.1×** (828 kB → 388 kB), not 10–50× — and against a 13 MB table that is not worth having. "No practical accuracy loss" is false: seeded, `0.001°` misassigns **0.057%** of points and leaves **0.038% unassigned**, i.e. silently unnamed. Baseline is 0.000% on both. There is precedent in the codebase — `000012` loaded *"geoBoundaries gbOpen ADM0 (simplified), further reduced"* — but that was ADM0 outlines for country labelling, a far coarser use than naming a state.

⚠️ But simplification must not be applied blindly: over-simplifying shared borders creates **gaps or overlaps between adjacent states**, which would make the enrichment trigger mis-assign or drop points. Use topology-preserving simplification (`ST_SimplifyPreserveTopology` or a shared-edge-aware tool like mapshaper), and validate against known coordinates before and after.

### Consequent scope additions

- **The generator takes GeoJSON only.** Either add SHP input or add an `ogr2ogr` pre-step for the 9 countries.
- **Per-country vintage must be recorded and surfaced** (work item 9), given the 2010–2026 spread.
- **Simplification + validation** is a work item in its own right, not a detail.

## 🔬 MEASURED 2026-08-04 — where the events actually are (settles open questions 4 and 5)

Counted live against EONET v3, `status=all`, 2024-10-01 → 2026-08-04, per candidate region:

| region | floods | wildfires | total |
|---|---|---|---|
| West Africa | 17 | 521 | 538 |
| Central Africa | 23 | 1,970 | 1,995 |
| **East / Horn** | **46** | 1,791 | 1,842 |
| Southern Africa | 18 | 577 | 600 |
| North Africa | 40 | 22 | 62 |
| **Africa (all)** | **159** | **3,109** | **3,268** |
| *ingested today (NG+GH)* | *9* | *281* | *290* |

*Regional boxes overlap slightly and are not a partition; they indicate distribution, not an exact decomposition.*

### ✅ Settles open question 5 — do NOT tranche by region

A West Africa first tranche yields **17 floods in 22 months** — under 2× today — while still requiring the *entire* loader, simplification and pacing build. **Floods are thinly spread; no region has flood density.** Even the richest (East/Horn, 46) is ~2 floods/month.

The flood payoff only arrives at full continental scale: **9 → 159 floods, 17.7×**. **Recommendation: go continental in one step, and tranche by *engineering* risk instead** — sequence South Africa's 89 MB and the 9 SHP-only countries late.

### ⚠️ 95.1% of Africa-wide events are wildfires — this forces a product decision

**Maintainer decision, 2026-08-04: floods remain the default view; wildfires become opt-in via filter.** Without it, widening the box converts a Nigeria *flood* product into an Africa *wildfire* map, and "VigilAfrica shows floods in your area" stops being true at a glance. Adds filter/default work to items 6–7.

### ✅ Open question 4 answered — EONET publishes no rate limit

The v3 API documentation states **no** rate limit, throttle, quota or usage policy. Decision 2's pacing therefore has no documented constraint to design against — pace conservatively by judgement and treat it as a politeness budget, not a compliance one.

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
| 4 | ~~Enrichment performance~~ | ✅ **SHIPPED 2026-08-04** as `perf-boundary-area-precompute` (migration `000013`), with a committed harness at [`scripts/bench-enrichment/bench.sql`](../../scripts/bench-enrichment/bench.sql). Measured **11–13×** production-realistic (not the 18.5× originally claimed — see the superseded section above). ⚠️ Three corrections to this row: the column is **`GENERATED ALWAYS ... STORED`**, not a plain stored column, so it cannot drift when `geom` changes; **do not index it** — `EXPLAIN` shows the planner uses the GIST index then an in-memory quicksort and never reads a btree on `area_m2`; and the `ALTER TABLE` **rewrites the table under an `ACCESS EXCLUSIVE` lock**, which blocks ingestion for the duration (~1.3 s at 795 polygons). |
| 5 | **ADM1 name disambiguation** | ~800 names across 54 countries **will** collide (multiple "Central", "Northern", "Eastern"). `/v1/events?state=` filters by `ILIKE` on `state_name` alone (`queries.go:50`) — ambiguous across countries. Needs country qualification in the API and UI. |
| 6 | API de-hardcoding | `handlers/country.go:15-16` (NG/GH map), its `errUnknownCountry` message, `handlers/context.go:35,53-54` (Nigeria defaults). |
| 7 | Frontend | `EventsDashboard.tsx:20` `SUPPORTED_COUNTRIES`, `:24-25` `COUNTRY_CENTERS`. A 54-entry dropdown needs a different UX than 2 — grouping or search. |
| 8 | Copy + metadata | "Nigeria and Ghana live" appears in `App.tsx` (×4), `ForPartners.tsx`, `index.html` meta description, and the generated `llms.txt`. |
| 9 | Provenance | Record per-country source + vintage. Mixed HDX/geoBoundaries provenance must be visible, not silent — this is a safety-adjacent product. |

## ⚠️ Coupling: what continental coverage actually requires first

> ### ⚠️ CORRECTED — this section's premise was wrong
>
> It claimed the events list and the map marker set grow with the feed. **They do not.** `GET /v1/events` defaults to `limit = 50` (`handlers/events.go:76`) and the web client sends no `limit` or `offset` (`web/src/api/events.ts`), so the list and the markers are capped at **50** — today and at continental scale. Widening the feed takes them from 43 items to 50, not to thousands.
>
> The two bullets below about 14,000px stacking and 2,000 markers are **retracted**. `perf-mobile-first-render` was never written, and the version described here solves a problem that does not exist. See [`feature-events-pagination`](../changes/feature-events-pagination/proposal.md), which supersedes it.

**What is genuinely required first is pagination — for correctness, not performance.** The API returns `meta.total`; the dashboard reads none of it. So at continental scale the UI shows **50 of ~3,268 and says nothing about the rest**. A user filtering to their country would reasonably conclude they were seeing every event near them. In a safety-adjacent product that is a trust failure, and it is the real prerequisite.

Retained for the record, now known to be wrong:

- ~~`div.events-list` already stacks to ~14,000px at tablet width with 43 events. At ~2,000 it is unusable.~~ — capped at 50.
- ~~The map renders a marker per event. 2,000 markers on a mid-range Android is a different problem from 43.~~ — capped at 50.
- `/v1/events` caps `limit` at **200** — still true, and it is why pagination, not a bigger page, is the answer.

Mobile TBT remains a separate and **currently unverified** question: the inherited 1,140 ms figure has not been reproduced, and `map-vendor` is already off the critical path (`lazy(Map)` inside `lazy(EventsDashboard)`; `index.html` modulepreloads only `rolldown-runtime` and `react-vendor`). Re-measure on staging before proposing bundle work.

## Out of Scope

- Sub-disaster/local-report ingestion (news, geocoding, confidence model) — the *other* density lever, and a much larger architectural change.
- `feature-secondary-oracle` (GDACS). Measured at **0 events for NG/GH**; only becomes meaningful *after* this proposal. See its own stop-block.
- New hazard categories. Orthogonal.
- Any change to the enrichment trigger's matching semantics.

## Open questions

1. ~~**Is HDX COD ADM1 available and current for all ~54 countries?**~~ **✅ ANSWERED 2026-08-02 — yes, 54/54.** See the survey section above. Residual sub-questions: what to do about **Madagascar (2010 boundaries)**, and whether to add SHP support to the generator or an `ogr2ogr` pre-step for the 9 non-GeoJSON countries.
2. ~~**What simplification tolerance?**~~ **✅ ANSWERED — none. Do not simplify.** The sweep was re-run from a committed, **seeded** harness ([`scripts/bench-simplification/sweep.sql`](../../scripts/bench-simplification/sweep.sql)) and overturned the earlier `0.001°` recommendation: its headline "zero unassigned points" was an artefact of unseeded sampling, and the real figure is **0.038%**. Since geometry volume was separately measured as *not* a constraint (13 MB at 795 polygons), simplification buys ~440 kB in exchange for a non-zero rate of silent enrichment failure. Load at full resolution; revisit only if volume ever becomes a measured constraint.
3. **How many ADM1 units are there in Africa, exactly?** Still estimated at ~700–800, **unverified**. Requires counting features in the downloaded files. *(Partially de-risked: a 795-polygon fixture built from real geometry occupies ~13 MB, confirming geometry volume is not a constraint — but the true count remains unverified.)*
4. ~~What are EONET's published rate limits?~~ **✅ ANSWERED 2026-08-04 — there are none published.** Pace by judgement; a politeness budget, not a compliance one.
5. ~~Do we widen to all of Africa at once, or in tranches?~~ **✅ ANSWERED 2026-08-04 — do NOT tranche by region.** West Africa alone yields 17 floods/22 months for the full build cost; the flood payoff exists only at continental scale. **Tranche by engineering risk instead** — sequence South Africa (89 MB) and the 9 SHP-only countries late.
6. Does "VigilAfrica" still position as Nigeria/Ghana-focused with continental data, or reposition entirely? Affects item 8 and the partnership narrative.

## Origin

Data-density investigation, 2026-08-02, prompted by a monetisation question. The measurement inverted the assumed problem: partnerships and distribution were believed to be the constraint on having a usable product; they are not. Content is, and content is a scope decision the project controls unilaterally. Full measurement record and the disproven "satellites miss local events" hypothesis in [[project-flood-data-source-gap]].
