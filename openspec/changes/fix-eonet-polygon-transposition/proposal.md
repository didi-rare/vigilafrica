---
id: fix-eonet-polygon-transposition
status: proposed
branch: fix/eonet-polygon-transposition
---

# Proposal: Stop Ingesting EONET Polygon Geometry With Transposed Coordinates (fix-eonet-polygon-transposition)

## ⚠️ This is a live data-correctness defect in production, not a hypothetical

Production currently shows **a flood in Kwara State, Nigeria that never happened.** The event is real,
but it occurred in **Cameroon**. It appears in Nigeria because its coordinates arrived reversed.

Of the three flood events in production on 2026-09-06, **two are mislocated** — both of the two that
carry `Polygon` geometry. The one `Point` flood is correct.

| event | geom | where we show it | where it actually is |
|---|---|---|---|
| `EONET_22248` "Flood in Cameroon 1104078" | Polygon | **Kwara, Nigeria** | **Cameroon** (lon 9.22–9.57, lat 4.38–4.83) |
| `EONET_23208` "Flood in Nigeria 1104105" | Polygon | **Adamawa** | **~Kano, northern Nigeria** (~400 km away) |
| `EONET_20881` "Flood in Nigeria 1103997" | Point | Lagos | Lagos ✅ |

For a product whose entire claim is *where* an event is, showing a Cameroonian flood as Nigerian is
the most damaging class of error available to us. Floods are also the default view, so this is
**2 of the 3 events (67%) in the headline category.**

## What is actually wrong — and it is not our code

**EONET republishes GDACS polygon geometry with latitude and longitude swapped.**

Verified against GDACS, the authoritative upstream, for both events. The vertex counts match
*exactly*, so these are demonstrably the same polygons with the axes reversed:

| event | vertices | GDACS (`Poly_Affected`) | EONET (what we ingest) |
|---|---|---|---|
| 1104078 | **1311** | lon 9.216–9.569, lat 4.377–4.828 | lon 4.377–4.828, lat 9.216–9.569 |
| 1104105 ep.2 | **28** | lon 9.852–9.931, lat 12.468–12.589 | lon 12.468–12.589, lat 9.852–9.931 |

GDACS is self-consistent: for 1104078 it reports `country: Cameroon`, `iso3: CMR`, and a centroid of
`[9.414, 4.6027]` — which agrees with its own polygon and disagrees with EONET's.

⚠️ **We are not the ones swapping.** [`normalizer.go`](../../../api/internal/normalizer/normalizer.go)
reads `[lon, lat]` explicitly for `Point` (which is why the Lagos point is right) and passes `Polygon`
coordinate arrays through **verbatim**. Our code is faithful to a bad feed.

## ⚠️ The defect is global and category-wide, not an African edge case

Sampling **40** EONET flood events, every one of which cites a GDACS source: **14 declare a latitude
outside ±90°**, which is physically impossible and proves transposition with no geographic reasoning
at all.

```
Japan        "lat" 136.77..137.76     New Zealand  "lat" 168.12..171.42
Vietnam      "lat" 107.95..108.04     Australia    "lat" 138.08..138.28
Philippines  "lat" 120.36..120.60     China        "lat" 115.85..117.75
South Korea  "lat" 126.58             Thailand     "lat"  98.18..99.88
```

The remaining 26 fail the same way but escape *this particular* test only because their longitudes
happen to fall inside ±90 — Africa, Europe and the Americas. Ours are in that group, which is exactly
why the error is silent for us rather than absurd.

**Reproduce in one call:**
`https://eonet.gsfc.nasa.gov/api/v3/events?category=floods&status=all&limit=40`

## ⚠️ Why nothing caught this, and why bbox validation never could

[`eonet.go:417-422`](../../../api/internal/ingestor/eonet.go) already anticipated this exact hazard:

> *Containment is unverifiable: the normalizer resolves lon/lat only for Point geometry and leaves
> them nil for Polygon. Such events are stored deliberately — we do not drop data we cannot verify —
> but the fact is counted so a polygon-shaped upstream leak stays discoverable in the run summary.*

The leak arrived, `events_unverified_geom` incremented, and **nobody reads the run summary.** A
counter is not a gate.

⚠️ **More importantly, bbox validation could not have caught it even if it ran.** Checked against
Nigeria's ingestion box (2–15°E, 4–14°N), **all four orderings of both events fall inside it** —
because the box is close to square over this range. The swap is geometrically undetectable from our
side. Any fix that leans on bbox containment is a fix that does not work.

## What Changes

**1. Treat GDACS as authoritative for GDACS-sourced geometry.** Every affected event carries the
GDACS event id in `source_url`. GDACS publishes both a centroid and the polygon. Fetching geometry
from GDACS for those events fixes both cases and **keeps working the day EONET fixes their bug** —
which a blanket swap would not.

⚠️ **Do NOT simply reverse every polygon.** It is correct today and silently corrupts everything the
moment upstream is repaired. On n=40 the pattern is strong, but the failure mode of guessing wrong is
that we start inventing locations again, which is the defect we are fixing.

**2. Add an ingest-time sanity gate that is impossible to pass with transposed data.** Reject any
geometry whose latitude falls outside ±90°. That is not a heuristic — it is a definitional
constraint, and it alone would have caught 14 of the 40 sampled events at the door.

**3. Quarantine, do not silently store, geometry that cannot be verified.** Polygons currently bypass
containment and are stored anyway. They should be stored **flagged**, and excluded from user-facing
results until verified, so an unverifiable event never renders as a confident location.

**4. Backfill the two production rows** once the resolver lands.

## Impact

- **Affected:** `api/internal/normalizer`, `api/internal/ingestor`, and the two stored rows.
- **User-visible:** the phantom Kwara flood disappears; the Adamawa flood moves to its real location.
- ⚠️ **Interim cost:** if step 3 ships before step 1, floods drop from **3 to 1** in production.
  That is the honest state — showing one real flood beats showing three of which two are fiction.

## Out of Scope

- The `ORDER BY ST_Area(geom) ASC LIMIT 1` tie-break in the enrichment trigger. It picks the
  *smallest state a geometry touches*, which is sensible for a point and arbitrary for a large
  polygon. It did **not** cause this defect — the enrichment was correct for the coordinates it was
  given — but it is worth its own look once polygons carry trustworthy geometry.
- Reporting upstream to NASA. Tracked separately; the evidence is in this document.
