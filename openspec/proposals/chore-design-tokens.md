---
id: chore-design-tokens
status: proposed
branch: tbd
---

# Proposal: Finish the Design-Token Migration (chore-design-tokens)

> **Supersedes and replaces four separate proposals**, collapsed 2026-08-07:
> `chore-spacing-tokens`, `chore-type-tokens`, `chore-z-index-tokens` and
> `chore-stylelint-suppressions-review`. All four were slices of the same
> [`docs/standards/developers-react.md`](../../docs/standards/developers-react.md)
> §7.5/§7.10 requirement, touching the same two files
> ([`tokens.css`](../../web/src/styles/tokens.css),
> [`.stylelintrc.json`](../../web/.stylelintrc.json)), and each would have been a
> near-identical audit-then-replace-then-enforce PR. Four proposals for one job
> made the backlog look four times as full as it is. Their originals are archived
> under `openspec/archive/proposal-chore-{spacing,type,z-index}-tokens.md` and
> `proposal-chore-stylelint-suppressions-review.md`.

## Why

[chore-css-tokens](../archive/spec-chore-css-tokens.md) closed the **colour** half of §7.5 and left the rest. Component CSS still carries literal `padding: 1.5rem`, `font-size: 0.875rem`, `font-weight: 600`. The cost is the one §7.5 names: theme or density changes mean hunting across files, and every new component picks values by feel rather than from a scale.

The stylelint suppression list belongs in the same piece of work, not beside it: `scale-unlimited/declaration-strict-value` **is** the enforcement mechanism for the tokens, and reviewing the suppressions without extending that rule leaves the tokens unenforced and free to drift straight back.

## Measured 2026-08-07 — the real size of the job

Counted fresh rather than inherited, because the previously circulated figures were wrong:

| slice | raw literals in `web/src/**/*.css` |
|---|---:|
| spacing (`padding`/`margin`/`gap`/`row-gap`/`column-gap`) | **148** |
| `font-size` | **67** |
| `font-weight` | **40** |
| `z-index` | **7** |

⚠️ **The z-index slice is nearly done already, and the correction matters.** A previously recorded figure of "12 raw `z-index:`" was a miscount — there are 12 *declarations*, of which **4 already use tokens** (`--z-skip-link`, `--z-nav`, `--z-map-hud`, `--z-dropdown`) and one is inside a comment.

The 7 remaining literals are `0`, `1`, `2` and `-1` — **local stacking within a contained context**, not global layering. `App.css:545`'s `z-index: -1` is explicitly contained by `isolation: isolate` on its parent. **These are legitimate and should mostly be left alone.** §7.10's target is the z-index arms race between global layers, which the four existing tokens already prevent. Do not tokenise a local `z-index: 1` to satisfy a counter.

`.stylelintrc.json` currently disables **15** rules.

## What Changes

1. **Spacing scale** in `tokens.css` — primitives (`--space-*`) plus semantic aliases (`--gap-inline`, `--gap-stack`, `--inset-card`, …). Replace the 148 literals.
2. **Typography scale** — `--font-size-*`, `--font-weight-*`, `--line-height-*`, `--letter-spacing-*`. Replace the 107 `font-size`/`font-weight` literals.
   ⚠️ The superseded `chore-type-tokens` contained two **false** claims about the font stack, corrected twice in place: the project **self-hosts IBM Plex Sans, IBM Plex Mono and Space Grotesk via `@fontsource`** (imported in `web/src/main.tsx`), and custom web fonts were **adopted, not rejected**. Write the scale against that stack.
3. **z-index** — audit the 7 remaining literals and tokenise **only** those participating in cross-component layering. Document the rest as deliberate local stacking. Expect this slice to produce almost no diff.
4. **Extend `scale-unlimited/declaration-strict-value`** to the spacing and typography properties, with an allow-list for `0`, `auto` and keywords. Without this the migration is cosmetic and reversible.
5. **Review the 15 stylelint suppressions**, each to one of: *re-enable* (original reason gone), *keep with recorded reason* (durable — e.g. Safari `-webkit-backdrop-filter`), or *re-scope* (narrower config — e.g. allow `clip` only for `.sr-only`). JSON has no comments, so durable reasons go in a side `.stylelintrc.suppressions.md`.
   - Likely *re-enable*: `media-feature-range-notation`, `color-hex-length`, `length-zero-no-unit`, `comment-empty-line-before`
   - Likely *re-scope*: `property-no-vendor-prefix`, `clip`

## Sequencing

Do it in **that order, as separate commits**, and stop at any point. Spacing is the largest win; typography is the largest count; z-index is nearly a no-op; the stylelint work only makes sense once the tokens it enforces exist.

⚠️ **Not one PR.** 300+ mechanical CSS edits in a single review is how a visual regression gets waved through — which this project has already experienced once, on this exact surface (#193 shipped a UX regression with 100%-green verification; only a screenshot caught it).

## Verification

- [ ] Visual diff is **zero** at 375/768/1280 px, verified by screenshot comparison — the same protocol `chore-css-tokens` used. Green CI is not sufficient evidence for a pure-CSS refactor.
- [ ] `npm run lint:styles` passes with the extended `declaration-strict-value` rule, i.e. new literals are actually rejected
- [ ] Every remaining raw literal has a recorded reason (local stacking, allow-listed keyword)
- [ ] Accessibility stays at 100 and CLS does not regress — typography changes alter line boxes, so re-run `scripts/bench-dashboard-cls/` if the dashboard's mounted height moves

## Out of Scope

- Dark mode — see [`feat-dark-mode-toggle`](feat-dark-mode-toggle.md). The token work makes it easier and should not absorb it.
- Component API or markup changes. This is CSS values only.
