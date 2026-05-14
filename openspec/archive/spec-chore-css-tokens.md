---
id: chore-css-tokens
status: proposed
branch: tbd
---

# Spec: Extract Hardcoded CSS Colours into Design Tokens (chore-css-tokens)

## Context

Surfaced by `/openspec-review` of [fix-public-trust-quick-wins](proposal-fix-public-trust-quick-wins.md) (finding F1). [docs/standards/developers-react.md §7.5](docs/standards/developers-react.md) requires CSS custom properties for colours/spacing/typography/z-index, but the codebase currently has hardcoded colour values across multiple component CSS files. This spec narrows the scope to **colours only** and refactors them into a token layer without changing visual output.

Companion: [openspec/archive/proposal-chore-css-tokens.md](proposal-chore-css-tokens.md).

## Decision Log

| # | Decision | Alternatives | Why |
|---|---|---|---|
| D1 | Colours only in this PR | Colours + spacing + typography + z-index at once | Spacing/typography refactors require visual diff review per page; bundling collapses risk into one PR. Stage them. |
| D2 | Semantic + primitive token layers | Single-layer named palette | Future theme overrides update primitives once; semantic tokens map intent to current palette. Standard two-layer design system pattern. |
| D3 | Add `stylelint-declaration-strict-value` to CI | Trust review | Lint enforcement prevents new violations after the cleanup |
| D4 | Zero visual change | Modernise palette while refactoring | Refactor + redesign in one PR makes review impossible. Keep them separate. |

## Components to Touch

### New files

| File | Purpose |
|---|---|
| `web/src/styles/tokens.css` | Two-layer token definitions: primitive palette (`--amber-400: ...`) and semantic tokens (`--color-warn: var(--amber-400)`). Imported once from `web/src/main.tsx` or `App.tsx`. |

### Modified files

| File | Change |
|---|---|
| `web/src/index.css` (if exists) or `App.css` | Import the new `tokens.css`; remove any duplicate `:root` declarations |
| `web/src/components/EventsDashboard.css` | Replace literals in `freshness-banner--ok/warn/error` and `dashboard-disclaimer` with token references |
| `web/src/components/Map.css` | Replace literals (audit during execution) |
| `web/src/pages/EventDetail.css` | Replace literals (audit during execution) |
| `web/src/App.css` | Replace literals in audience cards, status indicators, footer, staging banner |
| `web/package.json` | Add `stylelint` + `stylelint-config-standard` + `stylelint-declaration-strict-value` as devDependencies |
| `.github/workflows/ci-cd.yml` (or new `lint-styles.yml`) | Add `npm run lint:styles` step |

### Untouched

`api/`, all backend code, deployment, all non-CSS frontend code.

## Behaviour Contract

- **B1** — Rendered colour values across `https://vigilafrica.org`, `https://staging.vigilafrica.org`, and `npm run dev` MUST be visually identical before and after this PR (screenshot diff acceptable), with one documented exception: three previously-undefined CSS vars used by `Map.css` and `EventDetail.css` — `--color-text-dim`, `--color-primary`, `--color-border` — are now bound via the legacy-alias block in `tokens.css`. Before this PR they fell through to inherited / `currentColor`; after, they resolve to `--text-muted`, `--accent-amber`, and `--border` respectively. The intentional effect on `/events/:id` and map popups is that section/field labels render in muted grey, section headers render in the amber accent, and borders render as faint white instead of `currentColor`. Verified by local screenshot diff at 375 / 1280 px on 2026-05-14.
- **B2** — No `.css` file under `web/src/` MAY contain hardcoded `#hex`, `rgb()`, `rgba()`, `hsl()`, or `hsla()` colour literals after this PR (with the limited exception of `:root` / `tokens.css` itself)
- **B3** — Adding a hardcoded colour literal in a future PR MUST fail `npm run lint:styles` in CI
- **B4** — Theme switching via `[data-theme="dark"]` (§7.9) MUST work by overriding the semantic tokens only — no component-level CSS edits required to support a future dark theme PR

## Phase 1 — Token Layer + Audit

- [ ] Create `web/src/styles/tokens.css` with primitive + semantic layers
- [ ] Audit each `.css` file under `web/src/` for colour literals (suggested: `grep -rE "#[0-9a-fA-F]{3,8}|rgba?\(" web/src/**/*.css`)
- [ ] Map each literal to a token (introduce new tokens if needed)
- [ ] Replace literals; verify visual diff is zero via local screenshots

## Phase 2 — Lint Enforcement

- [ ] Add stylelint + plugins as devDependencies (pin exact versions per §14.5)
- [ ] Add `.stylelintrc.json` enforcing `declaration-strict-value` on colour properties (`color`, `background`, `border-color`, etc.) — exemption for `tokens.css` itself
- [ ] Add `lint:styles` npm script
- [ ] Add CI step in `ci-cd.yml`
- [ ] Verify CI catches a deliberately-added violation (manual test before merge)

## Acceptance Criteria

- [ ] `npm run lint:styles` exits 0 against the full diff
- [ ] `grep -rE "#[0-9a-fA-F]{3,8}|rgba?\(" web/src/**/*.css | grep -v tokens.css` returns empty (or only intentional exemptions like inline SVG fills)
- [ ] Visual screenshot diff at 375px, 768px, 1280px for homepage, dashboard, and event-detail shows no perceptible change
- [ ] §7.5 of `developers-react.md` no longer requires a "known violation" caveat in reviews

## Out of Scope (reaffirmed)

- Spacing / typography / z-index token extraction (future `chore-spacing-tokens`, `chore-type-tokens`, `chore-z-index-tokens`)
- Dark mode toggle (uses the new tokens; ships as a separate `feat-dark-mode-toggle` once these tokens exist)
- Palette redesign or contrast adjustments
- Tailwind / CSS-in-JS migration (rejected by ADR-013)

## Risks

- **R1**: Token extraction introduces subtle visual drift (rounding, opacity layering). **Mitigation**: explicit before/after screenshots at all three breakpoints captured in the PR.
- **R2**: `stylelint-declaration-strict-value` flags acceptable cases (e.g. `transparent`, `currentColor`, `inherit`). **Mitigation**: configure the lint rule's `ignoreValues` for those keywords.
- **R3**: Token names diverge from the actual rendered colour over time. **Mitigation**: primitive tokens map to a single source palette, semantic tokens reference primitives — drift only happens if someone manually edits the semantic mapping, which the lint can't catch but PR review can.

## Verification Plan

1. Local screenshot diff via Vite preview (homepage, dashboard, event detail) at 375 / 768 / 1280 px
2. `npm run lint:styles` passes
3. `npm run test` passes (no test changes expected)
4. `npm run build` succeeds
5. Manual deploy preview via PR review

No new automated tests required — the lint rule IS the regression guard.
