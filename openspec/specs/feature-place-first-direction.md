# Spec: Place-First Direction (feature-place-first-direction)

**Status:** Proposed — nothing here is implemented. Decisions D1–D4 in the proposal were made on 2026-11-17, so Phase 0 may start. D5 (build a near-me panel with a scope ladder, after the location marker) and D6 (option a: a display window, on `/v1/context` first) were decided on 2026-11-18. Phase order changed on 2026-11-18: **continental coverage is Phase 1** and starts once ADR-017 (Phase 0 task 3) is merged; the near-me panel is **Phase 3**, after the Phase 2 gate, and must not start until the D6 window length is set from Phase 0 task 2.
**Companion:** [`openspec/proposals/feature-place-first-direction.md`](../proposals/feature-place-first-direction.md) (rationale, evidence, decisions, risks). Decisions here are D1–D6; decisions inside `feature-continental-coverage` are referred to by their titles.

## Context

The product's stated question is "what is happening near me?", answered with administrative place names. Facts about the current tree that shape this spec. Line numbers are as of branch `docs/propose-place-first-direction` on 2026-11-17; re-check them before editing.

### What the product shows today

- **Ingestion** polls `status=open` plus `status=closed&days=30` per country (`api/internal/ingestor/eonet.go`, `runIngest`; `closedEventWindowDays = 30`). The 30 days is an ingest parameter only. A comment in that file says wildfires stay open for months.
- **Reading is unwindowed.** `/v1/events` applies category, country, state and status filters only when the caller sends them (`buildEventFilterClause`, `api/internal/database/queries.go`); `api/internal/handlers/events.go` sets no default and accepts no date parameters (`DateFrom`/`DateTo` are set only by `api/internal/digest/digest.go:68-69`). `GetNearbyEvents` (`queries.go`) has no date or status predicate. No code prunes old events. The list is ordered `event_date DESC NULLS LAST, id DESC` (`queries.go:136`).
- **So the list shows everything ingested**, including wildfires the feed still marks open that are many months old. `scripts/eonet-density/open-wildfire-ages.mjs` (2026-11-18) finds 42 open wildfires in the Nigeria and Ghana boxes, identical in the feed and the production API, dated 2024-10-24 to 2025-03-22; the youngest is 605 days old. This is the subject of **D6**.

### There is no near-me view today

- `nearby_events` is only a field on the `ContextResponse` type in `web/src/api/events.ts`; no component renders it. `openspec/proposals/feature-current-location-marker.md` records the same ("fetched but currently unused in the UI") and lists wiring it as out of scope.
- The context response feeds the `context_resolve` analytics event (`web/src/components/EventsDashboard.tsx:326`) and centres the map (`EventsDashboard.tsx:345-351`).
- The list always comes from the events fetch. `selectedCategory` falls back to `''` ("All Categories") at `EventsDashboard.tsx:207`; no country is selected by default. The only empty state is "No events match these filters" (`EventsDashboard.tsx:466`).
- Therefore today's default view already is the "all covered countries" rung, and **Phase 3 is the first near-me interface**, not a patch to an empty state. It is a new panel, an API change and a copy change. It is not small.
- The "floods default" decision recorded in `feature-continental-coverage` was never implemented, so D2 is a documentation change.

### Other facts

- **"Near me" in the API is a fixed 200 km radius, five events**, distance-ordered: `GetNearbyEvents(..., 200.0, 5)` at `api/internal/handlers/context.go:247`.
- **Flood-first wording lives in copy, not logic:** `web/index.html:9,16,26`, `web/src/App.tsx:88,105,219–220`, and `web/src/pages/ForPartners.tsx` (lines 44, 48–49, 70, 74, 93, 98, 131–135, 247).
- **Three hard-coded country lists exist**, which Phase 1 removes: `SUPPORTED_COUNTRIES` and `COUNTRY_CENTERS` in `EventsDashboard.tsx:22-25`, and `countryCodeToName` (NG, GH) in `api/internal/handlers/country.go:13`, which `/v1/events` uses to validate the `country` parameter.
- **Governing documents.** `product.md` is LOCKED; its lock rule has three clauses (maintainer review; matching `roadmap.md` update; no feature without an F-ID in the registry) and §2.1 requires a new ADR in `decisions.md` to promote "Coverage beyond Nigeria" and "Event types beyond Floods and Wildfires". `decisions.md` indexes ADR-001 to ADR-016; **ADR-004 "Initial Country Scope: Nigeria First"** is ACCEPTED and its consequences (bounding box locked to Nigeria; only Nigerian boundaries; non-Nigeria boundary data out of scope before v0.6) conflict with `feature-continental-coverage` and with the shipped Ghana support.
- **F-IDs in practice.** The registry in `product.md` holds F-001 to F-017 (last: F-017, `product.md:302`); no F-ID above F-017 exists anywhere under `openspec/` (an earlier draft's "F-101" was a substring of the example source id `WF-1012345`). Everything shipped since launch went through OpenSpec proposals and change records, not new F-IDs.
- **The context contract documentation already disagrees with the implementation** (see Phase 3, *API*).

## Phase 0 — record the direction (documentation only)

No change under `api/internal/`, `api/cmd/` or `web/src/`. This PR touches `openspec/` and `scripts/` only, so the sentinel gate is not engaged.

### Tasks

1. **Commit a by-month wildfire count** to `scripts/eonet-density/` (wildfires are point geometry, so coordinates are sound; apply the production boxes and `withinBBox`, and state that the Africa rectangle over-counts). Replace the "likely" seasonal claim in the proposal with its output. The same script reports the non-flood, non-wildfire categories that back D3 (the 13 storms and 4 volcanoes are currently scratch figures).
2. **Commit an age-of-open-wildfires script** (same directory): for the production Nigeria and Ghana boxes, count open wildfires by age band using the feed's event date. Quote its output in the proposal under D6, replacing the reviewer's uncommitted "42 wildfires, October 2024 to March 2025".
3. **Write ADR-017** in `openspec/specs/vigilafrica/decisions.md`: continental, hazard-neutral scope. It must (a) supersede ADR-004 and mark ADR-004 superseded in the index; (b) state which ADR-004 consequences are void (Nigeria-only bounding box and boundary loading) and which stand; (c) promote "coverage beyond Nigeria" and "event types beyond floods and wildfires" per `product.md` §2.1, or explicitly keep the latter out (D3); (d) **settle the F-ID rule**: either retire it, or assign F-IDs from now on and say who assigns them and how existing proposal-governed features are treated. The rule cannot stay as written while the practice differs from it.
4. **`openspec/specs/vigilafrica/product.md`:** add a dated direction addendum to §1–§2 referencing ADR-017. Mark the "MVP Scope (Locked)" table as historical. Do not rewrite the F-ID registry; record any registry entry verified as out of step with the code as **registered drift** (see below), not fixed here. "The categories the product supports" (floods and wildfires) is the wording for D3; there is no category registry (`roadmap.md`'s v1.3 note records the registry only as a deferred item, `chore-deferred-work-register` §C1; it was never built).
5. **`openspec/specs/vigilafrica/roadmap.md`:** set v1.3 to shipped in the milestone index (row currently reads *Proposed*, `roadmap.md:26`); add rows for v1.4, v1.5 and v1.6 from the release history; add the milestone for this direction with Phases 1–4 as its content.
6. **`openspec/proposals/feature-continental-coverage.md`:** dated note reversing "floods default, wildfires opt-in"; answer open question 6 per D1. Do not touch its "Decision 1" or "Decision 2".
7. **`openspec/proposals/feature-weekly-flood-brief.md`:** add a one-line pointer that its premise is affected by this direction. Do not rewrite it.

### Registered drift (not fixed here)

- **F-008 (`GET /v1/context`)** in `product.md`, and `api-contract.md` §5, disagree with the implementation: they describe a list `events` in the caller's resolved country/state, with location fields `country_code`/`country_name`/`state_name`, no query parameters, `X-Forwarded-For` read directly, and "always 200, never an error". The implementation returns `nearby_events` (within 200 km, at most five), a location with `country`/`state`/`lat`/`lng`, a `location_source` field, accepts `lat` and `lng`, honours forwarded headers only from a trusted proxy, returns 400 on malformed coordinates and 500 on a database failure (`api/internal/handlers/context.go`). F-008's own "Out of Scope" lists lat/long radius search, which is implemented. Bringing the contract documents into agreement with the endpoint is larger than it reads; Phase 3 must do it for the fields it touches and list what remains.

### Acceptance criteria

- [ ] Every figure in the proposal's evidence section is reproducible from a script committed under `scripts/`, or is explicitly labelled as not backed by one.
- [ ] `product.md` and `roadmap.md` agree with each other on **scope and milestone statements**. (They are not required to agree with production at feature-registry level while the registry is untouched; that is the registered drift above.)
- [ ] The roadmap milestone index has no row marked *Proposed* for work that has shipped.
- [ ] ADR-017 exists, supersedes ADR-004 in the ADR index, and states the F-ID outcome.
- [ ] No file under a sentinel-gated path is touched.

## Phase 1 — continental coverage

Specified in [`feature-continental-coverage`](../proposals/feature-continental-coverage.md): its work items, its two blocking decisions and its acceptance criteria apply. **Moved ahead of the gate and the near-me panel on 2026-11-18, by the maintainer.** It starts once ADR-017 (Phase 0 task 3) is merged, because `product.md` §2.1 requires an ADR before coverage beyond Nigeria. It touches `api/internal/` and `web/src/`, so each PR carries its own governance record under `openspec/changes/<id>/` (see the sentinel note in Phase 3).

Carried over from this change:

- **Decisions at the start:** that proposal's "Decision 1 — how are boundaries loaded?" and "Decision 2 — one Africa bbox, or 54 country bboxes?", and what to do with Madagascar's 2010 boundaries.
- **Tranching:** the first release covers the 45 countries whose HDX boundaries are published as GeoJSON. The 9 shapefile-only countries (BWA, DJI, GAB, LSO, MDG, MUS, RWA, SYC, TZA) and South Africa's 89 MB file follow.
- **Hard-coded country lists:** `SUPPORTED_COUNTRIES` and `COUNTRY_CENTERS` (`web/src/components/EventsDashboard.tsx:22-25`) and `countryCodeToName` (`api/internal/handlers/country.go:13`) are replaced by one list derived from the loaded boundaries, so Phase 3 is built once for every covered country.
- **Country picker:** that proposal's work item 7. Phase 3's country rung reuses it rather than adding a second way to choose a country.
- **Copy, shipped with the first release:** hazard-neutral landing and metadata text (`web/index.html`, `App.tsx`). The partner page changes per D4: the lead may become hazard-neutral, but it must still describe the digest as flood-only (`ForPartners.tsx:48-49`, `:74`), and partner material may lead with floods in season. **`web/vite.config.ts`** generates `llms.txt` (lines ~134-171) and defines `SITEMAP_ROUTES` (line 84); both stay in step (the `llms.txt` text currently says "floods and wildfires" and "Nigeria and Ghana are live").
- **Stale-open wildfires:** continental ingestion multiplies them. The list is ordered newest first (`api/internal/database/queries.go:136`), so they sink below current events; the D6 window arrives in Phase 3. After release, record how many appear on the first page; if they crowd it, bring the window forward.
- **Early stop rule:** checked and recorded before the first release (Phase 2).

### Acceptance criteria (in addition to `feature-continental-coverage`'s)

- [ ] ADR-017 is merged before the first Phase 1 PR.
- [ ] No two-country list remains in `api/internal/` or `web/src/` (grep for the three names above returns only the derived list).
- [ ] Landing and metadata copy, including `llms.txt`, does not name floods ahead of other hazards, and nothing claims local or real-time coverage. The partner page still describes the digest as flood-only; partner material may lead with floods in season. (Checked by a test or grep over the three copy sources: `index.html`, `App.tsx`, `ForPartners.tsx`.)
- [ ] The early stop check is recorded on the release PR before release.
- [ ] An independent review is recorded on each PR.

## Phase 2 — validation gate

Not engineering. Runs alongside Phase 0 and Phase 1. Recorded so the criteria are fixed before the results are known. Thresholds are the proposal's proposed values and need the maintainer's confirmation.

- [ ] Five structured conversations requested during Phase 0; each one held is written up the same day (how they learned of the last event, the location precision they needed, what they use today, their unprompted reaction, follow-up agreed or not).
- [ ] **Early stop check before the Phase 1 release** (proposed by 2026-12-15): the falsifier applied to the first three conversations held. If it triggers, Phase 1 is not released.
- [ ] Falsifier applied as written in the proposal at the gate (at least three of the conversations held, minimum three held, needing neighbourhood- or street-level precision means freeze).
- [ ] The traffic read-out is recorded each time analytics is read, including when it is zero, from the first full week after the Phase 1 release.
- [ ] A dated go / hold / freeze decision **before Phase 3 starts** (proposed 2027-01-12 if at least three conversations are held by then, otherwise 2027-03-31).

## Phase 3 — near-me panel with a scope ladder

**D5 and D6 decided 2026-11-18. Blocked on a *go* from the Phase 2 gate, on the D6 window length (Phase 0 task 2) and on `feature-current-location-marker` landing.** Built after Phase 1, so every rung serves all covered countries. Touches `api/internal/` and `web/src/`, so the PR needs a governance record **in its own diff**. The sentinel (`api/cmd/sentinel/main.go`, `isGovernanceRecord`) accepts only a changed file under `openspec/proposals/` or `openspec/changes/` (archive paths excluded). **A file under `openspec/specs/`, including this spec, does not count, and neither does a record merged in an earlier PR.** The Phase 3 PR must add `openspec/changes/<id>/` (proposal, spec, tasks) or modify a file under `openspec/proposals/`.

Sequencing: `feature-current-location-marker` (frontend only, IP-derived location labelled approximate) should land first. Phase 3 reuses its approximate-location wording; if the marker has not landed, Phase 3 carries equivalent wording itself.

### API

- **`GET /v1/context` gains scope counts.** Alongside `nearby_events`, return how many events fall in each rung for the caller's resolved location: within the nearby radius, within the caller's country, and across all covered countries.
- **The basis of every count is D6-dependent and must be stated by the API**, not inferred by the client. Under D6(a) the counts cover a display window and the API returns the window; under D6(b) the counts are unwindowed, split by `status`, and the API states that they cover everything ingested.
- **"Additive fields only" does not hold as first written.** A windowed count beside an unwindowed, distance-ordered, five-item `nearby_events` would disagree with it. Resolve under D6: under (a), `nearby_events` is windowed too, which changes existing behaviour of `/v1/context` (and `/v1/events` if the window is applied there); under (b), counts and `nearby_events` share the unwindowed basis and the change is additive. Whichever is chosen, say so in the contract.
- **Cases the ladder must define** (checked against `api/internal/handlers/context.go`):
  - *Explicit `lat`/`lng`.* The handler leaves `Country` and `State` empty by design (`parseExplicitLocation`: "no point-to-admin-name resolver exists"). The country rung is therefore unavailable for explicit coordinates unless a resolver is added, which is out of scope. Specified behaviour: the near rung and the all-countries rung are returned; the country rung is omitted and the response says why.
  - *`location: null`.* No GeoIP database (`h.geo == nil`), a failed lookup, or localhost leaves `Location` nil with `location_source: "unavailable"`; the handler deliberately has no hidden default centre. Specified behaviour: no near or country rung; the all-countries rung is returned so the panel is not empty.
  - *Visitor outside every covered country.* The IP resolves, the 200 km query returns nothing, and the country matches no covered country. Specified behaviour: the country rung is zero and the panel shows the all-countries rung, saying that the visitor's country is not covered.
- **Country matching.** The country rung matches the geolocation provider's country name (`geoip` reader, English name from the MaxMind record, `api/internal/geoip/reader.go`) to `events.country_name`, which comes from boundary data. This was built for two countries and is a **known risk at 54** (spelling and naming differences such as "Côte d'Ivoire" or "Democratic Republic of the Congo"). Prefer matching on country code if the boundary data carries one; otherwise add a test table that checks the provider's name against the stored name for every country loaded in Phase 1.
- **Performance.** The earlier "reuse the existing spatial index" is withdrawn. The GiST index is on `geom` as geometry (`api/db/migrations/000001_create_events.up.sql:30`) while `GetNearbyEvents` filters on `geom::geography`; whether it is used is unverified. The counting queries need an `EXPLAIN` check. The country and all-countries counts are not spatial. The budget is `product.md` §5: **Context endpoint < 200ms** (`product.md:320`; F-008 repeats it at `:186`; `api-contract.md` §5 states "< 200ms (p99)"). Three counts must fit that budget, and the budget's premise ("local GeoIP lookup only") no longer holds once the handler runs database queries; the existing nearby query already breaks that premise.
- **Contract first:** edit `openspec/specs/vigilafrica/openapi.yaml`, run `npm run sync:openapi`, and update `api-contract.md`, including the registered F-008 drift above for every field touched.

Components: `api/internal/handlers/context.go`, `api/internal/handlers/context_test.go`, `api/internal/database/queries.go` (counting queries), `api/internal/models/`, `openapi.yaml`.

### Web

- **Scope ladder** in `web/src/components/EventsDashboard.tsx` (or a new component it renders): a near-me panel showing near you, your country, all covered countries, each with a count. **The ladder action** is: choosing a rung applies that rung as the dashboard's list and map filter (near: nearby events; country: sets the country filter; all: clears the country filter), shows which rung is active, and can be undone by choosing another rung. Never render a bare "no events".
- **Approximate-location label.** Where the panel uses the IP-derived location, it says the location is approximate and based on the visitor's network (same requirement as `feature-current-location-marker`).
- **Basis statement** next to the list heading, driven by the API value (D6), for example "open, or closed since DATE" or "in the last N days".
- **Coverage statement:** one persistent sentence that events come from global satellite-and-model feeds (NASA EONET; floods are GDACS-sourced, which combines models with impact thresholds per `feature-continental-coverage`), and that street-level and urban flash flooding are not covered. It extends the standing honesty rule (the `dashboard-disclaimer` block, `EventsDashboard.tsx:167-176`, and `llms.txt` "Important limitation", `web/vite.config.ts:141`) and must not be dismissible into absence.
- **Types:** extend the `ContextResponse` type in `web/src/api/events.ts` with the new fields.
- **Analytics:** a new event for the ladder action (proposed name `scope_ladder_selected`, payload `{ rung }`), added to the table in `docs/operations/analytics.md` (existing rows at lines 118-119), so validation can tell whether anyone used the ladder.
- **Category rendering:** the `category === 'floods' ? 'flood' : 'fire'` branches in `web/src/components/EventsDashboard.tsx:562,576`, `web/src/components/Map.tsx:66,70` and `web/src/pages/EventDetail.tsx:79,92` are left as they are. They are correct for two categories; D3 defers a third.
- **Tests:** `web/src/components/EventsDashboard.test.tsx` (ladder, each rung's empty case, the null-location case, the basis statement); `api/internal/handlers/context_test.go` (explicit coordinates, null location, outside-covered-country, each count).

### Acceptance criteria

- [ ] With a location that has no event within the nearby radius, the panel shows a non-zero count for a wider rung whenever one exists, and the ladder action applies that rung to the list and map.
- [ ] With explicit `lat`/`lng`, with `location: null`, and with a location outside every covered country, the panel renders the specified fallback rung and no bare "no events"; each case has a handler test and a dashboard test.
- [ ] With no events at any rung, the dashboard says so and still shows the basis and coverage statements.
- [ ] The basis shown in the interface equals the value the API returns; changing it server-side changes the interface with no web edit.
- [ ] The behaviour chosen under D6 is recorded in `openapi.yaml`, and counts and `nearby_events` share one basis.
- [ ] An `EXPLAIN` of each counting query is recorded on the PR, and the endpoint stays within the `product.md` §5 context budget (measured, with the script committed under `scripts/bench-*`).
- [ ] The `scope_ladder_selected` event fires on the ladder action and is documented in `docs/operations/analytics.md`.
- [ ] `openapi.yaml`, the synced handlers copy and `api-contract.md` agree for every touched field; remaining F-008 drift is listed.
- [ ] `go vet ./...` and the Go suite pass (via `scripts/test-api.ps1` on the Windows host); `npm run lint`, `type-check`, `lint:styles`, `test` and `build` pass in `web/`.
- [ ] New colours, if any, are tokens in `web/src/styles/tokens.css`.
- [ ] The PR carries its own governance record under `openspec/changes/<id>/` or `openspec/proposals/`.
- [ ] An independent review is recorded on the PR (a review comment or review from someone other than the author, or a recorded `sol-review` run, with its findings and their resolution).

## Phase 4 — judge it in season

Not engineering. A dated go / hold / freeze decision at the end of the 2027 rainy season, using the proposal's proposed thresholds for external visitors, ladder use and a partner using the data.

## Out of scope

- New hazard categories and any category registry (D3).
- Any change to `/v1/digest/today.json` (D4).
- New data sources, alerts, accounts, ADM2 naming, local-report ingestion.
- Browser GPS and a point-to-admin-name resolver for explicit coordinates.
- Performance work beyond the counting-query check. Nothing here changes list or marker volume; both remain page-limited.

## Open questions

1. Should the nearby radius stay at 200 km once the country rung exists, or shrink so "near you" means something tighter?
2. For a visitor whose IP resolves outside every covered country, which rung is the default — all covered countries, or a prompt to pick one? (This spec specifies the all-covered-countries fallback; the maintainer may prefer a prompt.)
3. Does the coverage statement belong on the event detail page as well as the dashboard?
4. D6 is decided as option (a). Open: the window length (from Phase 0 task 2), and whether `/v1/events` adopts it or only `/v1/context`.
