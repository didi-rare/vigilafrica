---
id: chore-spacing-tokens
status: proposed
branch: tbd
---

# Proposal: Extract Hardcoded Spacing Values into Design Tokens (chore-spacing-tokens)

## Why

[docs/standards/developers-react.md §7.5](docs/standards/developers-react.md) requires spacing values to be CSS custom properties defined in `index.css`, mirroring the rule for colours. The recently-landed [chore-css-tokens](openspec/archive/spec-chore-css-tokens.md) closed the colour half of §7.5; spacing is the next slice.

Component CSS today still contains literal `padding: 1.5rem`, `gap: 0.75rem`, `margin-top: 2rem` etc., which produces the same problems §7.5 calls out:
- Theme/density tweaks (e.g. compact mode) need to hunt across files
- New components copy nearby spacing patterns by chance rather than by design
- No lint enforcement catches drift

## What Changes

1. Audit every `.css` file under `web/src/` for hardcoded spacing values (`rem`, `px`, `em`, `%`) on `padding*`, `margin*`, `gap`, `row-gap`, `column-gap`, `top/right/bottom/left` properties
2. Extend [web/src/styles/tokens.css](web/src/styles/tokens.css) with a spacing scale:
   - Primitives: `--space-0` through `--space-12` (or equivalent — t-shirt sizes are an alternative)
   - Semantic: `--gap-inline`, `--gap-stack`, `--inset-card`, `--inset-button`, etc.
3. Replace literals with token references
4. Extend [web/.stylelintrc.json](web/.stylelintrc.json) `scale-unlimited/declaration-strict-value` to cover the spacing properties (with allow-list for `0`, `auto`, `currentColor`-style keywords)
5. Verify visual diff is zero via Playwright screenshot diff at 375/768/1280 px (same protocol as chore-css-tokens)

## Out of Scope

- Typography (separate `chore-type-tokens` proposal)
- z-index (separate `chore-z-index-tokens` proposal)
- Layout primitive components (Stack, Inline, Box) — token extraction first, components later
- Density modes or responsive scale changes

## Origin

Named in chore-css-tokens spec "Out of Scope" as deferred. Captured as a proposal here so the work doesn't depend on memory.
