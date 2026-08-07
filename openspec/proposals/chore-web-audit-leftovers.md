---
id: chore-web-audit-leftovers
status: proposed
branch: tbd
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

## Verification

- [ ] Lighthouse on **staging**: "Avoid non-composited animations" reports **0 elements** (currently 1)
- [ ] `prefers-reduced-motion` still suppresses the staging-banner animation after the selector moves to `::before`
- [ ] Screen-reader announcement fires on entering and leaving both loading states
- [ ] A loading affordance is visible above the fold at 375×812
- [ ] `npm run type-check` / `lint` / `test` / `build` clean; CLS still **0** at 1350×940, 768×1024, 375×812

## Origin

Accepted-not-fixed findings from the 2026-07-26 production web audit (#189–#191, #193, #195) and the independent three-reviewer pass that followed (#197, #198). Recorded here on 2026-08-03 during a backlog cleanup, because until now they existed only in PR text and memory.
