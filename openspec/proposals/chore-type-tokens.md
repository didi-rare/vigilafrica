---
id: chore-type-tokens
status: proposed
branch: tbd
---

# Proposal: Extract Hardcoded Typography Values into Design Tokens (chore-type-tokens)

## Why

[docs/standards/developers-react.md §7.5](docs/standards/developers-react.md) requires typography values to be CSS custom properties. Following [chore-css-tokens](openspec/archive/spec-chore-css-tokens.md) (colours) and `chore-spacing-tokens` (spacing), typography is the third slice that closes §7.5 for atomic visual properties.

Component CSS today has literal `font-size: 0.875rem`, `font-weight: 600`, `line-height: 1.45`, `letter-spacing: -0.01em`. Same drift risk as colours / spacing — every new heading or label picks a value by feel.

## What Changes

1. Audit every `.css` file under `web/src/` for literal `font-size`, `font-weight`, `line-height`, `letter-spacing`, `font-family`
2. Extend [web/src/styles/tokens.css](web/src/styles/tokens.css) with a type scale:
   - Sizes: `--text-xs`, `--text-sm`, `--text-base`, `--text-lg`, `--text-xl`, `--text-2xl`, ... (or numeric scale)
   - Weights: `--weight-regular`, `--weight-medium`, `--weight-semibold`, `--weight-bold`
   - Line-heights: `--leading-tight`, `--leading-normal`, `--leading-relaxed`
   - Letter-spacing (only if used): `--tracking-tight`, `--tracking-normal`, `--tracking-wide`
   - Family: `--font-sans`, `--font-mono` (the project already loads system stacks; capture them as tokens)
3. Replace literals with token references
4. Extend [web/.stylelintrc.json](web/.stylelintrc.json) `scale-unlimited/declaration-strict-value` to cover the typography properties
5. Verify visual diff is zero via Playwright screenshot diff at 375/768/1280 px

## Out of Scope

- Custom web fonts (still rejected — system stack only)
- Responsive type ramps (clamp/fluid sizes) — out of scope until needed
- Dark mode typography overrides — `feat-dark-mode-toggle` handles those once colours+type tokens exist
- Per-locale typography variations

## Origin

Named in chore-css-tokens spec "Out of Scope" as deferred. Captured as a proposal here so the work doesn't depend on memory.
