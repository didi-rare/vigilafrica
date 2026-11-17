# Spec: Place-First Direction (feature-place-first-direction)

**Status:** Proposed — nothing here is implemented. Decisions D1–D5 in the proposal were made on 2026-11-17, so Phase 0 may start.
**Companion:** [`openspec/proposals/feature-place-first-direction.md`](../proposals/feature-place-first-direction.md) (rationale, evidence, decisions, risks).

## Context

The product's stated question is "what is happening near me?", answered with administrative place names. Three facts about the current tree shape this spec. Line numbers are as of `origin/development` on 2026-11-17; re-check them before editing.

- **The dashboard is already hazard-neutral by default.** `selectedCategory` falls back to `''` ("All Categories") in `web/src/components/EventsDashboard.tsx:207`, and the list is ordered by `event_date DESC` (`api/internal/database/queries.go:136`). The "floods default" decision recorded in `feature-continental-coverage` was never implemented, so D2 is a documentation change.
- **"Near me" is a fixed 200 km radius, five events.** `GetContext` calls `GetNearbyEvents(..., 200.0, 5)` (`api/internal/handlers/context.go:247`). When nothing falls inside that radius the response carries an empty `nearby_events` and nothing else to fall back on.
- **Flood-first wording lives in copy, not logic:** `web/index.html:9,16,26`, `web/src/App.tsx:88,105,219–220`, and throughout `web/src/pages/ForPartners.tsx` (notably `:48–49`, `:74`, `:93`, `:131–135`, `:247`).

⚠️ **Not verified for this spec:** how the dashboard consumes `nearby_events` today, and where the current empty state is rendered. Establish both at the start of Phase 1 before designing the ladder.

## Phase 0 — record the direction (documentation only)

No change under `api/internal/`, `api/cmd/` or `web/src/`.

### Tasks

1. **Commit a by-month wildfire count** to `scripts/eonet-density/` (wildfires are point geometry, so coordinates are sound; apply the production boxes and `withinBBox`, and state that the Africa rectangle over-counts). Replace the "likely" seasonal claim in the proposal with its output. Same script reports the non-flood, non-wildfire categories that back D3.
2. **`openspec/specs/vigilafrica/product.md`:** add a dated direction addendum to §1–§2. Mark the "MVP Scope (Locked)" table as historical; do not rewrite the F-ID registry.
3. **`openspec/specs/vigilafrica/roadmap.md`:** set v1.3 to shipped in the milestone index; add rows for v1.4, v1.5 and v1.6 from the release history; add the milestone for this direction with Phases 1–3 as its content.
4. **`openspec/proposals/feature-continental-coverage.md`:** dated note reversing "floods default, wildfires opt-in"; answer open question 6 per D1.
5. **`openspec/proposals/feature-weekly-flood-brief.md`:** add a one-line pointer that its premise is affected by this direction. Do not rewrite it.

### Acceptance criteria

- [ ] Every figure in the proposal's evidence section is reproducible from a script committed under `scripts/`.
- [ ] `product.md` and `roadmap.md` agree with each other and with what is in production.
- [ ] The roadmap milestone index has no row marked *Proposed* for work that has shipped.
- [ ] No file under a sentinel-gated path is touched.

## Phase 1 — an honest "near me"

Touches `api/internal/` and `web/src/`, so the PR needs this change record in its own diff.

### API

- **`GET /v1/context` gains scope counts.** Alongside `nearby_events`, return how many events fall in each rung of the ladder for the caller's resolved location: within the nearby radius, within the caller's country, and across all covered countries, each for the same recency window the events list uses. Additive fields only; existing fields and their meaning do not change.
- **The recency window is stated by the API**, not inferred by the client, so the interface can say "in the last 30 days" from one source of truth.
- **Contract first:** edit `openspec/specs/vigilafrica/openapi.yaml`, run `npm run sync:openapi`, and update `api-contract.md`.

Components: `api/internal/handlers/context.go`, `api/internal/database/queries.go` (one counting query; reuse the existing spatial index), `api/internal/models/`.

### Web

- **Scope ladder** in `web/src/components/EventsDashboard.tsx`: when the nearby rung is empty, show the count for the next non-empty rung and a one-action way to view it. Never render a bare "no events".
- **Recency statement** next to the list heading, driven by the API value.
- **Coverage statement:** one persistent sentence that the product shows satellite-detected events and does not see street-level or urban flash flooding. Wording is reviewed against the product's standing honesty rule; it must not be dismissible into absence.
- **Copy:** hazard-neutral landing, partner and metadata text. `index.html` metadata, the generated `llms.txt` and `SITEMAP_ROUTES` descriptions stay in step.
- **Category rendering:** the `category === 'floods' ? 'flood' : 'fire'` branches in `EventsDashboard.tsx:562,576`, `Map.tsx:66,70` and `EventDetail.tsx:79,92` are left as they are. They are correct for two categories; D3 defers a third.

### Acceptance criteria

- [ ] With a location that has no event within the nearby radius, the dashboard shows a non-zero count for a wider rung whenever one exists, and the action to view it works.
- [ ] With no events at any rung, the dashboard says so and still shows the recency and coverage statements.
- [ ] The recency window shown in the interface equals the value the API returns; changing it server-side changes the interface with no web edit.
- [ ] No landing, partner or metadata copy names floods ahead of other hazards or claims local or real-time coverage the feed cannot supply.
- [ ] `openapi.yaml`, the synced handlers copy and `api-contract.md` agree.
- [ ] `go vet ./...` and the Go suite pass (via `scripts/test-api.ps1` on the Windows host); `npm run lint`, `type-check`, `lint:styles`, `test` and `build` pass in `web/`.
- [ ] New colours, if any, are tokens in `web/src/styles/tokens.css`.
- [ ] Independent review before merge.

## Phase 2 — continental coverage

Specified in [`feature-continental-coverage`](../proposals/feature-continental-coverage.md). Two things carry over from this change:

- The ladder's top rung becomes "across Africa" with no web change, because it is defined as "all covered countries".
- Its work item 7 (country picker for 54 countries) should reuse the ladder's country rung rather than introduce a second way to choose a country.

## Phase 3 — validation

Not engineering. Recorded so the criteria are fixed before the results are known.

- [ ] Five structured conversations requested; at least three held, each written up the same day (how they learned of the last event, the location precision they needed, what they use today, their unprompted reaction, follow-up agreed or not).
- [ ] The traffic read-out is recorded each time analytics is read, including when it is zero.
- [ ] A dated go / hold / freeze decision before Phase 2 starts, and again at the end of the 2027 rainy season.

## Out of scope

- New hazard categories and the category registry (D3).
- Any change to `/v1/digest/today.json` (D4).
- New data sources, alerts, accounts, ADM2 naming, local-report ingestion.
- Performance work. Nothing here changes list or marker volume; both remain page-limited.

## Open questions

1. Should the nearby radius stay at 200 km once the country rung exists, or shrink so "near you" means something tighter?
2. For a visitor whose IP resolves outside every covered country, which rung is the default — all covered countries, or a prompt to pick one?
3. Does the coverage statement belong on the event detail page as well as the dashboard?
