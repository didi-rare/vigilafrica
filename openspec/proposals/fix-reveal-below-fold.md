---
id: fix-reveal-below-fold
status: proposed
branch: fix/reveal-below-fold-backstop
---

# Proposal: Reveal Backstop So Landing Sections Are Never Blank on Load (fix-reveal-below-fold)

## Why

The landing page's `#how-it-works`, `#built-for`, and `#roadmap` sections use `useReveal` — a one-shot IntersectionObserver that fades them in (`.reveal` → `.is-revealed`) when scrolled into view. `.reveal` sets `opacity: 0`, so any section the observer never reveals stays invisible.

Reported symptom: on first load the "How it works" section is blank; a refresh "fixes" it.

**Reproduced against production (headless Chrome, network-throttled cold loads).** The reported mechanism (an observer broken by a load-time race) was **disproven** — the observer works and reveals correctly on scroll. The real defect is viewport-dependent:

| Viewport height | `#how-it-works` at scroll-top, settled |
|---|---|
| ≤ 880px | `opacity: 0` — **blank**, until the user scrolls to it |
| ≥ 940px | revealed / visible |

At short viewports the section sits at `rectTop≈2144` (far below the fold) and, because the observer only fires on intersection, stays hidden until scrolled to — a blank dark void on landing. The 880-vs-940 split is itself fragile: at 940 the section happens to pass through the viewport during the lazy dashboard + MapLibre layout shift and the one-shot observer catches it; at 880 it misses. A bookmarks bar drops a maximized 1080p Chrome under the 940 threshold, which is why the maintainer sees it. ("Refresh fixes it" is **likely** browser scroll-restoration bringing an already-scrolled section into view — not reproducible in Playwright, which resets scroll on reload, so left unproven.)

## What Changes

`web/src/hooks/useReveal.ts` — add a **load-time backstop** to the existing effect. Keep the IntersectionObserver for scroll-triggered reveals, but also reveal any still-hidden element once the page has loaded and laid out (`window` `load`, or immediately if `document.readyState === 'complete'`, plus a 1500ms absolute floor). Content is therefore never left invisible on any viewport; the fade-up still plays via the CSS transition. All timers/listeners are cleaned up on unmount, and the existing reduced-motion / no-IntersectionObserver early-return paths are untouched.

Chosen behaviour (maintainer decision): guarantee visible-on-load, accepting that sections far down the page fade in at load rather than strictly on scroll.

## Out of Scope

- Redesigning the scroll-reveal choreography or adding scroll affordances.
- The lazy-dashboard / MapLibre layout shift itself (it is not a defect; the backstop makes the reveal robust to it).

## Verification

- [x] Bug reproduced on production before the fix — sub-940px viewports blank at scroll-top (0/… revealed), ≥940px visible
- [x] Fix verified against a local `vite preview` of the built artifact: at viewports 700–1060px the section is `revealed`/`opacity:1` at scroll-top **while still below the fold** (`rectTop≈2067` > viewport), proving the backstop — not a layout change — reveals it
- [x] `npm run type-check` / `npm run lint` / `npm run test` (8 files, 57 tests) / `npm run build` all clean
- [ ] Post-deploy: re-run the headless repro against staging, then production, to confirm the fix in the real environment

## Origin

Maintainer bug report, 2026-07-24: "How it works" blank on first load, appears on refresh. Diagnosis-first: the initial code-reading theory was wrong and the headless reproduction corrected it before any fix was written.
