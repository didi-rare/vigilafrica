---
id: chore-web-audit-leftovers
status: in-progress
branch: claude/chore-web-audit-leftovers-xt5v6i
spec: ../specs/chore-web-audit-leftovers.md
---

# Proposal: Close the Accepted Leftovers From the Web-Audit Batch (chore-web-audit-leftovers)

## Why

The 2026-07-26 web audit and the independent review that followed produced a set of findings that were **consciously accepted rather than fixed**, so v1.3.6 could ship. They were recorded in PR descriptions, commit messages and memory — but **not in the backlog**, which means they exist nowhere a future contributor would look.

This project has a documented failure mode of exactly this shape: real work that lives only in a source comment or a conversation and is therefore invisible (see `chore-post-v11-deferred-b6`, outstanding since v1.1). This proposal exists so these five do not join it.

None is urgent. Together they are one small PR.

## What Changes

### 1. `#staging-banner::before` still animates `box-shadow`

The last non-composited animation on the site. Lighthouse on production now reports **0 animated elements**, because the staging banner does not exist there — but on staging it is the one remaining offender, and it is the *same defect class* fixed for `.signal-dot` in #191.

Fix identically: pseudo-element ring animating `transform`/`opacity`, with the `prefers-reduced-motion` selector following the animation onto `::before`. ⚠️ That selector move is the exact regression that nearly shipped in #191 — see its notes.

### 2. Synthetic-user-agent regex is an unanchored substring match

`SYNTHETIC_USER_AGENT = /Chrome-Lighthouse|HeadlessChrome|PageSpeed/i` in [`web/src/analytics.ts`](../../web/src/analytics.ts) matches anywhere in the UA string. A real browser whose UA happens to contain one of those tokens (corporate proxies have been observed appending diagnostic strings) would be **silently excluded from analytics**.

Raised independently by two reviewers. **False-positive rate is unquantified** — that is precisely why it is uncomfortable: traffic dropped this way is invisible by construction.

Options: anchor more tightly, log-without-suppressing for a period to measure, or accept and document. **Measure before changing** — the current behaviour may well be fine.

### 3. No `aria-live` on either loading region

Neither `.dashboard-fallback` (Suspense boundary) nor `.dashboard-state.loading` (data fetch) carries `role="status"`/`aria-live`. Screen-reader users get no announcement that content is loading or has arrived. `FreshnessIndicator` in the same file already does this correctly — the pattern exists and is simply not applied here.

Pre-existing, not introduced by the batch, but #193/#195 touched these exact lines without adding it.

### 4. No visible loading affordance at phone widths

At 375×812 the dashboard fallback begins at **y≈1098** with a viewport of 812 — below the fold. A phone user on a slow connection sees the hero, scrolls, and hits blank space with no indication anything is loading.

Unchanged from before #193 (the taller mobile hero already pushed it down), so **not a regression** — but it is the worst-served case for the audience this product targets, and #198 deliberately removed the reservation at ≤480px, which makes it slightly more visible as a gap.

### 5. Two loading affordances for the same wait

The Suspense boundary shows plain text; the inner data-fetch state shows a spinner. A user sitting through both sees two different treatments for what is, to them, one wait. Reusing the existing spinner in the outer fallback would also improve item 4.

## Out of Scope

- **CSP nonces / Trusted Types.** Accepted risk, documented in `chore-web-hardening` — a static Vite build on Vercel has no per-request nonce mechanism, and `script-src` has already silently broken analytics once.
- **HSTS `preload`.** Deliberately excluded pending explicit sign-off; submission is effectively irreversible.
- **`label-content-name-mismatch`.** Fails with the same 2 items on production *and* pre-batch staging — pre-existing, weight 0, and unrelated to this work. Worth its own look, not this one.

## Resolution (branch `claude/chore-web-audit-leftovers-xt5v6i`)

Technical design in [`openspec/specs/chore-web-audit-leftovers.md`](../specs/chore-web-audit-leftovers.md); task-level evidence in the root `Task.md`.

| # | outcome |
|---|---|
| 1 | **Fixed.** Stripe stays static on `::before`; the glow is a static `box-shadow` on a new `::after` layer that animates `opacity` only. Reduced-motion rule moved with it. |
| 2 | **Accepted and documented, regex unchanged.** Anchoring does not address the described failure (an appended proxy token is a whole token either way); a log-only path would be a seventh event the module refuses by design; the rate cannot be measured from this repo (Umami stores a parsed browser name, not the raw UA). Decision recorded at the regex; a test pins it on the one observable input — a token embedded in a larger word is suppressed — so anchoring later would fail the test rather than drift. |
| 3 | **Fixed.** New `LoadingState` component — `role="status"`, `aria-live="polite"`, decorative spinner, visible message as the live text — used by the outer fallback and the data-fetch state. The map's Suspense fallback uses the same treatment with `announce={false}` so one wait is one announcement. `developers-react.md` §9.8 amended to match (it prescribed `aria-busy` on the container, which can suppress the announcement). |
| 4 | **Fixed.** `DashboardFallback` renders a fixed, zero-footprint 3px progress bar (`.dashboard-fallback__progress`, `transform`-only, static under reduced motion, z-index reusing `--z-dropdown` per §7.10) for as long as the chunk is pending. Measured in view at 375×812; the in-flow card is still below the fold there, as before. |
| 5 | **Fixed.** The outer fallback renders the same `LoadingState` card as the inner states. The component carries its own stylesheet, which `App.tsx` importing it puts in the eager bundle — the dashboard's CSS only arrives with the lazy chunk, which is the reason the outer fallback was text-only in the first place. |

Not done, recorded: the `EventDetail` loading region has the same two defects and was not in this proposal; the hero CTA's `#dashboard` anchor does not exist while the chunk loads.

**Review rounds.** `/openspec-review` plus two independent adversarial reads (one against the first commit, one against the branch head) produced: §7.2 co-located stylesheet, §7.3/§7.10 naming and z-index reuse, §2.5 props type, §13.3 test queries, a fallback-state test that did not exist, a margin collapsing through `.dashboard-fallback` that grew the reservation 32px past its cap, an analytics test that pinned nothing, two bench scripts that printed but never failed, a chatty map-fallback announcement, and a standards rule the component contradicted. All fixed on the branch; none was found by the author's own pass.

## Verification

- [x] Lighthouse 13.5.0 "Avoid non-composited animations" on a local **staging** build (mobile and desktop presets): control `origin/development` @ `3040a00` → **1 animated element** (`::before`, "Unsupported CSS Property: box-shadow"); branch → **not applicable, 0 elements**. The deployed staging URL is not reachable from this repo (nothing here deploys), so the control run is the positive control that the audit sees the defect.
- [x] `prefers-reduced-motion` still suppresses the glow after the selector moved to `::after`: `scripts/bench-dashboard-cls/check-reduced-motion.mjs` reads computed `animation=none … opacity=0` under `reduce` and `staging-banner-stripe-pulse ×infinite 2.5s` under `no-preference`, and exits non-zero otherwise (negative control: run against a production build it exits 1)
- [x] Both loading states are polite `role="status"` live regions whose text is the loading message; entering is the region mounting, leaving is announced by what replaces it (the result-count live region, or the error `role="alert"`). Asserted in `LoadingState.test.tsx`, `App.fallback.test.tsx` and `EventsDashboard.test.tsx`; **not** run against a real screen reader — the spec §3 records the fallback pattern to reach for if a manual pass shows the mount-time announcement is missed.
- [x] A loading affordance is visible above the fold at 375×812: `capture-fallback.mjs` places `.dashboard-fallback__progress` at y 0–3 inside an 812px viewport with the chunk held back (and exits non-zero if the chunk mounts or an element is missing); the control arm has no element in view (text-only fallback at y 1032).
- [x] `npm run lint` / `type-check` / `lint:styles` / `test` (106/106) / `build` clean
- [x] CLS A/B against the control build, `measure-cls.mjs` with the new `VIEWPORT` override, 8 runs per cell — table below. The proposal's "still 0" was never literally true: the control carries the pre-existing freshness-banner residual at 1920×1600 that #221 recorded. The bar is what this change must not move, and it does not.

| viewport | control (`3040a00`) | branch |
|---|---|---|
| 1920×1600 | 0.0054 (8/8 shifting) | 0.0054 (8/8) |
| 1350×940 | 0.0001 (0/8) | 0.0001 (0/8) |
| 768×1024 | 0.0002 (0/8) | 0.0002 (0/8) |
| 375×812 | 0.0003 (0/8) | 0.0003 (0/8) |

Identical in every cell to four decimals. The sub-0.001 residual at the three audit viewports is the nav (`nav-station`, `nav-actions`) settling as fonts load, present on the control too; the 1920×1600 residual is the freshness banner. The progress bar never appears as a shift source, as a fixed element cannot.

⚠️ Table measured before the `flow-root` margin fix (review finding). Re-measure after it: **in progress** — 1920×1600 so far: control 0.0054 (8/8), branch 0.0054 (6/8, two runs at 0.0000). Remaining cells to follow in the next commit.

## Origin

Accepted-not-fixed findings from the 2026-07-26 production web audit (#189–#191, #193, #195) and the independent three-reviewer pass that followed (#197, #198). Recorded here on 2026-08-03 during a backlog cleanup, because until now they existed only in PR text and memory.
