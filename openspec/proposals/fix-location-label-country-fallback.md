---
id: fix-location-label-country-fallback
status: in-progress
branch: fix/location-label-country-fallback
---

# Proposal: Show the Country When We Have No State, Instead of Raw Coordinates (fix-location-label-country-fallback)

## Why

Observed on staging on **2026-09-09**, immediately after `fix-eonet-polygon-transposition` landed. The
Cameroonian flood `EONET_22248` rendered its location as:

```
📍 4.6027, 9.4139
```

while the API row said `country_name: "Cameroon"`. **We knew the country and chose not to show it.**

The detail page was worse. Its markup was unconditional:

```tsx
<strong>{event.state_name}</strong>, {event.country_name}
```

so a null state produced an empty `<strong>` and a stray leading comma — `", Cameroon"`.

⚠️ **A missing STATE is not a missing PLACE.** Events reached through the NG/GH bounding-box overhang
are enriched by the ADM0 fallback added in migration `000012`: they carry a country but no state,
because ADM1 boundaries are loaded only for Nigeria and Ghana and we deliberately do not invent a
state for a country whose states we do not hold. Both renderers branched on `state_name` alone and so
treated that designed outcome as "no location".

On a warning product this matters more than a cosmetic slip: a Nigerian reader is not told the event
is **outside** their country, and a Cameroonian reader is not told it is **inside** theirs.
Coordinates are a last resort, not a substitute for a country we already have.

## Not a regression, but newly visible

Every ADM0-fallback neighbour event — Cameroon, Benin, Niger — has rendered this way since `000012`.
`fix-eonet-polygon-transposition` is simply the first time it landed on a **flood**, the headline
category, on the first page.

Before that fix the same row read *"Kwara, Nigeria"* — confidently wrong. It is now honestly
unhelpful, which is an improvement, and this closes the remaining gap.

## What Changes

A single `formatLocation()` helper in `web/src/formatLocation.ts`, used by both renderers, with an
explicit precedence:

| state | country | rendered |
| --- | --- | --- |
| ✅ | ✅ | **Jigawa**, Nigeria |
| ✖ | ✅ | **Cameroon** |
| ✅ | ✖ | **Lagos** |
| ✖ | ✖ | coordinates, or `Location unavailable` when there is no point either |

Blank-but-present strings are treated as absent, so a whitespace state cannot reintroduce the
dangling separator.

## Verification

- `web/src/formatLocation.test.ts` covers each row above, including the **exact staging case**
  (`null` state + `"Cameroon"`) with an assertion that the output does **not** contain `4.6027` —
  so a regression to coordinate-only rendering fails rather than merely looking different.
- Full frontend suite green: **97 tests**, up from 91.
- ⚠️ **Not verified by any automated check: how it actually looks.** The staging screenshots are what
  surfaced this in the first place — every gate was green while the card showed raw coordinates. The
  fix needs the same treatment: look at the card and the detail page on staging after deploy.

## Impact

- **Affected:** `web/src/formatLocation.ts` (new), `web/src/components/EventsDashboard.tsx`,
  `web/src/pages/EventDetail.tsx`.
- **Risk:** low — presentation only, no API or data change.
- **Out of scope:** loading ADM1 boundaries for neighbour countries, which would give these events a
  real state rather than a country-only label. That is a data question, not a display one.
