---
id: chore-typecheck-test-files
status: proposed
branch: chore/typecheck-test-files
---

# Proposal: Bring Test Files Under the Type Checker (chore-typecheck-test-files)

## Why

**No tool in this repository type-checks a single test file.** Two independent exclusions stack up:

1. `web/tsconfig.app.json` explicitly excludes `src/**/*.test.ts`, `src/**/*.test.tsx`, `src/**/*.spec.ts(x)` and `src/test/**`.
2. Vitest transpiles via **esbuild**, which strips types without checking them.

So `npm run type-check` (CI step since #168) and `npm run build` both skip the test suite entirely, and `npm run test` never type-checks it. A green pipeline says **nothing** about test-file types.

This matters more than it looks, because it undercuts the tests themselves:

- A mock whose shape has drifted from the real API type still compiles. The test passes while asserting against a shape production never produces — the failure mode is a **green test that proves nothing**.
- `src/api/events.ts` types are the single source of truth per §2.8. Tests are the main place that contract gets restated, and it is the one place the compiler isn't watching.
- Strict mode landed in #169 for `src/`. Tests are now the only TypeScript in the repo running unchecked, which is precisely backwards — they are the code that justifies trusting everything else.

## What Changes

Two viable shapes; pick during implementation after measuring:

**Option A — extend the existing project (preferred if fallout is small).**
Drop the `exclude` entries from `tsconfig.app.json` so tests are checked with the same strict settings as `src/`. Requires `types: ["vite/client", "vitest/globals"]` (or explicit imports) so `describe`/`it`/`expect` resolve, plus `@testing-library/jest-dom` matcher types for `setupTests.ts`.

**Option B — a dedicated `tsconfig.test.json` (preferred if fallout is large).**
A third project referenced from `tsconfig.json`, extending the app config, including only test files, with the test-runner types. Keeps app and test type environments cleanly separated and lets the test project start at a lower strictness that gets ratcheted up.

Then:
1. Measure first: `npx tsc -p <config> --noEmit` and count errors by file and code before choosing.
2. Fix the fallout. Expect most of it in hand-rolled mocks and in `vi.mock` factories, which are structurally typed against the real module.
3. Ensure `npm run type-check` covers the new project (`tsc -b` follows project references, so Option B is picked up automatically once referenced).
4. Update `developers-react.md` §2.1 — remove the "test files are not type-checked" warning — and §13, which should state that tests are type-checked and at what strictness.

## Measurement (2026-07-22) — Option A taken, zero fallout

Measured before choosing, as this proposal requires:

- Dropping the four `exclude` entries pulls all **8** test files into the program (23 files total, confirmed via `tsc --showConfig`).
- `tsc -p tsconfig.app.json --noEmit` then reports **0 errors** — at full strict, since #169.
- **Canary confirms the gate is live, not a config no-op.** A deliberately drifted mock (`id: 42` where `id` is a string; `category: 'earthquakes'`, absent from the `EventCategory` union) is rejected with two `TS2322`s — precisely the failure mode this proposal was written to catch. Canary removed after the check.
- `npm run lint`, `npm run test` (8 files / 57 tests) and `npm run build` all clean afterwards.

So **Option A** applies and Option B is unnecessary — no separate `tsconfig.test.json`, no strictness ratchet, no test edits. The predicted mock-drift backlog did not exist.

Two risks in the section below did not materialise and are recorded as closed:

- *"may not be free"* — it was free. Worth noting the reason: the tests were written by contributors following §2.8 (consume the API types, never redeclare a subset), so the mocks were already typed against the real shapes.
- *"`vitest/globals` could mask a missing import"* — moot. All 8 test files already import `describe`/`it`/`expect`/`vi` explicitly from `vitest`, so no global types were added. Codified as new rule §13.0 so it stays that way.

## Risks (as written before measurement — see above for what actually happened)

- **This one may not be free.** Unlike #169, mocks are exactly where loose typing accumulates, so a non-trivial error count is likely. If it is large, land Option B with a reduced strictness for tests and ratchet, rather than a single large PR that mixes a config change with dozens of test edits.
- Adding `vitest/globals` types repo-wide can mask a missing import in non-test code. Prefer explicit `import { describe, it, expect } from 'vitest'` if the project already does that.

## Out of Scope

- Adding new tests, or changing what existing tests assert. This is about type coverage of the code that is already there.
- The Go side. `go test ./...` compiles test files as a matter of course — this gap is TypeScript-specific.

## Verification

- [x] Error count measured and recorded in the PR before any fix is written — 8 test files join the program, **0 errors**
- [x] `npm run type-check` covers test files and is clean
- [x] `npm run test` still green — 8 files / 57 tests; no behaviour changed, only coverage
- [x] A deliberate canary (a mock with a wrong field type) is rejected by `type-check`, proving the gate is live and not a config no-op — two `TS2322`s, canary removed after
- [x] §2.1 warning removed; §13 states the new coverage (and the build/deploy coupling)

## Origin

Finding 4 of the Fable-5 review of `docs/standards/developers-{go,react}.md`, 2026-07-22. Discovered while verifying that strict mode (#169) was genuinely active — the same session established that a clean `type-check` was, at that point, silent about every test file.
