---
id: chore-detail-loading-and-cta-anchor
status: proposed
proposal: ../proposals/chore-detail-loading-and-cta-anchor.md
branch: docs/propose-detail-loading-and-cta-anchor
---

# Spec: EventDetail Loading Region and the Hero CTA Anchor

Technical spec for [`chore-detail-loading-and-cta-anchor`](../proposals/chore-detail-loading-and-cta-anchor.md).
Two independent items in `web/src/`, both reusing what #283 built.

## Components touched

| file | change |
|---|---|
| `web/src/components/EventDetailLoading.tsx` | **new**: the event page shell + back link + `LoadingState`, with an `announce` prop passed through |
| `web/src/components/EventDetailLoading.test.tsx` | **new**: renders the shell, back link and card; `announce={false}` has no status region |
| `web/src/App.tsx` | `/events/:id` Suspense fallback → `<EventDetailLoading announce={false} />`; `DashboardFallback`'s reservation `div` gets `id="dashboard"` |
| `web/src/pages/EventDetail.tsx` | pending branch → `<EventDetailLoading />` |
| `web/src/pages/EventDetail.test.tsx` | pending state: one polite status region with the message; not-found card is not a status region; axe |
| `web/src/App.css` | gains the `.event-detail-page` rule (moved); the scroll-margin rule gains a `#dashboard` selector |
| `web/src/pages/EventDetail.css` | loses `.event-detail-page` (moved); comment on `.event-detail-state` updated: it is the not-found card only |
| `web/src/App.fallback.test.tsx` | the CTA's `href` resolves to the reservation while the chunk is pending |
| `scripts/bench-dashboard-cls/capture-fallback.mjs` | chunk-mounted guard moves to `section#dashboard.dashboard`; new CTA-click, direct-load and post-release measurements |
| `scripts/bench-dashboard-cls/README.md` | the new measurements and their two-arm protocol |

No change to `LoadingState` itself or to `EventsDashboard`. If the direct-load measurement fails,
the contingency in §2 adds one effect to `App.tsx` and its test.

## 1. The event page's two waits (F1)

**Today:**

```tsx
// App.tsx, /events/:id route: wait 1, first event opened in a visit only
<Suspense fallback={<div className="container section">Loading event telemetry...</div>}>

// EventDetail.tsx: wait 2, every event opened
if (isPending) {
  return (
    <div className="event-detail-page">
      <div className="container section">
        <Link to="/" className="back-link">← Back to Sentinel Dashboard</Link>
        <div className="event-detail-state">Loading event telemetry...</div>
      </div>
    </div>
  )
}
```

**Design.** A new `components/EventDetailLoading.tsx`:

```tsx
export function EventDetailLoading({ announce = true }: { announce?: boolean }) {
  return (
    <div className="event-detail-page">
      <div className="container section">
        <Link to="/" className="back-link">← Back to Sentinel Dashboard</Link>
        <LoadingState message="Loading event telemetry..." announce={announce} />
      </div>
    </div>
  )
}
```

The route fallback renders `<EventDetailLoading announce={false} />`; `EventDetail`'s pending
branch renders `<EventDetailLoading />`.

- **One announcement per open.** Wait 2 happens on every open and wait 1 only on the first, so
  announcing wait 2 gives exactly one announcement either way. Announcing wait 1 instead would
  leave repeat opens silent; announcing both would queue two polite regions back to back on a
  first visit, which #283 already rules out (its map fallback is `announce={false}`).
- **It lives in `components/`, not `pages/`.** `App.tsx` imports it eagerly, so it must not pull
  the `EventDetail` chunk in with it. It imports only `react-router`'s `Link` and `LoadingState`,
  both already in the main bundle.
- **The shell's CSS moves to `App.css`.** `.event-detail-page { padding-top: 2rem; min-height:
  100vh; }` is in `EventDetail.css`, which loads with the lazy chunk. Wait 1 renders before
  that, so without the move wait 1 would sit 2rem higher and jump when the chunk's CSS arrives.
  The rule moves verbatim. `.back-link` (`App.css`), `.container` (`App.css`) and
  `.loading-state` (`LoadingState.css`, imported by the main bundle's `LoadingState`) already
  load up front.
- **The not-found card is untouched.** `.event-detail-state` stays for it. It is an end state,
  not a wait, and is not a status region.
- **Spacing.** `.loading-state` has `margin-top: 2rem` where `.event-detail-state` has `1.5rem`.
  That is the one treatment applied as-is (§7.3: no page-specific override of a component's own
  stylesheet). Nothing below the card is reserved, so the 8px moves nothing else.
- **Leaving the wait is not announced.** When the event arrives, the `h1`, the `role="note"`
  disclaimer and the rest replace the card, but none of them is a live region, and focus does
  not move. A screen-reader user is not told the content arrived. That is true of every route
  change in the app, so it is register item **F4**, not part of F1, and nothing here claims
  otherwise.
- **Markup, not speech.** The tests below prove the semantics (`role="status"`,
  `aria-live="polite"`, the visible text as the live text). Whether a given screen reader speaks
  a live region that mounts already containing its text varies by product; a manual check with a
  real screen reader is recommended, but it is not an acceptance gate.

Standards: `developers-react.md` §9.7/§9.8 (a loading state is a `role="status"` region whose
visible text is the announcement; a spinner beside text is `aria-hidden`), already satisfied by
`LoadingState`.

## 2. Hero CTA anchor during the chunk load (F2)

**Today:** `DashboardFallback` renders the fixed progress bar and
`<div className="container section dashboard-fallback">` around `LoadingState`. The `#dashboard`
`<section>` exists only once `EventsDashboard` mounts.

**Design:** the reservation `div` carries `id="dashboard"`:

```tsx
<div id="dashboard" className="container section dashboard-fallback">
  <LoadingState message="Loading dashboard telemetry..." />
</div>
```

- **Which element.** The reservation, not the progress bar (fixed, zero-footprint, so a scroll to
  it would go nowhere) and not the `LoadingState` card (its `margin-top` would land the scroll
  32px short of where the mounted section starts). The reservation's top is where
  `section#dashboard` will be.
- **Scroll margin (an `App.css` change).** `App.css` gives `section[id]`, `h2[id]` and
  `#dashboard-heading` `scroll-margin-top: 88px` so anchored targets clear the fixed nav. The
  fallback is a `div`, so `section[id]` does not match it; the rule gains `#dashboard` as a
  selector, which matches both the fallback and, redundantly, the mounted section. A selector
  addition, not a new value, so no token is involved.
- **Uniqueness.** React removes the Suspense fallback before committing the lazy subtree, and
  `App.tsx` lazy-loads the chunk once per page, so there is never a moment with two
  `#dashboard` elements. `App.test.tsx` already renders the mocked dashboard and asserts the
  CTA's `href`; `App.fallback.test.tsx` holds the chunk and asserts the target resolves.
- **Direct loads: measured, then decided.** On a page opened at `/#dashboard`, the HTML
  Standard's "try to scroll to the fragment" retries only until "its parser has stopped
  parsing". The app's entry point is a module script, which runs after parsing stops, so by the
  standard the fallback does not exist yet during those retries, and direct loads still fail.
  Browsers may retry longer than the standard requires, so the bench's direct-load arm (below)
  decides it. **Expect the contingency to be needed.**
- **Contingency, built only if the direct-load arm fails on the branch.** `App.tsx` adds one
  effect that runs once after the first render: if `location.hash === '#dashboard'` **and**
  `window.scrollY === 0` (the browser did not scroll, and neither did the user), it calls
  `document.getElementById('dashboard')?.scrollIntoView?.()` (the optional call because jsdom
  lacks `scrollIntoView`, the same guard `Select.tsx` uses). It never fires after the user has
  scrolled, and it does not run on CTA clicks. A unit test covers both guards.
- **Why not scroll-on-mount for the click.** A tap would still do nothing at tap time, and the
  page would then move on its own seconds later. The contingency is only for direct loads,
  where no tap happened.

**Bench script changes** (`capture-fallback.mjs`):

1. `chunkMounted` becomes `document.querySelector('section#dashboard.dashboard') !== null`,
   because the fallback now also matches `#dashboard`. Without this the script's own guard would
   be satisfied by the fallback and could never fire.
2. **Click arm.** With the chunk held, click `#hero-explore-cta`, let scrolling settle (motion
   forced to `auto` via `page.emulateMedia({ reducedMotion: 'reduce' })`, which the existing
   reduced-motion rule honours), and report `window.scrollY`, the reservation's rectangle, the
   nav's bottom edge, and whether the `role="status"` region is in the viewport.
3. **Direct-load arm** (`DEEP_LINK=1`). Open `/#dashboard` with the chunk held from the start,
   wait for the load event plus one second, and report the same values.
4. **Release.** Release the held chunk route, wait for `section#dashboard.dashboard`, and report
   the mounted section's rectangle, the `#dashboard-heading` rectangle and `window.scrollY`.
5. The exit code stays non-zero on any null rectangle or on the chunk having mounted before
   release. `CLICK_CTA=0` keeps the pre-existing #283 measurement reproducible on its own.

**F1 bench addition.** With the `EventDetail` chunk held on a cold load of `/events/<id>`, report
the loading card's top; release the chunk with the event query held, and report it again.

The protocol in the README: two arms, control (the recorded `origin/development` SHA the branch is
based on) and branch, at 375×812 and 1350×940.

## Acceptance criteria

### F1

- [ ] `EventDetailLoading` renders the page shell, the back link and the card. With
      `announce={false}` there is no `role="status"` in the document.
- [ ] With `fetchEventById` held pending, `EventDetail` renders **exactly one** `role="status"`
      region, with `aria-live="polite"`, accessible name "Loading status", and text
      "Loading event telemetry..."; the back link is present above it.
- [ ] The `/events/:id` Suspense fallback renders `EventDetailLoading` with no status region.
- [ ] With `fetchEventById` rejecting, the not-found card renders with no `role="status"` in the
      document.
- [ ] axe reports no violations in the pending state and in the not-found state.
- [ ] `EventDetailLoading.tsx` imports nothing from `web/src/pages/` (it must not pull in the lazy
      chunk), and the production build still emits `EventDetail` as its own chunk.
- [ ] Bench, cold load, both viewports: the loading card's top is the same (±2px) before and
      after the `EventDetail` chunk lands.
- [ ] No spec or code comment claims the event's arrival is announced. Register F4 exists.

### F2

- [ ] With the dashboard chunk held, `document.querySelector(cta.getAttribute('href'))` is the
      `.dashboard-fallback` element, and no other element has `id="dashboard"`.
- [ ] With the dashboard chunk mocked as mounted (`App.test.tsx`), exactly one element has
      `id="dashboard"` and it is the dashboard section.
- [ ] Click arm, chunk held: on the **control**, `scrollY` is 0 (the positive control); on the
      **branch**, the reservation's top is 88px (±2px) from the viewport top at both sizes, and
      at or below the nav's bottom edge.
- [ ] After release, branch: the mounted section's top equals the reservation's pre-release top
      (±2px), `scrollY` is unchanged (±2px), and `#dashboard-heading` is in the viewport and at
      or below the nav's bottom edge.
- [ ] Direct-load arm, branch: the same position criterion as the click arm. **If it fails,**
      the contingency is built and the arm must then pass; the result, either way, is recorded
      in the README with the run's numbers.
- [ ] `measure-cls.mjs`, 8 runs per cell at 1920×1600, 1350×940, 768×1024 and 375×812 against
      the control SHA (the README's protocol): for every cell, the branch median is no more than
      the control median + 0.001.
- [ ] `capture-fallback.mjs` with `CLICK_CTA=0` produces the #283 report unchanged
      (bar at y 0–3 at 375×812).

### Both

- [ ] `npm run lint`, `npm run type-check`, `npm run lint:styles`, `npm run test`,
      `npm run build` clean.
- [ ] The implementation PR updates this record (sentinel), and the register's F1 and F2 entries
      are marked closed by it; F3 and F4 are untouched.
