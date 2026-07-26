---
id: chore-type-tokens
status: proposed
branch: tbd
---

# Proposal: Extract Hardcoded Typography Values into Design Tokens (chore-type-tokens)

## Why

[docs/standards/developers-react.md §7.5](docs/standards/developers-react.md) requires typography values to be CSS custom properties. Following [chore-css-tokens](openspec/archive/spec-chore-css-tokens.md) (colours) and `chore-spacing-tokens` (spacing), typography is the third slice that closes §7.5 for atomic visual properties.

Component CSS today has literal `font-size: 0.875rem`, `font-weight: 600`, `line-height: 1.45`, `letter-spacing: -0.01em`. Same drift risk as colours / spacing — every new heading or label picks a value by feel.

## ⚠️ Corrected 2026-07-26 — this proposal was written against a superseded font stack

This proposal was authored 2026-05-22, **before the Ground Truth rebrand (ADR-015) replaced the system stack with self-hosted web fonts.** Two claims below were stale and are corrected here:

- ~~"the project already loads system stacks"~~ (What Changes §2) — **false.** The project self-hosts **IBM Plex Sans, IBM Plex Mono and Space Grotesk via `@fontsource`**, imported at [`web/src/main.tsx:7-13`](../../web/src/main.tsx).
- ~~"Custom web fonts (still rejected — system stack only)"~~ (Out of Scope) — **false and inverted.** Custom web fonts were *adopted*, not rejected.

**The family half of this proposal is also already DONE**, incidentally, as part of the rebrand: [`tokens.css:88-92`](../../web/src/styles/tokens.css) already defines `--font-display`, `--font-body`, `--font-mono` and a `--font-sans` legacy alias. What remains unbuilt is the **size / weight / line-height / tracking** scale.

## What Changes

1. Audit every `.css` file under `web/src/` for literal `font-size`, `font-weight`, `line-height`, `letter-spacing`, `font-family`
2. Extend [web/src/styles/tokens.css](web/src/styles/tokens.css) with a type scale:
   - Sizes: `--text-xs`, `--text-sm`, `--text-base`, `--text-lg`, `--text-xl`, `--text-2xl`, ... (or numeric scale)
   - Weights: `--weight-regular`, `--weight-medium`, `--weight-semibold`, `--weight-bold`
   - Line-heights: `--leading-tight`, `--leading-normal`, `--leading-relaxed`
   - Letter-spacing (only if used): `--tracking-tight`, `--tracking-normal`, `--tracking-wide`
   - ~~Family~~ — **already done** (`--font-display` / `--font-body` / `--font-mono` / `--font-sans` at `tokens.css:88-92`). Verify only; do not redefine.
3. Replace literals with token references
4. Extend [web/.stylelintrc.json](web/.stylelintrc.json) `scale-unlimited/declaration-strict-value` to cover the typography properties
5. Verify visual diff is zero via Playwright screenshot diff at 375/768/1280 px
6. **Reconcile the weight scale against the shipped font payload** (added 2026-07-26 from the Lighthouse audit). The `@fontsource` imports are the *production cost* of the weight decisions this proposal formalises — they are not separable:
   - **Measured today:** 7 weights imported (`space-grotesk` 500/700; `ibm-plex-sans` 400/500/600; `ibm-plex-mono` 400/500) shipping **~112 KiB of woff2 across 6 files**, which is the *entire* max-critical-path chain (704 ms mobile / 653 ms desktop) and ~15% of a 736 KiB mobile page.
   - **Measured usage in `web/src/**/*.css`:** 4 distinct weight values — `600` (×15), `500` (×12), `700` (×10), `400` (×1).
   - **Lead worth checking:** Lighthouse loaded **5 `ibm-plex-*` files (= all 5 imported) but only 1 `space-grotesk` file (of 2 imported)** — so one Space Grotesk weight never loaded during a full page render. Candidate for deletion.
   - **Latent-bug check:** Space Grotesk is imported at 500/700 only, but `font-weight: 600` appears 15 times. Cross-reference which of those selectors resolve to `var(--font-display)` — any that do are being **faux-bolded/substituted by the browser**, not rendered in the intended weight.
   - Deliverable: each `--weight-*` token maps to a weight that is actually imported for the families that use it, and every import is reachable. Delete unreachable imports.

## Out of Scope

- ~~Custom web fonts (still rejected — system stack only)~~ — **VOID, see the correction above.** Self-hosted `@fontsource` web fonts are the shipped stack (ADR-015). *Changing the typefaces* remains out of scope; *trimming unused weights of the existing typefaces* is now in scope via What Changes §6.
- Responsive type ramps (clamp/fluid sizes) — out of scope until needed
- Dark mode typography overrides — `feat-dark-mode-toggle` handles those once colours+type tokens exist
- Per-locale typography variations
- Font **loading strategy** (`preload` hints, subsetting, `size-adjust` metric overrides). Lighthouse shows `font-display` already passing and the fonts are not render-blocking, so this is a separate, lower-value change. Only the *number of weights shipped* belongs here.

## Sequencing note

The maintainer's recorded decision is to **collapse the four design-token chores into ONE change** (`chore-type-tokens` + `chore-spacing-tokens` + `chore-z-index-tokens` + `chore-stylelint-suppressions-review` — same shape, two already partial). Treat this file as one slice of that combined change, not a standalone PR.

## Origin

Named in chore-css-tokens spec "Out of Scope" as deferred. Captured as a proposal here so the work doesn't depend on memory. **Amended 2026-07-26** after a production Lighthouse audit (desktop 99 / mobile 70) surfaced the font payload, and a read of the shipped code found two claims in this file contradicted by the rebrand that landed after it was written.
