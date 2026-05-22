---
id: feat-dark-mode-toggle
status: proposed
branch: tbd
---

# Proposal: Dark / Light Mode Toggle (feat-dark-mode-toggle)

## Why

The frontend currently ships with a single dark theme baked in. The CSS-token refactor in [chore-css-tokens](openspec/archive/spec-chore-css-tokens.md) laid the infrastructure required for theme switching ([developers-react.md §7.9](docs/standards/developers-react.md)):

> Dark mode via `data-theme="dark"` on `<html>` with CSS custom property overrides.

With semantic tokens now in place, a light theme is mostly an override block on `[data-theme="light"]` plus a toggle UI — no per-component CSS changes needed.

## What Changes

1. Add a `[data-theme="light"]` override block in [web/src/styles/tokens.css](web/src/styles/tokens.css) that re-maps the semantic tokens (`--bg-primary`, `--text-primary`, `--accent-amber`, etc.) to light-mode equivalents while keeping the primitive palette unchanged
2. Default to `data-theme="dark"` on `<html>` to preserve the current rendering for unset preferences
3. Detect `prefers-color-scheme` from the browser; fall back to dark if unspecified
4. Add a small toggle UI in the dashboard header (sun/moon icon, accessible label)
5. Persist user preference in `localStorage` (`vigilafrica:theme`)
6. Add Playwright screenshot tests at both themes × the same three breakpoints (375/768/1280) to lock the contract
7. Update the staging banner / disclaimer / freshness colours to verify they still meet WCAG AA contrast under the light theme

## Dependencies

- [x] `chore-css-tokens` (colour tokens) — landed
- [ ] `chore-type-tokens` (typography tokens) — optional but recommended; light theme may need slightly different font weights
- [ ] `chore-z-index-tokens` — independent

## Out of Scope

- High-contrast / accessibility theme (separate `feat-high-contrast-theme` if pursued)
- Automatic time-of-day switching
- Per-component theme overrides (the architecture rejects this — all overrides go through the `[data-theme="*"]` block on `tokens.css`)
- Theme transition animations (instant switch is fine)

## Origin

Named in chore-css-tokens spec "Out of Scope" as the named follow-up that the colour-token infrastructure enables.
