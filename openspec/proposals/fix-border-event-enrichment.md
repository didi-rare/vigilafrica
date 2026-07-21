---
id: fix-border-event-enrichment
status: proposed
branch: fix/border-event-enrichment
---

# Proposal: Label neighbour-country border events (fix-border-event-enrichment)

## Why

Nigeria's ingestion bounding box `[2.0, 4.0, 15.0, 14.0]` (and Ghana's) deliberately overhangs into neighbouring countries so that border-region hazards near Nigerian/Ghanaian communities are captured. Those spillover events are ingested and — correctly, per [`fix-ingest-bbox-validation`](fix-ingest-bbox-validation.md) — **kept** (the containment guard rejects out-of-*box* events, never out-of-*country* ones).

But the enrichment trigger (`trg_enrich_event_location`) matches only `adm_level = 1` (state) polygons, and `admin_boundaries` is seeded with **Nigeria + Ghana only**. So a wildfire physically in Cameroon/Benin/Niger intersects no loaded state polygon and lands with **both** `country_name` and `state_name` = NULL.

Verified on staging (2026-07-20): **7 of 43 events** are affected — all border spillover (Cameroon ×4, Benin ×2, Niger ×1). `GET /v1/enrichment-stats` shows NG 100%, GH 100%, and a `null` country group of 7 at 0%. In the feed they render with blank location. This is a pre-existing enrichment-data gap, not a regression from the bbox guard or the closed-events fix.

## What Changes

Give border-spillover events their **country** label. `state_name` stays NULL — we deliberately do not load neighbour states (see *Out of Scope*).

1. **Enrichment trigger** — after the existing ADM1 (state) lookup, add an ADM0 (country) fallback that runs only when no state matched: sets `country_name` from the intersecting national polygon, leaves `state_name` NULL. This also rescues any NG/GH point that falls in a gap between real ADM1 polygons.
2. **Boundary data** — load ADM0 (national outline) polygons for the **7 countries whose territory intersects either ingestion box**: Benin, Niger, Chad, Cameroon (Nigeria's box) and Côte d'Ivoire, Burkina Faso, Togo (Ghana's box). Real geometry (geoBoundaries gbOpen, simplified), 2-letter country codes to match existing rows.
3. **Backfill** — re-enrich already-stored events (`UPDATE events SET geom = geom`) so the current 7 NULL rows pick up their country.

Delivered as a single migration `000012`. Enrichment-only: ingestion is driven by `DefaultCountries` and never reads `admin_boundaries`, so this changes labelling, not what is ingested.

## Out of Scope

- **Neighbour states (ADM1).** We label country only; `state_name` stays NULL for neighbours. Loading neighbour ADM1 would effectively onboard those countries (dozens of states × 7) for cosmetic gain and is not warranted for border spillover.
- **The enrichment success metric.** `/v1/enrichment-stats` counts `state_name IS NOT NULL`; country-only labelling leaves it state-based (NG/GH stay 100%; neighbours become their own country groups at 0% state instead of one `null` group). Not raising the ≥85% number is a deliberate decision, not an omission.
- **Ingestion boxes.** Unchanged. Do not edit `000011` (its bbox predicate is a frozen point-in-time snapshot).
- **Onboarding neighbours as ingested countries.** Not added to `DefaultCountries`.

## Capabilities

### Modified Capabilities
- `event-enrichment`: gains an ADM0 country fallback so events inside an ingestion box but outside all loaded ADM1 states are labelled by country (additive; NG/GH ADM1 behaviour unchanged).

## Risks

- **R1 — Mislabelling a real NG/GH event to a neighbour.** *Mitigation:* the fallback fires only when the ADM1 lookup returns nothing; any point inside a real NG/GH state matches ADM1 first. Real ADM0 polygons don't overlap, so the fallback is unambiguous. A regression test asserts a Lagos point still resolves to Nigeria/Lagos.
- **R2 — Border precision from simplified geometry.** *Mitigation:* the fallback only ever sees points already outside every NG/GH state (i.e. genuinely across the border), so country-level simplified outlines are sufficient; a few hundred metres of border imprecision cannot flip an interior neighbour point.
- **R3 — Backfill scope.** `UPDATE events SET geom = geom` re-fires the trigger for all rows. *Mitigation:* idempotent and bounded (tens of rows at current scale); the established repo idiom.

## Origin

Surfaced 2026-07-20 during the staging test sweep for the `development → main` promotion — 7 events showing `country_name = null`. Root-caused to the ADM1-only trigger + NG/GH-only boundary data.
