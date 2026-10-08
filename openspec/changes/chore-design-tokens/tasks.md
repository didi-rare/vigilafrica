# Tasks: Finish the Design-Token Migration (chore-design-tokens)

**22 tasks**, in the proposal's order — spacing, typography, z-index, enforcement,
suppressions — as separate commits, each independently green. Stop at any point.

⚠️ **Every slice is verified two independent ways before its commit**: the
computed-value equivalence check (resolve tokens back and compare every
declaration with the original) and the two-arm screenshot diff. Green CI alone
is not evidence for a pure-CSS refactor (#193).

**Status: all 22 complete on branch `chore/design-tokens` (2026-10-08).** Evidence
is recorded inline; the screenshot numbers come from
`scripts/bench-design-tokens/screenshot-diff.mjs` run with `PW_CHANNEL=chrome`
against a control built from `origin/development` at `1d6b92b`, both arms built
with `VITE_API_BASE_URL=https://api.vigilafrica.org`.

**Noise floor (control vs itself):** perceptual 0 on all nine cells; `exact`
up to 369,878 where the software-GL map canvas is visible. So the claim below is
"perceptual 0 and identical page height", never "exact 0".

## 1. Spacing scale — commit `ca7f0b5`

- [x] 1.1 Add the spacing scale to `tokens.css`: step tokens (`--space-0-5` … `--space-32`), the off-grid legacy block, and the semantic aliases `--container-gutter`, `--inset-card`, `--section-py`; move `--container-max` and `--section-py` out of `App.css`.
- [x] 1.2 Replace every padding / margin / gap literal in the eight component files. **156 declarations rewritten** (App 63, EventsDashboard 33, ForPartners 29, EventDetail 17, FeedbackPrompt 6, Map 4, Select 4, index 0); the proposal's 148 was counted differently (it excluded the single-line media-query rules). Left unmapped: the two `margin: -1px` in the visually-hidden pattern, excepted in 4.2.
- [x] 1.3 Equivalence check: **1258 declarations compared, 0 differ.**
- [x] 1.4 Screenshot diff: **perceptual 0 and exact 0 on all nine cells**, page heights identical (375: 19442 / 5636 / 1895 px; 768: 17948 / 4091 / 1734; 1280: 5178 / 3724 / 1271 for home / partners / event).

## 2. Typography scale — commit `df606ec`

- [x] 2.1 Add sizes (px-named), the three fluid display sizes, the off-scale legacy sizes, weights, line heights and letter spacing to `tokens.css`.
- [x] 2.2 Replace every `font-size`, `font-weight`, `line-height`, `letter-spacing` literal. **146 declarations rewritten** (App 76, EventsDashboard 35, ForPartners 17, EventDetail 7, FeedbackPrompt 5, Map 3, Select 3). Zero raw literals remain.
- [x] 2.3 Equivalence check: 1258 compared, 0 differ.
- [x] 2.4 Screenshot diff: perceptual 0, exact 0, all nine cells, heights identical.

## 3. z-index — commit `293dddd`

- [x] 3.1 Move `--z-map-hud`, `--z-nav`, `--z-dropdown`, `--z-skip-link` from `index.css` / `App.css` into `tokens.css`. `App.css` now defines no tokens at all.
- [x] 3.2 Audit the seven remaining literals: `.hero__graticule` 0 / `.hero__inner` 1 (hero background behind hero content), `.signal-dot::after` −1 (under `isolation: isolate`), `.milestone-list::before` 0 / `.milestone::after` 1 (timeline rail behind dots), `.map-marker__badge` 2 / `.map-marker__pointer` 1 (badge over pointer). All local stacking; left literal. Policy recorded in `.stylelintrc.suppressions.md` and encoded as the lint's −1…2 range.
- [x] 3.3 Screenshot diff: perceptual 0 on all nine cells; the single non-zero `exact` (home-768, 369,878) is the noise-floor value exactly.

## 4. Enforcement — commit `bbada17`

- [x] 4.1 Extend `scale-unlimited/declaration-strict-value` with per-property allow-lists (B4), including shorthand sequences such as `0 var(--space-6)`.
- [x] 4.2 Annotate the two `.sr-only`-pattern `margin: -1px` declarations with a disable comment and reason.
- [x] 4.3 Re-break test, 8 of 8 rejected: `padding: 12px 0`, `margin-top: 1.5rem`, `gap: 0 8px`, `font-size: 0.9rem`, `font-weight: 600`, `line-height: 1.7`, `letter-spacing: 0.08em`, `z-index: 50`. The control line `padding: 0 var(--space-6); margin: 0 auto; gap: var(--space-2) 0; z-index: 2; line-height: normal; font-size: inherit` raised no strict-value error.

## 5. Stylelint suppressions — commit `49f6c70`

- [x] 5.1 Census with all 15 enabled: 328 findings — `selector-class-pattern` 102, the three colour-notation rules 61 each (all in `tokens.css`), `comment-empty-line-before` 12, `media-feature-range-notation` 9, `declaration-block-single-line-max-declarations` 6, `rule-empty-line-before` 5, `property-no-vendor-prefix` 4, `no-descending-specificity` 3, `property-no-deprecated` 2, `declaration-property-value-keyword-no-deprecated` 1, `color-hex-length` 1, `custom-property-pattern` 0, `length-zero-no-unit` 0. Verdicts: 14 re-enabled or re-scoped, 1 kept off (`media-feature-range-notation`, support floor — spec D6).
- [x] 5.2 Applied; lint green; equivalence check 1258 compared, 0 differ, 11 declarations reordered with the same values; screenshot diff perceptual 0, exact 0 on all nine cells. ⚠️ The first autofix pass deleted the four `-webkit-` fallbacks because `property-no-vendor-prefix`'s `ignoreProperties` matches the full prefixed name — caught by the declaration count (553 → 549), restored, config corrected, and the rule shown to still reject `-webkit-transform`.
- [x] 5.3 `web/.stylelintrc.suppressions.md` written.

## 6. Close out

- [x] 6.1 `npm run lint` (0 problems), `type-check` (clean), `lint:styles` (0 problems), `test` (10 files, 97 tests passed), `build` (ok).
- [x] 6.2 Final screenshot diff across all slices: perceptual 0, exact 0, all nine cells, every page height identical to the control. The dashboard reservation in `App.css` is untouched and the mounted height did not move, so `bench-dashboard-cls` was not re-run.
- [x] 6.3 `docs/standards/developers-react.md` §7 header, §7.5 and §7.10 updated.
- [x] 6.4 `proposal.md` status/branch updated and its verification checklist ticked with the evidence.
- [x] 6.5 Harness committed: `scripts/bench-design-tokens/screenshot-diff.mjs` + README (including the WebGL, same-`VITE_API_BASE_URL` and noise-floor notes it earned the hard way).

## Not done, deliberately

- Snapping the off-grid values (`--space-0-4` … `--space-5-6`, `--font-size-11-2` … `--font-size-17-6`) to the grid. That is a visual change and gets its own visually-reviewed change; the blocks in `tokens.css` list every site.
- Semantic spacing aliases beyond `--container-gutter`, `--inset-card`, `--section-py` (spec D4).
- Accessibility re-score: the proposal asks for it when "typography changes alter line boxes"; with identical page heights and 0 differing pixels no line box moved.
