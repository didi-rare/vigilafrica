---
id: chore-web-dependency-hygiene
status: archived
branch: chore/web-dependency-hygiene
merged_pr: https://github.com/didi-rare/vigilafrica/pull/171
archived_on: 2026-07-23
---

# Proposal: Remove the Stale React Router v5 Types (chore-web-dependency-hygiene)

## Why

`web/package.json` lists `@types/react-router-dom` at `5.3.3` in **`dependencies`**. Three things are wrong with that one line:

1. **Wrong major.** The project is on React Router **v7**, which ships its own types. The v5 `@types` package describes a different API surface.
2. **Wrong section.** [docs/standards/developers-react.md §14.7](../../docs/standards/developers-react.md) — "Build tools, linters, and **type packages** do not go in `dependencies`."
3. **Actively misleading.** With two sets of router types resolvable, an editor can autocomplete against the v5 shapes. Nothing fails loudly; you just get quietly wrong guidance.

Low severity, near-zero effort — the reason to do it is that it is the kind of thing that stays wrong for years because it never breaks a build.

## What Changes

1. Confirm nothing imports from it: `grep -rn "react-router-dom" web/src` and check that all type usage resolves from `react-router-dom`'s own bundled types.
2. `npm uninstall @types/react-router-dom` in `web/`.
3. Commit the resulting `package.json` + `package-lock.json`.
4. Verify: `npm run type-check` (strict, since #169) and `npm run build` both clean. If the type check fails, the dependency was load-bearing after all — stop and reassess rather than reinstating it.
5. Optional in the same pass: `npx depcheck` (§14.10) and record anything else it flags, without acting on it in this change.

**depcheck result (2026-07-22, recorded not acted on):** zero unused *production* dependencies. Four `devDependencies` flagged, three of them false positives — `stylelint-config-standard` and `stylelint-declaration-strict-value` are referenced from `.stylelintrc.json`, and `axe-core` is required by `vitest-axe`; depcheck cannot see config-file or peer references. The fourth, **`@tanstack/eslint-plugin-query`, is genuinely unused** — it is installed but absent from `eslint.config.js`, independently confirming the review finding. Left alone here on purpose: enabling a lint plugin surfaces real findings and needs its own change.

## Out of Scope

- Wiring `@tanstack/eslint-plugin-query`, which is installed but absent from `eslint.config.js`. Same "installed but wrong" smell, but enabling a lint plugin can surface real findings and deserves its own change.
- Any dependency upgrade. This is a removal, not a bump.

## Verification

- [x] `npm run type-check` clean
- [x] `npm run lint` clean
- [x] `npm run build` clean
- [x] `git diff` touches only `web/package.json` and `web/package-lock.json` (plus this proposal's depcheck note)

## Origin

Finding 3 of the Fable-5 review of `docs/standards/developers-{go,react}.md`, 2026-07-22. Recorded in §14.3 as a pending cleanup.
