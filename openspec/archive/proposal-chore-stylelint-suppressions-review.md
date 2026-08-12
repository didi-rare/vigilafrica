---
id: chore-stylelint-suppressions-review
status: proposed
branch: tbd
---

> ## 📦 ARCHIVED 2026-08-07 — SUPERSEDED, NOT SHIPPED
>
> Collapsed into a single proposal, **[`chore-design-tokens`](../proposals/chore-design-tokens.md)**, together with the other three token/stylelint chores.
>
> All four were slices of the same `developers-react.md` §7.5/§7.10 requirement, touching the same two files, each a near-identical audit → replace → enforce PR. Four proposals for one job made the backlog look four times as full as it is.
>
> **The work is not cancelled.** Read the successor — it carries freshly measured counts, and it corrects figures that were wrong here.

# Proposal: Periodic Audit of Stylelint Rule Suppressions (chore-stylelint-suppressions-review)

## Why

[web/.stylelintrc.json](web/.stylelintrc.json) suppresses ~14 rules from `stylelint-config-standard` to keep the focus on the colour-literal goal of [chore-css-tokens](openspec/archive/spec-chore-css-tokens.md). Some of those suppressions are durable (vendor prefixes for Safari support); others were deferred because they would have ballooned that PR's scope.

The suppression list is a hidden backlog. A six-month audit lets us re-enable rules whose underlying reasons have evaporated (e.g. browser support changes, code patterns the team has since moved away from).

## What Changes

1. For each suppression in `.stylelintrc.json`, decide:
   - **Re-enable** — the original reason no longer applies; remove the rule from the override block and fix any new violations
   - **Keep, with comment** — durable reason (e.g. Safari `-webkit-backdrop-filter` is non-negotiable). Add an inline JSON comment captured via a side `.stylelintrc.suppressions.md` doc, since JSON has no comments
   - **Re-scope** — rule is mostly fine, but needs narrower configuration (e.g. allow `clip` only in `.sr-only` per accessibility pattern)
2. Sweep candidates likely to flip to "Re-enable":
   - `media-feature-range-notation` — modern syntax is well-supported now
   - `color-hex-length` — stylistic, low-cost to standardise
   - `length-zero-no-unit` — purely cosmetic
   - `comment-empty-line-before` — formatting consistency
3. Sweep candidates likely to flip to "Re-scope":
   - `property-no-vendor-prefix` — keep allowed for the specific Safari prefixes used; reject all others
   - `clip` — restrict to `.sr-only` selector via context-aware config
4. Update `.stylelintrc.json` with the new ruleset and run `npm run lint:styles` until clean

## Out of Scope

- Switching off `stylelint-config-standard` for a different preset (no need)
- Auto-fixing — the tightenings should be human-reviewed, not auto-applied
- Anything outside `.stylelintrc.json` (component CSS structure, naming conventions are §7.3, not lint-enforced)

## Cadence

Run this audit roughly every 6 months, or whenever a new chore-tokens proposal extends the strict-value rule (since that's a natural moment to revisit which other rules to enable).

## Origin

Surfaced during the chore-css-tokens PR description as "Known pre-existing diagnostics (NOT in scope)". Capturing here so the suppressions are tracked rather than forgotten.
