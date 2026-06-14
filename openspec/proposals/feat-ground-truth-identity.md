---
id: feat-ground-truth-identity
status: proposed
branch: feat/ground-truth-identity
---

# Proposal: Brand Identity Suite + Premium Polish Pass (feat-ground-truth-identity)

## Why

The Ground Truth rebrand (`feat-ground-truth-redesign`, PR #120) established the
visual language — type system (ADR-015), cartographic motifs, single-accent
colour discipline — but deliberately left two things on the table:

1. **The brand mark was a single first-pass concept.** The `BrandMark` "vigil
   reticle" was hand-authored in one sitting as the aesthetic contract for the
   nav. It works, but it was never tested against alternatives. A mark that will
   sit on partner decks, grant applications, and the public repo deserves a real
   selection: five concepts, one deliberate winner.
2. **Identity assets don't exist.** There is no Open Graph / share image (every
   link shared with NRCS / Code for Africa / a grant reviewer renders as a bare
   URL), the favicon predates the rebrand, the README has no banner (the repo
   front page IS the first impression for partner due-diligence), and the map
   markers predate the identity work. SEO/meta infrastructure was explicitly
   out-of-scope in the redesign proposal — this change picks it up.

A third, smaller pillar: a **targeted polish pass** on top of Ground Truth —
re-audit the shipped surfaces with `/frontend-design` + `/ui-ux-pro-max` +
`/ui-ux-designer` and fix what a fresh premium-grade audit finds (interaction
details, spacing rhythm, hierarchy refinements). Direction proposed first, then
refactor — same discipline as the rebrand itself.

**Builds on:** PR #120 (`feat-ground-truth-redesign`), merged to `development`
2026-06-11. This change iterates the direction — it does not supersede ADR-015.

## What Changes

### 1. Logo exploration → one chosen mark (direction-first)

- **Five hand-authored SVG logo concepts**, each with a written rationale and a
  DFII-style score (aesthetic impact / context fit / feasibility / consistency).
  The current vigil reticle may stand as one of the five. All concepts:
  token-themed (no hardcoded colours), legible at 16px (favicon) and 480px
  (banner), no emoji, no stock clipart.
- Maintainer picks the winner **before any asset work starts**. The decision is
  recorded as a dated ADR (brand mark selection) in `decisions.md`.
- If the winner ≠ the current reticle, `web/src/components/BrandMark.tsx` is
  replaced and the nav/test updated; if the reticle wins, it gets a refinement
  pass from the exploration learnings.

### 2. Asset suite derived from the chosen mark

- **Favicon + app icons:** `favicon.svg` (+ `.ico` fallback) and
  `apple-touch-icon.png` in `web/public/`, wired in `web/index.html`.
- **Social/OG image:** a 1200×630 share card (dark Ground Truth styling, mark +
  wordmark + one-line positioning) committed as a static asset; OG + Twitter
  meta tags added to `web/index.html` for the production origin.
- **README banner:** repo header banner (mark + wordmark on the graticule
  motif) embedded at the top of `README.md`.
- **Map marker set:** flood/fire markers redrawn consistent with the chosen
  mark's geometry, preserving the existing data-semantic colours
  (`--marker-flood-*` cyan, `--marker-fire-*` lime) and online indicator.

### 3. Premium polish pass (audit-first, scoped after findings)

- Fresh audit of the shipped Ground Truth surfaces using the three design
  skills; findings triaged into a short, concrete refactor list (interaction
  details, spacing/hierarchy, responsive edge cases) **proposed to the
  maintainer before implementation**.
- Constraints preserved: ADR-013 (plain CSS + tokens), ADR-015 (type tokens,
  no-emoji, reduced-motion gating), no copy or user-flow changes.

## Out of Scope

- **No new routes, features, API calls, or analytics events.**
- **No copy/messaging rewrite** — content stays as shipped.
- **No dark/light theming** (`feat-dark-mode-toggle` owns that).
- **No backend or data-model change.**
- **No paid design tooling or external designers** — all SVG is hand-authored
  in-repo.
- **No animated logo / motion identity** — static mark + assets only.

## Capabilities

### New Capabilities

- `brand-identity`: A deliberately chosen brand mark with a complete asset
  suite (favicon/app icons, OG share card, README banner, map markers) so the
  identity is consistent everywhere the project appears — browser tab, link
  preview, repo front page, and the map itself.

### Modified Capabilities

- `visual-identity`: The Ground Truth design language gains its selected mark
  and a polish-pass refinement; ADR-015 rules unchanged.

## Acceptance Criteria

- [ ] Five SVG logo concepts delivered with rationale + scores; maintainer
      decision recorded as a dated ADR in `decisions.md`.
- [ ] Chosen mark integrated: `BrandMark.tsx` reflects the winner; nav renders
      it; existing tests updated and green.
- [ ] `web/public/` contains `favicon.svg`, `favicon.ico`,
      `apple-touch-icon.png` derived from the chosen mark; `index.html` links
      them; the old favicon is removed.
- [ ] `index.html` carries OG + Twitter meta (title, description, image, url)
      pointing at a committed 1200×630 share image; staging keeps
      `noindex` behaviour unchanged.
- [ ] `README.md` opens with the banner asset; image renders on GitHub.
- [ ] Map flood/fire markers redrawn consistent with the mark; data-semantic
      colours and `--marker-*` tokens preserved; Map tests green.
- [ ] Polish-pass findings list reviewed by maintainer before implementation;
      implemented items each verified in browser.
- [ ] All gates green: `tsc` build, eslint 0, stylelint clean, full vitest
      (incl. axe), no emoji-as-icons, reduced-motion unaffected.
- [ ] No colour literals outside `tokens.css` in any new SVG-in-JSX/CSS
      (static assets like the OG PNG excepted).

## Risks

- **R1 — Identity churn.** Five options invite endless iteration. Mitigation:
  one selection round, decision locked by ADR, assets only after the lock.
- **R2 — Binary assets in-repo** (PNG/ICO can't be code-reviewed line-by-line).
  Mitigation: keep the SVG source-of-truth committed beside every binary;
  binaries regenerable from it.
- **R3 — Marker redesign harms map readability.** Mitigation: markers keep
  current size/contrast/data colours; only geometry aligns with the mark;
  verified against the live map via the Docker stack.
- **R4 — Polish pass scope-creeps.** Mitigation: findings list is approved
  before implementation; anything larger becomes its own change.

## Verification Plan

1. Concepts page rendered locally (all five marks at 16/32/120/480px, light
   inspection at favicon size).
2. After integration: build + eslint + stylelint + vitest (incl. axe) green.
3. Favicon + touch icon verified in browser tab; OG tags validated with a
   card-preview tool against the committed image.
4. README banner checked on the actual GitHub repo page.
5. Markers verified against the live map (local Docker stack, real EONET data).
6. Polish items: per-item browser verification at 1440/768/390 +
   reduced-motion pass.

## Origin

Requested 2026-06-11 via `/openspec-explore`: extend the completed Ground Truth
rebrand (PR #120) with a real identity selection (5 logo concepts), the asset
suite the redesign deferred (favicon, OG image, README banner, markers), and an
audit-driven polish pass. Scoped to build on — not supersede — ADR-015.
