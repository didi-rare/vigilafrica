---
id: fix-staging-vite-env-flag
status: proposed
branch: tbd
---

# Proposal: Wire `VITE_ENV=staging` into Vercel Staging Deploy (fix-staging-vite-env-flag)

## Why

Two PR #76 features depend on the build-time `VITE_ENV=staging` flag:

1. **Robots tag flip** — [web/vite.config.ts:13-14](web/vite.config.ts#L13-L14) reads `process.env.VITE_ENV === 'staging'` and rewrites the `<meta name="robots">` content from `index, follow` to `noindex, nofollow`. This is how staging is supposed to stop competing with production in search results.
2. **Staging banner** — [web/src/App.tsx:91](web/src/App.tsx#L91) gates a "this is a test environment" banner on `import.meta.env.VITE_ENV === 'staging'`.

Live validation on 2026-05-14 confirmed **both fail on staging.vigilafrica.org**:

| Check | Expected | Actual |
|---|---|---|
| `curl staging.vigilafrica.org` `<meta name="robots">` | `noindex, nofollow` | `index, follow` |
| Visible staging banner on the dashboard | Banner present | Banner absent |

Root cause: the staging Vercel deploy isn't setting `VITE_ENV=staging` at build time. The plugin and the gate both fall through to the production default.

User impact: search engines can (and likely do) index staging URLs, and staging users have no visual signal that they're not on production.

## What Changes

This is a deploy/config chore, not a code change. The repository does not own Vercel project settings, so the fix is operational.

1. In the Vercel dashboard for the staging frontend project:
   - Add environment variable `VITE_ENV=staging`
   - Scope it to the deploy that serves `staging.vigilafrica.org` (likely the `main` branch deploy or the `staging` preview environment, depending on the current branch-to-deploy mapping)
2. Trigger a fresh staging deploy (or wait for the next push to the staging branch) so the new env var is picked up
3. Verify:
   - `curl https://staging.vigilafrica.org/ | grep robots` returns `noindex, nofollow`
   - Loading `https://staging.vigilafrica.org/` in a browser shows the staging banner
4. (Optional, repo-side belt-and-suspenders) Add a `VITE_ENV=production` entry to [web/vercel.json](web/vercel.json) `build.env` block so the production deploy is also explicit. Currently it falls through correctly by default, but explicit > implicit.

## Out of Scope

- Adding more environments (preview, demo, etc.) — single staging/production split is fine
- Replacing `VITE_ENV` with a different config mechanism — the current build-time substitution is correct
- Refactoring how the staging banner renders — it's already correct, just gated on a flag that isn't being set

## Verification

After the change:
- [ ] `curl -s https://staging.vigilafrica.org/ | grep '<meta name="robots"'` → `noindex, nofollow`
- [ ] `curl -s https://vigilafrica.org/ | grep '<meta name="robots"'` → `index, follow` (unchanged)
- [ ] Staging banner visible at the top of `https://staging.vigilafrica.org/`
- [ ] No staging banner on `https://vigilafrica.org/`

## Origin

Surfaced during the 2026-05-14 staging+production validation pass, run after PR #80 promoted CSS-tokens + trust-quick-wins through to release. Both bugs are latent — they predate this work (v1.1.0 had them too).
