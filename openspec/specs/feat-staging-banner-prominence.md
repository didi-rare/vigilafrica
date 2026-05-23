---
id: feat-staging-banner-prominence
status: proposed
branch: feat/staging-banner-prominence
---

# Spec: Make Staging Banner More Visible (feat-staging-banner-prominence)

## Context

`fix-staging-vite-env-flag` (PR #86) wired `VITE_ENV=staging` through the Vercel build so the `<StagingBanner>` component in [web/src/App.tsx:90-102](web/src/App.tsx#L90-L102) renders on `staging.vigilafrica.org`. Browser confirmation on 2026-05-23 showed the banner appears, but reads as a thin amber text strip — easy to miss. This spec adds visual weight without changing the banner's gate, copy, or position.

Companion: [openspec/proposals/feat-staging-banner-prominence.md](openspec/proposals/feat-staging-banner-prominence.md).

## Decision Log

| # | Decision | Alternatives | Why |
|---|---|---|---|
| D1 | Left stripe rendered via `::before` pseudo-element | Render via `border-left` on the banner itself | `::before` is independently animatable without dragging the whole banner. Also keeps the existing `border-bottom` rule structurally separate |
| D2 | Pulse animates `box-shadow` on the stripe (`::before`), not the whole banner | Pulse the banner background or border colour | Pulsing the background would be noisy across the full banner width. Localising the pulse to the stripe gives a single focal point. `box-shadow` is GPU-accelerated and doesn't trigger layout |
| D3 | `AlertTriangle` icon from `lucide-react` | `FlaskConical`, `TestTube`, `ShieldAlert`, custom SVG | `AlertTriangle` is the canonical "warning notice" icon, semantically matches the banner's role. `ShieldAlert` is already used elsewhere in App.tsx for the audience-card "Civic Responders"; reusing it would dilute the meaning |
| D4 | Animation duration: 2.5s ease-in-out | 1s (too distracting), 5s (barely noticeable) | 2.5s slow enough to be ambient, fast enough that a first-time visitor catches it on initial paint |
| D5 | Respect `prefers-reduced-motion` | Always animate | A11y baseline. Project follows `developers-react.md` §9 which mandates this |
| D6 | Amber palette unchanged | Switch to red / orange for higher alarm | Amber matches the existing token system + the staging colour identity. The goal is "you can't miss it", not "panic". Existing tokens `--accent-amber`, `--surface-amber-*`, `--border-amber-*` cover everything needed |

## Components to Touch

### Modified files

| File | Change |
|---|---|
| [web/src/App.tsx](web/src/App.tsx) | Import `AlertTriangle` from `lucide-react`; insert it inside the `<div className="staging-banner">` before the text, with `aria-hidden="true"` |
| [web/src/App.css](web/src/App.css) | Update `.staging-banner` rule (currently line 147-156) to use flex layout for icon+text alignment; add `::before` pseudo-element for the stripe; add `@keyframes staging-banner-stripe-pulse`; add `prefers-reduced-motion` override |

### Untouched

- The `StagingBanner` gate (`if (import.meta.env.VITE_ENV !== 'staging') return null`) — unchanged
- All other components, routes, pages, and styles
- Backend code — N/A
- `web/src/styles/tokens.css` — all required tokens already exist (`--accent-amber`, `--surface-amber-strong`, etc.)
- No new npm dependencies — `lucide-react` is already in `web/package.json`

## Behaviour Contract

- **B1** — When `VITE_ENV=staging`, the banner renders with a 4px-wide vertical amber stripe on its left edge, an `AlertTriangle` icon, and the existing copy
- **B2** — The stripe MUST pulse continuously via a 2.5s ease-in-out box-shadow animation, low-amplitude (peak inset-shadow ≤ 12px spread, baseline 0)
- **B3** — When the user's OS or browser reports `prefers-reduced-motion: reduce`, the pulse animation MUST be disabled. The stripe MUST still render as a static amber bar
- **B4** — When `VITE_ENV !== 'staging'`, no banner DOM is rendered at all (unchanged from current behaviour)
- **B5** — The icon MUST have `aria-hidden="true"` so screen readers do not announce it as separate from the textual notice (the `<div role="note" aria-label="Test environment notice">` wrapper already carries the semantics)
- **B6** — Layout MUST NOT shift on the production deploy (no banner DOM, no reserved space). On staging, the banner sits in the same position it does today — above the `<nav>`, full width
- **B7** — `npm run lint:styles` (stylelint with declaration-strict-value) MUST pass — no hardcoded colour literals in App.css

## Phase 1 — Implementation

- [ ] Add `AlertTriangle` to the lucide-react import block in [App.tsx](web/src/App.tsx)
- [ ] Restructure `<StagingBanner>` JSX to wrap the icon + text (icon first, `aria-hidden`)
- [ ] Update `.staging-banner` rule in [App.css](web/src/App.css) — add `position: relative`, `display: flex`, `align-items: center`, `justify-content: center`, `gap: 8px`, adjust padding-left for stripe room
- [ ] Add `.staging-banner::before` pseudo-element for the stripe
- [ ] Add `@keyframes staging-banner-stripe-pulse`
- [ ] Add `@media (prefers-reduced-motion: reduce) { .staging-banner::before { animation: none; } }`

## Phase 2 — Verification

- [ ] `npm run build` succeeds (Vite TS pipeline + stylelint)
- [ ] `npm run test` — existing App.tsx accessibility tests pass (the `aria-allowed-role` axe test in App.test.tsx must remain green)
- [ ] Local dev verification: run `npm run web:dev` with `VITE_ENV=staging` → confirm stripe + icon + pulse render
- [ ] Local dev verification with OS reduced-motion enabled (or DevTools "Emulate prefers-reduced-motion: reduce") → confirm pulse stops, stripe stays
- [ ] After staging deploy: open `https://staging.vigilafrica.org/` → confirm visible stripe + icon + animated pulse
- [ ] After staging deploy: `curl -s https://staging.vigilafrica.org/` → `<meta name="robots" content="noindex, nofollow">` unchanged
- [ ] After staging deploy: `https://vigilafrica.org/` shows no banner (production unchanged)

## Acceptance Criteria

- [ ] B1-B7 of the Behaviour Contract verified locally
- [ ] `npm run build` and `npm run test` pass
- [ ] `npm run lint:styles` passes — no hardcoded colour literals
- [ ] Stylesheet additions reference only existing semantic tokens from `web/src/styles/tokens.css`
- [ ] Browser-confirmed on staging post-deploy: stripe visible, icon present, pulse smooth, banner remains unintrusive (no full-screen takeover, no flashing)

## Out of Scope (reaffirmed)

- Changing banner copy, position, or visibility gate
- Adding new colour tokens — all required tokens already exist
- A separate banner for `local` / `demo` environments
- Sticky/fixed positioning behaviour
- Different banner for `staging` vs `preview` Vercel environments

## Risks

- **R1 — Pulse becomes annoying on prolonged sessions**: a 2.5s loop is roughly 24 cycles per minute. Mitigation: amplitude is bounded to a soft box-shadow (no opacity/color flicker), and `prefers-reduced-motion` opt-out is honoured. If user testing flags this, easy follow-up is to make the pulse fire only for the first ~30 seconds via animation-iteration-count
- **R2 — Layout regression on narrow viewports**: adding `display: flex` could affect 375px rendering. Mitigation: `npm run build` + manual viewport check at 375 / 768 / 1280 before merging
- **R3 — stylelint rejects a new colour literal**: project has `declaration-strict-value` enforcing tokens. Mitigation: spec D6 mandates token-only usage; this is also verified by `npm run lint:styles` in Phase 2
- **R4 — Accessibility regression**: icon could be picked up by screen readers as separate content. Mitigation: B5 — `aria-hidden="true"` on the icon; the wrapper `<div role="note" aria-label="…">` already carries the semantics

## Verification Plan

1. Local dev: `VITE_ENV=staging npm run web:dev`, visually inspect at 375 / 1280 viewports
2. Local dev: toggle "Emulate prefers-reduced-motion" in DevTools → confirm pulse stops
3. CI: `npm run build`, `npm run test`, `npm run lint:styles` all green
4. Open PR to `development`; reviewer probes the local screenshot in the PR description
5. Post-merge → main: visual confirmation on `staging.vigilafrica.org`

No automated tests added — existing axe-based a11y tests in `App.test.tsx` provide the regression guard; the prominence change is a visual refinement that doesn't warrant a snapshot test.
