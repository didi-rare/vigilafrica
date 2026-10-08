---
id: chore-design-tokens
status: in-progress
branch: chore/design-tokens
---

# Spec: Finish the Design-Token Migration (chore-design-tokens)

## Context

[`proposal.md`](proposal.md) collapses four earlier proposals into one job: the
non-colour half of [`developers-react.md`](../../../docs/standards/developers-react.md)
§7.5 and §7.10. [chore-css-tokens](../../archive/spec-chore-css-tokens.md) closed
colours in 2026-05 and left spacing, typography and z-index as "review-enforced
only", which in practice means not enforced. This spec is the technical plan for
closing that gap **without changing a single rendered pixel**.

## Decision Log

| # | Decision | Alternatives | Why |
|---|---|---|---|
| D1 | **Exact-value migration first; consolidation later.** Every literal maps to a token holding the *same* value. Off-grid values (`0.45rem`, `0.9rem` font-size, `3px` chip insets) become tokens too rather than being snapped to the nearest step. | Snap to a clean 4px / modular scale in the same change | Snapping is a redesign, and the proposal's acceptance bar is a zero screenshot diff at three widths. Sub-pixel deltas (0.2–0.4px) re-rasterise glyphs and can move line wraps, which is exactly the "regression waved through on green CI" the proposal warns about. The off-grid tokens are isolated in labelled blocks so the follow-up can snap each one with its own visual review. |
| D2 | **Spacing tokens named by step count** (`--space-6` = 6 × 0.25rem = 24px), including half and quarter steps that exist only because the UI already uses them (`--space-0-75` = 3px). | Named by px (`--space-24`); t-shirt sizes (`--space-md`) | Step naming is the convention developers already know from utility frameworks, and it keeps the distance to the grid visible: `--space-1-8` (7.2px) sits visibly next to `--space-1-75` (7px). T-shirt names cannot hold 30 steps honestly. |
| D3 | **Type sizes named by px equivalent** (`--font-size-14` = 0.875rem), display sizes semantic (`--font-size-display-xl`). | rem-step names; t-shirt names | The design was authored in px-equivalents (11/12/13/14/15/16/17/18/20) and the GOV.UK design system uses the same scheme. Fractional names (`--font-size-14-4` = 0.9rem) make the off-scale legacy sizes self-evidently odd, which is the point. |
| D4 | **Semantic aliases only where one role owns the value**: `--container-gutter`, `--inset-card`, `--section-py`. Everything else references the scale. | A full semantic layer (`--gap-inline`, `--gap-stack`, …) as the proposal sketched | With 8px, 10px and 0.5rem all playing "icon gap" in different components, naming one of them `--gap-inline` would be a false abstraction. Semantic naming belongs with the consolidation in D1's follow-up, when there is one value per role. |
| D5 | **z-index literals in the −1…2 range stay literal, and stylelint allows exactly that range.** The four global layers move to `tokens.css`. | Tokenise every literal (`--z-local-raised: 1`) | The proposal rules this out explicitly: the seven remaining literals are local stacking inside a contained context, not layers. A lint range encodes the policy ("a new global layer must be a token") instead of a counter. |
| D6 | **`media-feature-range-notation` stays disabled**, contrary to the proposal's "likely re-enable". | Re-enable with autofix | Autofix rewrites `(max-width: 768px)` to `(width <= 768px)`, which Safari before 16.4 ignores *entirely* — the whole responsive block silently drops on older iPhones. That is a browser-support floor decision, not a style one, and this change is not the place to lower it. Recorded in `.stylelintrc.suppressions.md`. |
| D7 | **The screenshot harness is committed** (`scripts/bench-design-tokens/`). | Attach screenshots to the PR | Standing rule since the 2026-08-04 reviews: a number that drives a decision ships with a runnable script or it is an assertion. The harness mocks every API call and pins the clock so the two arms differ only in CSS. |
| D8 | **Separate commits per slice**, in the proposal's order, each independently green. | One commit | So review can stop at any point and bisect lands on a slice, not on 300 edits. |

## Components to Touch

### Modified files

| File | Change |
|---|---|
| `web/src/styles/tokens.css` | Add the spacing scale, off-grid spacing block, semantic spacing aliases, type scale (sizes, display sizes, off-scale sizes, weights, line heights, letter spacing) and the four `--z-*` layers. `--container-max` and `--section-py` move here from `App.css`. |
| `web/src/App.css` | Drop the ad-hoc `:root` block (`--container-max`, `--section-py`, `--z-*`); replace every spacing / type literal; the 768px `--section-py` override references `--space-16`. |
| `web/src/index.css` | Drop the ad-hoc `--z-map-hud`. |
| `web/src/components/EventsDashboard.css`, `Map.css`, `Select.css`, `FeedbackPrompt.css`, `web/src/pages/EventDetail.css`, `ForPartners.css` | Replace literals; the two `.sr-only`-pattern `margin: -1px` declarations carry a `stylelint-disable-next-line` with the reason. |
| `web/.stylelintrc.json` | Extend `scale-unlimited/declaration-strict-value` to padding/margin/gap, font-size/weight, line-height, letter-spacing and z-index with per-property allow-lists; re-enable or re-scope reviewed suppressions. |
| `web/.stylelintrc.suppressions.md` | **New.** One entry per remaining suppression: the rule, the verdict (keep / re-scoped), and the durable reason. |
| `scripts/bench-design-tokens/` | **New.** `screenshot-diff.mjs` + README — the two-arm screenshot comparison. |
| `docs/standards/developers-react.md` | §7 header and §7.5 / §7.10 notes: spacing, type and z-index are now machine-checked; the ad-hoc `--z-*` caveat is gone. |

### Untouched

`api/`, all `.tsx`, markup, class names, colours, radii, widths, heights, offsets
(`top`/`left`), transitions. Dark mode (`feat-dark-mode-toggle`) is out of scope.

## Behaviour Contract

- **B1** — Rendered output of `/`, `/for-partners` and `/events/:id` at 375, 768 and 1280px is pixel-identical before and after each slice, measured by `scripts/bench-design-tokens/screenshot-diff.mjs` against a control build of the base branch: **0 perceptual and 0 exact differing pixels**, same page height.
- **B2** — Independently of B1, resolving every `var(--token)` back to its value yields the same computed value as the original literal for every declaration in every touched file (the `verify-equivalence` check in the tasks).
- **B3** — After this change, `npm run lint:styles` rejects a new literal in any of: padding and margin (and their longhands), gap/row-gap/column-gap, font-size, font-weight, line-height, letter-spacing, and any z-index outside −1…2. Proven by re-breaking: a deliberate literal in each family fails the lint before the change is declared done.
- **B4** — Allowed non-token values are explicit: `0`, `auto`, `inherit`/`initial`/`unset` for spacing; `normal` for line-height and letter-spacing; `inherit` for font-size/weight; `-1…2` for z-index; `var(--…)` everywhere, including inside shorthands (`0 var(--space-6)`).
- **B5** — Every remaining literal in a governed property has a recorded reason at the site (`stylelint-disable-next-line … -- reason`) or in `.stylelintrc.suppressions.md`.

## Verification Plan

1. `verify-equivalence` after each slice: 0 declarations differ in computed value.
2. `npm run lint`, `npm run type-check`, `npm run lint:styles`, `npm run test`, `npm run build` in `web/` — all green after each slice.
3. Two-arm screenshot diff after each slice and at the end: 0 / 0 on all 9 cells.
4. Lint re-break test: one literal per governed family, each rejected.
5. CLS: B1 at 0 differing pixels and identical page heights implies the dashboard's mounted height did not move, so `scripts/bench-dashboard-cls/` does not need re-running; this is stated, not assumed, because a page-height mismatch would fail step 3.

## Risks

- **R1** Regex-driven rewrite misses or mangles a declaration. *Mitigation:* the rewrite reports every unmapped declaration; B2 and B3 then catch anything it touched wrongly or left behind.
- **R2** Shorthand values such as `padding: 0 var(--x)` confuse the strict-value rule. *Mitigation:* per-property regex allow-lists that accept a sequence of `0 | auto | var(--…)`, verified by the re-break test rather than assumed.
- **R3** The off-grid token blocks get treated as a design. *Mitigation:* each is labelled "legacy, to be snapped with visual review" and lists where it is used.
