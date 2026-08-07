---
id: chore-deferred-work-register
status: proposed
branch: tbd
---

# Proposal: Register the Work Deferred by the v1.3.7–v1.3.8 Batch (chore-deferred-work-register)

## Why

Archiving a completed change **deletes its "Out of scope" section from the working set.** Three changes are archived in this batch. Two of them — [`feature-events-pagination`](../changes/archive/2026-08-07-feature-events-pagination/) (#221) and [`perf-boundary-area-precompute`](../changes/archive/2026-08-07-perf-boundary-area-precompute/) (#211) — carry **six unchecked deferrals** between them, plus two findings that were *measured during* #221 and never recorded anywhere at all. The third, [`feature-impact-categories`](../changes/archive/2026-08-07-feature-impact-categories/), is closed unimplemented and leaves one surviving idea (§C).

This project has a documented failure mode of exactly this shape, and it is not hypothetical: **B6 below has been outstanding since v1.1** because it lives in a source comment that names a follow-up proposal nobody ever created. [`chore-web-audit-leftovers`](chore-web-audit-leftovers.md) exists for the same reason, for a different batch. This is that register for this batch.

**Nothing here is urgent, and nothing here should be done as one PR.** This is a holding record so the items are *visible*, not a plan to execute them together. Each numbered item is independently schedulable, and several will turn out to be "no, and here is why" — which is a valid outcome as long as it is a recorded one.

## What Changes

Nothing in the product. This proposal creates the record. Items graduate out of it into their own proposals when scheduled.

---

## A. Deferred by `feature-events-pagination` (#221, archived `2026-08-07-feature-events-pagination`)

### A1. Mobile TBT / bundle work — ⚠️ re-measure before proposing anything

The **1,140 ms TBT** figure has been quoted across several documents and is **unverified since 2026-07-26**. It should not be used to justify work.

What *was* checked during #221: `map-vendor` (946 kB raw / 246 kB gzip) is **already off the critical path** — `lazy(Map)` sits inside `lazy(EventsDashboard)`, and `index.html` modulepreloads only `rolldown-runtime` and `react-vendor`. So the obvious "defer the map bundle" fix is already in place.

**First action is a staging re-measure, not a code change.** Run Lighthouse in incognito, ≥3 runs (a single run has twice produced phantom regressions on this project).

### A2. `COUNT(*)` now dominates the filtered `/v1/events` request — measured, new

Migration `000014` made the list query so much faster that **the count is now the bottleneck for filtered requests**. Measured by the committed harness at 3,268 rows:

| statement | median |
|---|---:|
| `COUNT(*)`, no filter | 0.17 ms |
| `COUNT(*)`, `country=Nigeria` | **1.61 ms** |
| indexed list query, `country=Nigeria` | 0.13 ms |

So a filtered request spends **~12× longer counting than fetching**. The cause is `country_name ILIKE $1` — an unanchored case-insensitive match that cannot use a plain btree.

⚠️ **Do not "fix" this on the strength of the ratio.** 1.61 ms is small in absolute terms, and this project has just been bitten by quoting a large ratio on a small number. Candidate approaches if it is ever worth doing: a functional index on `lower(country_name)`, an exact-match path when the filter is already canonical (`resolveCountry` canonicalises before the query), or an approximate count above a threshold. **Reproduce with `scripts/bench-events-ordering/bench.sql` first** — it now measures both count shapes.

### A3. Residual CLS ~0.0054 from the freshness banner — measured, pre-existing, new

`FreshnessIndicator` returns `null` until `/health` resolves, then renders a banner that pushes `.dashboard-filters` and `.dashboard-layout` down. Measured at 1920x1600 with the dashboard chunk delayed: **0.0059 on `development` before #221, 8/8 runs**; unchanged in kind after.

It is well under the 0.1 "good" threshold and was **deliberately left alone** by #221 as out of scope. The fix is the same one #193/#198/#221 applied twice already — reserve the height rather than gate the render. Harness: `scripts/bench-dashboard-cls/`.

### A4. Cursor / keyset pagination

Would remove the residual cases the shipped ordering cannot: a re-dated event, a genuinely new event sorting ahead of the current page, and an event whose `category`/`status`/location change moves it in or out of a filtered set. Rejected in #221 as disproportionate — a breaking contract change bought for a scale problem this product does not have, and `total` (which keyset makes awkward) is the number the whole change exists to surface.

**Revisit only if** deep pagination becomes a real access pattern, which today it is not.

### A5. Raising the `limit` cap above 200

No demand identified. Recorded so the 200 is understood as a deliberate ceiling rather than an accident.

### A6. Server-side clustering / marker aggregation

Now that the map shows one page at a time, the marker count is bounded by `limit`. This only becomes interesting if A4 or A5 changes that.

### A7. Infinite scroll — **rejected, not deferred**

Recorded so it is not re-proposed as an improvement. It hides the total, which is the single number `feature-events-pagination` existed to make visible. Listed here as a decision, not a task.

---

## B. Deferred by `perf-boundary-area-precompute` (#211, archived `2026-08-07-perf-boundary-area-precompute`)

### B1. Re-assess table-rewriting migrations on `admin_boundaries` after continental boundaries land

Migration `000013` rewrote `admin_boundaries` under `ACCESS EXCLUSIVE` — sub-millisecond at 62 rows, **~1.3 s at the 795-polygon continental scale** it was built for. Anyone adding a further generated or non-volatile-default column to that table **after** [`feature-continental-coverage`](feature-continental-coverage.md) lands should expect a real outage window and schedule it against the ingestion cadence.

This is a **standing constraint on future work**, not a task to complete. It belongs in the record because the migration comment is the only place it currently lives.

### B2. Codify the adversarial enrichment cases as regression tests — ⚠️ pre-existing CI gap

The tie-break, shared-border and geometry-update cases that the `000013` review exercised by hand are **not covered by CI**. A future change to the boundary-matching logic would not be caught. Explicitly flagged as pre-existing rather than introduced by #211.

Highest-value item in section B: it protects the enrichment correctness that the whole product's location labelling rests on.

### B3. Geometry simplification at `0.001°`

Separate work item, never opened. ⚠️ **Probably already settled — check before proposing.** [`feature-continental-coverage`](feature-continental-coverage.md) reversed its own recommendation to **"do not simplify"** after re-running a committed harness ([`scripts/bench-simplification/sweep.sql`](../../scripts/bench-simplification/sweep.sql), 10,600 ground-truth points): the storage win measured **2.2×**, not the 10–50× originally assumed, and enrichment speed came from the stored-area column rather than from simplification. If that reasoning covers this item too, close it as decided rather than leaving it open.

### B4. Re-measure enrichment cost against real continental boundaries

The 11–13× figure comes from **replicated fixtures** (the real 53 ADM1 polygons translated to ~795), which is representative of volume but not of real continental geometry complexity. Re-run `scripts/bench-enrichment/bench.sql` once real boundaries are loaded.

---

## C. Surviving from `feature-impact-categories` (archived unimplemented)

### C1. The category registry

The proposal's own banner concluded **keep the registry, drop the categories** — `landslides` and `tempExtremes` are valid EONET category IDs that return **zero events worldwide**, so implementing them ships UI, API validation, DB constraints and ingestion for nothing.

The registry itself — a single declared source of truth for categories, replacing the values currently duplicated across the API validator, the DB `CHECK` constraint, the TS union and the filter options — is genuine groundwork and a real prerequisite for any future expansion. It survives the archive; the categories do not.

⚠️ **Re-verify the zero-event finding before acting on it.** It was measured 2026-08-03 against a moving upstream.

---

## D. Long-outstanding, never in the backlog

### D1. B6 — Ingestor struct refactor (outstanding since v1.1)

⚠️ **This is the item that justifies this whole document.** [`api/internal/ingestor/eonet.go:41-49`](../../api/internal/ingestor/eonet.go) carries a deferral comment that names a follow-up proposal — `chore-eonet-ingestor-struct` — **which was never created.** It has therefore been invisible to the backlog for two minor versions.

Verified still outstanding: package-level test seams at `eonet.go:36`, `:54`, `:58`, `:62`, `:83`. Three of them (`eonetHTTPClient`, `eonetURL`, `eonetSleepFn`) form an implicit Ingestor surface that tests override via `install*` helpers.

Consolidating them into an explicit `Ingestor` struct touches `scheduler.go`, all eonet tests, and every caller of `Ingest` — which is why it was split out, and the split was authorised by R1 of the quality-sweep spec. The reasoning was sound; only the bookkeeping failed.

---

## E. Found while doing this housekeeping

### E1. Two stray spec documents in `openspec/specs/` — ⚠️ deliberately not fixed here

[`openspec/specs/fix-border-event-enrichment.md`](../specs/fix-border-event-enrichment.md) and [`openspec/specs/fix-ingest-bbox-validation.md`](../specs/fix-ingest-bbox-validation.md) both:

- describe work that **shipped in v1.3.2**, while still reading **"Status: Proposed — implementation in `fix/…`"**;
- link to companion proposals that were archived long ago, leaving **4 broken relative links** — the only ones remaining anywhere under `openspec/` after this PR;
- sit beside `openspec/archive/spec-fix-*.md` files that are **related but not identical** — the archived ones are *spec deltas*, these are *full spec documents*.

**Not fixed here on purpose.** Deciding whether they should be merged into [`specs/vigilafrica/spec.md`](../specs/vigilafrica/spec.md), archived alongside their deltas, or deleted as superseded is a real editorial call about the spec's structure, not link rot — and this PR had no mandate for it. Everything else broken by the archive moves in this PR **was** repaired.

## Out of Scope

- Doing any of the above. This proposal is the record, not the work.
- [`chore-web-audit-leftovers`](chore-web-audit-leftovers.md) — the equivalent register for the 2026-07-26 web-audit batch, still open and unaffected.

## Verification

- [ ] Every unchecked item in the two archived changes' task lists appears here, with its provenance
- [ ] Each item states whether it is a task, a standing constraint, or a recorded decision — these are not the same thing and the distinction is why B1 and A7 look like tasks but are not
- [ ] Every quoted measurement names the committed harness that reproduces it, or is marked unverified
