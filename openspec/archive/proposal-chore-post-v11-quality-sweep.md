---
id: chore-post-v11-quality-sweep
status: proposed
branch: chore/post-v11-quality-sweep-phase-all
---

# Proposal: Post-v1.1 Quality Sweep — Audit Followups (chore-post-v11-quality-sweep)

## Why

After PR #84 (`chore-alert-env-label`) cleared `/openspec-review`, a follow-on `/007` security audit and `/bug-hunter` regression sweep were run against the entire codebase and live production endpoint. Both produced clean reports on the PR itself, but surfaced **14 pre-existing items** in the broader application that are worth fixing as one focused cleanup PR.

The two highest-impact findings are not in PR #84 — they are:

1. **The staleness watchdog can silently drop alert emails.** [api/internal/alert/watchdog.go:65-82](api/internal/alert/watchdog.go#L65-L82) writes the dedupe row to Postgres *before* attempting the Resend send. If the send fails, the dedupe row blocks any retry — by this replica or any other — so the alert is permanently lost. This is a real incident-response gap: the whole point of the watchdog is to catch ingestion silence, and a single Resend hiccup currently breaks the safety net.
2. **The public landing page's "Project Status" copy is wildly stale.** [web/src/App.tsx:236-239](web/src/App.tsx#L236-L239) says "v0.7 complete, v1.0 active, production deploy is gated on final reviewer approval" — but production is **already live at v1.1.1**. The footer link points at `v1.1.0` release notes. [web/src/data/milestones.json](web/src/data/milestones.json) ends at v1.0 marked active. This is a public-trust issue: visitors see a "still building" message on a deployed product.

The other twelve items are smaller — defensive nil-checks, dead code, fragile regexes, a broken React error boundary, an out-of-date `version` constant in main.go, and several drift-prone hardcoded strings. They each cost minutes to fix and individually don't warrant a proposal, but bundled as one "post-v1.1 hygiene pass" they're an honest representation of what the audit found.

## What Changes

**Backend (Go):**

1. **B1 (HIGH)** Restructure the watchdog so that `TryRecordStalenessAlert` happens *after* a successful `SendStalenessAlert`, OR add a `sent_at` column and only treat rows with `sent_at IS NOT NULL` as dedupe-eligible. Either way, failed sends must be retried on the next tick
2. **B2 (MEDIUM)** Scheduler lock TTL should not be coupled to ingestion interval. Use `max(5min, expected_run_duration × 2)` with periodic heartbeat refresh, or migrate to `pg_advisory_lock` for session-scoped lifecycle
3. **B3 (MEDIUM)** Add defensive `if run == nil { return nil }` to [health.go:50](api/internal/handlers/health.go#L50) `runToResponse`. Current callers guard, but the function is a foot-gun for future callers
4. **B4 (LOW)** Remove dead `if events == nil` block in [events.go:99-101](api/internal/handlers/events.go#L99-L101) — the repo at [queries.go:124-126](api/internal/database/queries.go#L124-L126) already guarantees a non-nil slice
5. **B5 (LOW)** Fix misleading "after %d retries" message in [eonet.go:177](api/internal/ingestor/eonet.go#L177) — should say "after %d attempts" since the loop tries 4 times total when `maxRetries=3`
6. **B6 (LOW)** Refactor [eonet.go:26-31](api/internal/ingestor/eonet.go#L26-L31) package-level `var eonetURL` and `var eonetSleepFn` into an `Ingestor` struct (already TODO'd in source citing §9.5)
7. **B7 (LOW)** Move `firstRun` declaration into the `if lastSuccessRun == nil` block in [watchdog.go:94-101](api/internal/alert/watchdog.go#L94-L101) — minor readability
8. **B8 (LOW)** Bump source-default `var version = "0.7.0"` in [main.go:24](api/cmd/server/main.go#L24) to match the current release (currently `1.1.1`). Fallback should not lag the actual release by an entire major
9. **B9 (LOW)** Add `// best-effort — body already framed` comments (matching the existing `//nolint:errcheck` convention elsewhere in the file) for the uncommented `json.NewEncoder(w).Encode(...)` ignored errors — per §4.7, ignored errors must be explained

**Frontend (React / TS):**

1. **F1 (HIGH)** Refresh "Project Status" section [App.tsx:236-239](web/src/App.tsx#L236-L239), footer release link [App.tsx:307-312](web/src/App.tsx#L307-L312), and [milestones.json](web/src/data/milestones.json). v1.1 is in production; the copy should reflect that. Consider sourcing the active milestone from the deployed version (read from `/health.version`) instead of hardcoding
2. **F2 (HIGH)** Restore the React error boundary. [App.tsx:21, 132, 287, 293](web/src/App.tsx#L21) use `<Routes>` + `<Route errorElement={…}>` — but `errorElement` is silently ignored unless you migrate to `createBrowserRouter` (the data-router API). The current `PageError` component is dead code. Either migrate the router OR wrap routed content in a `react-error-boundary` `<ErrorBoundary>`
3. **F3 (MEDIUM)** Add a build-time assertion in [vite.config.ts](web/vite.config.ts) that fails if `VITE_API_BASE_URL` is unset when `VITE_ENV !== 'local'`. Same failure class as the open `fix-staging-vite-env-flag` proposal — Vercel build-env misconfig silently falls back to `window.location.origin`
4. **F4 (MEDIUM)** Convert remaining `throw new Error(…)` in [events.ts:78-153](web/src/api/events.ts#L78) (`fetchEventById`, `fetchContext`, `fetchHealth`, `fetchStates`) to `throw new ApiError(message, res.status)` for consistent error shape with status visibility
5. **F5 (LOW)** Title parsing regex [EventsDashboard.tsx:323-325](web/src/components/EventsDashboard.tsx#L323-L325) treats any trailing number as an "ID". Either enforce the shape upstream in the normalizer, or drop the parsing and render the raw title
6. **F6 (LOW)** Move `Date.now()` out of the `selectFreshness` selector in [EventsDashboard.tsx:58](web/src/components/EventsDashboard.tsx#L58) — current behaviour can show stale "X minutes ago" labels between refetches
7. **F7 (LOW)** Replace `as number` type cast in [EventsDashboard.tsx:202-203](web/src/components/EventsDashboard.tsx#L202-L203) with an explicit type-guard filter to handle the `undefined` case if the API contract ever drifts
8. **F8 (LOW)** Pass an explicit locale to `toLocaleDateString()` in [EventsDashboard.tsx:339](web/src/components/EventsDashboard.tsx#L339) so the same event shows the same date to all viewers

**Coverage gap to chase down before fixing:**

- The original `/007` sweep guessed wrong hostnames for the staging API (`api.staging.vigilafrica.org` per [docs/deployment/vps.md:9](docs/deployment/vps.md#L9), not the variants I tried). Re-run the live probes against the correct staging hostname before this cleanup PR ships — there may be additional staging-only findings. Specifically, check whether items still marked `patched-local` in [docs/security/priority-fixes.md](docs/security/priority-fixes.md) (e.g. SEC-002, SEC-013) have actually deployed to staging

## Out of Scope

- **L1 — Staging publicly indexable** (`<meta name="robots" content="index, follow">` on staging.vigilafrica.org). Already tracked as the existing open proposal `fix-staging-vite-env-flag`. This proposal does not duplicate it
- **All `patched-local` items from `docs/security/priority-fixes.md`** (SEC-001..SEC-026). Those have explicit owners and verification plans already; this proposal only adds *new* findings the v1.0 audit missed
- Bigger structural changes (auth, multi-replica scaleout, ORM migration). The dedupe-vs-send fix (B1) is the only architectural-shape change in scope; everything else is local cleanup
- React Router data-router migration (F2 could be solved either way; choose during implementation)
- Adding paid CI scanners; existing `govulncheck`, `npm audit`, image scanners cover the supply-chain surface

## Origin

- `/007` security audit on 2026-05-22 against PR #84 + codebase: score 90.25/100, approved. Findings A1-A9 in that report; A1 (staging indexability) and A7-A8 (container hardening, image digest pins) are tracked elsewhere
- `/bug-hunter` regression sweep on 2026-05-22 against the same surface. Findings B1-B9 (backend), F1-F8 (frontend), L1-L2 (live ops)
- Both reports recommended bundling the non-deferred items as one followup PR; this proposal is that bundle

## Followup

When this proposal is approved, the implementer should:

1. Re-run `/007` + `/bug-hunter` against `api.staging.vigilafrica.org` first to close the coverage gap noted above
2. Branch off `development` as `chore/post-v11-quality-sweep`
3. Implement Phase 1 (HIGH severity: B1, F1, F2) — this is the user-visible value
4. Run `/openspec-review` after each phase, not just at the end
5. Implement remaining phases as size allows, splitting into multiple PRs if any single phase grows past ~300 LOC of churn
