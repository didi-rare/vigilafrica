# Tasks: Finish the Design-Token Migration (chore-design-tokens)

**22 tasks**, in the proposal's order — spacing, typography, z-index, enforcement,
suppressions — as separate commits, each independently green. Stop at any point.

⚠️ **Every slice is verified two independent ways before its commit**: the
computed-value equivalence check (resolve tokens back and compare every
declaration with the original) and the two-arm screenshot diff. Green CI alone
is not evidence for a pure-CSS refactor (#193).

## 1. Spacing scale

- [ ] 1.1 Add the spacing scale to `tokens.css`: step tokens (`--space-0-5` … `--space-32`), the off-grid legacy block, and the semantic aliases `--container-gutter`, `--inset-card`, `--section-py`; move `--container-max` and `--section-py` out of `App.css`.
- [ ] 1.2 Replace every padding / margin / gap literal in the eight component files. Record the count rewritten and anything left unmapped.
- [ ] 1.3 Equivalence check: 0 declarations differ in computed value.
- [ ] 1.4 Screenshot diff vs the base-branch build: 0 / 0 on all nine cells.

## 2. Typography scale

- [ ] 2.1 Add sizes (px-named), the three fluid display sizes, the off-scale legacy sizes, weights, line heights and letter spacing to `tokens.css`.
- [ ] 2.2 Replace every `font-size`, `font-weight`, `line-height`, `letter-spacing` literal.
- [ ] 2.3 Equivalence check: 0 differ.
- [ ] 2.4 Screenshot diff: 0 / 0 on all nine cells.

## 3. z-index

- [ ] 3.1 Move `--z-map-hud`, `--z-nav`, `--z-dropdown`, `--z-skip-link` from `index.css` / `App.css` into `tokens.css`.
- [ ] 3.2 Audit the seven remaining literals; confirm each is local stacking inside a contained context and leave it literal. Record the audit in `.stylelintrc.suppressions.md` alongside the lint range that encodes the policy.
- [ ] 3.3 Screenshot diff: 0 / 0.

## 4. Enforcement

- [ ] 4.1 Extend `scale-unlimited/declaration-strict-value` to the spacing, typography and z-index properties with per-property allow-lists (B4), including shorthand sequences.
- [ ] 4.2 Annotate the two `.sr-only`-pattern `margin: -1px` declarations with a disable comment and reason.
- [ ] 4.3 Re-break test: add one literal per governed family (padding shorthand, gap, font-size, font-weight, line-height, letter-spacing, `z-index: 50`) and confirm `npm run lint:styles` rejects each; then remove them. Record the output.

## 5. Stylelint suppressions

- [ ] 5.1 For each of the 15 disabled rules, run the lint with it enabled, count the findings, and decide: re-enable (autofix where the fix is non-visual), keep with reason, or re-scope.
- [ ] 5.2 Apply the re-enables and re-scopes; confirm the screenshot diff is still 0 / 0 (notation autofixes must not change rendering).
- [ ] 5.3 Write `web/.stylelintrc.suppressions.md` with one entry per remaining suppression.

## 6. Close out

- [ ] 6.1 `npm run lint`, `type-check`, `lint:styles`, `test`, `build` all green.
- [ ] 6.2 Final screenshot diff across all slices: 0 / 0, identical page heights (so the dashboard reservation in `App.css` is untouched and `bench-dashboard-cls` does not need re-running).
- [ ] 6.3 Update `docs/standards/developers-react.md` §7 header, §7.5 and §7.10 to say spacing, type and z-index are machine-checked and the `--z-*` scale lives in `tokens.css`.
- [ ] 6.4 Update `proposal.md` status/branch, and tick the proposal's verification checklist with the evidence.
- [ ] 6.5 Commit the harness (`scripts/bench-design-tokens/`) with its README.
