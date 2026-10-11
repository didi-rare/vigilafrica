---
id: chore-detail-loading-and-cta-anchor
status: proposed
branch: claude/chore-web-audit-leftovers-xt5v6i
spec: ../specs/chore-detail-loading-and-cta-anchor.md
---

# Proposal: Close F1 and F2 From the Web-Audit Leftovers (chore-detail-loading-and-cta-anchor)

## Why

`chore-web-audit-leftovers` (#283) built one loading treatment, `LoadingState`, and applied it
to the three loading regions on the landing page. Its independent review found two places the
same defects still live and, on the review's advice, recorded them in the deferred-work register
as **F1** and **F2** rather than in the spec's "Out of scope" section, so they would survive the
archive. This proposal closes both. Each is small; together they are one web PR.

### F1. `EventDetail` loading region has no live region and its own text-only treatment

[`web/src/pages/EventDetail.tsx`](../../web/src/pages/EventDetail.tsx) renders
`<div className="event-detail-state">Loading event telemetry...</div>` while the event query is
pending. That is exactly the pre-#283 dashboard shape: no `role="status"`, so a screen-reader
user hears nothing while the page waits, and a dashed text card where every other wait in the
app now shows the spinner card. A user who taps an event on the dashboard sees the dashboard's
spinner card, then a different treatment for the next wait.

### F2. The hero CTA's `#dashboard` anchor does not exist while the chunk loads

`Explore latest events` is `href="#dashboard"`. `#dashboard` is the `<section>` rendered by the
lazy `EventsDashboard` chunk, so while the chunk is in flight the anchor target does not exist
and a tap does nothing: no scroll, no focus move, no feedback. On a slow phone connection that
is precisely the moment the user is most likely to tap it. The same gap breaks a deep link: a
visitor arriving at `/#dashboard` gets the browser's initial hash scroll before the chunk exists,
so the page stays at the top.

## What Changes

### F1 — the detail page's loading branch renders `LoadingState`

The pending branch renders `<LoadingState message="Loading event telemetry..." />` inside the
same page shell. `.event-detail-state` stays: it is also the not-found card (`EventDetail.tsx`
line 73), which is not a loading state and keeps its text-only treatment. No new CSS: the
component ships its own stylesheet, and the detail page already imports from `components/`.

### F2 — the Suspense fallback carries `id="dashboard"` while the chunk is pending

The register offered two mechanisms, "giving the Suspense fallback the id, or scrolling on
mount", and asked for a measurement of which reads better with the #193 reservation. The
proposal is the first one, for reasons that can be stated before measuring and are then checked
by the measurement:

- **It is the browser's own anchor behaviour.** A tap scrolls to the reservation, which starts
  where the dashboard will start. Nothing in React has to know about the hash, `:target`
  works, and the deep-link case (`/#dashboard` on first load) is fixed by the same line because
  the browser's initial hash scroll finds a target.
- **The reservation makes the landing stable.** Above 480px `.dashboard-fallback` reserves
  `min(100vh, 1530px)`, so when the chunk mounts in place of the fallback the user's scroll
  position does not move; the dashboard appears where the fallback was. At ≤480px there is no
  reservation (#198), so the fallback is the card's own height; the mount replaces it in place
  and grows downward, and the user is still looking at the top of the dashboard.
- **Scroll-on-mount reads worse by construction.** The tap would still do nothing at the time
  of the tap; the page would then jump on its own some seconds later, after the user has
  scrolled or given up. It also needs code (a hash check in an effect, a `scrollIntoView`
  that jsdom lacks and `Select.tsx` already has to guard), where the id needs none.

Only one element carries the id at any time: React unmounts the Suspense fallback before the
lazy content mounts, and the chunk loads once, so `#dashboard` never appears twice.

`scripts/bench-dashboard-cls/capture-fallback.mjs` uses `#dashboard` as its "the chunk mounted
anyway" check; with the fallback carrying the id that check would pass vacuously, so it moves to
a selector only the mounted dashboard has (`section#dashboard.dashboard`). The same script
gains the measurement the register asked for: with the chunk held, click the CTA and report the
reservation's position relative to the viewport; then release the chunk and report where the
mounted dashboard's heading sits, so "the user lands on the dashboard" is a measured rectangle
at 375×812 and 1350×940, on the branch and on a control build where the tap does nothing.

## Out of Scope

- **The detail page's map Suspense fallback** (`Loading map telemetry...` in
  `.map-unavailable`). It is a third text-only wait, but it also reserves the map's 24rem box,
  which `LoadingState` does not, and it mounts only after the event has arrived, so it is a
  separate wait with a different shape. Not in F1; if it is wanted it is its own one-line item
  in the register, not a silent widening here.
- **F3** (synthetic-user-agent rate) is unmeasurable from this repo, as the register records.
- Any change to the reservation heights or the progress bar from #283.

## Verification

- `EventDetail.test.tsx`: with the event query held pending, a polite `role="status"` region
  named "Loading status" carries the message, the not-found card is not a status region, and
  axe passes on both states.
- `App.fallback.test.tsx`: with the chunk held, the hero CTA's `href` target resolves to an
  element in the document, and that element is the reservation.
- `capture-fallback.mjs`, two arms (control build vs branch) at 375×812 and 1350×940: after
  the CTA click the control arm reports the reservation out of view and the branch arm reports
  it in view; after the chunk is released the branch arm reports the dashboard heading at the
  top of the viewport. CLS is re-measured with `measure-cls.mjs` against the control arm because
  this PR touches the fallback markup, even though it changes no geometry.
- `npm run lint`, `type-check`, `lint:styles`, `test`, `build` clean.
