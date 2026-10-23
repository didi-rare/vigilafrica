---
id: chore-detail-loading-and-cta-anchor
status: proposed
branch: docs/propose-detail-loading-and-cta-anchor
spec: ../specs/chore-detail-loading-and-cta-anchor.md
---

# Proposal: Close F1 and F2 From the Web-Audit Leftovers (chore-detail-loading-and-cta-anchor)

## Why

`chore-web-audit-leftovers` (#283) built one loading treatment, `LoadingState`, and applied it
to the three loading regions on the landing page. Its independent review found two places the
same defects still live and, on the review's advice, recorded them in the deferred-work register
as **F1** and **F2** rather than in the spec's "Out of scope" section, so they would survive the
archive. This proposal closes both. Each is small; together they are one web PR.

### F1. The event page shows two different text-only waits, neither announced

Opening an event can show **two consecutive waits**, and today both are plain text with no
`role="status"`:

1. **The route-level Suspense fallback** while the `EventDetail` chunk downloads
   ([`App.tsx`](../../web/src/App.tsx), the `/events/:id` route):
   `<div className="container section">Loading event telemetry...</div>`, with no page shell and
   no back link. The chunk is not preloaded, so this shows on the **first** event opened in a
   visit; later opens reuse the loaded chunk and skip it.
2. **`EventDetail`'s own pending branch** while the event query runs
   ([`EventDetail.tsx`](../../web/src/pages/EventDetail.tsx)): the same words in the dashed
   `.event-detail-state` card, inside the page shell, under the back link. This shows on
   **every** event opened.

So a first visit flashes unstyled text, then a different card, while every other wait in the
app now shows the spinner card. A screen-reader user hears nothing through either wait.

### F2. The hero CTA's `#dashboard` anchor does not exist while the chunk loads

`Explore latest events` is `href="#dashboard"`. `#dashboard` is the `<section>` rendered by the
lazy `EventsDashboard` chunk, so while the chunk is in flight the anchor target does not exist
and a tap does nothing: no scroll, no focus move, no feedback. On a slow phone connection that
is precisely the moment the user is most likely to tap it. A visitor arriving at `/#dashboard`
directly (a shared link or bookmark) may have the same problem: the browser's fragment scroll
can run before the target exists.

## What Changes

### F1: both waits become one spinner card, announced once

Both waits render the **same** component, `EventDetailLoading`: the event page shell, the back
link, and `LoadingState` with "Loading event telemetry...". Swapping one for the other is then
invisible, so a first visit reads as one continuous wait.

- **Only the second wait announces.** The route fallback renders it with `announce={false}`;
  `EventDetail`'s pending branch with the default `announce`. The second wait happens on every
  open and the first only on the first, so announcing the second gives exactly one announcement
  per open either way. This is the rule #283 already applies on the dashboard: one wait, one
  announcement (the map fallback is `announce={false}` for the same reason).
- **The shell's styles must load before the chunk.** `.event-detail-page` (2rem top padding,
  `min-height: 100vh`) lives in `EventDetail.css`, which ships inside the lazy chunk. A route
  fallback using it would render without the padding and then jump 32px when the chunk lands.
  The rule moves to `App.css`, which loads with the app. `.back-link`, `.container` and
  `LoadingState`'s own stylesheet already load up front.
- **The not-found card is untouched.** `.event-detail-state` stays: it is an end state, not a
  wait.
- **What F1 does not claim.** It does not announce the event *arriving*. Nothing in the app
  announces content arriving or moves focus on a route change; that is a site-wide gap, recorded
  as register item **F4**, not something to imply here.

### F2: the Suspense fallback carries `id="dashboard"` while the chunk is pending

The register offered two mechanisms, "giving the Suspense fallback the id, or scrolling on
mount". This proposal takes the first, **and measures before adding the second**:

- **The click.** The reservation `div` (`.dashboard-fallback`) carries `id="dashboard"`. A tap
  during the load scrolls to it, and the reservation starts where the dashboard will start, so
  the dashboard appears where the user landed. It is the browser's own anchor behaviour: no
  JavaScript, `:target` works, and React unmounts the fallback before mounting the dashboard, so
  the id is never on two elements.
- **Clearing the fixed nav.** `App.css` gives `section[id]`, `h2[id]` and `#dashboard-heading`
  `scroll-margin-top: 88px`. The fallback is a `div`, which `section[id]` does not match, so the
  rule gains a `#dashboard` selector. Without it the landing sits under the nav.
- **Direct `/#dashboard` loads: measured, not assumed, and probably needing the contingency.**
  The HTML Standard's "try to scroll to the fragment" gives up once the document's parser has
  stopped parsing. The app is a module script, which runs after parsing stops, so by the standard
  even the placeholder is created too late, and direct loads would still fail. Browsers may retry
  longer than the standard requires, so the bench measures it in a real browser with the
  dashboard chunk held. **If the measurement fails, as the standard predicts,** the same PR adds
  a one-time scroll for that case (spec §2, *Contingency*); if it passes, nothing is added.
- **Why not scroll-on-mount for the click.** A tap would still do nothing at tap time, and the
  page would then move on its own seconds later, after the user may have scrolled or given up.
  The contingency is limited to the direct-load case, where no tap has happened.

The bench (`scripts/bench-dashboard-cls/capture-fallback.mjs`) gains the CTA-click and direct-load
measurements; its existing "the chunk mounted anyway" guard moves to a selector only the mounted
dashboard has (`section#dashboard.dashboard`), since the fallback now matches `#dashboard` too.

## Out of Scope

- **The detail page's map Suspense fallback** (`Loading map telemetry...` in
  `.map-unavailable`). It reserves the map's 24rem box, which `LoadingState` does not, and it
  mounts only after the event has arrived, so it is a separate wait with a different shape.
- **The `/for-partners` route fallback** (`Loading...`, text-only). Same shape of gap as F1's
  first wait, on a different page; it can follow the same pattern later if wanted.
- **Announcing content arrival / focus on route change**: register **F4**.
- **F3** (synthetic-user-agent rate) is unmeasurable from this repo, as the register records.
- Any change to the reservation heights or the progress bar from #283.

## Implementation note: the sentinel

The sentinel counts a governance record only if it is **changed in the same PR** as the code
(`api/cmd/sentinel/main.go`). The implementation PR must therefore update this proposal (at least
`status: in-progress`, then the verification below), or it will be blocked.

## Verification

Full criteria in the spec. In short:

- **F1, unit:** both waits render the same shell and card; only `EventDetail`'s announces; axe
  clean on the pending and not-found states. These tests prove the **markup** (a polite status
  region with the right text), not that a given screen reader speaks it; a manual check with a
  real screen reader is recommended but not a gate.
- **F1, no jump:** in a real browser on a cold load, the loading card's position is the same
  (±2px) before and after the chunk lands.
- **F2, click and direct load:** two-arm bench (control build vs branch) at 375×812 and
  1350×940, with the chunk held: the control shows the click doing nothing (the positive
  control); the branch lands the reservation at the scroll margin, and after release the
  dashboard appears in place. The direct-load arm decides whether the contingency is built.
- **CLS:** `measure-cls.mjs`, 8 runs per viewport against a recorded control SHA, no viewport
  worse than control beyond the stated tolerance.
- `npm run lint`, `type-check`, `lint:styles`, `test`, `build` clean.
