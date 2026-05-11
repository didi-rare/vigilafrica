---
id: chore-automate-release-tagging
status: proposed
branch: tbd
---

# Spec: Automate Release Tagging (chore-automate-release-tagging)

## Context

Production deploys today follow `feature/* → development → main → release → manual git tag → deploy-production.yml → Environment approval`. The manual tag is the only hand-typed step and produces no release notes. This spec replaces that step with a release-please–driven Release PR flow on the `release` branch, plus an automated back-merge cascade and PR-title convention enforcement.

The design was reached via the `/brainstorming` skill and the seven decisions reached are recorded inline in the Decision Log section below.

## Decision Log

| # | Decision | Alternatives | Why |
|---|---|---|---|
| D1 | Adopt full release-please-style flow (CHANGELOG + SemVer + PR-on-merge) | Toil-only thin tagger; error-only validator; no automation | Maintainer wants the full package, including release notes |
| D2 | PR-title-level convention enforcement via `amannn/action-semantic-pull-request`; squash-merge feature/hotfix PRs; merge-commit promotion PRs | Strict commitlint; no enforcement; hybrid | Lowest-friction signal capture that still gives release-please clean conventional commits to parse |
| D3 | Release-please runs on `release` (Option 1) | On `main` with promote-to-release workflow (Option 2); collapse `main → release` into Release PR (Option 3) | Preserves current branch model: release IS prod; Release PR replaces manual `git tag` 1:1 |
| D4 | Release-please manages `CHANGELOG.md` only — no `package.json` version bumps | Bump package.json; monorepo per-component versions; no CHANGELOG (GH Releases only) | Tag is already canonical version source; package.json versions are cosmetic |
| D5 | Automated back-merge cascade `release → main → development` | Single PR to dev only; manual sync; CHANGELOG only on release | Same machinery handles release sync AND hotfix backport |
| D6 | PR title CI check on entry-point PRs only (`feature/* → development`, `hotfix/* → release`) | All PRs; no enforcement | Promotion PRs become merge commits release-please ignores by default |
| D7 | Fine-grained PAT (`RELEASE_PLEASE_TOKEN`) | GitHub App; accept manual `workflow_dispatch` trigger | Standard release-please pattern; ~5 min setup; can migrate to App later without redesign |

## Architecture

```
feature/*  ──squash (feat:/fix:)──▶ development ──merge──▶ main ──merge──▶ release
                                                                              │
                                                                              ▼
                                                              release-please scans commits
                                                              since last tag and opens
                                                              Release PR on release
                                                                              │
                                                                  human merges Release PR
                                                                              │
                                                                              ▼
                                                              vX.Y.Z tag created via PAT
                                                                              │
                                                  ┌───────────────────────────┴──────────────────────────┐
                                                  ▼                                                      ▼
                                          deploy-production.yml                            cascade-back-merge.yml
                                          (existing; tag-triggered;                        opens auto-merge PRs:
                                          Environment gate stays)                          release → main, then
                                                                                           main → development
```

## Components

### New files

| File | Purpose |
|---|---|
| `release-please-config.json` | release-please config — `release-type: simple` (CHANGELOG-only, no version files), target branch `release`, default change-type → bump mapping |
| `.release-please-manifest.json` | Seeded `{ ".": "1.0.1" }` so release-please knows the current version |
| `.github/workflows/release-please.yml` | Triggers on `push` to `release`; runs `googleapis/release-please-action`; uses `RELEASE_PLEASE_TOKEN` |
| `.github/workflows/cascade-back-merge.yml` | Triggers on Release PR merge; opens auto-merge PRs `release → main` and (on its merge) `main → development` |
| `.github/workflows/pr-title-check.yml` | Runs `amannn/action-semantic-pull-request` on PRs whose base is `development` or `release` |

### New repository secret

| Secret | Type | Scope | Notes |
|---|---|---|---|
| `RELEASE_PLEASE_TOKEN` | Fine-grained PAT | `contents: write` + `pull-requests: write` on `didi-rare/vigilafrica` only | 12-month expiry; calendar reminder at 11 months for rotation |

### Modified files

| File | Change |
|---|---|
| `docs/deployment/release-process.md` | Remove manual `git tag` block; document Release PR flow + back-merge cascade; document break-glass `workflow_dispatch` path |
| `CONTRIBUTING.md` | Add conventional-commits-via-PR-title rule with examples |
| `openspec/specs/vigilafrica/roadmap.md` | Remove "Automated release tagging" from Post-MVP Backlog (delivered) |

### Untouched

`api/`, `web/`, `db/`, `docker-compose.*.yml`, `deploy/`, `deploy-staging.yml`, `deploy-production.yml` — none of these change. This is pure CI / release-tooling work.

## Behaviour Contract

- **B1** — A push to `release` whose new commits since the last tag include any conventional-commit type that warrants a bump (`feat:`, `fix:`, `feat!:` / `BREAKING CHANGE:`) MUST result in release-please opening or updating a Release PR on `release` within 5 minutes.
- **B2** — Merging a Release PR MUST create an annotated tag `vMAJOR.MINOR.PATCH` and MUST cause `deploy-production.yml` to run.
- **B3** — `deploy-production.yml`'s existing `production` Environment approval gate MUST remain in place and MUST continue to require maintainer approval before the API stack deploys.
- **B4** — After a Release PR merge, `cascade-back-merge.yml` MUST open an auto-merge PR from `release` to `main`. Once that PR merges, it MUST open an auto-merge PR from `main` to `development`.
- **B5** — A PR targeting `development` or `release` whose title does not match the conventional-commits regex MUST fail the `pr-title-check` CI status. Promotion PRs (`development → main`, `main → release`) are NOT subject to this check.
- **B6** — release-please MUST NOT use the default `GITHUB_TOKEN` for tag creation, since tags pushed by `GITHUB_TOKEN` do not trigger downstream workflows.
- **B7** — The `workflow_dispatch` trigger on `deploy-production.yml` MUST remain functional as a break-glass: if release-please is unavailable, a maintainer can still tag manually and dispatch the deploy.

## Phase 1 — CI Wiring (Dry-Run)

Single PR to `development`, normal `feature/* → development → main → release` promotion.

- [ ] Add `release-please-config.json` (release-type: `simple`, target branch: `release`)
- [ ] Add `.release-please-manifest.json` seeded `{ ".": "1.0.1" }`
- [ ] Add `.github/workflows/release-please.yml` in **dry-run mode** (commented `on:` trigger or `if: false` guard) — does not yet open Release PRs
- [ ] Add `.github/workflows/pr-title-check.yml` (active immediately on PRs to `development` and `release`)
- [ ] Add `.github/workflows/cascade-back-merge.yml` (inert until a Release PR exists to trigger it)
- [ ] Update `CONTRIBUTING.md` with the PR-title convention
- [ ] Update `docs/deployment/release-process.md` to describe the new flow

The Phase 1 PR title is `feat(ci): scaffold release-please automation (dry-run)`. PR-title check is now active.

## Phase 2 — Enable Release-Please

Separate small PR after Phase 1 lands on `release`.

- [ ] Create `RELEASE_PLEASE_TOKEN` secret (fine-grained PAT, scopes per Components table)
- [ ] Flip `release-please.yml` from dry-run to active (remove the guard)

The next push to `release` after Phase 2 lands triggers the first auto Release PR.

## Phase 3 — First Auto Release

- [ ] First Release PR opens on `release`. Title format: `chore(release): release X.Y.Z`
- [ ] Maintainer reviews the proposed CHANGELOG and version bump
- [ ] Merge Release PR
- [ ] Tag is created automatically by release-please
- [ ] `deploy-production.yml` fires on the tag; Environment gate prompts for approval
- [ ] Smoke-test production endpoint reports the new tag in `version`
- [ ] `cascade-back-merge.yml` opens auto-merge PR `release → main`; on its merge, opens `main → development`
- [ ] Both cascade PRs auto-merge once CI passes

## Phase 4 — Closeout

- [ ] Remove "Automated release tagging" from Post-MVP Backlog in `openspec/specs/vigilafrica/roadmap.md`
- [ ] Archive this spec to `openspec/archive/spec-chore-automate-release-tagging.md`
- [ ] Calendar reminder created for `RELEASE_PLEASE_TOKEN` rotation 11 months out

## Acceptance Criteria

- [ ] First auto Release PR opens against `release` after Phase 2 enable
- [ ] Merging the auto Release PR creates a properly-named annotated tag (`vX.Y.Z`)
- [ ] `deploy-production.yml` fires automatically on the auto-created tag (proves PAT trigger works, B6)
- [ ] `production` Environment approval gate still gates the deploy (B3)
- [ ] `https://api.vigilafrica.org/health` reports the new tag in `version` after deploy
- [ ] `cascade-back-merge.yml` successfully opens and auto-merges PRs `release → main` and `main → development`
- [ ] `CHANGELOG.md` exists at repo root with at least one entry on `release`, `main`, and `development` after the cascade
- [ ] `pr-title-check` blocks a deliberately-malformed PR title on a PR targeting `development` (manual smoke test during rollout)
- [ ] `pr-title-check` does NOT block a `main → release` promotion PR (manual smoke test)
- [ ] `release-process.md` no longer instructs the maintainer to run `git tag` manually
- [ ] Roadmap entry "Automated release tagging" removed from Post-MVP Backlog

## Failure Modes & Recovery

| Failure | Symptom | Recovery |
|---|---|---|
| `RELEASE_PLEASE_TOKEN` expires | release-please.yml fails authentication; no Release PRs open | Rotate PAT; re-run workflow. Manual `git tag` + `workflow_dispatch` deploy still works. |
| release-please action breaks (upstream regression) | Workflow fails or opens malformed Release PR | Pin release-please to a known-good SHA in workflow file; or revert to manual `git tag` + `workflow_dispatch` |
| Cascade back-merge conflict (e.g. someone manually edited CHANGELOG.md on `main`) | Auto-merge fails; PR sits open with conflict marker | Maintainer resolves by hand; convention/docs say only release-please touches CHANGELOG.md to prevent recurrence |
| PR title typo (`fx:` instead of `fix:`) slips through (e.g. CI check disabled temporarily) | release-please skips the version bump; that PR's change appears as "Other Changes" without bump | Next correctly-typed PR triggers a release that catches up |
| Concurrent hotfix + normal release in flight | Two Release PRs could open in sequence | Cascade is serial; release-please rebases its PR before merge — safe in practice |

## Verification Plan

All acceptance criteria have observable outputs (PR opened, tag created, workflow run pass/fail, CI status, HTTP response). No new automated tests are added — the existing smoke-test step in `deploy-production.yml` (asserts `/health.version` matches the deploy tag) is the automated gate that proves the chain end-to-end.

The Phase 3 first-release run IS the integration test for this entire change. If it succeeds, the system is validated; if it fails, recovery uses the break-glass `workflow_dispatch` path and the bug is fixed in a follow-up before re-enabling.

## Risks Acknowledged

- **R1**: PAT expires unnoticed → release-please silently stops opening Release PRs. **Mitigation**: 12-month expiry + calendar reminder at 11mo + `workflow_dispatch` fallback always available.
- **R2**: PR-title typo evades the convention check (e.g. someone disables the workflow). **Mitigation**: Action runs in CI and is required by branch protection; bypass is visible in audit log.
- **R3**: Concurrent releases create ordering ambiguity. **Mitigation**: Serial cascade + release-please rebase-before-merge.
- **R4**: Cascade conflicts due to manual edits of release-please-owned files. **Mitigation**: Documented convention that only release-please touches CHANGELOG.md.

## Out of Scope (re-stated for the spec record)

- No strict commit-level commitlint (PR-title only)
- No `package.json` version bumps
- No monorepo per-component versioning
- No removal of `workflow_dispatch` deploy fallback
- No rewrite of existing `v1.0.0` / `v1.0.1` tags
- No change to `production` Environment approval gate
- No change to `feature/* → development → main → release` branch topology
