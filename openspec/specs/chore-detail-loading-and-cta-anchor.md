---
id: chore-detail-loading-and-cta-anchor
status: proposed
proposal: ../proposals/chore-detail-loading-and-cta-anchor.md
branch: claude/chore-web-audit-leftovers-xt5v6i
---

# Spec: EventDetail Loading Region and the Hero CTA Anchor

Technical spec for [`chore-detail-loading-and-cta-anchor`](../proposals/chore-detail-loading-and-cta-anchor.md).
Two independent items in `web/src/`; both reuse what #283 built and add no CSS.

## Components touched

| file | change |
|---|---|
| `web/src/pages/EventDetail.tsx` | pending branch renders `LoadingState`; `.event-detail-state` kept for the not-found card |
| `web/src/pages/EventDetail.test.tsx` | pending-state test (status region, message, axe); not-found card asserted as not a status region |
| `web/src/pages/EventDetail.css` | comment on `.event-detail-state` updated: it is now the not-found card only |
| `web/src/App.tsx` | `DashboardFallback`'s reservation `div` gets `id="dashboard"`; the comment block records why |
| `web/src/App.fallback.test.tsx` | the CTA's `href` resolves to the reservation while the chunk is pending |
| `scripts/bench-dashboard-cls/capture-fallback.mjs` | chunk-mounted check moves to `section#dashboard.dashboard`; new CTA-click and post-release measurements |
| `scripts/bench-dashboard-cls/README.md` | the new measurement and its two-arm protocol |

No change to `LoadingState`, to any stylesheet's rules, or to `EventsDashboard`.

## 1. EventDetail pending branch (F1)

**Today:**

```tsx
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

**Design:** the inner `div` becomes `<LoadingState message="Loading event telemetry..." />`.
Everything around it stays: the page shell, the back link above the region. The message text is
unchanged so the wait reads the same as before to a sighted user, now with the spinner.

- `announce` is left at its default (`true`): this is the page's only wait at that moment, so
  there is nothing to double-announce. Leaving the state is announced by what replaces it: the
  `h1` and the `role="note"` disclaimer on success (a page-level content change, not a live
  region; the same convention the dashboard uses for the map's arrival), or the not-found card.
- The not-found card is **not** a status region and stays as it is. It is an end state, not a
  wait, and the proposal scopes F1 to the loading branch.
- `.loading-state` has `margin-top: 2rem` where `.event-detail-state` has `1.5rem`, and `5rem`
  padding where the card had `5rem 2rem`. That is the one treatment applied as-is (§7.3: no
  page-specific override of a component's own stylesheet); the detail page does not reserve
  height for this card, so the 8px difference moves nothing else.

Standards: `developers-react.md` §9.8 (a loading state is a `role="status"` region whose visible
text is the announcement; a spinner beside text is `aria-hidden`), already satisfied by the
component.

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

- **Which element.** The reservation, not the progress bar (fixed, zero-footprint, so a scroll
  to it would go nowhere) and not the `LoadingState` card (it has a `margin-top` that would
  land the scroll 32px short of where the mounted section starts). The reservation's top is
  where `section#dashboard` will be.
- **Scroll margin.** `App.css` gives `section[id]`, `h2[id]` and `#dashboard-heading` a
  `scroll-margin-top: 88px` so anchored targets clear the fixed nav. The fallback is a `div`,
  so it is not matched by `section[id]`; the rule gains `#dashboard` as a selector, which
  matches both the fallback and, redundantly, the mounted section. That is a selector addition,
  not a new value, so no token is involved.
- **Uniqueness.** React removes the Suspense fallback before committing the lazy subtree, and
  `App.tsx` lazy-loads the chunk once per page, so there is never a moment with two
  `#dashboard` elements. `App.test.tsx` already renders the mocked dashboard and asserts the
  CTA's `href`; `App.fallback.test.tsx` holds the chunk and asserts the target resolves. Together
  they pin both halves.
- **Deep links.** `/#dashboard` on first load now scrolls to the reservation before the chunk
  arrives. Nothing is added for this; it falls out of the same line and is asserted only by the
  bench script (jsdom does not scroll).
- **Why not scroll-on-mount.** Recorded in the proposal: the tap would still do nothing at tap
  time; the page would move later on its own; and it needs an effect, a hash read and a
  `scrollIntoView` guard where the id needs nothing.

**Bench script changes** (`capture-fallback.mjs`):

1. `chunkMounted` becomes `document.querySelector('section#dashboard.dashboard') !== null`,
   because the fallback now also matches `#dashboard`. Without this the script's own guard would
   be satisfied by the fallback and could never fire.
2. After the existing viewport report, the script clicks `#hero-explore-cta`, waits for scrolling
   to settle (`scroll-behavior: smooth` is on `html`; the page is forced to `auto` for the
   measurement via `page.emulateMedia({ reducedMotion: 'reduce' })`, which the existing
   reduced-motion rule honours), and reports `window.scrollY`, the reservation's rectangle and
   whether the `role="status"` region is inside the viewport.
3. It then releases the held chunk route (the route handler resolves to `continue()`), waits
   for `section#dashboard.dashboard`, and reports the `#dashboard-heading` rectangle and
   `window.scrollY` again, so the post-mount landing is measured.
4. The exit code stays non-zero on any null rectangle or on the chunk having mounted before
   release. A new `CLICK_CTA=0` switch keeps the pre-existing measurement reproducible on its own.

The two-arm protocol in the README: control (`origin/development`) and branch, both at 375×812
and 1350×940. The expected control result after the click is `scrollY: 0` (the tap did nothing),
which is the positive control that the measurement sees the defect.

## Acceptance criteria

- [ ] With `fetchEventById` held pending, `EventDetail` renders a `role="status"` region with
      `aria-live="polite"`, accessible name "Loading status", and text
      "Loading event telemetry..."; the back link is still present above it.
- [ ] With `fetchEventById` rejecting, the not-found card renders with no `role="status"` in the
      document.
- [ ] axe reports no violations in the pending state and in the not-found state.
- [ ] With the dashboard chunk held, `document.querySelector(cta.getAttribute('href'))` is the
      `.dashboard-fallback` element, and no other element has `id="dashboard"`.
- [ ] With the dashboard chunk mocked as mounted (`App.test.tsx`), exactly one element has
      `id="dashboard"` and it is the dashboard section.
- [ ] `capture-fallback.mjs` on the branch, chunk held, after the CTA click: the reservation's
      top is within the viewport at 375×812 and 1350×940; on the control build `scrollY` is 0.
- [ ] `capture-fallback.mjs` on the branch after release: `#dashboard-heading` is within the
      viewport at both sizes, and `scrollY` did not change by more than the heading's own
      `scroll-margin-top` between the pre-release and post-release reports.
- [ ] `measure-cls.mjs` against the control arm shows no CLS change at 1350×940 and 768×1024.
- [ ] `capture-fallback.mjs` with `CLICK_CTA=0` produces the #283 report unchanged
      (bar at y 0–3 at 375×812).
- [ ] `npm run lint`, `npm run type-check`, `npm run lint:styles`, `npm run test`,
      `npm run build` clean.
- [ ] The register's F1 and F2 entries are marked closed by this record; F3 is untouched.
