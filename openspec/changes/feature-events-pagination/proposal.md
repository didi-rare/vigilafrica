# Proposal: Paginate the Events List (feature-events-pagination)

**Status:** Proposed — prerequisite for [feature-continental-coverage](../../proposals/feature-continental-coverage.md).

## Why

`GET /v1/events` defaults to `limit = 50` ([`events.go:76`](../../../api/internal/handlers/events.go)) and the web client **never sends a `limit` or `offset`** ([`events.ts:63-68`](../../../web/src/api/events.ts)). The API returns `meta.total`, `meta.limit` and `meta.offset` — and `EventsDashboard.tsx` **reads none of them**.

So the dashboard shows **at most 50 events and never says there are more.**

Today that is invisible: production holds 43 events, so nothing is hidden. **At continental scale it shows 50 of ~3,268 and tells the user nothing.** A resident filtering to their country would reasonably conclude they are seeing every event near them.

⚠️ **This is a trust defect, not a performance one.** In a safety-adjacent product, silently withholding 98% of the data is worse than being slow. It is the reason this must ship *before* the ingestion box widens.

### It also corrects a false statement in the live spec

> *"the map SHALL display event markers for **all open events** fetched from `GET /v1/events`"* — Requirement: Interactive Map Frontend

That has never been true since the 50-item default existed. The spec is amended here to describe what the system does and will do.

## ⚠️ This supersedes the scope of `perf-mobile-first-render`

That proposal (referenced in three archived documents, **never actually written**) was premised on the events list growing unbounded — *"~14,000px stacked at 768px"*, *"2,000 markers on a mid-range Android"*. **Both premises are false.** The list and the marker set derive from the same `limit`-capped array, so they go from 43 items to 50, not to thousands.

Virtualising a 50-item list buys nothing. The real work is pagination. Mobile TBT remains a separate, **currently unverified** question — see Out of Scope.

## Decision: keep offset pagination; do not move to cursors

| | offset (chosen) | cursor / keyset |
|---|---|---|
| already in the contract | ✅ `limit`/`offset` parsed and validated; `meta` returned | ❌ breaking change |
| supports "showing 1–50 of ~3,268" | ✅ natural | ⚠️ awkward; totals need a separate count |
| stable under concurrent writes | ⚠️ needs a deterministic order (fixed below) | ✅ inherently |
| deep-page cost | ⚠️ degrades at high offsets | ✅ constant |

**Offset wins here.** Users reach this data through country/category/state filters, so deep pagination is not the access pattern; and `total` is precisely the number that makes the truncation visible, which is the whole point of the change. Cursors would be a breaking contract change bought for a scale problem this product does not have.

## ⚠️ The correctness bug offset pagination would otherwise expose

```sql
ORDER BY event_date DESC NULLS LAST, ingested_at DESC   -- queries.go:109
```

**There is no unique tiebreaker.** Rows tying on both columns have no guaranteed order between queries, so a paging client can see a row twice or miss it entirely.

Measured against real data:

- **43 events across 32 distinct `event_date` values — largest tie group is 3.** So `event_date` ties are real and routine.
- Those ties are currently broken by `ingested_at`, which had **0 duplicates** — each upsert runs as its own implicit transaction (`pool.Exec`, no explicit `Begin`), so `now()` differs per row.
- **But `ON CONFLICT DO UPDATE SET ingested_at = NOW()`** ([`db.go:114`](../../../api/internal/database/db.go)) means **re-ingesting an event bumps its sort key.** Every ingestion tick reshuffles the order *within* each `event_date` tie group.

So a user paging through during an ingestion run can legitimately see a duplicate or skip an event. Invisible at 43 events on one page; a real defect once pages exist.

### ⚠️ A tiebreaker alone is NOT enough — corrected after independent review

An earlier revision of this proposal claimed appending `id` made paging exactly-once. **It does not.** `id` only resolves rows whose sort keys are *equal*; it cannot stop a row from **moving**. Because `ingested_at` is reset on every re-upsert, a re-ingested event jumps to the top of its `event_date` group — so a client on page 2 can see an event it already saw on page 1, and miss the one that moved. Offset pagination is only exactly-once over an ordering that does not change under it.

**Fix: order by columns that do not move, and drop the one that does.**

```sql
ORDER BY event_date DESC NULLS LAST, id DESC
```

`id` is a `UUID PRIMARY KEY` — immutable. `event_date` comes from the source event and is stable in practice (an upsert may revise it, but that is a genuine data correction, not routine churn). Removing `ingested_at` from the sort removes the only term that changes on every ingestion tick.

⚠️ **This is a real, if reduced, weakening of the guarantee, and it is stated rather than hidden.** Offset pagination over a live table cannot be exactly-once in the strict sense: a newly-inserted event with a recent `event_date` still shifts later rows by one. Eliminating that entirely needs keyset pagination or a snapshot, both rejected above as disproportionate here. What this achieves is that the ordering no longer churns on **every ingestion tick** — it changes only when events are genuinely added or re-dated. The spec is written to require that, not perfection.

Only `ListEvents` needs this. `GetNearbyEvents` takes a `LIMIT` but no `OFFSET`, so it is not paginated.

## What Changes

### API

1. Order by `event_date DESC NULLS LAST, id DESC` in `ListEvents` — **removing** the mutable `ingested_at` from the sort, not merely appending to it.
2. No contract change. `limit` (default 50, max 200), `offset`, and the `meta` block already exist and are already validated.

### Frontend

3. Send `limit` and `offset` from `fetchEvents`; carry `offset` in the query key so pages cache independently.
4. Render pagination controls and an explicit **"Showing 1–50 of ~3,268"** count — the truncation must be *visible*, which is the point of the change.
5. **Reset `offset` to 0 whenever a filter changes.** Otherwise changing country while on page 5 lands on an empty page.
6. Keep the map showing the **current page's** markers, consistent with the list.

## Out of Scope

- **Mobile TBT / bundle work.** The 1,140 ms figure is inherited and **unverified this session**; `map-vendor` (946 kB raw / 246 kB gzip) was found to be **already off the critical path** — `lazy(Map)` inside `lazy(EventsDashboard)`, and `index.html` modulepreloads only `rolldown-runtime` and `react-vendor`. Re-measure on staging before proposing anything there.
- Cursor pagination, and raising the 200 cap.
- Server-side clustering or marker aggregation for the map.
- Infinite scroll — explicitly rejected; it hides the total, which is the number that matters here.

## Verification

- [ ] Paging through a filtered result set returns each event exactly once **while an ingestion run re-upserts existing events** — the case the old ordering failed, since `ingested_at` churn no longer moves rows
- [ ] `meta.total` matches a direct database `COUNT(*)` for the same filters — *not* an "unpaginated fetch", which the API has no mode for (`limit` is capped at 200)
- [ ] Changing any filter resets to page 1
- [ ] Requesting an offset beyond `total` returns `200` with an empty `data` array, not an error
- [ ] The visible count reflects the real total, not the page size
- [ ] Existing 70+ web tests and the API suite still pass
