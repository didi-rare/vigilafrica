---
id: chore-post-v11-quality-sweep
status: proposed
branch: tbd
---

# Spec: Post-v1.1 Quality Sweep — Audit Followups (chore-post-v11-quality-sweep)

## Context

After PR #84 (`chore-alert-env-label`) cleared two rounds of `/openspec-review`, two follow-on agents (`/007` security audit + `/bug-hunter` regression sweep) audited the broader codebase and live production endpoint on 2026-05-22. Both produced clean reports on PR #84 itself (no security blocker, no PR-introduced bugs), but flagged 14 pre-existing items in the wider application. This spec consolidates those items into a single planned cleanup.

The spec is **planning-only** at proposed status — no implementation is included. The phased task list below is intended to be ticked off by a follow-up PR.

Companion: [openspec/proposals/chore-post-v11-quality-sweep.md](openspec/proposals/chore-post-v11-quality-sweep.md).

## Decision Log

| # | Decision | Alternatives | Why |
|---|---|---|---|
| D1 | One consolidated proposal | Split per area (3 changes), one per HIGH finding (~4 changes) | The 14 items share an origin (single audit pass) and a shared review context. Splitting adds openspec overhead; bundling lets one PR close a known set. Implementation can still split phases into separate PRs if needed |
| D2 | Branch off `development`, not `main` | Branch off `main` | `development` is the integration target for PR #84 and the natural base for followups. `main` is the staging mirror; branching there would force a `development → main` promotion before this work can land |
| D3 | Re-run audits against `api.staging.vigilafrica.org` before implementing | Skip and trust the initial sweep | The initial `/007` ran against the wrong staging hostname guesses. There may be staging-only findings (deploy-but-not-staging-verified items from `priority-fixes.md`) that show up against the correct host. Cheap to re-run; expensive to skip |
| D4 | Watchdog fix (B1) is the **only** architectural change in scope | Bundle B1 with single-replica → multi-replica scheduler work | Multi-replica scaleout is a separate ADR; B1 is a defensibility issue at single replica too (failed Resend send → permanent silent drop) |
| D5 | Keep `App.tsx` "Project Status" copy hardcoded for now; design a dynamic version as a future iteration | Read milestone state from `/health.version` and a milestones-by-version map | Dynamic version-aware copy is the right long-term shape but pulls in routing complexity (what does each version *mean*?). For this sweep, just update the strings to match v1.1 reality and capture the dynamic-rendering idea as out-of-scope |
| D6 | F2 (broken error boundary) is implementer's choice: data-router migration OR `react-error-boundary` wrap | Pre-pick one in the spec | Both are valid; the choice depends on whether the implementer wants to take on the wider router refactor. Capture trade-offs in implementation PR description |

## Components to Touch

### Backend (Go) — `api/`

| File | Change | Finding |
|---|---|---|
| [api/internal/alert/watchdog.go](api/internal/alert/watchdog.go) | Reorder dedupe-vs-send OR introduce `sent_at` column; failed sends must retry on next tick | B1 |
| [api/internal/ingestor/scheduler.go](api/internal/ingestor/scheduler.go) | TTL decoupled from interval; heartbeat refresh OR pg_advisory_lock | B2 |
| [api/internal/handlers/health.go](api/internal/handlers/health.go) | Add nil-guard at top of `runToResponse` | B3 |
| [api/internal/handlers/events.go](api/internal/handlers/events.go) | Remove dead `if events == nil` block | B4 |
| [api/internal/ingestor/eonet.go](api/internal/ingestor/eonet.go) | Fix "%d retries" → "%d attempts"; refactor package globals into `Ingestor` struct | B5, B6 |
| [api/cmd/server/main.go](api/cmd/server/main.go) | Bump source-default `version` from `"0.7.0"` to current release | B8 |
| Multiple handlers | Add `// best-effort` comments or Debug logs for uncommented ignored `Encode` errors | B9 |
| New: `api/db/migrations/000007_alert_dedupe_sent_at.up.sql` (only if D1 chooses the `sent_at` path) | Add nullable `sent_at TIMESTAMPTZ` to `alert_dedupe` | B1 |

### Frontend (React / TypeScript) — `web/src/`

| File | Change | Finding |
|---|---|---|
| [web/src/App.tsx](web/src/App.tsx) | Update "Project Status" copy; fix footer release link; restore error boundary | F1, F2 |
| [web/src/data/milestones.json](web/src/data/milestones.json) | Mark v1.0 complete; add v1.1 entry | F1 |
| [web/vite.config.ts](web/vite.config.ts) | Build-time assertion on `VITE_API_BASE_URL` when `VITE_ENV !== 'local'` | F3 |
| [web/src/api/events.ts](web/src/api/events.ts) | Convert `fetchEventById`/`fetchContext`/`fetchHealth`/`fetchStates` to throw `ApiError` | F4 |
| [web/src/components/EventsDashboard.tsx](web/src/components/EventsDashboard.tsx) | Drop fragile title regex OR move to normalizer; move `Date.now()` out of `selectFreshness`; type-guard filter for lat/lng; explicit locale on `toLocaleDateString()` | F5, F6, F7, F8 |
| `web/package.json` (only if F2 picks the `react-error-boundary` path) | Add `react-error-boundary` dependency (~5KB, ADR-rationale needed per §10.2) | F2 |

### Deliberately untouched

- All `patched-local` items from [docs/security/priority-fixes.md](docs/security/priority-fixes.md) (SEC-001 through SEC-026) — those have their own ownership and verification trail
- `fix-staging-vite-env-flag` proposal — separately tracked
- Anything in PR #84 (`chore-alert-env-label`) — that PR is the trigger for this sweep, not a target of it

## Behaviour Contract

### B1 — Watchdog alert delivery resilience

- **B1.1** — A failed `SendStalenessAlert` (Resend HTTP error, network timeout, etc.) MUST cause a retry on the next watchdog tick, not silently drop the alert
- **B1.2** — Multi-replica deduplication (the original SEC-023 fix) MUST still hold: only one alert email per reference time across all replicas, including across restart
- **B1.3** — If the implementer chooses the `sent_at` column path, the migration MUST be reversible (`down.sql` drops the column)
- **B1.4** — Unit test MUST cover: (a) send succeeds → next tick suppressed, (b) send fails → next tick retries, (c) another replica already succeeded → this replica suppresses

### F1 — Project Status truthfulness

- **F1.1** — The "Project Status" section MUST NOT claim "production deploy is gated on final reviewer approval" when production is actually live and serving traffic
- **F1.2** — The footer release-notes link MUST match the deployed version (verified via `/health.version` if dynamic, or kept manually in sync if hardcoded)
- **F1.3** — `milestones.json` MUST include an entry for the current major.minor version with appropriate `active`/`complete` flags

### F2 — Error boundary functionality

- **F2.1** — A runtime error during route rendering (e.g. `EventsDashboard` throws) MUST render a user-visible fallback (the current `PageError` component or equivalent), not a blank white screen
- **F2.2** — The fallback MUST include a "back to dashboard" link
- **F2.3** — A unit test or playwright spec MUST verify the fallback renders when a child component throws

### General

- **G1** — No change in this sweep may regress any acceptance criterion from `priority-fixes.md` SEC-001..SEC-026 or any previously-shipped `openspec` change
- **G2** — Each phase below MUST be `/openspec-review`-able as its own slice; reviewer should be able to ship phase 1 without phase 2-3

## Phase 1 — HIGH Severity (User-Visible Risk)

- [ ] **B1** — Restructure watchdog dedupe order; add unit tests covering all three retry scenarios
- [ ] **F1** — Refresh "Project Status" copy + footer + milestones.json
- [ ] **F2** — Restore error boundary (pick data-router migration OR react-error-boundary wrap; document choice in PR)
- [ ] Re-verify against `api.staging.vigilafrica.org` (per D3) before opening the PR

## Phase 2 — MEDIUM Severity (Reliability / Maintainability)

- [ ] **B2** — Scheduler lock TTL + heartbeat refactor (or pg_advisory_lock migration)
- [ ] **B3** — Defensive nil-check on `runToResponse`
- [ ] **F3** — `VITE_API_BASE_URL` build-time assertion
- [ ] **F4** — `events.ts` errors → `ApiError` with status

## Phase 3 — LOW Severity (Cleanup)

- [ ] **B4** — Remove dead `events == nil` block
- [ ] **B5** — Fix "after %d retries" wording
- [ ] **B6** — Refactor `eonet.go` package globals into `Ingestor` struct (this is the bigger of the low-severity items; may be its own PR)
- [ ] **B7** — Move `firstRun` declaration inside the branch where it's used
- [ ] **B8** — Bump source-default `version` constant
- [ ] **B9** — Annotate ignored `Encode` errors
- [ ] **F5** — Title regex hardening (or removal)
- [ ] **F6** — `Date.now()` ticking
- [ ] **F7** — Type-guard filter for lat/lng
- [ ] **F8** — Explicit locale on `toLocaleDateString()`

## Acceptance Criteria

- [ ] All Phase 1 items completed and verified — these are the user-visible value
- [ ] At least one of Phase 2 / Phase 3 PRs has shipped; the rest may be deferred to future cleanup proposals if the implementer prefers to split them
- [ ] `go test ./...` (from `api/`) passes; `npm run test` (from `web/`) passes
- [ ] `go vet ./...` clean
- [ ] `npm run build` (frontend) succeeds; static type-check passes
- [ ] Live staging probe against `api.staging.vigilafrica.org` shows no regression vs. current behaviour
- [ ] B1 acceptance test: simulate a Resend failure → verify next tick retries → verify second tick (after success) suppresses
- [ ] F2 acceptance test: force a render error in a child component → verify `PageError` (or replacement) renders → verify link back to dashboard works
- [ ] Each phase PR has its own `/openspec-review` round-1 passing report attached

## Out of Scope (reaffirmed)

- **L1** — Staging publicly indexable. Already tracked as `fix-staging-vite-env-flag`
- **All `patched-local` items in `priority-fixes.md`** (SEC-001..SEC-026) — separate ownership
- **Multi-replica scaleout** — separate ADR conversation
- **Dynamic milestone rendering from `/health.version`** — captured as a future-iteration idea; this sweep just updates the strings
- **React Router architectural redesign** — F2 only needs the error boundary to work, not a full router rewrite (though data-router migration is one valid path)
- **`testify/require` migration** — the §9.4 standard drift is real but a separate cleanup chore; not in scope here

## Risks

- **R1 — Scope creep**: 14 items is a lot. Mitigation: phase 1 is the only mandatory deliverable; phases 2-3 can split into follow-up proposals if they bloat
- **R2 — B1 regression risk**: changing watchdog dedupe order touches a tested, deployed path. Mitigation: explicit B1.4 test matrix; consider feature-flagging the new order under an env var for one release cycle
- **R3 — F2 router migration is a one-way door**: if implementer picks data-router migration, that's a wider architectural change than the bug warrants. Mitigation: D6 explicitly leaves the choice to the implementer; PR description must justify
- **R4 — Stale "Project Status" copy returns**: if F1 hardcodes new strings, this whole proposal recurs in 6 months. Mitigation: document the dynamic-rendering idea in PR description so the next "stale status" reviewer reaches for that pattern instead of another string update
- **R5 — Coverage gap repeats**: the original `/007` ran against wrong hostnames. Mitigation: D3 + the explicit Phase 1 re-verification step

## Verification Plan

1. Re-run `/007` and `/bug-hunter` against the correct staging hostname (`api.staging.vigilafrica.org`) to close the coverage gap (D3)
2. Implement Phase 1 on a branch off `development` named `chore/post-v11-quality-sweep`
3. Run `go test ./...`, `go vet ./...`, `npm run test`, `npm run build` locally — all green
4. Run `/openspec-review` after Phase 1 — round-1 PASS required before opening PR
5. Open PR to `development`; reviewer probes staging post-merge to verify no regressions
6. Decide at review time whether to roll Phase 2-3 into the same PR or split

No new automated CI changes required.
