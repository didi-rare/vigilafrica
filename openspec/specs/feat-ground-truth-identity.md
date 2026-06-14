# Spec: Brand Identity Suite + Premium Polish Pass (feat-ground-truth-identity)

**Status:** Proposed — PR #120 (`feat-ground-truth-redesign`) is merged; implementation may begin.
**Companion:** [`openspec/proposals/feat-ground-truth-identity.md`](../proposals/feat-ground-truth-identity.md) (rationale, risks, scope boundaries).

## Context

Ground Truth (ADR-015) gave VigilAfrica its design language but a single
unexplored brand mark and zero identity assets: no OG/share image, a
pre-rebrand favicon, no README banner, and map markers that predate the
identity. This spec covers a five-concept logo selection, the asset suite
derived from the winner, and an audit-driven polish pass — in that order, with
a maintainer decision gate between each phase.

## Components to Touch

### New files

| Path | Purpose |
| --- | --- |
| `web/src/components/brand-concepts/` (temporary) | Five candidate marks as token-themed SVG components + a local-only concepts page for side-by-side review at 16/32/120/480px. Deleted after selection. |
| `web/public/favicon.svg` | Vector favicon from the chosen mark. |
| `web/public/favicon.ico` | Raster fallback (16/32/48), generated from the SVG. |
| `web/public/apple-touch-icon.png` | 180×180 touch icon. |
| `web/public/og-image.png` | 1200×630 share card (static; SVG source committed beside it). |
| `web/public/brand/og-image.svg` | Source-of-truth for the share card (regenerable). |
| `docs/screenshots/readme-banner.png` | README header banner (SVG source beside it). |

### Modified files

| Path | Change |
| --- | --- |
| `web/src/components/BrandMark.tsx` | Replaced by (or refined to) the winning mark; same props/class API so nav + tests keep working. |
| `web/index.html` | Favicon/touch-icon links; OG + Twitter meta (title, description, image, url). Staging `noindex` transform untouched. |
| `README.md` | Banner image at top. |
| `web/src/components/Map.tsx` | Marker SVG geometry aligned with the chosen mark (built via `createElementNS`, as today — no `innerHTML`). |
| `web/src/components/Map.css` / `tokens.css` | Marker styling only if geometry change requires it; `--marker-*` tokens preserved. |
| `web/src/App.test.tsx` / `Map.test.tsx` | Updated assertions if mark/markers change accessible names or structure. |
| `openspec/specs/vigilafrica/decisions.md` | New dated ADR: brand mark selection (which concept won and why). |
| Polish pass: `web/src/App.css` + surface CSS | Only items from the approved findings list — each cited to a `/openspec-review`-able rule or audit finding. |

## Implementation Plan (phased — decision gates between phases)

1. **Phase A — Concepts (no production code).** Author 5 SVG marks + rationale
   + DFII scores; render the local concepts page; present to maintainer.
   → **Gate: maintainer picks the winner; ADR recorded.**
2. **Phase B — Mark integration.** Swap/refine `BrandMark.tsx`; update tests;
   delete `brand-concepts/`.
3. **Phase C — Asset suite.** Favicon set, touch icon, OG image + meta, README
   banner, map markers. Verify each per the proposal's Verification Plan.
4. **Phase D — Polish pass.** Run the three design-skill audits on the merged
   Ground Truth surfaces; produce a findings list.
   → **Gate: maintainer approves the list.** Implement approved items only.

## Constraints

- ADR-013 (plain CSS + stylelint token rule) and ADR-015 (type tokens,
  no-emoji, reduced-motion gating) bind all new code.
- Every binary asset has its SVG source committed beside it (R2).
- Marker data-semantics (cyan = flood, lime = fire) are immutable (R3).
- This change touches `web/src/` → the Sentinel gate is satisfied by the
  companion proposal.

## Acceptance Criteria

- [ ] Phase A: 5 concepts + rationale + scores delivered; selection ADR merged.
- [ ] Phase B: winning mark live in the nav; `brand-concepts/` removed; tests
      green.
- [ ] Phase C: favicon set + touch icon linked and rendering; OG/Twitter meta
      validate against the committed 1200×630 image; README banner renders on
      GitHub; markers redrawn with tokens + colours preserved.
- [ ] Phase D: findings list approved before implementation; each shipped item
      browser-verified at 1440/768/390 + reduced-motion.
- [ ] All gates green throughout: `tsc` build, eslint 0, stylelint clean,
      vitest (incl. axe) full pass; no emoji-as-icons; no colour literals
      outside `tokens.css` (static binaries excepted).

## Verification Plan

1. Concepts page: all five marks legible at 16px and crisp at 480px.
2. Per-phase: build + eslint + stylelint + vitest after every increment.
3. Favicon/touch icon in a real browser tab; OG card via a preview/validator
   tool; README banner on the live GitHub repo page.
4. Markers against real EONET data via the local Docker stack.
5. Final `/openspec-review` pass before merge.
