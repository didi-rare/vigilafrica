# Tasks: Paginate the Events List

## 1. API — deterministic ordering (correctness, do this first)

- [x] 1.1 Order by `event_date DESC NULLS LAST, id DESC` in `ListEvents` (`queries.go:109`) — **remove** the mutable `ingested_at` from the sort rather than appending after it
- [x] 1.2 Test that events tying on `event_date` return in a stable order across repeated identical queries — `TestListEventsOrderingIsStableAcrossIdenticalQueries`. ⚠️ **This test passes on the OLD ordering too**: with 6 tied rows Postgres happened to be deterministic anyway. It is a guard, not the regression detector. 1.3 is the detector.
- [x] 1.3 Test that walking a result set by `offset` yields each event exactly once while existing events are re-upserted (which resets `ingested_at`) — `TestListEventsPagingIsExactlyOnceDuringReIngestion`. **Verified to fail on the old ordering before the fix landed**: 2 duplicates and 2 skipped events out of 6. Passes on the new ordering.
- [x] 1.4 Document the residual limit honestly: a genuinely NEW event with a recent `event_date` still shifts later rows by one. Only keyset pagination or a snapshot removes that, and both are out of scope. — stated in the `ListEvents` comment and the spec delta.
- [x] 1.5 Confirm `GetNearbyEvents` needs no change — confirmed at `queries.go:189-206`: it takes `LIMIT $4` and no `OFFSET`, so it is not paginated. Unchanged.
- [x] 1.6 Measure whether a composite index is warranted. **Measured — and the answer is yes, which overturned the prior.** Harness committed at `scripts/bench-events-ordering/bench.sql`; migration `000014` adds it.
  - ⚠️ The candidate is **not** the one this task guessed. `ingested_at` is gone from the sort, and `idx_events_event_date` cannot serve the ordering at all: it is `(event_date DESC)`, which Postgres defaults to **NULLS FIRST**, while the query asks **NULLS LAST**. The working form is `(event_date DESC NULLS LAST, id DESC)`.
  - Median of 40 runs at 3,268 rows, two independent runs: page 1 **2.11/2.15 ms → 0.09/0.15 ms**; `country=Nigeria` **3.31/4.16 ms → 0.22/0.36 ms**. Plan goes `Seq Scan` + top-N heapsort → `Index Scan`; buffer hits 128 → 51 (a second, independent signal).
  - ⚠️ Deep pages (`offset 3200`) do **not** use it — the planner correctly reverts to `Seq Scan` + quicksort. And the absolute saving is only ~2–4 ms; the handler's `COUNT(*)` still seq-scans and remains the dominant cost.

## 2. Frontend — data layer

- [x] 2.1 Add `limit` and `offset` parameters to `fetchEvents` (`web/src/api/events.ts`) — both always sent, with `offset` clamped at 0 so a bad value cannot surface as a 400.
- [x] 2.2 Include `offset` in the TanStack query key so pages cache independently — `eventKeys.list(country, category, state, offset)`.
- [x] 2.3 Evaluated `placeholderData`, and **adopted for page changes only** — see 4b.2. The reason is specific to this dashboard, not a general preference: without it, changing page empties the list for a round trip, and on mobile — where `.dashboard-layout` is `height: auto` — the section collapses and re-expands, the exact shift class #193/#198 removed. ⚠️ Plain `keepPreviousData` was **wrong** here because it also carried rows across a *filter* change; it is now a function gated on the filters being unchanged. While placeholder data is on screen the pager is frozen (`isPlaceholderData`) and the range label describes the rendered rows, not the requested page.

## 3. Frontend — interface

- [x] 3.1 Surface `meta.total` as an explicit "Showing X–Y of Z" count — "Showing 1–50 of 3,268 matching events", derived from `meta` **as the server applied it**, not from what the client requested.
- [x] 3.2 Add previous/next page controls, disabled correctly at both ends. `canNext` is computed from rows actually returned rather than `currentPage < totalPages`, so an out-of-range page cannot offer a Next that leads nowhere. A past-the-end page gets a **First page** button, because stepping back one page from `?page=999` lands on another empty page.
- [x] 3.3 Reset to page 1 whenever country, state, or category changes — each filter handler drops `page` from the URL.
- [x] 3.4 Keep map markers in sync with the current page — `mapEvents` already derives from `eventsData.data`, so this holds by construction; covered by a test that asserts page 2's markers replace page 1's rather than merging.
- [x] 3.5 Keyboard reachable and announced. Native `<button>`s (tested: still in the tab order, and Enter drives them). The count is a `role="status"` live region, so the range announces on every page change. ⚠️ That makes **two** polite live regions on the page, so both it and the freshness banner gained an `aria-label` to stay individually addressable. **axe reports 0 violations with the pager rendered** (new test).

## 4. Verification

- [x] 4.1 Paging returns each event exactly once during an active ingestion run — `TestListEventsPagingIsExactlyOnceDuringReIngestion` against real PostGIS. **Confirmed to fail before the fix** (2 duplicates, 2 skips out of 6) and pass after.
- [x] 4.2 `meta.total` equals a direct count for the same filters — asserted at every offset in the same test (`total != n` fails the run), including on the past-the-end page. Note the proposal's correction: the API has no unpaginated mode (`limit` caps at 200), so this is checked against the known seeded count, not against a second fetch.
- [x] 4.3 Offset beyond `total` returns 200 with empty `data`, not an error — `TestListEventsOffsetBeyondTotal` at the repository layer, plus a UI test asserting no `alert` role appears and a way back is offered.
- [x] 4.4 Filter change returns to page 1 — UI test starts at `?page=4` (offset 150), changes country, asserts the next fetch uses offset **0**.
- [x] 4.5 Existing web tests and the API suite pass — web **82/82** (was 70; 12 new), API `go test ./...` all packages, database integration suite green. `npm run type-check`, `lint`, `lint:styles` and `audit:ci` all clean locally.
- [x] 4.6 Re-checked CLS, and it **caught a regression this change introduced**. Measured at 1920x1600 with the dashboard chunk delayed 1,200 ms, n=8 per arm, against a build of `origin/development` as a real control:
  - The mounted `#dashboard` grows **1459px → 1523px** (+64px, identical at 43 and 3,268 events — the bar's height is fixed, not row-dependent). `.dashboard-fallback`'s cap raised 1460px → 1530px to keep it at the real mounted height. ⚠️ **That cap change was measured to affect CLS by nothing** — at any realistic viewport the content below the reservation is already off-screen. It is invariant maintenance; the comment says so rather than implying a fix.
  - **The real regression:** gating the results bar on `eventsData` meant it appeared when the fetch resolved and shoved the 800px layout down 64px — **CLS 0.0059 → 0.0122, 5/5 runs**. Fixed by rendering the container unconditionally so `min-height` reserves the space from mount. **Re-measured: 0.0054 vs baseline 0.0059, 8/8 stable — at or below baseline.**
  - ⚠️ **Pre-existing and NOT fixed here:** the residual ~0.0054 is the freshness banner appearing and pushing `.dashboard-filters`. It is present identically on `origin/development` and is out of scope for this change.

## 4b. Independent review round (post-PR)

An independent reviewer (`gpt-5.6-sol`, given the change but **not** the author's
conclusions) found **7 substantiated defects**. All 7 were confirmed and fixed;
two of the confirmations required checking sources the author had only assumed.

- [x] 4b.1 **The central ordering claim was overstated.** The code comment and spec said the ordering used only columns ingestion does not rewrite. **False** — the same `ON CONFLICT` clause sets `event_date = EXCLUDED.event_date`, so a re-dated event *does* move, as does one whose `category`/`status`/geometry change moves it in or out of a filtered set. The guarantee is narrower than claimed: re-ingesting an **unchanged** event no longer reorders anything, which is the routine per-tick case. Comment, `spec.md` and the residual-limits scenario all corrected, and `TestListEventsReDatingMovesARow` now **pins the limitation** so it cannot be quietly re-claimed.
- [x] 4b.2 **The screen could describe rows that were not on it.** Two distinct halves; the first was fixed immediately, the second only after re-probing the finding rather than trusting the first fix.
  - *Page change:* with `keepPreviousData`, `eventsData` is the previous page while the URL offset has advanced — and the range was computed from the URL offset, so it read "Showing 51–100 / Page 2" over page-1 rows. All pagination arithmetic now derives from `meta.offset`, i.e. the rows actually rendered. Test holds page 2 in flight and asserts the label still reads 1–50.
  - *Filter change:* ⚠️ **this half was initially missed.** The reviewer also noted a filter change shows the previous filter's rows; re-probed and **reproduced** — selecting Ghana left **50 Nigeria-era cards** on screen under a country control already reading "Ghana", with the count reporting the previous filter's 3,268 total. The range was truthful about the rows, but the screen as a whole was not. `placeholderData` is now a function that carries the previous response **only when the filters are unchanged**, so a page change still avoids the mobile collapse while a filter change falls back to an honest loading state. Both halves are pinned by tests, and the filter-change test was **verified to fail on plain `keepPreviousData`**.
- [x] 4b.3 **The new axe test raced Vitest's 5s default.** It renders a full 50-card page (25× the older axe fixture) — ~2.0s here, but it timed out on the reviewer's hardware, so the committed suite was red under `npm test` for them. Given an explicit 20s timeout; the realistic DOM is the point of the test, so the timeout moved rather than the fixture.
- [x] 4b.4 **Migration `000014` gave a false reason for not using `CONCURRENTLY`.** It claimed golang-migrate wraps migrations in a transaction. **Verified false in the pinned v4.19.1 driver**: `Run` → `runStatement` → `conn.ExecContext`, no `Begin` (the only `BeginTx` is in `SetVersion`). `CONCURRENTLY` would work. The *choice* stands but is now justified on proportionality — milliseconds of ShareLock at 3,268 rows beats two table scans and a possible INVALID index.
- [x] 4b.5 **The benchmark was not rerunnable, and outran what it measured.** Once `000014` is applied, arm A silently measured the *indexed* plan as the "baseline" and the script then aborted on `relation already exists`. Now drops the candidate inside the transaction first. Separately, the claim that `COUNT(*)` "remains the dominant cost" was never measured — it is now, and it is **conditional**: 0.17 ms unfiltered, but 1.61 ms for `country=Nigeria`, ~12× the indexed list query. Figures across **four** independent runs are now published as ranges, and deep pagination is recorded as *no gain* (marginally slower on two runs) rather than improved.
- [x] 4b.6 **`page` parsing accepted values it should not.** `Number.parseInt` took `2junk` as page 2 and `999999999999999999999` as `1e21`, producing an offset that serialises as `5e+22` and returns HTTP 400. Now whole-string `/^\d+$/`, `Number.isSafeInteger`, and a `MAX_PAGE` bound. Seven-case table test.
- [x] 4b.7 **The CLS figures had no committed artifact** — the exact defect the "commit the measurement harness" rule exists to prevent. Both Playwright harnesses are now committed at `scripts/bench-dashboard-cls/` with a README, and **re-run from the documented path to confirm the +64px figure reproduces**. The first draft of that README documented an invocation that did not actually work (Node resolves the import from the script's directory); corrected and verified.

## 5. Explicitly not in this change

> **These stay unchecked on purpose — they were never tasks of this change.** They are
> carried forward to [`chore-deferred-work-register`](../../../proposals/chore-deferred-work-register.md)
> §A, so archiving this record does not delete them from the working set. Two further
> items **measured during** this change are recorded there too: the `COUNT(*)` cost that
> now dominates filtered requests (§A2), and the pre-existing freshness-banner CLS (§A3).

- [ ] 5.1 ~~Mobile TBT / bundle work~~ — the 1,140 ms figure is unverified this session and `map-vendor` is already off the critical path. Re-measure on staging first. → register §A1
- [ ] 5.2 ~~Cursor pagination, raising the 200 cap, map clustering, infinite scroll~~ → register §A4–A7
