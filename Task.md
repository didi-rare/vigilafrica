# chore-css-tokens

**Branch:** `chore/css-tokens`
**Spec:** [openspec/archive/spec-chore-css-tokens.md](openspec/archive/spec-chore-css-tokens.md) (archived 2026-05-14)
**Proposal:** [openspec/archive/proposal-chore-css-tokens.md](openspec/archive/proposal-chore-css-tokens.md) (archived 2026-05-14)
**Origin:** finding F1 from `/openspec-review` of fix-public-trust-quick-wins

## Phase 1 — Token Layer + Audit

- [x] Create [web/src/styles/tokens.css](web/src/styles/tokens.css) with two-layer model: primitive palette + semantic tokens
- [x] Import tokens.css from [web/src/main.tsx](web/src/main.tsx) before any component CSS
- [x] Define legacy aliases (`--color-text-dim`, `--color-primary`, `--color-border`) so orphan references in Map.css and EventDetail.css resolve correctly
- [x] Replace every colour literal in [web/src/index.css](web/src/index.css) → tokens
- [x] Replace every colour literal in [web/src/App.css](web/src/App.css) → tokens (removed embedded colour `:root` block; kept non-colour tokens for typography/spacing/z-index)
- [x] Replace every colour literal in [web/src/components/EventsDashboard.css](web/src/components/EventsDashboard.css) → tokens
- [x] Replace every colour literal in [web/src/components/Map.css](web/src/components/Map.css) → tokens
- [x] Replace every colour literal in [web/src/pages/EventDetail.css](web/src/pages/EventDetail.css) → tokens

## Phase 2 — Lint Enforcement

- [x] Add `stylelint@17.11.0`, `stylelint-config-standard@40.0.0`, `stylelint-declaration-strict-value@1.11.1` to [web/package.json](web/package.json) devDependencies (pinned exact)
- [x] Add [web/.stylelintrc.json](web/.stylelintrc.json) with `scale-unlimited/declaration-strict-value` enforcing token references on every colour-bearing property
- [x] Add `lint:styles` npm script to [web/package.json](web/package.json)
- [x] Add `Run Frontend Style Lint` step to [.github/workflows/ci-cd.yml](.github/workflows/ci-cd.yml)
- [x] Sanity-tested: deliberately-added `color: #abc` flagged by stylelint with `scale-unlimited/declaration-strict-value` ✓
- [x] tokens.css exempted from the strict-value rule via `overrides`

## Phase 3 — Verification

- [x] `npm run lint` clean
- [x] `npm run lint:styles` clean
- [x] `npm run test` — 31/31 passing (no test changes expected; visual-only refactor)
- [x] `npm run build` succeeds
- [x] Visual diff captured via Playwright at 375 / 768 / 1280 px on 2026-05-14; B1 exception documented in spec (three orphan CSS vars now bound; section labels render in muted grey + amber accent on `/events/:id`)

## Follow-up specs (NOT in this PR)

These were named in the spec's "Out of Scope" and are explicitly deferred:

- `chore-spacing-tokens` — extract spacing literals
- `chore-type-tokens` — extract typography literals
- `chore-z-index-tokens` — extract z-index literals
- `feat-dark-mode-toggle` — uses the new colour tokens once they're in place
