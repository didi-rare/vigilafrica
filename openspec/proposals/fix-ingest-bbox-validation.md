---
id: fix-ingest-bbox-validation
status: proposed
branch: tbd
---

# Proposal: Validate ingested EONET event coordinates against the query bbox (fix-ingest-bbox-validation)

## Why

A wildfire event physically located in **Wakulla, Florida, USA** (`lat 30.05, lng -84.57`) is present in the **production** database and shows up in the public event feed (first card on `vigilafrica.org`, sorted to the top by date). VigilAfrica is an Africa-scoped tool; a US event is a visible data-quality defect and undermines trust.

Confirmed live on **2026-07-18** (production `prod-db`):

```sql
-- events outside both configured bboxes (NG: lon 2..15/lat 4..14, GH: lon -3.5..1.2/lat 4.5..11.2)
SELECT title, country_name, latitude, longitude, ingested_at
FROM events
WHERE NOT (
  (longitude BETWEEN 2.0 AND 15.0 AND latitude BETWEEN 4.0 AND 14.0) OR
  (longitude BETWEEN -3.5 AND 1.2 AND latitude BETWEEN 4.5 AND 11.2));
-- → 1 row: "340 Wildfire, Wakulla, Florida" | country_name='' | 30.05 | -84.57 | 2026-07-18 07:49:51+00
```

Two tells: `ingested_at` is from the **current hourly run** (not stale pre-bbox data — it is actively re-ingested every cycle), and `country_name` is **empty** (the geo-lookup matched no African boundary, but the event was stored anyway).

### Root cause (verified against the live EONET API)

- **EONET's `bbox` filter leaks this event.** Querying `https://eonet.gsfc.nasa.gov/api/v3/events?bbox=<NG box>&category=floods,wildfires&status=open,closed` returns 31 events — 30 African **plus** EONET_20263 ("340 Wildfire, Wakulla, Florida"). The event has a **single** geometry point, in Florida, with no African point. EONET returns it for the Nigeria box regardless — an upstream EONET quirk we cannot control.
- **The bbox coordinate order is NOT the cause.** [`api/internal/ingestor/eonet.go`](../../api/internal/ingestor/eonet.go) builds `bbox = min_lon, min_lat, max_lon, max_lat` (W,S,E,N) whereas EONET documents W,N,E,S (`min_lon, max_lat, max_lon, min_lat`). Empirically both orderings return the **identical** 31 events including Florida — EONET normalizes min/max internally — so the order mismatch is cosmetic, not functional. (Optional tidy; see Out of Scope.)
- **The normalizer is correct.** [`api/internal/normalizer/normalizer.go`](../../api/internal/normalizer/normalizer.go) `selectMostRecentGeometry` correctly picks the event's only geometry point. No multi-geometry mis-selection is involved.
- **The actual defect: the ingestor trusts EONET's filtering and never validates the coordinate.** [`eonet.go` ingest loop](../../api/internal/ingestor/eonet.go) calls `normalizer.Normalize` then `repo.UpsertEvent` with **no check that the resolved point lies within `country.BBox`**. EONET's one leaked event flows straight through and is stored with empty `country_name`.

### Side-findings (not bugs)

- The Nigeria bbox (`2..15°E, 4..14°N`) legitimately also captures **Cameroon** and **Benin** border wildfires — these are genuinely inside the box and correctly stored with their real country names. They are **not** leaks. If per-country precision is later desired, that is a separate scoping decision (point-in-ADM0-polygon rather than bbox), out of scope here.
- `nearby_events` was verified correct in the same session (43/43 events have geometry; the closest event to Lagos is 226 km, outside the 200 km radius, so `/v1/context` correctly returns `[]` for a Lagos IP). Unrelated to this bug.

## What Changes

Add a **client-side coordinate guard** in the ingestion loop (defense-in-depth, robust to EONET leakiness):

- After `normalizer.Normalize` returns a point, verify the coordinate falls within the querying `country.BBox`. If it does not, **skip** the event (do not upsert) and log at `warn` with `source_id` and the out-of-box coordinate.
- This also eliminates the empty-`country_name` rows, since any point that fails the geo-lookup for an African country will also fail the bbox containment check.
- Add a regression test in `api/internal/ingestor/eonet_test.go`: an EONET response containing one in-box event and one out-of-box event (Florida fixture) asserts only the in-box event is upserted.

Because this touches `api/internal/`, it requires this OpenSpec record (Sentinel gate) plus tests, and flows through `development → main → release` normally.

### Post-fix data cleanup

A `DELETE` of the Florida row is **futile until the fix is deployed** — it is re-ingested hourly. Once the guarded ingestor reaches production, run once:

```sql
DELETE FROM events WHERE source_id = 'EONET_20263';  -- or the WHERE NOT(bbox) predicate above
```

Then confirm the next hourly ingest does not re-add it.

## Out of Scope

- **Fixing EONET upstream** — we cannot; the guard is the correct response.
- **The cosmetic bbox coordinate-order mismatch** (W,S,E,N vs EONET's documented W,N,E,S). Functionally inert (EONET normalizes), but tidying `eonet.go` to the documented order is a reasonable optional cleanup to fold in while touching this file — clearly labelled as no-behavior-change.
- **Per-country precision** (point-in-ADM0-polygon instead of bbox), which would also re-scope Cameroon/Benin border events. Separate future proposal if desired.
- **Backfill audit** of historical leaked rows beyond the single current one — the `WHERE NOT(bbox)` query is the audit; today it returns exactly one row.

## Verification

After the fix deploys:
- [ ] `WHERE NOT(bbox)` query on `prod-db` returns **0 rows** (after the one-time DELETE).
- [ ] A full ingest cycle logs the skip for EONET_20263 (or whatever the current leaked id is) and does not upsert it.
- [ ] No new empty-`country_name` rows appear.
- [ ] Legitimate Cameroon/Benin/Niger border events (inside the box) are still ingested — the guard must not over-reject.
- [ ] The public `vigilafrica.org` feed shows only African events.

## Origin

Surfaced 2026-07-18 by the maintainer noticing a Florida wildfire on the production dashboard while reviewing the `/v1/context` "near me" feature during the post-v1.3.0-cut verification pass. Diagnosis (DB queries + live EONET API cross-check) captured the same day. Logged as a known issue for a future implementation session — no code in this proposal.
