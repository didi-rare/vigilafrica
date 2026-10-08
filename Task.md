# chore-web-audit-leftovers

**Branch:** `claude/chore-web-audit-leftovers-xt5v6i`
**Proposal:** [openspec/proposals/chore-web-audit-leftovers.md](openspec/proposals/chore-web-audit-leftovers.md)
**Spec:** [openspec/specs/chore-web-audit-leftovers.md](openspec/specs/chore-web-audit-leftovers.md)
**Origin:** five findings accepted-not-fixed in the 2026-07-26 web-audit batch
(#189–#191, #193, #195, #197, #198), registered in #205

## 1. Staging banner glow off the main thread

- [x] 1.1 Stripe stays on `::before`; glow moves to `::after` with a static
      `box-shadow` and an `opacity` animation (`web/src/App.css`)
- [x] 1.2 `prefers-reduced-motion` rule moves to `::after` with it (the #191
      near-miss) and the resting `opacity: 0` is declared, not inherited from
      the first keyframe
- [x] 1.3 Lighthouse 13.5.0, `non-composited-animations`, local staging builds
      (`VITE_ENV=staging`, placeholder env), mobile **and** desktop presets:
      control (`origin/development` @ `3040a00`) → **1 animated element,
      `::before`, "Unsupported CSS Property: box-shadow"**; branch →
      **notApplicable (0 elements)**. The control run is what proves the audit
      sees the defect; a 0 alone would not.
- [x] 1.4 `check-reduced-motion.mjs` on the staging build: under `reduce` the
      glow reads `animation=none … opacity=0`; under `no-preference` it reads
      `staging-banner-stripe-pulse ×infinite 2.5s`. Peak-frame crops of both
      arms (animations paused at t=1250ms) show the same glow extent.

## 2. Synthetic user-agent regex

- [x] 2.1 **Accept and document** — regex unchanged. Recorded in the spec §2
      and at the regex in `analytics.ts`: anchoring does not address the
      described failure (an appended proxy token is a whole token either
      way), a log-only path would be a seventh event the module deliberately
      refuses, and the rate cannot be measured from this repo (Umami stores a
      parsed browser name, not the raw UA).
- [x] 2.2 `analytics.test.ts`: a real Chrome UA with an appended
      `CorpProxy/3.1 (diag; lighthouse-audit-policy)` token is still recorded.

## 3 + 5. One announced loading treatment

- [x] 3.1 `components/LoadingState.tsx`: `role="status"`, `aria-live="polite"`,
      `aria-hidden` spinner, visible message as the live text; no `aria-busy`
      on an ancestor (would defer the announcement — spec §3)
- [x] 3.2 Used by `DashboardFallback` (App.tsx), the `eventsLoading` state and
      the map Suspense fallback (EventsDashboard.tsx)
- [x] 3.3 `.dashboard-state`, `.spinner`, `@keyframes spin` and the ≤768px
      padding override moved from `EventsDashboard.css` (lazy chunk CSS) to
      `App.css` (eager) — confirmed by `grep` on the built `index-*.css`
- [x] 3.4 `LoadingState.test.tsx` (3 cases, axe clean) and a new
      `EventsDashboard.test.tsx` case that holds the fetch open, asserts the
      status region + text + decorative spinner, runs axe, then resolves and
      asserts the region is gone. 102/102 tests.

## 4. Above-the-fold affordance at phone widths

- [x] 4.1 `.load-progress`: fixed 3px bar, `--z-load-progress: 300`,
      `transform`-only segment, static full-width under reduced motion
      (`check-reduced-motion.mjs`: `animation=none … width=1350px`)
- [x] 4.2 `capture-fallback.mjs` at 375×812, chunk held back: bar at
      y 0–3 (**in view**), card at y 1064–1244 (below the fold, as before);
      control arm has no bar and a 26px text-only fallback at y 1032.
      Screenshot confirms the amber bar across the top of the hero.

## Verification

- [x] V1 `npm run lint` / `type-check` / `lint:styles` / `test` / `build` —
      all clean, 102/102
- [x] V2 CLS A/B against the control build (`measure-cls.mjs`, new `VIEWPORT`
      override, 8 runs/cell): control = branch to four decimals in every cell —
      1920×1600 0.0054/0.0054, 1350×940 0.0001/0.0001, 768×1024 0.0002/0.0002,
      375×812 0.0003/0.0003. Table in the proposal's Verification section.
- [x] V3 Proposal updated with the Resolution and verification results;
      status `in-progress` — ready for `/openspec-review`

## Deliberately not done (recorded in spec "Out of scope")

- `EventDetail.tsx` loading region — same defect class, not in the proposal
- `#dashboard` anchor missing while the chunk loads — backlog candidate
- Lighthouse on the **deployed** staging URL — nothing in this repo deploys;
  the local staging build is the proxy, with the control run as the positive
  control
