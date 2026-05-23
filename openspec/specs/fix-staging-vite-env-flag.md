---
id: fix-staging-vite-env-flag
status: proposed
branch: fix/staging-vite-env-flag
---

# Spec: Wire `VITE_ENV=staging` into Vercel Staging Deploy (fix-staging-vite-env-flag)

## Context

PR #76 shipped two build-time features that depend on a `VITE_ENV` flag:

1. The `robotsMetaPlugin` in [web/vite.config.ts:9-21](web/vite.config.ts#L9-L21) rewrites `<meta name="robots">` to `noindex, nofollow` when `process.env.VITE_ENV === 'staging'`.
2. The `StagingBanner` component in [web/src/App.tsx:90-102](web/src/App.tsx#L90-L102) renders the "test environment" notice when `import.meta.env.VITE_ENV === 'staging'`.

Live validation on 2026-05-14 confirmed both fail on `staging.vigilafrica.org` — the staging Vercel project doesn't set `VITE_ENV=staging` at build time. The `/007` audit on 2026-05-22 re-verified: `curl https://staging.vigilafrica.org/` still returns `<meta name="robots" content="index, follow">`. Search engines have been free to index staging, diluting production SEO and exposing pre-release copy as canonical content.

Both Vercel projects (production: `vigilafrica.org`, staging: `staging.vigilafrica.org`) share the same `web/` directory and the same [web/vercel.json](web/vercel.json) per [docs/deployment/staging-production-topology.md:46-56](docs/deployment/staging-production-topology.md#L46-L56). They are differentiated only by their respective dashboard env vars and the Ignored Build Step.

Companion: [openspec/proposals/fix-staging-vite-env-flag.md](openspec/proposals/fix-staging-vite-env-flag.md).

## Decision Log

| # | Decision | Alternatives | Why |
|---|---|---|---|
| D1 | Set `VITE_ENV` per project via the Vercel dashboard (staging project = `staging`, production project = `production`) | Hardcode `VITE_ENV` in `vercel.json` `build.env` | Both Vercel projects read the same `vercel.json` (topology doc line 56). A `build.env` entry would apply to both projects, defeating the purpose. Dashboard-scoped vars are the only per-project mechanism that works |
| D2 | Also set `VITE_ENV=production` explicitly on the production project (not just staging) | Leave production to fall through to the "not staging" default in the plugin | Explicit > implicit. A future contributor adding a third state (e.g. `demo`) would have to remember the production deploy needs to set the var too. Setting both makes the contract symmetric |
| D3 | No code changes in this fix | Add a build-time assertion that throws if `VITE_ENV` is unset in non-local builds | A build-time assertion is captured as F3 in `chore-post-v11-quality-sweep` (the broader audit followup). Keeping this fix scoped to the operational gap means it can ship in minutes; the assertion lands when the quality-sweep PR does |
| D4 | Document `VITE_ENV` in the topology env matrix | Just set the dashboard var without docs | The env matrix in `staging-production-topology.md` is the single source of truth for "which var goes where". If `VITE_ENV` doesn't appear there, the next operator recreating a Vercel project will miss it. Adding it closes the doc-source loop |

## Components to Touch

### Modified files

| File | Change |
|---|---|
| [openspec/proposals/fix-staging-vite-env-flag.md](openspec/proposals/fix-staging-vite-env-flag.md) | Update `branch:` frontmatter from `tbd` to `fix/staging-vite-env-flag` |
| [docs/deployment/staging-production-topology.md](docs/deployment/staging-production-topology.md) | Add `VITE_ENV` row to the Environment Matrix (line 27-36 table); add a one-line note in the Vercel Project Settings section pointing operators at the var |

### Untouched

- [web/vite.config.ts](web/vite.config.ts) — the plugin already handles the flag correctly; the bug is purely operational
- [web/src/App.tsx](web/src/App.tsx) — `StagingBanner` is already correctly gated
- [web/vercel.json](web/vercel.json) — see D1; adding to `build.env` would break the staging deploy
- All backend code

## Behaviour Contract

- **B1** — After the staging Vercel project's `VITE_ENV=staging` env var is set and a deploy completes, `curl -s https://staging.vigilafrica.org/ | grep robots` MUST return `<meta name="robots" content="noindex, nofollow"`
- **B2** — Production output MUST be unchanged: `curl -s https://vigilafrica.org/ | grep robots` MUST return `<meta name="robots" content="index, follow"`. Setting `VITE_ENV=production` on the production project is explicit-confirmation; the rendered value MUST NOT change
- **B3** — Loading `https://staging.vigilafrica.org/` in a browser MUST show the "Staging environment — pre-release/test data" banner above the navigation
- **B4** — Loading `https://vigilafrica.org/` MUST NOT show the banner
- **B5** — The change MUST NOT alter any build artefact other than the rewritten `<meta name="robots">` tag in `index.html` and the gated banner DOM

## Phase 1 — Operator (Vercel Dashboard)

Cannot be done from the repo. The maintainer with Vercel access must:

- [ ] In the `vigilafrica-staging` Vercel project → Settings → Environments → All Environments, add env var `VITE_ENV = staging`
- [ ] In the `vigilafrica-production` Vercel project → Settings → Environments → All Environments, add env var `VITE_ENV = production`
- [ ] Trigger a redeploy on the staging project (push to `main` or click Redeploy in the dashboard)
- [ ] Trigger a redeploy on the production project (the next release tag will pick it up; or click Redeploy if a manual verification is needed before next release)

## Phase 2 — Repo (Docs + Spec)

- [ ] Update the `branch:` frontmatter in the proposal
- [ ] Add `VITE_ENV` to the Environment Matrix in `staging-production-topology.md`
- [ ] Add a one-line note in the Vercel Project Settings section explaining that `VITE_ENV` is dashboard-scoped (not `vercel.json`) because both projects share the file
- [ ] Open this PR before Phase 1 so the docs land alongside the operational change

## Acceptance Criteria

- [ ] `curl -s https://staging.vigilafrica.org/ | grep '<meta name="robots"'` → `noindex, nofollow`
- [ ] `curl -s https://vigilafrica.org/ | grep '<meta name="robots"'` → `index, follow` (unchanged)
- [ ] Staging banner visible on `https://staging.vigilafrica.org/` (top of page, above nav)
- [ ] No banner on `https://vigilafrica.org/`
- [ ] `VITE_ENV` appears in the Environment Matrix in `staging-production-topology.md`
- [ ] No other content drift in the rendered HTML (compare staging shell before/after — only `robots` meta and the banner div should differ)

## Out of Scope (reaffirmed)

- Build-time assertion that throws when `VITE_ENV` is unset in non-local builds — captured as F3 in `chore-post-v11-quality-sweep`
- Adding more environments (preview, demo, etc.) — current staging/production split is sufficient
- Replacing `VITE_ENV` with a different config mechanism — the build-time substitution via the Vite plugin is the right shape
- Refactoring how the staging banner renders or is styled — it's already correct, just gated on a flag that isn't being set
- Reconciling backend `APP_ENV` and frontend `VITE_ENV` — both should agree per deploy but are independent variables; reconciling them is a future refactor

## Risks

- **R1 — `vercel.json` precedence trap**: a well-meaning contributor sees this proposal mention `VITE_ENV` and adds it to `vercel.json` `build.env`. That would override BOTH dashboard settings (since both Vercel projects read the same `vercel.json`) and break the staging deploy. Mitigation: D1 captured this explicitly; the docs update also reinforces it
- **R2 — Operator forgets one of the two projects**: setting `VITE_ENV` on staging but not production. Mitigation: D2 makes setting both explicit; Phase 1 checklist enumerates both projects
- **R3 — Vercel env-var scope confusion**: Vercel offers Production / Preview / Development scopes. Mitigation: Phase 1 specifies "All Environments" scope so the var applies regardless of deploy type
- **R4 — Cached builds don't pick up the new var**: Vercel sometimes serves a cached HTML response. Mitigation: explicit "trigger a redeploy" step + `curl` verification

## Verification Plan

1. Land this PR (spec + docs update) on `development` → `main` → release
2. Maintainer applies the two Vercel dashboard env vars (Phase 1)
3. Maintainer triggers a staging redeploy
4. Run the `curl` checks in Acceptance Criteria
5. Visually confirm the banner appears/disappears on the right hostnames
6. (Optional but recommended) Submit `staging.vigilafrica.org` for re-indexing via Google Search Console once the `noindex` tag is live; verify previously-indexed staging URLs drop out of search results within ~2 weeks

No automated CI changes required.
