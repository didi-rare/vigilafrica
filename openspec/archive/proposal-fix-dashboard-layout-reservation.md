---
id: fix-dashboard-layout-reservation
status: archived
branch: fix/dashboard-layout-reservation
merged_pr: https://github.com/didi-rare/vigilafrica/pull/193
archived_on: 2026-08-03
---

# Proposal: Reserve Viewport Height for the Lazy Dashboard So Visible Content Stops Jumping (fix-dashboard-layout-reservation)

## Why

On the landing page the `EventsDashboard` is lazily loaded behind a `Suspense` boundary whose fallback is a single line of text — **~26px**. The mounted dashboard is **~1459px** at desktop widths (**~14000px** at 768px, where the events list stacks instead of scroll-containing).

When it mounts, everything below it moves. `#how-it-works` sits **visible at the bottom of the viewport** at load (y=712 in a 940px viewport, 228px of it on screen); mounting the dashboard shoves it entirely off-screen. The layout-shift entry records exactly that — a visible rect collapsing to nothing:

```
#how-it-works   {y:712, h:228} -> {y:0, h:0}   Δy=-712
```

This was found while verifying `chore-web-hardening`, by running production and staging back-to-back as a control. **It is not caused by that work** — production, which lacks those changes, exhibited it too.

### It is a race, and that hid it

The shift is only *attributed* when the reveal animation (`fix-reveal-below-fold`, #182) has faded `#how-it-works` in **before** the dashboard mounts. CLS ignores movement of invisible elements. On an idle machine the dashboard usually wins that race and CLS reads **0.0000**; under load it does not, and CLS reads **~0.26**.

That intermittency is why it never surfaced: a single Lighthouse run cannot see it. An initial sample suggested 2/9 runs; a larger one gave **6/10**. Delaying only the lazy chunks — i.e. simulating a slow connection — makes it **6/6, deterministic**.

⚠️ **The reveal is not the root cause and must not be "fixed".** Forcing `.reveal` to stay invisible scores 0/10 while the jump still happens — that hides the metric and leaves the jank. Conversely, forcing it always-visible gives 10/10 at a steady 0.2617, proving **the layout jump occurs on every single load** regardless of the reveal. The reveal only decides whether the jump is counted.

## What Changes

Two edits.

1. **`web/src/App.tsx`** — tag the dashboard's Suspense fallback with a `dashboard-fallback` class. A distinct class is required: all three Suspense fallbacks in this file share `container section`, and the `/events/:id` and `/for-partners` boundaries must not inherit a screen-height reservation.
2. **`web/src/App.css`** — `.dashboard-fallback { min-height: 100vh }`, plus flex-centring so a single line of text in a now screen-tall box reads as a load state rather than a broken layout.

### Why a viewport-relative reservation, not the dashboard's real height

Matching the true height would mean reserving **~14000px at 768px wide** — screens-deep of blank placeholder, worse than the jump it prevents. Reserving one viewport height is sufficient because **CLS only counts, and users only see, movement of on-screen content**: after the reservation nothing below the fallback is visible at load, so nothing visible moves when the dashboard arrives.

`vh` rather than `svh` deliberately: mobile URL-bar height changes must not make the reservation too small.

### ⚠️ Amended 2026-07-27 after independent review — the reservation was the right idea at the wrong size

This section originally ended *"Over-reserving is harmless — the displaced content is already below the fold."* **That is true of CLS but false of the user.** Two independent reviewers reached the same conclusion from opposite directions:

- **Above 768px the dashboard mounts at a FIXED height.** [`EventsDashboard.css`](../../web/src/components/EventsDashboard.css) sets `.dashboard-layout { height: 800px }` — not viewport-relative — so the section measures ~1459px regardless of viewport height, while `100vh` scales. On a tall display the reservation exceeds anything the dashboard will ever fill. Measured at 1350×2000: ~540px of band the content never reaches. (The predicted *upward* CLS jump did **not** materialise — measured 0.0111, ~40× below the bug it fixes — because the displaced content is off-screen before and after. The waste is visual, not metric.)
- **At phone widths the reservation buys nothing.** 375×812 measured **0/5 shifting both with and without it** — the taller mobile hero already pushes the next section off-screen — yet it still imposed a near-empty screen-height band on the audience least able to tolerate ambiguity about whether a page is loading or broken.

**Amended shape:**

```css
.dashboard-fallback { min-height: min(100vh, 1460px); }
@media (max-width: 480px) { .dashboard-fallback { min-height: 0; } }
```

`min()` engages only on viewports taller than ~1460px — precisely where `100vh` was waste — so every verified-working case is unchanged. The 480px cut-off is deliberate and not 768px: **at 768 the reservation is load-bearing** (5/5 shifting at 0.0410 without it, 0/5 with it), because there `.dashboard-layout` becomes `height: auto` and stacks to ~14000px while the hero is still short enough to leave the next section on screen.

## Out of Scope

- **Virtualising / paginating `div.events-list`** (43 children; ~14000px stacked at 768px). That is the real reason the tablet layout is so tall and belongs to `perf-mobile-first-render`.
- Any change to `useReveal` or the `.reveal` CSS — see the warning above.
- The `/events/:id` Map boundary. It also lazy-loads MapLibre and may have its own shift; unmeasured, so not claimed either way.
- Replacing the text fallback with a full skeleton UI. A nicety, not required to fix the jump.

## Verification

Trials against **production** with the `EventsDashboard`/`Map`/`map-vendor` chunks delayed 2500ms to force the losing side of the race deterministically:

| viewport | baseline | with reservation |
|---|---|---|
| 1350x940 | **6/6 shifting, 0.2617** | **0/5** |
| 768x1024 | **5/5 shifting, 0.0820** | **0/5** |
| 375x812 | 0/5 — unaffected | 0/5 |

375px is genuinely unaffected: the taller mobile hero already pushes `#how-it-works` below the fold, so nothing visible moves. **The impact is desktop and tablet, not phones.**

Also confirmed on production by CSS injection that `min-height: 60vh` and a fixed `1460px` fix desktop equally — `100vh` was chosen for robustness across viewports, not because it is the only value that works.

- [ ] `npm run type-check` / `lint` / `stylelint` / `test` / `build` clean
- [ ] Local `vite preview` of the built artifact reproduces the fix (baseline shifting, patched build 0/N) — verifying the **real implementation**, not the injected CSS the trials above used
- [ ] Post-deploy on staging: same delayed-chunk harness, 0/N at 1350x940 and 768x1024
- [ ] Visual check that the fallback reads as a deliberate loading state at 375 / 768 / 1350
- [ ] `#how-it-works` still reveals correctly (the #182 backstop contract is untouched)

## Follow-up 2026-07-27 — the first implementation hid the loading text

Staging verification passed on the numbers (see the table above, reproduced there with a working control) but a **screenshot** of the fallback state showed the reserved area as an empty dark band.

Cause: the reservation begins near the bottom of the fold (y≈710 at 1350×940) and the box is a full viewport tall, so `align-items: center` placed the text at **y≈1180 — below the fold.** Users previously saw "Loading dashboard telemetry…" at y=667; they would now see nothing at all while the chunk loads. The centring was meant to stop a screen-tall box looking broken and instead removed the only loading affordance on the page.

Corrected to `align-items: flex-start`, which is the only position that keeps the text on screen: at 1350×940 it sits at y≈710, at 768×1024 at y≈957. At 375×812 the fallback begins at y=1095 and is below the fold either way — unchanged from before this proposal, not a regression.

**Lesson:** the geometry assertions (`h=940px`, `display=flex`, centred) were all green. Only rendering the page and looking at it caught this. Verify visual changes visually.

## Origin

Surfaced 2026-07-27 while confirming `chore-web-hardening` on staging with Lighthouse: production scored CLS 0.245 where every earlier report had recorded 0. Those earlier reports measured `/assets/index-CSNSPx3b.js`; production now serves `index-B47hUzKa.js`, so the baseline they established was never current. Root-caused over ~60 instrumented trials.
