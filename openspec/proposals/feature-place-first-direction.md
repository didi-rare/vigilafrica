---
id: feature-place-first-direction
status: proposed
branch: docs/propose-place-first-direction
---

# Proposal: Place-First Direction — "What Is Happening Near Me", for Any Hazard, Across Africa (feature-place-first-direction)

> **Direction proposal.** It changes what the product says it is, records the evidence for that, and sequences the work that follows. Decisions D1 to D4 were made by the maintainer on 2026-11-17. D5 was re-opened and D6 added the same day, because both were first framed on a wrong picture of what the product shows today; **the maintainer decided both on 2026-11-18**, as recommended. The size of the D6 display window is still to be set from Phase 0 evidence. Phase 0 is documentation only.
>
> Naming: this proposal's decisions are D1 to D6. The decisions inside `feature-continental-coverage` are referred to by their titles in that file, never by number.

## Why

VigilAfrica's problem statement has always been hazard-neutral: *translate raw geospatial event signals into familiar African administrative names, answering "what is happening near me?"* ([`product.md` §1](../specs/vigilafrica/product.md)). In practice the project drifted flood-first and stayed two countries wide:

- the partner page leads with flood response and a daily **flood** digest (`web/src/pages/ForPartners.tsx`, `api/internal/handlers/digest.go`);
- the outreach plan and the grant narrative are flood-led, as is `feature-weekly-flood-brief`. For the outreach, the repository records NRCS, Code for Africa and Bezos Earth Fund contacts ([`proposal-feat-for-partners-page.md`](../archive/proposal-feat-for-partners-page.md) lines 19 and 208). It holds no outreach plan or grant narrative as a document, so what those say is the maintainer's account, not recorded in this repository;
- `feature-continental-coverage` recorded a maintainer decision (2026-08-04) to make **floods the default view and wildfires opt-in**;
- `product.md` §2 still locks scope to "Nigeria only", and the roadmap index stops at v1.3.

### What the product shows today

An earlier draft of this proposal said every event in the feed is already closed and that production shows open events plus those closed in the last 30 days. Both statements were wrong. What the code does:

- **Ingestion** polls `status=open` plus `status=closed&days=30` per country (`api/internal/ingestor/eonet.go`, `runIngest` and the `closedEventWindowDays` constant). The 30 days is an *ingest* parameter only. The same file notes that wildfires stay open for months and that the open query is deliberately unwindowed.
- **Reading** applies no window. `/v1/events` filters by category, country, state and status only when the caller sends them (`api/internal/handlers/events.go`, filter builder in `api/internal/database/queries.go`). The handler accepts no date parameters; `DateFrom`/`DateTo` exist on the repository filter but only the digest sets them (`api/internal/digest/digest.go`). The nearby-events query (`GetNearbyEvents`) has no date or status predicate. No code deletes old event rows (the only `DELETE FROM events` is a one-off migration, `api/db/migrations/000011_purge_out_of_bbox_events.up.sql`).
- **So the list shows everything ever ingested**, newest first, including wildfires the feed still marks open that are many months old. An independent reviewer's probe on 2026-11-17 found 42 open wildfires across the Nigeria and Ghana boxes dated October 2024 to March 2025. **That probe was not committed**, so treat the 42 as the reviewer's uncommitted observation; Phase 0 task 1 commits a script for it.

That is why "what does *now* mean" is a decision (D6) and not a wording tweak.

### What the "near me" feature is today

There is no near-me view. Verified in the tree:

- `nearby_events` appears only as a field of the `ContextResponse` type in `web/src/api/events.ts`; no component renders it. (`openspec/proposals/feature-current-location-marker.md` already records this: "fetched but currently unused in the UI".)
- The context response is used for the `context_resolve` analytics event and to centre the map (`web/src/components/EventsDashboard.tsx`).
- The list always comes from the events fetch, unfiltered by default (category "All Categories", all countries). The only empty state is "No events match these filters".

So today's default view already is the "all covered countries" rung of any ladder, and Phase 1 is **the first near-me interface**, not a patch to an empty state.

### What the feed supports

Measured on 2026-11-17. The record of African floods in the feed **begins on 2025-04-24**, so every flood figure below covers about nineteen months and contains **one** Nigerian dry season, not several.

| measurement (feed, 2024-10-01 → 2026-11-17) | Africa | Nigeria + Ghana |
|---|---|---|
| Floods, by title country | **123** (about six to seven a month since 2025-04) | **10** (NG 7, GH 3) |
| Floods dated November–March | **37** | **0** |
| Countries with at least one flood | **32** | 2 |

*Source: [`scripts/eonet-density/floods-by-title.mjs`](../../scripts/eonet-density/floods-by-title.mjs), run 2026-11-17. Its sources line reads `{"GDACS":891}`: every flood in the feed is GDACS-sourced. Re-run it; do not trust the figures here.*

Consequences:

1. **A Nigeria + Ghana flood view has been empty for long stretches.** Nigeria and Ghana had no flood in nine consecutive months (September 2025 to May 2026) in the script's month table. That rests on **one** dry season; it is a pattern consistent with the rains, not a measured annual cycle. Over the whole record the two countries average about one flood every two months (10 in about nineteen).
2. **Floods continue at continental scale.** In the same table Africa had between 5 and 10 floods in each of November 2025 to March 2026, in other parts of the continent. The Africa figure is about twelve times the two-country figure (123 versus 10; the script prints 12.3x). The denominator is small: ten events, so the ratio moves a lot with one or two events.
3. **Wildfires are most of what there is to show.** `scripts/eonet-density/measure.mjs`, run 2026-11-17 over its fixed window 2024-10-01 to 2026-08-04: Africa-wide 3,457 events of which 3,296 wildfires (95.3%); Nigeria + Ghana 291 events of which 281 wildfires. The same script printed 3,109 wildfires of 3,268 when the earlier draft was written; counts for a closed window drift upward because the feed back-fills (**likely**: the window end is fixed, so back-fill is the only known variable; not investigated). The totals include **coordinate-counted floods (161), which `floods-by-title.mjs` shows are overstated**. Against title-counted floods the wildfire share is higher, about 97%: 3,296 wildfires against the 90 floods `floods-by-title.mjs --end 2026-08-04` prints for the same window (3,296 of 3,386).

⚠️ **Not backed by a committed script:** a scratch by-month run on 2026-11-17 showed wildfires in the Nigeria and Ghana boxes concentrated in November to February (130 in January 2026) and at zero during the rains. That is the seasonal argument for showing both hazards together. Until Phase 0 task 1 commits the script, treat it as **likely**, not measured. The same applies to the storm and volcano counts in D3.

The thing that is always true, in every season and every country, is the translation itself: an event with coordinates becomes an event with a place name. That is the product. The hazard is a property of the event, not of the product.

## What Changes

### The direction, in one paragraph

VigilAfrica shows **what is happening, by place name, across Africa**, for whatever hazards the feed carries. "Near me" is the default lens, and when nothing is near the visitor — which is common — the product says so plainly and widens to their country and then to all covered countries, instead of showing an empty list.

### Phase 0 — record the direction (documentation only)

- Amend [`product.md`](../specs/vigilafrica/product.md) §1–§2 with a dated direction addendum. That file's lock rule reads, in full: *"Any change to this document requires explicit maintainer review and a corresponding update to `roadmap.md`. No feature may be implemented unless it has an F-ID in this registry. Any implementation diverging from acceptance criteria here is drift."* Three clauses: maintainer review; a matching roadmap update; and the F-ID rule. Its §2.1 "Explicitly Out of Scope" list says the items there "must not be implemented until explicitly promoted by a new ADR in `decisions.md`", and it names **"Coverage beyond Nigeria (v0.6+ expansion model)"** and **"Event types beyond Floods and Wildfires"**. So this direction needs an ADR, not only an addendum.
- **Write the next ADR.** `decisions.md` indexes ADR-001 to ADR-016, so the next is **ADR-017**: continental, hazard-neutral scope. It must supersede **ADR-004 — Initial Country Scope: Nigeria First** (status ACCEPTED, 2026-04-12). ADR-004's stated consequences conflict with `feature-continental-coverage`: it locks the EONET bounding box to Nigeria (lat 4.0–14.0, long 2.0–15.0), loads only Nigerian ADM0/ADM1 boundaries, and calls any non-Nigeria boundary data before v0.6 out of scope. The shipped system already departs from it (Ghana is live), and `feature-continental-coverage` assumes 54 countries.
- **The F-ID question belongs to that ADR.** The F-ID rule says no feature may be implemented without an F-ID. In `openspec/`, the F-IDs in use are F-001 to F-017 in `product.md`, plus a prospective F-101 in `feature-secondary-oracle.md`. There is none above F-017 in the product registry, and the features shipped since launch (digest, pagination, `location_source`, nearby events, and so on) were governed by OpenSpec proposals and change records, not by new F-IDs. The registry has therefore been bypassed in practice since launch. ADR-017 must settle one of: retire the F-ID rule, or assign F-IDs going forward (and say how). Until it does, this proposal's Phase 1 changes the context endpoint contract without an F-ID, which the written rule forbids.
- Bring the [`roadmap.md`](../specs/vigilafrica/roadmap.md) milestone index up to date: v1.3 is shipped (the index row still says *Proposed*), v1.4–v1.6 are missing, and a new milestone records this direction.
- In [`feature-continental-coverage`](feature-continental-coverage.md): reverse the 2026-08-04 "floods default, wildfires opt-in" decision, and answer its open question 6 (positioning).
- Commit the by-month wildfire script and the stale-open-wildfire age script (spec, Phase 0 tasks 1 and 2).
- **Registry drift is recorded, not fixed.** The F-ID registry is not rewritten. Entries verified as out of step with the code are listed as drift in the spec (F-008 so far).

### Phase 1 — a near-me panel with a scope ladder

This is new interface work, not a small patch (see *What the "near me" feature is today*). See D5.

- **A near-me panel with a scope ladder.** Near you, then your country, then all covered countries, each with a count, so a visitor with nothing nearby still learns something true. Today's default list stays as the "all covered countries" rung.
- **Say what "now" means**, according to D6. Whichever option is chosen, the interface states the basis of every count and list ("open, or closed since DATE", or "in the last N days") rather than implying live status, and the stale-open wildfires are handled by name.
- **Say what the product cannot see.** One persistent, plain sentence: events come from global satellite-and-model feeds (NASA EONET, with floods sourced from GDACS); street-level and urban flash flooding are not covered. It extends the standing honesty rule defined below, and the Lagos geocoding spike measured why the gap is not closable soon.
- **Hazard-neutral copy** on the landing page and metadata, and on the partner page, subject to D4.
- **Absorb or sequence the location marker?** `feature-current-location-marker` already requires IP-derived location to be labelled approximate and lists wiring `nearby_events` as out of scope. **Recommendation: sequence the marker first, as its own small frontend-only change, and do not absorb it.** It is independent and cheap; its non-goal text explicitly assigns the near-me panel to a separate feature; and Phase 1 then reuses its "approximate, based on your network" wording instead of inventing a second one. If the marker slips, Phase 1 must carry the same approximate-location wording itself.

**The standing honesty rule** (defined here because no document names it): VigilAfrica is an awareness tool, not an emergency alert system; event locations and timing may be approximate; users must confirm with local authorities. It is stated in the dashboard disclaimer (`web/src/components/EventsDashboard.tsx`, the `dashboard-disclaimer` block, lines 167–176) and in `llms.txt` under "Important limitation" (`web/vite.config.ts`, line 141). New copy must not contradict it.

### Phase 2 — validation gate

The demand question cannot be answered from the feed. Before more is built: hold the conversations and take a dated **go / hold / freeze** decision (see *How we will know*). The gate sits here so that Phase 3 does not start on supply evidence alone.

### Phase 3 — continental coverage

Unchanged in substance; it stays its own proposal, [`feature-continental-coverage`](feature-continental-coverage.md), and starts only on a *go* from Phase 2. That proposal contains two decisions titled "Decision 1 — how are boundaries loaded?" and "Decision 2 — one Africa bbox, or 54 country bboxes?". **This proposal settles neither.** What it settles there is open question 6 (positioning: reposition as continental, per D1) and the 2026-08-04 floods-default decision (reversed, per D2).

### Phase 4 — judge it in season

The 2027 rains are the test; see *How we will know*.

## Decisions

D1 to D4 were made on 2026-11-17 and D5 and D6 on 2026-11-18, each as recommended. One parameter remains open: the length of the D6 window.

| # | Question | Status |
|---|---|---|
| D1 | **Positioning.** A continental product, or a Nigeria + Ghana product with continental data? | **Decided 2026-11-17: continental.** See reasoning below. |
| D2 | **Default view.** All hazards by recency, or floods first? | **Decided 2026-11-17: all hazards.** The dashboard already defaults to "All Categories", so this formally reverses the unimplemented 2026-08-04 decision. Note that an all-hazards view currently includes the stale-open wildfires; see D6. |
| D3 | **More categories now?** The feed also carries severe storms (13 in the window, mostly Indian Ocean cyclones) and volcanoes (4). | **Decided 2026-11-17: not in this change.** Seventeen events in 26 months does not justify new category support yet. Revisit after Phase 3, when Mozambique and Madagascar are covered. The 13 and 4 are from a 2026-11-17 scratch run, **not backed by a committed script**. The wording was "the category registry"; the registry was never built (`roadmap.md`'s v1.3 note records it only as a deferred item, `chore-deferred-work-register` §C1), so read this as "the categories the product supports". |
| D4 | **The flood digest and the partner offer.** `/v1/digest/today.json` and the partner page are flood-specific. | **Decided 2026-11-17: reword the partner page's lead; leave the digest endpoint alone.** The partner page must continue to describe the digest as flood-only. Partner material may lead with floods in season. Changing the digest's content is a contract change and deserves its own proposal once a partner exists to ask for it. |
| D5 | **Build a near-me panel with a scope ladder** (Phase 1). | **Decided 2026-11-18: yes, after the location marker.** Re-opened on 2026-11-17 because the earlier "yes, smallest change" rested on there being an existing near-me view to patch; there is none. This is new interface plus an API change, built because it is the only change that answers the product's own question. `feature-current-location-marker` lands first, as its own frontend-only change. |
| D6 | **What "now" means.** (a) introduce a display window, or (b) keep showing everything and say so plainly. | **Decided 2026-11-18: (a), a display window, applied to `/v1/context` first.** The window length is set from the Phase 0 age-of-open-wildfires script. See below. |

### D1 — reasoning

Alternatives considered:

- **A different or additional anchor country.** Today's output of `floods-by-title.mjs` shows four countries with more floods than Nigeria's 7: Algeria 14, Democratic Republic of Congo 13, South Africa 12, Kenya 8. Anchoring elsewhere would give a denser flood record than Nigeria. It would also abandon the geography the project has boundary data and contacts for, and one-country anchors keep the same single-season exposure.
- **Freeze now.** The supply evidence does not by itself show demand, so freezing is a coherent choice. It is addressed by the Phase 2 gate rather than by deciding it today.

Continental is still recommended because the feed carries events across the continent in every season and a two-country frame is what produces the sparse view. **The evidence is about the supply of events, not demand.** It says nothing about whether anyone wants the product.

### D6 — what "now" means (decided 2026-11-18: option a)

Options:

- **(a) Introduce a display window.** For example, wildfires by `event_date` in the last N days, floods likewise. Fixes the stale-open wildfire problem at source. **It is a behaviour change to `/v1/events` and `/v1/context`, not an additive field**: existing consumers, including the partner page's documented endpoints, would see fewer rows.
- **(b) Keep showing everything and say so.** State plainly "open, or closed since DATE" and show each event's date. Additive; no consumer sees fewer rows. Does not by itself stop a 14-month-old open fire counting as "near you".

Either way the stale-open wildfires are a named problem to handle: under (a) they fall outside the window; under (b) they need separate treatment in counts and wording.

**Decision (2026-11-18): (a), applied consistently to `/v1/context` first.** Whether `/v1/events` adopts the same window is decided in Phase 1, when its effect on the partner page's documented endpoints is known. A near-me count that includes an old fire the feed never closed is wrong in a safety-adjacent product. Whether those old open fires are still burning or were simply never closed is not known (**assumption**: mostly never closed; if wrong, a window would hide real long burns). Phase 0 task 2 commits a script that reports the age of open wildfires; the window length is set from its output, before Phase 1 starts.

## Non-goals

- **Not dropping floods.** The marginal cost of keeping them is low because the expensive work is already done: floods are why the GDACS geometry resolver and the degraded-run status exist (`api/internal/ingestor/gdacs.go`). They are, in the author's judgement, the events with the most recognisable news value, and the main hook for the humanitarian partner story.
- **Not a local-relevance fix.** A Lagos resident still sees no urban flooding. Nothing here changes that, and the copy must not imply otherwise. The Lagos geocoding spike ([`proposal-spike-lagos-report-geocoding.md`](../archive/proposal-spike-lagos-report-geocoding.md), complete 2026-08-04, harness in `scripts/spike-lagos-geocoding/`) found 23.5% of reported locations placed in the wrong part of the city.
- **Not new data sources, alerts, accounts or sub-state (ADM2) naming.**
- **Not the continental build itself** — that remains `feature-continental-coverage`.

## How we will know

This direction is a bet and should be falsifiable. The thresholds below are **proposed, for the maintainer to confirm**; none is decided.

- **Before the rains — proposed date 2027-03-31:** at least three of five structured conversations with responders, journalists or logistics planners held and written up the same day; at least one reply to partner outreach. Analytics traffic is not a dry-season criterion: an empty season cannot produce it.
- **Falsifier — proposed threshold:** if at least three of the conversations held (minimum three held) say they need neighbourhood- or street-level precision and would not use country- and state-level data, this direction does not serve them, and the honest outcome is to freeze rather than build Phase 3.
- **In the 2027 rainy season (proposed window June to September 2027):**
  - *External visitors:* at least 50 Umami sessions a week on average with the maintainer excluded through the existing self-exclusion mechanism (`openspec/archive/proposal-chore-analytics-self-exclusion.md`). Analytics is cookieless (`docs/operations/analytics.md`), so counts undercount and the relative trend is the signal; 50 is a placeholder to be set once a baseline exists.
  - *A partner using the data:* a named organisation outside the maintainer's circle confirms in writing that it used VigilAfrica data (API, digest or the map) for an event-response or reporting task, and the confirmation is recorded in the repository (an issue or a dated note).
  - *Ladder use:* the analytics event named in the spec shows the ladder action used at least once from a non-maintainer session. Without it validation cannot tell whether the panel is used.

## Risks

- **A wildfire map nobody asked for.** Most satellite fire detections in African savanna are seasonal land burning, and free tools already map fires in more detail. *(General knowledge, not measured here.)* The Phase 2 conversations are the check.
- **The flood partner story weakens** if the product stops leading with floods. Mitigation: floods stay first-class, and partner material can still lead with them in season. The partner page keeps describing the digest as flood-only (D4).
- **Scope creep.** "Any hazard, all of Africa" invites every category and every source. D3 and the non-goals are the guard.
- **Country matching at scale.** The country rung depends on matching the geolocation provider's country name to `events.country_name`. That works for two countries and is a known risk at 54 (see spec).

## Origin

Project review, 2026-11-17: production is stable at v1.6.4; the maintainer raised that the dry season makes a flood product hard to show and considered dropping floods. The maintainer's account is that there are no recorded external users; that is **not recorded in this repository** (`docs/operations/analytics.md` describes how traffic is measured and holds no figures). The by-month recount (branch `docs/eonet-flood-density-by-title`) showed the problem was the two-country, flood-first frame rather than floods themselves. Technical detail in [`openspec/specs/feature-place-first-direction.md`](../specs/feature-place-first-direction.md).

## Review history

An independent review on 2026-11-17 found errors in the first draft. In substance: the draft said every event is closed and the product shows a 30-day window (it shows everything ingested, including stale-open wildfires; now D6); it treated the near-me ladder as a patch to an existing empty state (there is no near-me view; D5 re-opened); it called floods "satellite-detected" (all are GDACS-sourced); it quoted figures that no longer reproduce and an arithmetic slip about floods outside the two countries; it overstated the evidence for a five-month flood gap (one dry season); it omitted the governing lock rule, ADR-004 and the F-ID question; and it listed acceptance criteria that could not be tested. The phases were reordered so the validation gate precedes continental coverage.
