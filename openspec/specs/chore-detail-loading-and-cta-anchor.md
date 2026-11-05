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
| `web/src/components/EventDetailRoute.tsx` | **new**, eager: page frame, back link, local `ErrorBoundary`, one `Suspense` around the lazy `EventDetail` (the `lazy()` call moves here from `App.tsx`) |
| `web/src/components/EventDetailRoute.css` | **new**: `.event-detail-page` and `.event-detail-state`, moved verbatim from `EventDetail.css` |
| `web/src/components/EventDetailRoute.test.tsx` | **new**: the single-region, node-identity, focus and error-routing tests (§1) |
| `web/src/pages/EventDetail.tsx` | `useQuery` → `useSuspenseQuery`; the pending and error branches and the page frame/back link are removed; it renders the event only |
| `web/src/pages/EventDetail.css` | loses the two moved rules |
| `web/src/pages/EventDetail.test.tsx` | renders `EventDetail` inside a `Suspense` with a resolved query; the pending/not-found cases move to `EventDetailRoute.test.tsx` |
| `web/src/App.tsx` | `/events/:id` route element → `<EventDetailRoute />`; `DashboardFallback`'s reservation `div` gets `id="dashboard"` and the first-load scroll effect (§2) |
| `web/src/App.css` | the scroll-margin rule gains a `#dashboard` selector |
| `web/src/App.test.tsx` | with the dashboard mocked as mounted, exactly one `#dashboard`, and it is the section |
| `web/src/App.fallback.test.tsx` | the CTA's `href` resolves to the reservation while the chunk is pending; the first-load scroll's unit cases (§2) |
| `scripts/bench-dashboard-cls/capture-fallback.mjs` | chunk-mounted guard → `section#dashboard.dashboard`; new `CLICK_CTA`, `DEEP_LINK` and `EVENT_DETAIL` modes (§2, §3) |
| `scripts/bench-dashboard-cls/README.md` | the new modes, the two-arm protocol, and the CLS results table |

No change to `LoadingState` or to `EventsDashboard`.

## 1. The event page: one suspension, one fallback (F1)

**Today**, two separate handoffs:

```tsx
// App.tsx: wait 1, while the EventDetail chunk downloads
<Suspense fallback={<div className="container section">Loading event telemetry...</div>}>
  <EventDetail />
</Suspense>

// EventDetail.tsx: wait 2, while useQuery is pending
if (isPending) return (
  <div className="event-detail-page"><div className="container section">
    <Link to="/" className="back-link">← Back to Sentinel Dashboard</Link>
    <div className="event-detail-state">Loading event telemetry...</div>
  </div></div>
)
```

React does not reuse DOM across a Suspense handoff: when the content resolves, the fallback is
unmounted and the content mounted. Rendering the same-looking card in both places would still
restart the spinner, remove a focused back link, and mount a second live region.

**Design.** `components/EventDetailRoute.tsx`:

```tsx
const EventDetail = lazy(async () => ({ default: (await import('../pages/EventDetail')).EventDetail }))

export function EventDetailRoute() {
  const { id = '' } = useParams<{ id: string }>()
  return (
    <div className="event-detail-page">
      <div className="container">
        <Link to="/" className="back-link">← Back to Sentinel Dashboard</Link>
        <ErrorBoundary resetKeys={[id]} fallbackRender={EventNotFoundOrRethrow}>
          <Suspense key={id} fallback={<LoadingState message="Loading event telemetry..." />}>
            <EventDetail />
          </Suspense>
        </ErrorBoundary>
      </div>
    </div>
  )
}
```

and `EventDetail` reads the event with
`useSuspenseQuery({ queryKey: eventKeys.detail(id), queryFn: () => fetchEventById(id) })`.

- **One suspension.** On a cold open the boundary suspends on the lazy chunk and shows its
  fallback. When the chunk resolves, `EventDetail` renders and suspends on the query; the
  boundary is still in its fallback state, so React keeps the **committed** fallback instead of
  remounting it. The fallback (and its `role="status"` region and spinner) is therefore mounted
  once, from the first frame until the event renders. **This is the claim the tests must prove**
  (node identity across the chunk resolving); it rests on React keeping a committed fallback while
  a boundary re-suspends, not on an assumption about timing.
  *Probed 2026-10-23* in vitest/jsdom against this repo's React 19.2.8 and TanStack Query 5.99.0:
  with a held `lazy()` and a held `useSuspenseQuery`, the status node was the same object before
  and after the chunk resolved, there was one status region throughout, and an outside link kept
  focus. A control using today's shape (the inner component rendering its own pending card with
  `useQuery`) got a **different** node, so the probe distinguishes the two. jsdom is not a
  browser, which is why the `EVENT_DETAIL=1` bench criterion stays.
- **Warm paths.** Chunk cached, data not: one suspension on the query, same fallback. Data
  cached (revisiting an event): no suspension, no wait, nothing to announce.
- **The frame and back link never unmount.** They sit outside the boundary, so keyboard focus on
  the back link survives the whole wait.
- **`key={id}` on the `Suspense`.** Moving to a different event remounts the boundary, so it
  shows the fallback for the new event instead of holding the previous one on screen, and
  `resetKeys={[id]}` clears a previous not-found.
- **Error routing.** `fetchEventById` throws `ApiError` for any non-OK response, which
  `useSuspenseQuery` rethrows. `EventNotFoundOrRethrow` renders the existing card
  (`<div className="event-detail-state">Event not found in Command Center.</div>`) for an
  `ApiError`, and **rethrows anything else** (a failed chunk import, a render error) so it reaches
  the app-level `ErrorBoundary` and `PageError`, which is where those go today.
- **`useParams` and `enabled`.** The route only matches with an `:id`, so `id` is always
  present; `useSuspenseQuery` has no `enabled` option and needs none.
- **Styles.** `.event-detail-page` and `.event-detail-state` move verbatim to
  `EventDetailRoute.css`, imported by `EventDetailRoute.tsx` (co-location, `developers-react.md`
  §1.5). That file loads with the main bundle, so the frame renders correctly before the chunk
  arrives. The classes keep their `event-detail-` names: they style the event page, of which
  this component is the eager half, and renaming them is churn with no behaviour change. The
  frame uses `container`; `.section` has no rule of its own outside the dashboard, so dropping
  it from the old loading wrapper changes nothing visible.
- **Spacing.** `.loading-state` has `margin-top: 2rem` where `.event-detail-state` has `1.5rem`.
  The component's own stylesheet is applied as-is; nothing below the card is reserved, so the 8px
  moves nothing else.
- **Markup, not speech.** The tests prove one polite `role="status"` region whose visible text is
  the message (`developers-react.md` §9.8). Whether a screen reader speaks a live region that
  mounts already containing its text varies by product; a manual check is recommended, not
  gated.
- **The event's arrival is not announced.** When the event renders, the fallback unmounts and
  the content appears, with no live region and no focus move. That falls short of §9.7 (dynamic
  updates are announced via `aria-live` or focus movement), and it is the same for every route
  change in the app, so it is register item **F4**, not part of F1.

## 2. Hero CTA anchor during the chunk load (F2)

**Today:** `DashboardFallback` renders the fixed progress bar and
`<div className="container section dashboard-fallback">` around `LoadingState`. The `#dashboard`
`<section>` exists only once `EventsDashboard` mounts.

**Design, the click:** the reservation `div` carries `id="dashboard"`:

```tsx
<div id="dashboard" className="container section dashboard-fallback">
  <LoadingState message="Loading dashboard telemetry..." />
</div>
```

- **Which element.** The reservation, not the progress bar (fixed, zero-footprint) and not the
  `LoadingState` card (its `margin-top` would land the scroll 32px short). The reservation's top
  is where `section#dashboard` will be.
- **Scroll margin (an `App.css` change).** `App.css` gives `section[id]`, `h2[id]` and
  `#dashboard-heading` `scroll-margin-top: 88px`, which clears the sticky nav (its bottom edge
  measured 73.75px at 375×812 and 81.75px at 1350×940). The fallback is a `div`, so the rule
  gains a `#dashboard` selector.
- **Uniqueness.** React unmounts the fallback before mounting the dashboard, and the chunk loads
  once, so `#dashboard` is never on two elements.
- **`:target` is not relied on.** The fallback matches `:target` while it exists, but the mounted
  section does not inherit it after the swap, and nothing in the app styles `:target`.

**Design, direct `/#dashboard` loads.** The HTML Standard's "try to scroll to the fragment"
aborts once the parser has stopped; non-async module scripts run only after that, and
`web/index.html` loads the app as one. So no target exists during the retries, and Chromium 156
was observed leaving `scrollY` at 0 with the chunk held, with no retry after release.
`DashboardFallback` therefore runs a first-load scroll in an effect:

1. **Once per document.** A module-level flag makes it run at most once, including under
   StrictMode's double-invoked effects.
2. **Only on a fresh navigation.** It proceeds only if
   `performance.getEntriesByType('navigation')[0]?.type === 'navigate'`. On `reload` and
   `back_forward` the browser restores the user's own position, and overriding it would be wrong.
3. **Only for this hash.** It proceeds only if `location.hash === '#dashboard'`.
4. **Judged by the outcome.** In the next animation frame it reads the target's
   `getBoundingClientRect().top`. If that is within ±4px of the target's computed
   `scroll-margin-top` (the browser already landed it), it does nothing; otherwise it calls
   `document.getElementById('dashboard')?.scrollIntoView?.()`. The optional call is because jsdom
   lacks `scrollIntoView`, the same guard `Select.tsx` uses. `scrollIntoView` honours the scroll
   margin, and `html`'s `scroll-behavior` (smooth, or `auto` under reduced motion) applies.
5. **When it runs.** The fallback mounts at the app's first commit, immediately after the module
   script runs, before the user can have scrolled, so there is no user position to protect.
   Client-side navigation back to `/` mounts no new fallback once the chunk is loaded, and the
   flag stops a re-run in any case.

**Why not scroll-on-mount for the click.** A tap would still do nothing at tap time, and the page
would then move on its own seconds later. The first-load scroll is only for direct loads, where
no tap has happened.

## 3. Bench script changes (`capture-fallback.mjs`)

1. `chunkMounted` becomes `document.querySelector('section#dashboard.dashboard') !== null`,
   because the fallback now also matches `#dashboard`.
2. **`CLICK_CTA=1`** (default on). With the dashboard chunk held, click `#hero-explore-cta`, let
   scrolling settle (motion forced to `auto` via `page.emulateMedia({ reducedMotion: 'reduce' })`),
   and report `window.scrollY`, the reservation's rectangle and the nav's bottom edge.
   `CLICK_CTA=0` reproduces the pre-existing #283 report on its own.
3. **`DEEP_LINK=1`.** Three cases, each with the chunk held: (a) a fresh `page.goto('/#dashboard')`,
   once with reduced motion and once without (waiting for smooth scrolling to settle); (b) scroll
   to y = 400, then `page.reload()`; (c) navigate away to `/for-partners`, scroll, and `page.goBack()`.
   Report the reservation's rectangle and `scrollY` for each.
4. **Release.** Release the held chunk, wait for `section#dashboard.dashboard`, and report the
   mounted section's rectangle, the `#dashboard-heading` rectangle and `scrollY`.
5. **`EVENT_DETAIL=1`.** Serve a fixture event for `GET /v1/events/<fixture-id>` via
   `page.route`, holding both the `EventDetail-*.js` chunk and that API response. Open
   `/events/<fixture-id>`, tag the `role="status"` element (`data-bench-id`), focus the back link,
   and record the card's rectangle. Release the chunk; after 500ms confirm the same tagged element
   is still attached, the back link still has focus, and the card's top is unchanged. Release the
   API response and confirm the event's `h1` renders and no `role="status"` remains.
6. The exit code stays non-zero on any null rectangle or on a chunk mounting before release.

**Protocol (README):** two arms, control (the `origin/development` SHA the branch is based on,
recorded) and branch, at 375×812 and 1350×940.

## Acceptance criteria

### F1

- [ ] `EventDetailRoute.test.tsx`, with the lazy import and `fetchEventById` both held: exactly
      one `role="status"` region (polite, name "Loading status", text "Loading event
      telemetry..."), with the back link above it.
- [ ] Same test: resolve the lazy import with the query still held. The status region is the
      **same node** as before (`toBe`), there is still exactly one, and a back link focused
      beforehand still has focus. Then resolve the query: the event's `h1` renders, no
      `role="status"` remains, and the back link is the same node throughout.
- [ ] `fetchEventById` rejecting with `ApiError` (404): the not-found card renders inside the
      frame, with no `role="status"`. A non-`ApiError` (e.g. the lazy import rejecting) is **not**
      caught by the local boundary; it reaches the app-level `ErrorBoundary`.
- [ ] Changing `:id` after a not-found shows the new event's wait and then the event.
- [ ] axe reports no violations in the waiting state and in the not-found state.
- [ ] `EventDetailRoute.tsx` imports nothing from `web/src/pages/` except through `lazy()`, and
      the production build still emits `EventDetail` as its own JS and CSS chunks.
- [ ] Bench `EVENT_DETAIL=1`, both viewports: the tagged status element is still attached after
      the chunk is released, the back link keeps focus, and the card's top is unchanged (±2px).
- [ ] No document claims the event's arrival is announced. Register F4 exists.

### F2

- [ ] With the dashboard chunk held, `document.querySelector(cta.getAttribute('href'))` is the
      `.dashboard-fallback` element, and no other element has `id="dashboard"`.
- [ ] `App.test.tsx`, dashboard mocked as mounted: exactly one element has `id="dashboard"`, and
      it is the dashboard section.
- [ ] Unit (`App.fallback.test.tsx`), first-load scroll: called for `navigate` + `#dashboard` +
      target off its landing; **not** called for `reload`, for `back_forward`, without the hash,
      when the target already sits at its scroll margin, or on a second mount.
- [ ] Bench, click: on the **control**, `scrollY` is 0 (the positive control); on the **branch**,
      the reservation's top is 88px (±2px) from the viewport top at both sizes, and at or below
      the nav's bottom edge.
- [ ] Bench, direct load (a), both motion settings: the reservation's top is 88px (±2px).
      Cases (b) and (c): `scrollY` is the restored position (±2px), not the reservation.
- [ ] After release, branch: the mounted section's top equals the reservation's pre-release top
      (±2px), `scrollY` is unchanged (±2px), and `#dashboard-heading` is in the viewport and at or
      below the nav's bottom edge.
- [ ] `capture-fallback.mjs` with `CLICK_CTA=0` produces the #283 report unchanged
      (bar at y 0–3 at 375×812).

### CLS

- [ ] `measure-cls.mjs`, 8 runs per cell at 1920×1600, 1350×940, 768×1024 and 375×812, control
      SHA recorded. The README gains a table with both arms' printed per-run values and each
      cell's median (computed by hand from the printed values; the script prints runs, not
      medians). For every cell, the branch median is no more than the control median + 0.001.

### Both

- [ ] `npm run lint`, `npm run type-check`, `npm run lint:styles`, `npm run test`,
      `npm run build` clean.
- [ ] The implementation PR updates this record (sentinel), and the register's F1 and F2 entries
      are marked closed by it; F3 and F4 are untouched.
