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

Opening an event whose data is not already cached can show **two consecutive waits**, and today
both are plain text with no `role="status"`:

1. **The route-level Suspense fallback** while the `EventDetail` chunk downloads
   ([`App.tsx`](../../web/src/App.tsx), the `/events/:id` route):
   `<div className="container section">Loading event telemetry...</div>`, with no page frame and
   no back link. The chunk is not preloaded, so this shows on the **first** event opened in a
   visit.
2. **`EventDetail`'s own pending branch** while the event query runs
   ([`EventDetail.tsx`](../../web/src/pages/EventDetail.tsx)): the same words in the dashed
   `.event-detail-state` card, inside the page frame, under the back link. This shows whenever
   the event's data is not already in the query cache.

So a first visit flashes unstyled text, then a different card, while every other wait in the
app now shows the spinner card. Neither wait is a live region.

### F2. The hero CTA's `#dashboard` anchor does not exist while the chunk loads

`Explore latest events` is `href="#dashboard"`. `#dashboard` is the `<section>` rendered by the
lazy `EventsDashboard` chunk, so while the chunk is in flight the anchor target does not exist
and a tap does nothing: no scroll, no focus move, no feedback. On a slow phone connection that
is precisely the moment the user is most likely to tap it. A visitor arriving at `/#dashboard`
directly (a shared link or bookmark) is not scrolled at all (verified in Chromium 156, below).

## What Changes

### F1: one loading region that is never replaced

The two waits are two Suspense handoffs, and React does not reuse DOM across one: when a
boundary's content resolves, its fallback is unmounted and the content mounted fresh. Two
look-alike placeholders would still restart the spinner, drop keyboard focus from a back link
inside them, and mount a second live region. So F1 makes the whole wait **one** suspension
behind **one** fallback:

- **An eager page frame.** The route renders a new eager component, `EventDetailRoute`: the
  page frame (`.event-detail-page`), the back link, and, inside them, one `Suspense` whose
  fallback is `<LoadingState message="Loading event telemetry..." />` around the lazy
  `EventDetail`. The frame and back link are outside the boundary, so they never unmount.
- **The data wait suspends too.** `EventDetail` reads the event with `useSuspenseQuery` instead
  of `useQuery`. While the chunk downloads, the boundary shows its fallback. When the chunk
  arrives, `EventDetail` suspends again on the query, and React keeps the **already-committed**
  fallback rather than remounting it. One region, one spinner, one announcement opportunity,
  from the first frame to the event. That React behaviour is what the tests must prove (spec §1).
- **"Event not found" moves to a local error boundary.** `fetchEventById` throws `ApiError` on a
  non-OK response; `useSuspenseQuery` rethrows it. A local `ErrorBoundary` inside the frame
  renders the existing not-found card for `ApiError` only. Any other error (a failed chunk
  download, a render bug) is rethrown to the app-level `ErrorBoundary`, as today.
- **Styles live with the frame.** `.event-detail-page` and `.event-detail-state` move from
  `EventDetail.css` (lazy) to `EventDetailRoute.css` beside the new component, which loads up
  front. That is the co-location rule (`developers-react.md` §1.5), and it is what lets the frame
  render correctly before the chunk arrives.
- **What F1 does not claim.** It does not announce the event *arriving*, and it does not
  guarantee speech: the tests prove markup (one polite `role="status"` region with the visible
  text), and whether a given screen reader speaks a live region that mounts already holding its
  text varies. The arrival gap is register item **F4**.

### F2: the Suspense fallback carries `id="dashboard"`, plus a first-load scroll

- **The click.** The reservation `div` (`.dashboard-fallback`) carries `id="dashboard"`. A tap
  during the load scrolls to it, and the reservation starts where the dashboard will start, so
  the dashboard appears where the user landed. React unmounts the fallback before mounting the
  dashboard, so the id is never on two elements.
- **Clearing the sticky nav.** `App.css` gives `section[id]`, `h2[id]` and `#dashboard-heading`
  `scroll-margin-top: 88px`. The fallback is a `div`, which `section[id]` does not match, so the
  rule gains a `#dashboard` selector. Without it the landing sits under the nav.
- **Direct `/#dashboard` loads need code.** The HTML Standard's "try to scroll to the fragment"
  stops retrying once the parser has stopped, and the app's entry point is a non-async module
  script, which runs only after that. So no element exists during the retries. The review
  confirmed it in Chromium 156: with the chunk held, `scrollY` stayed 0, and releasing the chunk
  did not retry. `DashboardFallback` therefore scrolls once on the app's first load, judged by
  where the target actually is, not by `scrollY` (spec §2). It never runs on reload or
  back/forward, where the browser restores the user's position, and never on a CTA click.
- **Why not scroll-on-mount for the click.** A tap would still do nothing at tap time, and the
  page would then move on its own seconds later. The first-load scroll runs at the app's first
  commit, before the user can have scrolled.

The bench (`scripts/bench-dashboard-cls/capture-fallback.mjs`) gains the CTA-click, direct-load
and event-page measurements; its existing "the chunk mounted anyway" guard moves to a selector
only the mounted dashboard has (`section#dashboard.dashboard`).

## Out of Scope

- **The detail page's map Suspense fallback** (`Loading map telemetry...` in
  `.map-unavailable`). It reserves the map's 24rem box, which `LoadingState` does not, and it
  mounts only after the event has arrived, so it is a separate wait with a different shape.
- **The `/for-partners` route fallback** (`Loading...`, text-only). Same shape of gap as F1's
  first wait, on a different page; it can follow the same pattern later if wanted.
- **Route-change focus and announcing the event's arrival**: register **F4**.
- **Retry behaviour on a not-found event.** The default `QueryClient` retries a failing query
  three times before the not-found card appears; that is unchanged.
- **F3** (synthetic-user-agent rate) is unmeasurable from this repo, as the register records.
- Any change to the reservation heights or the progress bar from #283.

## Implementation note: the sentinel

The sentinel counts a governance record only if it is **changed in the same PR** as the code
(`api/cmd/sentinel/main.go`). The implementation PR must therefore update this proposal (at least
`status: in-progress`, then the verification below), or it will be blocked.

## Verification

Full criteria in the spec. In short:

- **F1, unit:** the status region and the back link are the **same DOM nodes** from the first
  fallback frame through the chunk arriving to the query resolving, there is never more than one
  status region, focus on the back link survives, and not-found and other errors route correctly.
- **F1, real browser:** on a cold load, the loading card does not move (±2px) or remount when
  the chunk lands.
- **F2:** two-arm bench (control build vs branch) at 375×812 and 1350×940, with the chunk held:
  the control shows the click doing nothing (the positive control); the branch lands the
  reservation at the scroll margin; the direct-load arm lands it on first load and leaves reload
  and back/forward positions alone.
- **CLS:** `measure-cls.mjs`, 8 runs per viewport against a recorded control SHA, medians within
  the stated tolerance.
- `npm run lint`, `type-check`, `lint:styles`, `test`, `build` clean.
