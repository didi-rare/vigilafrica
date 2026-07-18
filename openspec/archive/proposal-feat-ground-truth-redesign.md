---
id: feat-ground-truth-redesign
status: proposed
branch: feat/ground-truth-redesign
---

# Proposal: "Ground Truth" Visual Redesign (feat-ground-truth-redesign)

## Why

The frontend shipped as a competent but generic dark-SaaS landing page: emoji used
as UI icons (🌊🔥 hero badges, ✅🔄 roadmap, a `◉` text-glyph logo), a glow-orb +
dot-grid hero, an evenly-balanced four-accent palette used decoratively, and a
predictable `label → title → subtitle → symmetric card grid` rhythm on every
section. It reads as a template — which undersells a credible, grant-seeking
humanitarian instrument and fails a basic "screenshot with the logo removed →
would you recognise it?" test.

VigilAfrica is a geospatial situational-awareness tool (NASA EONET floods +
wildfires, localised to Nigeria & Ghana). Its interface should *look like the
instrument it is*. **Ground Truth** is an instrument-grade cartographic design
language: lat/long graticules, coordinate readouts, a live-signal station
aesthetic, and colour that encodes data meaning rather than decoration. This is
both more distinctive and more honest — and the credibility a partner / grant
audience needs.

Approved 2026-06-08: direction **Ground Truth**, ambition **full rebrand**, scope
**all surfaces**.

## What Changes

### Type system (ADR-015)

- Self-hosted via `@fontsource` (no Google-Fonts CDN call — privacy + perf; Vite
  bundles the woff2): **Space Grotesk** (display), **IBM Plex Sans** (body/UI),
  **IBM Plex Mono** (coordinates & data readouts). Imported in `main.tsx`.
- New `--font-display` / `--font-body` / `--font-mono` tokens in `tokens.css`;
  the legacy `--font-sans` is aliased to `--font-body` so existing component CSS
  keeps working. Replaces the prior Inter-only stack.

### Design tokens (`tokens.css`)

- **Cartographic motif tokens:** `--graticule-line`, `--graticule-line-strong`,
  `--graticule-tick`, `--contour-line`, `--signal-live`, `--signal-live-glow`,
  `--coordinate-text`.
- **Colour discipline:** amber becomes the single brand accent; cyan/lime are
  demoted to strict data semantics (cyan = flood/water, amber = fire/alert). No
  token deletions — existing semantic tokens are retained.

### Brand mark

- New hand-authored SVG component `components/BrandMark.tsx` — a "vigil reticle"
  (cartographic crosshair graticule + a single live centre point), themed via CSS
  classes so it inherits the token palette on any surface. Replaces the `◉` glyph.

### Surfaces (incremental, all `web/src/`)

1. **Landing** (`App.tsx` + `App.css`): nav (brand mark + mono station tag),
   hero (graticule + live-feed coordinate readout + telemetry panel, SVG
   flood/fire tags), how-it-works, audience, roadmap (de-emoji ✅/🔄 → SVG),
   footer.
2. **`/for-partners`**, 3. **`EventDetail`**, 4. **`EventsDashboard` / Map chrome**.

### Motion

- A sparse, reusable, instrument-themed motion layer (entrance reveals on scroll,
  the live-signal ping, hover micro-interactions). **Every animation is gated
  behind `@media (prefers-reduced-motion: reduce)`.**

### Dependencies & standards

- Adds `@fontsource/space-grotesk`, `@fontsource/ibm-plex-sans`,
  `@fontsource/ibm-plex-mono` (exact-pinned, §14.5). Updates `developers-react.md`
  §14.3 approved-deps list + §7 styling notes to reflect the type system.

## Out of Scope

- **No Tailwind / CSS-in-JS** — stays plain CSS + the token system (ADR-013 holds;
  this proposal complements it, does not supersede it).
- **No new product features, routes, API calls, or analytics events** — purely
  presentational. Copy/user-flow preserved.
- **No data-model or backend change.**
- **No dark/light theming work** — the product is dark-only by design (separate
  `feat-dark-mode-toggle` proposal owns any theming).
- **Map rendering logic** (MapLibre layers/markers) — only the surrounding chrome
  restyles; marker/data behaviour is untouched.

## Capabilities

### New Capabilities

- `visual-identity`: A cohesive "Ground Truth" cartographic design language —
  brand mark, type system, graticule/coordinate motif, single-accent colour
  discipline, and an instrument-grade motion layer — applied across all frontend
  surfaces.

### Modified Capabilities

- `frontend-landing`, `frontend-navigation`, `partner-landing`: restyled to the
  Ground Truth system with no change to function or content.

## Acceptance Criteria

- [ ] No emoji used as UI icons anywhere in `web/src/` (badges, roadmap, logo).
- [ ] Type system: Space Grotesk display + IBM Plex Sans body + IBM Plex Mono
      data, self-hosted (no external font CDN request at runtime).
- [ ] A real SVG brand mark replaces the `◉` glyph, themed via tokens.
- [ ] All decorative colour resolves to the amber single-accent; cyan/lime appear
      only as flood/fire data semantics.
- [ ] Every animation is disabled under `prefers-reduced-motion: reduce`.
- [ ] All surfaces responsive at 375 / 768 / 1024 / 1440 with no horizontal
      scroll; touch targets ≥ 44px; focus states visible.
- [ ] WCAG AA contrast (4.5:1 body) preserved on the dark base.
- [ ] `tsc` build, eslint (0 errors), stylelint (clean — no colour literals
      outside `tokens.css`), and the full vitest suite (incl. axe checks) pass.
- [ ] ADR-015 registered; `developers-react.md` §14.3/§7 updated.

## Risks

- **R1 — Scope creep across four surfaces.** Mitigation: ship per-surface in
  verified increments (landing first, already proven), each its own PR.
- **R2 — New fonts regress bundle size / first paint.** Mitigation: self-hosted
  woff2, only the 7 weights used are imported (Space Grotesk 500/700, Plex Sans
  400/500/600, Plex Mono 400/500). **Measured:** 34 subset woff2 files, ~405 KB
  on disk total — but the browser fetches only the subsets a page actually needs
  (latin ≈ ~18 KB/weight), fonts are separate cached assets **not** in the JS
  bundle (JS output unchanged vs `development`), and `font-display: swap`
  (fontsource default) keeps them off the first-paint critical path.
- **R3 — Motion harms accessibility or feels gratuitous.** Mitigation: sparse +
  purposeful, `prefers-reduced-motion` gating is an acceptance criterion.
- **R4 — Colour-only data encoding (flood cyan / fire amber).** Mitigation: tags
  always pair the colour with a text label and an SVG icon (not colour-only).

## Verification Plan

1. `tsc` build + `vite build`; eslint; stylelint; full vitest (incl. axe).
2. Browser verification per surface at 1440 / 768 / 375, plus a
   `prefers-reduced-motion` pass (all motion stilled).
3. Keyboard walkthrough: focus visible on every interactive element, tab order
   matches visual order.
4. Confirm no runtime request to `fonts.googleapis.com` / `fonts.gstatic.com`.

## Origin

Requested 2026-06-08 via the `/frontend-design` + `/ui-ux-pro-max` skills: audit
the landing UI and upgrade it to a premium, production-grade interface without
generic-AI tropes. Direction, ambition, and scope confirmed by the maintainer.
The nav + hero slice was built and verified first as the aesthetic contract;
this proposal governs rolling it across all surfaces.
