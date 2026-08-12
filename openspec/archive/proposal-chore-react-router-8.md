---
id: chore-react-router-8
status: proposed
branch: chore/react-router-8-advisory-fix
---

> ## ✅ ARCHIVED 2026-08-07 — SHIPPED in #208 (`db9135d`), released in v1.3.7
>
> All four stated outcomes verified in the tree at archive time:
> `react-router@8.3.0` in `web/package.json`; **zero** `react-router-dom` references under `web/src`; React at 19.2.8; and `npm run audit:ci` reporting **0 documented exceptions** — the allowlist is empty, and `GHSA-qwww-vcr4-c8h2` was retired ahead of its 2026-08-14 review date.

# Proposal: Move to react-router 8 and Retire the Last Audit Exception (chore-react-router-8)

## Why

`GHSA-qwww-vcr4-c8h2` (react-router RSC-mode CSRF bypass) has been the project's **only** allowlisted advisory since #183. Its allowlist entry recorded a precise exit condition:

> *"No fixed react-router-dom exists: latest published is 7.18.1, inside the vulnerable 7.12.0–8.2.0 range… **Remove this entry when react-router publishes a fix > 8.2.0.**"*

**That condition is now met.** `react-router@8.3.0` was published **2026-07-22** — outside the vulnerable range. The exception was correct when written and is now obsolete.

Checked 2026-08-03, before its **2026-08-14** review date.

## What Changes

**The upgrade is mechanical, not a migration.** This was verified by doing it, not by predicting it — the project has a recorded near-miss where an unverified "big migration" claim almost deferred a clean eslint-10 bump that *fixed* an advisory instead of suppressing it.

1. **`react-router-dom` → `react-router` 8.3.0.** There is **no `react-router-dom` 8.x** — the line ends at 7.18.2, and 7.18.2 still depends on the vulnerable `react-router@7.18.2`. So staying on `react-router-dom` cannot clear the advisory at all.
2. **Import specifier change in 7 files.** All symbols the app uses — `BrowserRouter`, `MemoryRouter`, `Routes`, `Route`, `Link`, `useParams`, `useSearchParams` — resolve from the `react-router` **root** export in v8 (verified against the installed package, not the docs). No `react-router/dom` subpath needed, no API changes, no call-site changes.
3. **React 19.2.5 → 19.2.8.** `react-router@8` declares `react >= 19.2.7` / `react-dom >= 19.2.7`. A patch-level bump within React 19; 19.2.8 is current.
4. **Empty the `audit-ci` allowlist.** It now has **zero** documented exceptions.

## Why this is low risk

- The app uses only the long-stable declarative routing surface. It does **not** use `createBrowserRouter`, RSC mode, data routers, loaders or actions — which is exactly why the advisory was unreachable in the first place, and also why v8's changes do not touch us.
- The full gate suite passes unchanged, including **70/70 tests** covering the routed components (`EventsDashboard`, `EventDetail`, `ForPartners` all render under `MemoryRouter`).

## Out of Scope

- Adopting any v8 feature. This is an advisory fix, not a modernisation — no data routers, no RSC, no framework mode.
- Any routing behaviour change. The route table, paths and SPA rewrite are untouched.
- Other dependency upgrades.

## Verification

- [x] `npm run audit:ci` — **passes with 0 exceptions**; the wrapper's stale-detection independently confirmed the advisory is gone (`STALE  GHSA-qwww-vcr4-c8h2 … is no longer reported`)
- [x] `npm run type-check` / `lint` / `build` — clean
- [x] `npm run test` — **70/70** across 9 files
- [x] No `react-router-dom` references remain in `src/` or `package.json`
- [x] All 7 imported symbols verified present on `react-router`'s root export at runtime
- [ ] Post-deploy: SPA deep links still resolve (`/events/:id`, `/for-partners`) — the `vercel.json` rewrite is unchanged but routing is the one thing worth re-checking live

## Origin

Maintainer asked on 2026-08-03 whether the react-router advisory had a permanent fix yet. It did — published 12 days earlier and 11 days before the entry's own review date. The `audit-ci` stale-warning mechanism built in #183 flagged it automatically the moment the dependency moved, which is the mechanism working exactly as intended.
