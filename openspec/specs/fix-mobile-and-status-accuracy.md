---
id: fix-mobile-and-status-accuracy
status: in-progress
branch: fix/mobile-and-status
---

# Spec: Mobile Layout + Status Accuracy Fix

## 1. Scope

Frontend-only. Mobile responsive bugs, stale status copy, embarrassing MapLibre glyph 404s, and three accessibility issues caught by axe-core and a manual focus walkthrough on `staging.vigilafrica.org`. No backend, no schema, no API contract changes.

## 2. Components Touched

| File | Change |
| --- | --- |
| `web/src/components/EventsDashboard.css` | Add `@media (max-width: 768px)` block for layout; fix `.event-location` wrapping; styles for skip-link if hosted here |
| `web/src/App.tsx` | Banner copy update; body copy refresh; remove literal `**v0.7**`; add skip-to-main link; wrap milestone emojis in `aria-hidden`; restructure `<article role="listitem">` steps |
| `web/src/App.css` | `scroll-margin-top` on section anchors; `prefers-reduced-motion` block; skip-link styles if hosted here |
| `web/src/data/milestones.json` | Flip v0.7 → complete; v1.0 → active |
| `web/index.html` | `<meta name="description">` rewrite |
| `web/src/components/Map.tsx` | MapLibre style/glyphs swap |
| `web/src/components/EventsDashboard.test.tsx` | Add tests covering skip-link presence and milestone aria-hidden emoji |
| `.env.example` | (Optional, only if MapTiler key chosen) document `VITE_MAPTILER_KEY` |

## 3. Mobile Layout

### 3.1 Dashboard layout breakpoint

In `EventsDashboard.css` add:

```css
@media (max-width: 768px) {
  .dashboard-layout {
    flex-direction: column;
    height: auto;
    gap: 1.25rem;
  }
  .dashboard-sidebar {
    flex: 1 1 auto;
    width: 100%;
    padding-right: 0;
    max-height: none;
    overflow-y: visible; /* let the page scroll naturally — no nested scroll */
  }
  .dashboard-map-container {
    flex: 1 1 auto;
    height: 60vh;
    min-height: 360px;
  }
}
```

Acceptance: at any viewport ≤ 768 px, no horizontal overflow exists from the dashboard region; the page has a single scroll container; touching the map area does not trap a nested scroll gesture.

### 3.2 `.event-location` wrapping

```css
.event-location {
  flex-wrap: wrap;
  min-width: 0;
  word-break: break-word;
}
.event-location > * {
  min-width: 0;
}
```

Acceptance: a card with a long state name (e.g. "Greater Accra Region") plus the country suffix renders without overflowing the card or the viewport.

### 3.3 Section anchor offset

Add to `App.css`:

```css
section[id], h2[id], #dashboard-heading {
  scroll-margin-top: 88px; /* nav height ~81 px + breathing room */
}
```

Acceptance: clicking an in-page anchor or scrolling a section into view via JS lands the title fully visible below the sticky nav.

## 4. Map Glyph Source

### 4.1 Decision

Replace the current demotiles-pointing style. Final apply choice: omit the `glyphs`
property and render cluster-count text from local fonts. MapLibre GL JS 5.11+
supports local-font rendering when `glyphs` is omitted, and this repo is on
MapLibre GL JS 5.23.0. This avoids a new API key, avoids committed glyph PBFs,
and removes the failing demotiles network dependency.

The acceptance criterion below must be met without adding `VITE_MAPTILER_KEY`.

### 4.2 Acceptance

- Browser console contains **zero** `Unable to load glyph range` warnings from MapLibre on the dashboard map and the `EventDetail` page.
- All map text labels render with a real font (no fallback box-glyphs).
- If option (a) is chosen, the key is documented in `.env.example` and surfaced in `docs/deployment/staging-production-topology.md` for both staging and production env matrices.

## 5. Status Accuracy

### 5.1 Banner text

`web/src/App.tsx:97` becomes:

```tsx
🛰️ Active Development — v0.7 complete · v1.0 staging live · production launch in progress
```

### 5.2 Body copy

`web/src/App.tsx:223-225` rewritten to one short paragraph that:

- Acknowledges Ghana + Nigeria are live.
- States v1.0 staging is live and production is gated on operator action.
- Removes the literal `**v0.7**` markdown asterisks (replace with plain "v0.7" or strong tags).
- Stays under ~3 sentences.

Suggested:
> VigilAfrica is being built milestone by milestone. v0.7 (Second Country Stable) is complete — Nigeria and Ghana run end-to-end on the same pipeline. v1.0 (Credible Public Launch) is the active milestone: staging is live; production deploy is gated on a final reviewer approval.

### 5.3 `milestones.json`

```json
{ "label": "v0.7 · Second country stable", "complete": true,  "active": false }
{ "label": "v0.8 · Pre-demo setup",        "complete": true,  "active": false }
{ "label": "v1.0 · Credible public launch","complete": false, "active": true  }
```

(Pre-existing entries v0.1–v0.6 unchanged.)

### 5.4 `<meta name="description">` (Option A locked)

```html
<meta name="description" content="VigilAfrica translates raw NASA satellite event data into local African context — floods and wildfires by country and state. Open-source. Nigeria and Ghana live." />
```

## 6. Accessibility

### 6.1 Skip-to-main link

First focusable element inside `<App>`. Visually hidden until focus, then visible top-left. Targets `#main` (add `id="main"` to `<main>` if not present).

```tsx
<a href="#main" className="skip-link">Skip to main content</a>
```

CSS (in `App.css`):

```css
.skip-link {
  position: absolute;
  top: -100px;
  left: 8px;
  z-index: 1000;
  background: var(--accent-amber);
  color: #07091A;
  padding: 8px 14px;
  border-radius: 6px;
  font-weight: 600;
}
.skip-link:focus {
  top: 8px;
}
```

### 6.2 Step list semantics

Drop `role="listitem"` from `<article class="step">` (axe `aria-allowed-role` violation). Either:

- Wrap steps in `<ul role="list" class="steps">` and change `<article>` to `<li>`, **or**
- Keep `<article>` and remove the role; the parent `.steps` keeps `role="list"`. Apply phase picks based on which keeps the existing CSS untouched.

Acceptance: `axe.run(document)` returns zero `aria-allowed-role` violations.

### 6.3 Milestone emoji a11y + spacing

Wrap `✅` / `🔄` in `aria-hidden="true"` spans, and put a literal whitespace between the label and the tag so screen readers don't read "stable🔄". Render output should be `v0.7 · Second country stable Complete` (the green visible "✅" is hidden from AT).

### 6.4 Reduced motion

Add to `App.css` end-of-file:

```css
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    animation-duration: 0.001ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.001ms !important;
  }
}
```

(Or scoped to `.logo-icon`, `.status-dot`, `.hero-glow` — use blanket rule unless it breaks an intentional UX cue.)

## 7. Tests

`web/src/components/EventsDashboard.test.tsx` (or a new `App.test.tsx` if cleaner):

1. **"renders skip-to-main link as the first focusable element"** — assert.
2. **"axe finds zero `aria-allowed-role` violations on the landing page"** — render `<App>`, run `vitest-axe` with `rules: { 'aria-allowed-role': { enabled: true } }`.
3. **"milestone tag emoji is aria-hidden"** — query for the emoji span and assert `aria-hidden="true"`.
4. **(Manual)** mobile layout verification post-merge on `staging.vigilafrica.org` at viewport ≤ 390 px — captured in PR description with a screenshot.

Existing tests must continue to pass.

## 8. Acceptance Criteria

- [ ] At ≤ 768 px viewport, the dashboard region has zero horizontal overflow.
- [ ] At ≤ 768 px viewport, scrolling the page does not trigger nested-scroll fighting.
- [ ] `.event-location` wraps gracefully with long names; no horizontal overflow on `.event-card`.
- [ ] In-page anchors land titles fully visible below the sticky nav.
- [ ] Browser console shows zero MapLibre glyph 404s on dashboard and EventDetail.
- [ ] Banner, body copy, milestones data, and `<meta name="description">` all reflect "v0.7 complete · v1.0 staging live · production pending" reality.
- [ ] Skip-to-main link is the first focusable element and visible on focus.
- [ ] axe-core run against the rendered landing page reports zero `aria-allowed-role` violations.
- [ ] Milestone emojis are `aria-hidden="true"`.
- [ ] `prefers-reduced-motion: reduce` disables `pulse`, `blink`, `float`.
- [ ] `npm run lint`, `npm run build`, and the full `npm test` suite all pass with no new warnings.
- [ ] No code changes outside §2.

## 9. Governance Notes

- This change does **not** overlap `chore-vps-v1-launch` — it polishes the staging frontend the soft-launch revealed; it does not progress the production launch.
- Branch: `fix/mobile-and-status`. PR target: `development`.
- The `link-in-text-block` axe finding from MapLibre attribution is explicitly **deferred** and will be tracked under a future "map style overhaul" change.
- Static-asset performance findings (P1, P2) are deferred — captured here as out-of-scope so they aren't lost.
