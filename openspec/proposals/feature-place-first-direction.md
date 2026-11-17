---
id: feature-place-first-direction
status: proposed
branch: docs/propose-place-first-direction
---

# Proposal: Place-First Direction — "What Is Happening Near Me", for Any Hazard, Across Africa (feature-place-first-direction)

> **Direction proposal.** It changes what the product says it is, records the evidence for that, and sequences the work that follows. The five decisions below were made by the maintainer on 2026-11-17, each as recommended. Phase 0 is documentation only.

## Why

VigilAfrica's problem statement has always been hazard-neutral: *translate raw geospatial event signals into familiar African administrative names, answering "what is happening near me?"* ([`product.md` §1](../specs/vigilafrica/product.md)). In practice the project drifted flood-first and stayed two countries wide:

- the partner page leads with flood response and a daily **flood** digest;
- the outreach plan, the grant narrative and `feature-weekly-flood-brief` are all built on floods;
- `feature-continental-coverage` recorded a maintainer decision (2026-08-04) to make **floods the default view and wildfires opt-in**;
- `product.md` §2 still locks scope to "Nigeria only", and the roadmap index stops at v1.3.

Measured on 2026-11-17, the feed does not support a flood-first, two-country product.

| measurement (2024-10-01 → 2026-11-17) | Africa | Nigeria + Ghana |
|---|---|---|
| Floods, by title country | **123** (~6 a month since the record starts 2025-04-24) | **10** (NG 7, GH 3) |
| Floods dated November–March | **37** | **0** |
| Countries with at least one flood | **32** | 2 |

*Source: [`scripts/eonet-density/floods-by-title.mjs`](../../scripts/eonet-density/floods-by-title.mjs). Re-run it; do not trust the figures here.*

Three consequences:

1. **A Nigeria + Ghana flood view is empty for five months a year, and nearly empty for the other seven** — about one flood every two months across both countries.
2. **Floods never stop at continental scale.** Five to ten a month continue through the West African dry season, in other parts of the continent.
3. **Wildfires are most of what there is to show** — ~95% of events ([`measure.mjs`](../../scripts/eonet-density/measure.mjs): 3,109 of 3,268 to 2026-08-04) — and hiding them behind a filter is what would make the product look empty.

⚠️ **Not yet backed by a committed script:** a scratch by-month run on 2026-11-17 showed wildfires in the Nigeria and Ghana boxes concentrated in November–February (130 in January 2026) and at zero during the rains — the mirror image of the flood season. That is the seasonal argument for showing both hazards together, and it is the reason the spec makes a committed by-month wildfire count its first task. Until then treat it as **likely**, not measured.

The thing that is always true, in every season and every country, is the translation itself: an event with coordinates becomes an event with a place name. That is the product. The hazard is a property of the event, not of the product.

## What Changes

### The direction, in one paragraph

VigilAfrica shows **what is happening, by place name, across Africa**, for whatever hazards the feed carries. "Near me" is the default lens, and when nothing is near the visitor — which is common — the product says so plainly and widens to their country and then the continent, instead of showing an empty list.

### Phase 0 — record the direction (documentation only)

- Amend [`product.md`](../specs/vigilafrica/product.md) §1–§2 with a dated direction addendum: scope is Africa, hazards are whatever the category registry supports, and the "MVP: Nigeria only" table is marked historical. The lock rule in that file requires a matching roadmap update — done in the same change.
- Bring the [`roadmap.md`](../specs/vigilafrica/roadmap.md) milestone index up to date: v1.3 is shipped (the index still says *Proposed*), v1.4–v1.6 are missing, and a new milestone records this direction.
- In [`feature-continental-coverage`](feature-continental-coverage.md): reverse the 2026-08-04 "floods default, wildfires opt-in" decision, and answer its open question 6 (positioning).

### Phase 1 — an honest "near me" (small; works on today's two countries)

- **A scope ladder in place of an empty state.** Near you → your country → all covered countries, each with a count, so a visitor with nothing nearby still learns something true.
- **Say what "now" means.** Every event in the feed is already closed; production shows open events plus those closed in the last 30 days. The interface states that window instead of implying live status.
- **Say what the product cannot see.** One persistent, plain sentence: satellite-detected events only; street-level and urban flash flooding are not covered. This is the standing honesty rule of a safety-adjacent product, and the Lagos geocoding spike measured why the gap is not closable soon.
- **Hazard-neutral copy** on the landing page, partner page and metadata. No flood-first headline.

### Phase 2 — continental coverage

Unchanged in substance; it stays its own proposal, [`feature-continental-coverage`](feature-continental-coverage.md). This proposal makes it the next build and supplies two of its pending decisions.

### Phase 3 — judge it in season

The demand question cannot be answered in the dry season. The 2027 rains are the test; see *How we will know* below.

## Decisions — made 2026-11-17

The maintainer accepted all five recommendations on 2026-11-17. The right-hand column is the decision.

| # | Question | Decision |
|---|---|---|
| D1 | **Positioning.** A continental product, or a Nigeria + Ghana product with continental data? | **Continental.** Twelve times the floods are outside the two countries, and a two-country frame is what produces the empty view. |
| D2 | **Default view.** All hazards by recency, or floods first? | **All hazards.** The dashboard already defaults to "All Categories" today; this formally reverses the unimplemented 2026-08-04 decision rather than changing behaviour. |
| D3 | **More categories now?** The feed also carries severe storms (13 in the window, mostly Indian Ocean cyclones) and volcanoes (4). | **Not in this change.** Seventeen events in 26 months does not justify the registry work yet. Register it; revisit after Phase 2, when Mozambique and Madagascar are covered. *(Counts are from the 2026-11-17 scratch run — see the caveat above.)* |
| D4 | **The flood digest and the partner offer.** `/v1/digest/today.json` and the partner page are flood-specific. | **Reword the partner page; leave the digest endpoint alone.** Changing the digest's content is a contract change and deserves its own proposal once a partner exists to ask for it. |
| D5 | **The scope ladder** (Phase 1). | **Yes.** It is the smallest change that makes "near me" honest in every season. |

## Non-goals

- **Not dropping floods.** They cost nothing to keep — same feed, same code — and they are the only events with recognisable news value and the only hook for the humanitarian partner story.
- **Not a local-relevance fix.** A Lagos resident still sees no urban flooding. Nothing here changes that, and the copy must not imply otherwise.
- **Not new data sources, alerts, accounts or sub-state (ADM2) naming.**
- **Not the continental build itself** — that remains `feature-continental-coverage`.

## How we will know

This direction is a bet, and it should be falsifiable.

- **Before the rains (by early 2027):** at least three of five structured conversations with responders, journalists or logistics planners held and written down; at least one reply to partner outreach. Analytics traffic is **not** a dry-season criterion — an empty season cannot produce it.
- **Falsifier:** if those conversations say people need street- or neighbourhood-level precision, this direction does not serve them, and the honest outcome is to freeze the project rather than build Phase 2.
- **In the 2027 rainy season:** external visitors who are not the maintainer, and at least one partner using the data during a real event.

## Risks

- **A wildfire map nobody asked for.** Most satellite fire detections in African savanna are seasonal land burning, and free tools already map fires in more detail. *(General knowledge, not measured here.)* The conversations in *How we will know* are the check.
- **The flood partner story weakens** if the product stops leading with floods. Mitigation: floods stay first-class, and partner material can still lead with them in season.
- **Scope creep.** "Any hazard, all of Africa" invites every category and every source. D3 and the non-goals are the guard.

## Origin

Project review, 2026-11-17: production is stable at v1.6.4 with no recorded external users; the maintainer raised that the dry season makes a flood product hard to show and considered dropping floods. The by-month recount (branch `docs/eonet-flood-density-by-title`) showed the problem was the two-country, flood-first frame rather than floods themselves. Technical detail in [`openspec/specs/feature-place-first-direction.md`](../specs/feature-place-first-direction.md).
