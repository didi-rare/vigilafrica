# Tasks: Paginate the Events List

## 1. API — deterministic ordering (correctness, do this first)

- [ ] 1.1 Order by `event_date DESC NULLS LAST, id DESC` in `ListEvents` (`queries.go:109`) — **remove** the mutable `ingested_at` from the sort rather than appending after it
- [ ] 1.2 Test that events tying on `event_date` return in a stable order across repeated identical queries
- [ ] 1.3 Test that walking a result set by `offset` yields each event exactly once while existing events are re-upserted (which resets `ingested_at`) — the case the previous ordering failed
- [ ] 1.4 Document the residual limit honestly: a genuinely NEW event with a recent `event_date` still shifts later rows by one. Only keyset pagination or a snapshot removes that, and both are out of scope.
- [ ] 1.5 Confirm `GetNearbyEvents` needs no change — it takes `LIMIT` but no `OFFSET`, so it is not paginated
- [ ] 1.6 Measure whether a composite index on `(event_date DESC, ingested_at DESC, id DESC)` is warranted, or whether the existing `idx_events_event_date` suffices. **Measure before adding — the `area_m2` work showed an index that `EXPLAIN` never reads.**

## 2. Frontend — data layer

- [ ] 2.1 Add `limit` and `offset` parameters to `fetchEvents` (`web/src/api/events.ts`)
- [ ] 2.2 Include `offset` in the TanStack query key so pages cache independently
- [ ] 2.3 Consider `placeholderData` to avoid a full loading state on page change (evaluate; do not assume it improves the experience)

## 3. Frontend — interface

- [ ] 3.1 Surface `meta.total` as an explicit "Showing X–Y of Z" count
- [ ] 3.2 Add previous/next page controls, disabled correctly at both ends
- [ ] 3.3 Reset `offset` to 0 whenever country, state, or category changes
- [ ] 3.4 Keep map markers in sync with the current page
- [ ] 3.5 Ensure controls are keyboard reachable and announce page changes to assistive technology — the site is at Accessibility 100 and must stay there

## 4. Verification

- [ ] 4.1 Paging returns each event exactly once during an active ingestion run
- [ ] 4.2 `meta.total` equals the count from an unpaginated fetch with the same filters
- [ ] 4.3 Offset beyond `total` returns 200 with empty `data`, not an error
- [ ] 4.4 Filter change returns to page 1
- [ ] 4.5 Existing web tests and the API suite pass
- [ ] 4.6 Re-check CLS — the dashboard height reservation (`#193`/`#198`) is sized against today's list; a page-size change may need it re-measured

## 5. Explicitly not in this change

- [ ] 5.1 ~~Mobile TBT / bundle work~~ — the 1,140 ms figure is unverified this session and `map-vendor` is already off the critical path. Re-measure on staging first.
- [ ] 5.2 ~~Cursor pagination, raising the 200 cap, map clustering, infinite scroll~~
