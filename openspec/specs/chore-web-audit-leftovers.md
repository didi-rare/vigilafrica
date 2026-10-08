---
id: chore-web-audit-leftovers
status: in-progress
proposal: ../proposals/chore-web-audit-leftovers.md
branch: claude/chore-web-audit-leftovers-xt5v6i
---

# Spec: Close the Accepted Leftovers From the Web-Audit Batch

Technical spec for [`chore-web-audit-leftovers`](../proposals/chore-web-audit-leftovers.md).
Five findings, all in `web/src/`, all small. Items 3, 4 and 5 share one mechanism
(a single loading component used by every loading region), so they are designed
together below; items 1 and 2 are independent.

## Components touched

| file | change |
|---|---|
| `web/src/App.css` | staging-banner glow moves to a `::after` layer animating `opacity`; `.dashboard-state`, `.spinner`, `@keyframes spin` move here from `EventsDashboard.css` so the eagerly-loaded fallback can use them; new `.load-progress` bar; reduced-motion rules follow the animations |
| `web/src/App.tsx` | Suspense fallback becomes `DashboardFallback`: the progress bar + the shared `LoadingState` |
| `web/src/components/LoadingState.tsx` | **new** — the one loading treatment: `role="status"` live region, decorative spinner, visible message |
| `web/src/components/EventsDashboard.tsx` | both inner loading regions (`eventsLoading`, the map Suspense fallback) render `LoadingState` |
| `web/src/components/EventsDashboard.css` | `.dashboard-state`, `.spinner`, `@keyframes spin` and the ≤768px `.dashboard-state` override removed (moved, not deleted) |
| `web/src/analytics.ts` | comment only — records the item-2 decision at the regex |
| tests | `LoadingState.test.tsx` (new), `EventsDashboard.test.tsx`, `analytics.test.ts` |

## 1. Staging banner: compositor-driven glow

**Today:** `.staging-banner::before` is the 4px amber stripe and animates
`box-shadow` from nothing to `0 0 12px 1px var(--accent-amber)`. `box-shadow` is
not a compositable property; Lighthouse flags it on staging.

**Design:** the stripe stays on `::before`, unanimated. A second pseudo-element,
`.staging-banner::after`, occupies the same 4px box with a transparent fill and
a **static** `box-shadow: 0 0 12px 1px var(--accent-amber)`, and animates only
`opacity` 0 → 1 → 0 on the same 2.5s ease-in-out cycle. The shadow is painted
once into the layer and the compositor fades the layer, which is the same
mechanism #191 used for `.signal-dot::after` (there with `transform`; here the
shadow does not need to grow, so `opacity` alone is enough).

Visual result: identical glow extent (12px blur, 1px spread) at the 50%
keyframe; between keyframes the glow fades rather than shrinks. Nobody will
tell the difference at 2.5s, and the stripe itself no longer repaints.

**The regression #191 nearly shipped:** the `prefers-reduced-motion` rule
currently targets `.staging-banner::before`. The animation is leaving that
element, so the rule must move to `.staging-banner::after` **and** also set
`opacity: 0`, because the glow layer's resting state is `opacity: 0` only via
the animation's first keyframe; with `animation: none` it would otherwise fall
back to the declared `opacity`, so the declaration sets `opacity: 0` and the
animation lifts it. Verified by reading the computed style under reduced
motion, not by assuming the catch-all `*::after` block covers it (it would,
but that is rule-order luck, as #191 recorded).

## 2. Synthetic user-agent regex: measured as far as it can be, then kept

The proposal's instruction is **measure before changing**. This repo cannot
measure live traffic: the Umami instance stores a parsed browser name, not the
raw user-agent string, and deploy or analytics credentials are never requested
for development work. So the measurement available here is structural, and it
is recorded rather than skipped:

- The three tokens are product identifiers, not words. `Chrome-Lighthouse` is
  emitted by Lighthouse and PageSpeed Insights at the end of their UA strings;
  `HeadlessChrome/<version>` replaces the `Chrome/<version>` product token in
  headless Chromium; `PageSpeed` has no known occurrence in a real browser's UA.
- The failure the reviewers described, a corporate proxy appending diagnostic
  tokens, produces *whole* tokens. Anchoring the regex with word boundaries
  would not change the outcome for that case at all; it would only reject
  substrings inside other words, which no observed UA contains. Anchoring is
  therefore a change without a benefit, and the proposal's own rule says not to
  change what has not been measured.
- Log-without-suppressing would mean a seventh custom event. `analytics.ts`
  deliberately fixes the event set at six (over-instrumentation guard), and
  the proposal does not ask for one.

**Decision: accept and document.** The regex is unchanged. The comment at the
regex now states the known false-positive surface (any UA that carries one of
the three tokens verbatim), why it is believed negligible, and what a real
measurement would need (raw UA access on the analytics side, or a temporary
server-side UA sample from the Caddy access log, neither of which is a web
change). A test pins that a real browser UA with an appended proxy token is
still recorded, so the accepted behaviour is asserted rather than assumed.

## 3. Loading regions announce themselves

**Today:** `.dashboard-fallback` (Suspense boundary in `App.tsx`) and
`.dashboard-state.loading` (data fetch in `EventsDashboard.tsx`) are plain
`div`s. The map's own Suspense fallback on the same line is a third one.

**Design:** one component, `LoadingState`:

```tsx
<div className="dashboard-state loading" role="status" aria-live="polite">
  <span className="spinner" aria-hidden="true" />
  <p>{message}</p>
</div>
```

- `role="status"` + explicit `aria-live="polite"` is the pattern
  `FreshnessIndicator` and the result count already use in the same file
  (`developers-react.md` §9.7); explicit `aria-live` is redundant with the
  role's implicit value and kept for the same reason the existing code keeps
  it — older screen readers.
- The visible message *is* the live text. §9.8's `aria-label` on a bare spinner
  is for the case where there is no visible text; here there is, and labelling
  the spinner as well would announce the wait twice.
- `aria-busy` is **not** placed on an ancestor of the live region. `aria-busy`
  asks assistive technology to defer exposing changes under the busy element
  until it clears, which would suppress exactly the announcement item 3 exists
  to add. The §9.8 example puts `aria-busy` on the results container and the
  status inside it as a sibling of the results; this dashboard's loading state
  and results list are both children of `.dashboard-sidebar`, so there is no
  container to mark busy that does not also contain the live region.
- "Leaving" the loading state is announced by what replaces it: the result
  count live region ("Showing 1–50 of N") when data arrives, the `role="alert"`
  error card on failure. The Suspense boundary's exit is followed immediately
  by the inner loading state's entry, so a screen-reader user hears
  "Loading dashboard telemetry" → "Fetching satellite telemetry" → the count.
- Regions mount with their text already present, as `FreshnessIndicator` does.
  This is the project's established pattern; the alternative (an always-mounted
  empty region filled after mount) is more robust on some older VoiceOver
  builds but is not what the rest of the file does, and is recorded here as the
  thing to reach for if a real screen-reader test shows the mount-time
  announcement is missed.

## 4. A loading affordance above the fold at phone widths

**Today:** at 375×812 the fallback starts at y≈1098. Nothing above the fold
says anything is loading; #198 removed the reservation at ≤480px so the fallback
is one line of grey text that the user has to scroll to.

**Design:** `DashboardFallback` renders, alongside the in-flow fallback, a
`position: fixed` 3px progress bar across the top of the viewport
(`.load-progress`) for exactly as long as the dashboard chunk is pending. It is
`aria-hidden` (the live region carries the announcement), `pointer-events:
none`, sits at `--z-load-progress: 300` (above the sticky nav's 100 and the
staging banner, below the skip link's 1000), and takes **no layout space**, so
it cannot contribute to CLS at any viewport. The indeterminate motion is a 40%
segment translating across the track — `transform` only, compositor-driven, so
it does not reintroduce the defect item 1 removes.

Under `prefers-reduced-motion: reduce` the segment is replaced by a static,
full-width bar: still an affordance, no motion.

The bar is visible at every viewport, including desktop, where the reservation
already keeps the fallback text on screen. That is deliberate: a global
"something is loading" indicator plus a local "this is what is loading" region
is the conventional pairing, and keeping the bar unconditional means it needs
no viewport-sniffing logic.

## 5. One loading treatment for one wait

Reuse of `LoadingState` (item 3) in the outer fallback is the whole of item 5.
Two consequences:

- `.dashboard-state`, `.spinner` and `@keyframes spin` currently live in
  `EventsDashboard.css`, which Vite bundles **into the lazy chunk's CSS**. The
  outer fallback renders before that chunk exists, which is why it was plain
  text in the first place. They move to `App.css` (eager), following the
  precedent `EventDetail.css` records for `.back-link` ("shared across
  sub-pages and now lives in App.css"). The error-state-only rules
  (`.dashboard-state-detail*`, `.dashboard-retry-button`) stay where they are.
- `.dashboard-fallback` changes from a centred flex box to block flow with the
  card as its only child, which top-aligns the card the way the #195 comment
  requires without the flex alignment; the card spans the container width as
  it does inside the dashboard sidebar.

The outer card sits inside the existing reservation at ≥481px, so the measured
CLS behaviour of #193/#198 is unchanged by construction. It is re-measured
anyway (below) because "unchanged by construction" is the sentence that
preceded both regressions the CLS harness has caught so far.

## Out of scope (recorded, not forgotten)

- `EventDetail.tsx` has a fourth loading region (`.event-detail-state`,
  "Loading event telemetry…") with the same two defects. The proposal names the
  two dashboard regions; the detail page is one `LoadingState` away and is left
  for its own small change rather than widened into this one.
- The "Explore latest events" CTA targets `#dashboard`, which does not exist
  while the chunk is loading, so a tap during the wait does nothing. Related to
  item 4's user story but not in the proposal; recorded for the backlog.
- Everything the proposal lists as out of scope (CSP nonces, HSTS preload,
  `label-content-name-mismatch`).

## Acceptance criteria

1. No `@keyframes` in `web/src/` animates `box-shadow`; Lighthouse "Avoid
   non-composited animations" on a **staging** build reports 0 elements.
2. With `prefers-reduced-motion: reduce` emulated, the computed
   `animation-name` of `.staging-banner::after` is `none` and its opacity is 0.
3. Each of the three loading regions is a `role="status"` element whose
   accessible text is the visible loading message; axe reports no violations
   with any of them mounted.
4. At 375×812 with the dashboard chunk delayed, a screenshot taken before the
   chunk arrives shows the progress bar inside the viewport.
5. The outer fallback and the inner loading state render the same DOM shape
   (spinner + message inside `.dashboard-state`).
6. `npm run lint`, `type-check`, `lint:styles`, `test`, `build` clean.
7. CLS measured with `scripts/bench-dashboard-cls/measure-cls.mjs` against a
   control build of `origin/development` at 1920×1600, and additionally at
   1350×940, 768×1024 and 375×812: branch ≤ control in every cell.
