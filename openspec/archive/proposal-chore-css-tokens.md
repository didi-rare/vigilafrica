# Proposal: Extract Hardcoded CSS Colours into Design Tokens (chore-css-tokens)

**Status:** Archived 2026-05-14 — surfaced by `/openspec-review` of [fix-public-trust-quick-wins](proposal-fix-public-trust-quick-wins.md) as finding **F1**.

## Why

[docs/standards/developers-react.md §7.5](docs/standards/developers-react.md) requires:

> Colours, spacing, typography, and z-index are CSS custom properties defined in `index.css`. Never hardcode values in component CSS.

This rule is currently violated across the existing codebase. Examples found during the trust-quick-wins review:

- `web/src/components/EventsDashboard.css` — `rgba(245, 158, 11, 0.08)`, `#fbbf24`, `rgba(244, 63, 94, 0.08)`, `#fb7185`, `rgba(34, 197, 94, 0.08)`, `#86efac` in `freshness-banner--*` variants and the new `dashboard-disclaimer`
- `web/src/App.css` — various amber/cyan literals in audience cards, status indicators, footer
- `web/src/pages/EventDetail.css` — pre-existing literals

The violations are pre-existing and cosmetic (the styles render correctly), but they:

1. Make theming brittle — changing the "warning amber" colour requires hunting through multiple files
2. Block a future dark/light theme switch (the codebase already supports `[data-theme="dark"]` per §7.9 but the tokens it would override don't exist)
3. Compound — every new component that copies the existing pattern adds more hardcoded values

This proposal does NOT introduce visual change. It refactors literals into named tokens so future style work has a single source of truth.

## What Changes

A single PR that:

1. Audits every `.css` file under `web/src/` for hardcoded colour values (`#...`, `rgba(...)`, `hsl(...)`)
2. Introduces a `web/src/styles/tokens.css` (or extends `index.css`) defining named tokens:
   - Semantic tokens: `--color-warn`, `--color-warn-bg`, `--color-warn-border`, `--color-error`, `--color-ok`, `--color-info`
   - Primitive tokens: `--amber-400`, `--rose-400`, `--green-400`, etc. as the source palette
3. Updates each `.css` file to reference tokens instead of literals
4. Adds a CI lint rule (e.g. `stylelint-declaration-strict-value`) to fail builds that introduce new hardcoded colours
5. Does NOT change any rendered colour — visual diff should be zero

## Out of Scope

- Visual redesign or palette change
- Dark mode toggle (the infrastructure is in §7.9 already; this proposal lays the groundwork but doesn't ship the toggle)
- Spacing / typography / z-index token extraction (§7.5 covers those too, but they're a separate refactor with different risk profile — track as follow-up `chore-spacing-tokens` if desired)
- Tailwind / CSS-in-JS migration (ADR-013 explicitly rejects these)
- Any component behavioural change

## User Impact

Zero user-visible change. Internal contributor experience improves:

- Adding a new "warning"-styled component becomes `background: var(--color-warn-bg)` instead of "look up the colour someone else used and hope it matches"
- Future theme work (dark mode, high-contrast mode) has a single place to override
- Standards rule §7.5 stops being a known-violated rule, restoring its weight in `/openspec-review`

## How This Was Surfaced

During `/openspec-review` of `fix-public-trust-quick-wins`, finding F1 noted that the new `dashboard-disclaimer` and `freshness-banner--ok` CSS introduced literals matching the existing pattern. The decision was to ship the trust-quick-wins PR as-is (matching existing style) and track the codebase-wide cleanup as this separate proposal.
