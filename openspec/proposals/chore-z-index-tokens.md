---
id: chore-z-index-tokens
status: proposed
branch: tbd
---

# Proposal: Extract Hardcoded z-index Values into Design Tokens (chore-z-index-tokens)

## Why

[docs/standards/developers-react.md §7.10](docs/standards/developers-react.md) is explicit:

> z-index values are named custom properties in `index.css`. Never hardcode a `z-index` value in a component file.

with the example pattern:

```css
:root { --z-modal: 300; --z-nav: 200; --z-map-controls: 100; }
```

Today, z-index values are scattered as literals in component CSS, which produces the classic z-index-arms-race antipattern: every new overlay needs to be eyeballed against the highest number anyone remembers using.

## What Changes

1. Audit every `.css` file under `web/src/` for literal `z-index:` declarations
2. Add a small named scale to [web/src/styles/tokens.css](web/src/styles/tokens.css):
   - `--z-base: 0`
   - `--z-elevated: 10`
   - `--z-sticky: 100`
   - `--z-map-controls: 200`
   - `--z-nav: 300`
   - `--z-popup: 400`
   - `--z-modal: 500`
   - `--z-toast: 600`
   - Adjust the actual numbers to match the existing z-index values discovered during the audit
3. Replace literal `z-index:` with `var(--z-*)` references
4. Extend [web/.stylelintrc.json](web/.stylelintrc.json) `scale-unlimited/declaration-strict-value` to cover `z-index`
5. Verify the stacking order is unchanged (no rendering regressions on popups, dropdowns, map overlays, modals)

## Out of Scope

- Stacking-context refactors (using `isolation: isolate` to scope z-index locally) — that's a deeper redesign
- z-index for transient elements like `--z-drag-preview` — add tokens only for what exists

## Origin

Named in chore-css-tokens spec "Out of Scope" as deferred, and is the §7.10 follow-up to the §7.5 colour work.
