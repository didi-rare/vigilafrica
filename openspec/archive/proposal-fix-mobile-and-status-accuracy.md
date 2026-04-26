---
id: fix-mobile-and-status-accuracy
status: archived
branch: fix/mobile-and-status
merged_pr: https://github.com/didi-rare/vigilafrica/pull/36
archived_on: 2026-04-26
---

# Proposal: Mobile Layout + Status Accuracy Fix (fix-mobile-and-status-accuracy)

## Why

Two distinct but related credibility issues surfaced on staging during the v1.0 soft-launch:

1. **Mobile is broken.** A live audit (axe-core + computed-style probes via Claude in Chrome on `staging.vigilafrica.org`) confirmed `.dashboard-sidebar` has `flex-basis: 400px; flex-shrink: 0`, so on any viewport ≤ 424 px the sidebar deterministically overflows. Combined with the layout's fixed `height: 800px` and the sidebar's internal `overflow-y: auto`, this creates the "shaking + leaning right" the user reported on a phone. Section titles also disappear behind the 81 px sticky nav because `scroll-margin-top` is `0`.
2. **The page doesn't reflect reality.** The hero banner reads "v0.7 complete · v0.8 Pre-demo Setup planned"; `milestones.json` renders v0.7 as "🔄 In progress"; meanwhile v0.7 is in fact done and v1.0 staging is live. The `<meta name="description">` still ends with "Nigeria first" — Ghana shipped two milestones ago. Body copy in the Status section still references v0.6/v0.7. The page is internally contradictory and externally stale.

A third issue surfaced during the live pass: the embedded MapLibre style points at `demotiles.maplibre.org` for glyph fonts and 404s on every map render. Labels degrade to a fallback. Acceptable on a prototype, not on a public-launch staging URL.

## What Changes

Frontend-only — CSS, copy, JSON data, plus a MapLibre style swap. **No backend, no schema, no API contract changes.**

### Mobile layout (CSS-only, all in `web/src/components/EventsDashboard.css`)

- Add `@media (max-width: 768px)` block stacking `.dashboard-layout` to column, dropping the fixed `800px` height, normalising sidebar padding, and giving the map a sensible mobile height.
- Make `.event-location` wrap on narrow widths (`flex-wrap: wrap; min-width: 0; word-break: break-word`) so long state names / coordinate fallbacks don't overflow.
- Add `scroll-margin-top: 88px` (matching nav height + breathing room) to section headings and `[id]` anchors.

### Map glyphs (MapLibre)

- Replace the demotiles glyph URL with a working source. Default plan: switch the map style to a self-contained one whose glyph URL resolves (e.g. MapTiler free tier behind `VITE_MAPTILER_KEY`, or an OSM-Liberty style hosted reliably). Decision-point captured in spec §4.

### Status accuracy

- Update banner text to: `🛰️ Active Development — v0.7 complete · v1.0 staging live · production launch in progress`.
- Refresh body copy in the Status section to describe the v1.0 stage, drop the v0.6/v0.7 narrative.
- Remove the literal `**v0.7**` markdown asterisks from the source string.
- Update `milestones.json`: v0.7 → `complete: true`, v1.0 → `active: true`, drop `active: true` from v0.7.
- Rewrite `<meta name="description">` to: *"VigilAfrica translates raw NASA satellite event data into local African context — floods and wildfires by country and state. Open-source. Nigeria and Ghana live."* (Option A from review).

### Accessibility

- Add a "Skip to main content" link as the first focusable element in `<App>`, visible on focus only.
- Wrap milestone tag emojis in `aria-hidden="true"` and add a whitespace separator between the milestone label and the tag (currently the DOM concatenates them with no space — screen readers read "stable🔄 In progress" as one word).
- Drop `role="listitem"` from the `<article class="step">` elements (axe `aria-allowed-role` violation, 3 nodes); restructure the steps as a `<ul role="list">` with `<li>` children, or keep `<article>` and remove the conflicting role. Decision-point in spec §6.
- Add a `@media (prefers-reduced-motion: reduce)` block disabling the `pulse`, `blink`, and `float` animations.

## Out of Scope

- MapLibre attribution `link-in-text-block` axe finding (library-internal, will track as a separate "map style overhaul" follow-up).
- `map-vendor` bundle size (946 KB / 246 KB gz) — deferred.
- Hero `filter: blur(80px)` mobile GPU optimisation — deferred.
- `.event-title` line-clamp truncation behaviour — deferred.
- `<small>` mis-use in `EventDetail.tsx` — deferred.
- Production launch tasks (Phase 5 of `chore-vps-v1-launch`) — separate.

## Capabilities

### Modified Capabilities

- `web-frontend` (dashboard layout, status copy, accessibility) — mobile responsive behaviour and content accuracy become first-class.
- `web-frontend` (map rendering) — glyph font source becomes deterministic and self-contained.

## Impact

- **Files modified:** `web/src/components/EventsDashboard.css`, `web/src/App.tsx`, `web/src/App.css`, `web/src/data/milestones.json`, `web/index.html`, `web/src/components/Map.tsx` (or whichever module sets the MapLibre style), `web/src/components/EventsDashboard.test.tsx` (axe + skip-link assertions).
- **Files possibly added:** `.env.example` entry if MapTiler key is chosen for glyphs.
- **No API or schema changes.**
- **No new heavy dependencies.** A glyph-source swap may add a small CSS/style URL — no new npm package required.
