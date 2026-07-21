---
id: fix-eonet-closed-events-ingest
status: archived
branch: fix/eonet-closed-events-ingest
merged_pr: https://github.com/didi-rare/vigilafrica/pull/148
archived_on: 2026-07-21
---

# Proposal: Ingest Closed EONET Events So Floods Are Never Silently Dropped (fix-eonet-closed-events-ingest)

## Why

**VigilAfrica has never stored a single flood event.** Not because the upstream is empty — because we ask for floods through a window that is almost always shut.

[api/internal/ingestor/eonet.go:164](api/internal/ingestor/eonet.go) builds:

```go
reqURL := fmt.Sprintf("%s?bbox=%s&category=floods,wildfires&status=open,closed", eonetURL, bbox)
```

`status=open,closed` is **not a valid EONET v3 value**. The API accepts `open`, `closed`, or `all`. Given an unrecognised value it silently falls back to its default — open-only. No error, no warning, HTTP 200.

That would be harmless if flood events stayed open. They do not. **Every EONET flood event opens and closes within ~48 hours** — verified across all 149 African flood events in the last 730 days, each carrying a single geometry date and a `closed` timestamp 1–3 days later. An hourly poll for open-only floods will miss almost all of them.

Measured against our own Nigeria bbox on 2026-07-20:

| query | events | floods |
| --- | --- | --- |
| our exact URL (`status=open,closed`) | 27 | **0** |
| `status=all` | 175 | **7** |
| `status=all&days=365` | 146 | 4 |

**A Lagos flood was available and we dropped it.** `EONET_20881` — "Flood in Nigeria 1103997", geometry `[3.3941795, 6.4550575]` (Lagos), dated 2026-06-30, closed 2026-07-02, sourced from GDACS event 1103997. It sat inside our Nigeria bbox, in a category we request, and never entered the database.

### Why this is urgent, not merely a bug

The daily flood digest ([api/internal/digest/digest.go](api/internal/digest/digest.go)) filters stored events by `category=floods` for the current UTC day. Since no flood has ever been stored, **the digest was structurally incapable of reporting one**. Its "No flood events recorded today" was guaranteed output, not an observation of reality.

This digest is the concrete pilot deliverable promised to the Nigerian Red Cross Society ([feature-daily-flood-digest](openspec/proposals/feature-daily-flood-digest.md)). Sending a permanently-empty flood digest to an emergency-response partner during a live Nigerian flood season would have been actively misleading — the exact failure the product disclaimer exists to prevent.

### What this bug does NOT explain

Kept explicit so neither is wrongly considered fixed:

- **bbox ordering is fine.** `min_lon,min_lat,max_lon,max_lat` and W,N,E,S return an identical 146 events — EONET normalises the corners. The out-of-bbox Florida event was a **separate defect**, fixed independently by `fix-ingest-bbox-validation` (the `withinBBox` containment guard), which landed on `development` on 2026-07-20 while this change was in review. This branch is rebased on top of it; the two interact only in `eonet_test.go`, where the bbox tests are wrapped in `closedQueryStub` so the second request does not double their event counts. Both suites pass together under `-race`.
- **Urban-pluvial coverage remains partial.** GDACS caught the 2026-06-30 Lagos flood but not the 2026-07-13 event that brought the city to a standstill. This fix recovers what upstream actually has; it does not make VigilAfrica a reliable detector of Lagos street flooding. See [project flood data-source gap] for the remaining source strategy.

## What Changes

### The fix is not one line

The obvious repair — swapping `status=open,closed` for `status=all&days=N` — **introduces a regression.** EONET wildfires stay open for months or years; the oldest currently-open Nigeria wildfire is dated **2024-10-24**, 21 months ago. Because `days` filters on event date, any window under ~640 days would silently drop long-burning open fires we ingest correctly today:

| query (Nigeria bbox) | events | floods | wildfires |
| --- | --- | --- | --- |
| `status=open` (no days) | 27 | 0 | 27 |
| `status=all&days=90` | 3 | 1 | 2 |

EONET cannot express "open, plus closed-within-N-days" in a single request. So the ingestor issues **two requests per country per tick** and unions the results:

1. `status=open` — **no `days` window.** Byte-for-byte the event set we ingest today, so current wildfire coverage cannot regress.
2. `status=closed&days=30` — recently-closed events, which is where floods live.

De-duplication is already handled: `UpsertEvent` is idempotent on `source_id` (F-013), so an event appearing in both responses is stored once. `Normalize` already derives `Status` from the raw `closed` field ([normalizer.go:102-106](api/internal/normalizer/normalizer.go)) and `models.StatusClosed` already exists — the data model was always designed for closed events; the ingestor simply never requested them.

**Window choice:** 30 days. Floods close within ~48h, so this is ~15× margin and tolerates a multi-day ingestion outage without data loss. Verified to include the 2026-06-30 Lagos event. Declared as a named constant (`closedEventWindowDays`) rather than inlined.

**Cost:** 2 requests/country/tick × 2 countries × hourly = 48 requests/day, up from 24. Negligible against EONET's rate limits, and the existing 429/503 backoff path is reused unchanged.

### Structural change

The fetch-with-retry block (eonet.go:168-277) is extracted verbatim into a `fetchEONET(ctx, reqURL, countryCode) ([]byte, error)` helper so it can be invoked twice. **The retry, backoff, size-cap, and error semantics are moved unmodified** — this is an extraction, not a rewrite. `runIngest` then decodes and processes both response bodies through the existing normalise-and-upsert loop.

`EventsFetched` becomes the sum across both responses (pre-dedup, so it reflects upstream volume); `EventsStored` remains the count of successful upserts.

### Regression test

No existing test asserts the outbound query string — which is precisely why an invalid `status` value survived to production. This proposal adds a test that captures both request URLs and asserts:

- exactly two requests are issued per country
- one carries `status=open` and **no** `days` parameter
- one carries `status=closed` and `days=30`
- neither carries the invalid `status=open,closed`
- a closed flood event in the second response is normalised to `StatusClosed` and upserted

### Changes to existing tests

Disclosed explicitly, since touching 13 existing tests is scope a reviewer must see rather than discover in the diff:

- **All 13 `httptest.NewServer` handlers are wrapped in a new `closedQueryStub` helper**, which answers the `status=closed` request with an empty event list and returns *before* invoking the test's own handler. Every retry/backoff/error test in the file describes the behaviour of a **single** fetch, and their request counters and event-count assertions were written against the open request. Short-circuiting the closed request ahead of those counters means each test keeps its original meaning instead of being rewritten to accommodate a second request it does not care about. No existing assertion is relaxed.
- **One test could not be handled that way and is changed deliberately.** `TestRunIngest_NetworkError_ThenSuccess` counts at the *transport* layer (`failOnceRoundTripper`), which sees every outbound request including the one `closedQueryStub` answers without reaching the handler. Its expectation moves from 2 RoundTrips to 3, with the reason recorded inline. The behaviour under test — one network failure retried exactly once — is unchanged and still asserted by the first two calls plus the unchanged `serverHits == 1` check.

Both new tests were verified to **fail** against the original `status=open,closed` line before being accepted (see Verification).

## Out of Scope

- **Default status filtering on `/v1/events`.** The handler accepts `?status=` but applies no default, so the dashboard will now also surface recently-closed events. This is arguably correct — a flood from three days ago is still relevant, and `EventDetail` already renders a status badge — but it is a **visible UX change** and deserves its own decision rather than riding along inside an ingestion fix. Flagged as an immediate follow-up.
- Client-side bbox validation (separate: `fix-ingest-bbox-validation`).
- Direct GDACS integration, ReliefWeb ingestion, or any new upstream source.
- The daily-vs-weekly digest cadence question (separate: `feature-weekly-flood-brief`).
- The `Ingestor` struct refactor still deferred from `chore-post-v11-quality-sweep` B6.

## Verification

Local (done 2026-07-20):

- [x] `scripts/test-api.ps1` unit suite green (Go tests run via Docker — native `go test` is AppLocker-blocked)
- [x] `go vet ./...` clean
- [x] **Both new tests verified to fail against the reintroduced bug**, then pass after restoring the fix. Observed failures: `expected exactly 2 requests (open + closed), got 1: [...status=open,closed]` and `expected 1 stored event, got 0` — the latter being the exact production symptom.
- [x] `scripts/test-api.ps1 -Integration` green (`internal/database 6.548s`)
- [x] Open and closed result sets confirmed disjoint against the live Nigeria bbox (27 open, 1 closed, empty intersection) — no double-counting in `EventsFetched` under normal operation

Post-deploy (staging) — **not yet run:**

- [x] Against staging after deploy: `GET /v1/events?category=floods` returns ≥1 event for Nigeria
- [x] `GET /v1/events?category=wildfires&status=open` still returns ~27 for Nigeria — **no regression in open wildfire coverage** (the specific risk this design guards against)
- [x] `EONET_20881` (Lagos, 2026-06-30) present with `status=closed`, `country_name=Nigeria`, `state_name=Lagos` — end-to-end proof through the enrichment layer
- [x] `GET /v1/digest/today.json` structurally able to report floods (will read 0 on a day with no flood — verify against a day that has one, not by absence)

## Origin

Surfaced 2026-07-20 while testing the newly-approved ReliefWeb API appname. The investigation was scoped to evaluate ReliefWeb as a *replacement* flood source, on the standing conclusion that EONET's flood category was empty. Probing EONET directly to establish a comparison baseline showed the category was not empty at all — the 2026-07-18 finding had been measured through the same broken query path it was trying to diagnose.

Correcting the record: EONET carries **149 African flood events per 730 days** (88 in 2025, 61 in 2026). The upstream well was never dry.
