---
id: chore-wire-sentinel-gate
status: proposed
branch: feat/wire-sentinel-gate
---

# Proposal: Wire Up the Sentinel Governance Gate (chore-wire-sentinel-gate)

## Why

ADR-010 ("Automated Governance: The Sentinel") was accepted on 2026-04-14 and the
auditor binary (`api/cmd/sentinel/main.go`) was built — it already lists
`api/internal/`, `api/cmd/`, **and `web/src/`** as critical paths. But the
`openspec-verify` workflow never actually ran it: it ran two placeholder steps
(`go vet` and a `go build` smoke test of `cmd/server`), so the gate the ADR
promised was never enforced. A review of the 2026-05-29 partnership-readiness
sprint surfaced the consequence directly — the `/for-partners` page (a new
`web/src/` route + page) merged with no governance prompt, purely on reviewer
discretion.

Separately, the auditor only recognised records under `openspec/changes/`, while
the project's active workflow writes flat proposals under `openspec/proposals/`
(analytics, daily-digest, dark-mode, and the two sprint backfills all live there).
Enforcing the gate as-built would have hard-failed PRs whose records use the
layout the team actually uses.

This change makes the dormant gate real and reconciles it with current practice.

## What Changes

### Governance auditor (Go)

- `api/cmd/sentinel/main.go`: add an `isGovernanceRecord(path)` helper that
  accepts an active record in **either** `openspec/proposals/` or
  `openspec/changes/` (excluding any `/archive/` path), replacing the previous
  `openspec/changes/`-only inline check. The remediation message now points to
  both record locations.
- `api/cmd/sentinel/main_test.go` (new): table-driven unit tests for
  `isGovernanceRecord`, `isCritical`, and `isAllowed`, including the
  migration-is-critical-but-allowed case and the archived-record exclusion.

### CI (GitHub Actions)

- `.github/workflows/openspec-verify.yml`: replace the placeholder
  "Binary Smoke Test" with a real **"Sentinel Audit — Governance Gate"** step
  that runs `go run ./cmd/sentinel` (hard-fail). It first fetches
  `origin/development` into the remote-tracking ref the auditor diffs against.

### Documentation / governance

- `openspec/specs/vigilafrica/decisions.md`: amend ADR-010 — broaden the
  Governance Link rule to accept either record layout, and record (dated
  amendment) that the gate is now wired into CI.

## Out of Scope

- **Migrating existing records between layouts.** Both `openspec/proposals/` and
  `openspec/changes/` are accepted; no files are moved.
- **Changing the critical-path or allow-list sets.** `web/src/` was already in
  the auditor's critical paths; this change does not add or remove paths.
- **A `web/`-language-specific governance binary.** The existing Go auditor diffs
  paths and is language-agnostic; no separate web tool is introduced.
- **Backfilling records for past unspecced merges** — handled separately
  (`chore-sprint-infra-and-deps-backfill`, `feat-for-partners-page`).

## Capabilities

### Modified Capabilities

- `governance-sentinel`: The ADR-010 auditor is now enforced in CI and accepts
  the `openspec/proposals/` record layout in addition to `openspec/changes/`.

## Acceptance Criteria

- [x] `openspec-verify` runs `go run ./cmd/sentinel` as a hard-fail step.
- [x] A change to `api/internal/`, `api/cmd/`, or `web/src/` without any active
      OpenSpec record (and no `[trivial]` tag) fails the gate.
- [x] A record under `openspec/proposals/` satisfies the gate (this PR touches
      `api/cmd/sentinel/` and is itself covered by this proposal).
- [x] A record under `openspec/changes/<id>/` still satisfies the gate.
- [x] Archived records (`/archive/`) do not satisfy the gate.
- [x] `[trivial]` in a commit message still bypasses the audit.
- [x] `api/cmd/sentinel` unit tests pass; `go vet ./...` clean.
- [x] ADR-010 reflects both the CI wiring and the dual-layout acceptance.

## Risks

- **R1 — The gate blocks legitimate PRs that genuinely need no record.** Mitigated
  by the `[trivial]` exemption and the `api/db/migrations/` allow-list (existing),
  and by accepting the lightweight `openspec/proposals/` layout.
- **R2 — `origin/development` ref missing in CI, breaking the diff.** Mitigated by
  an explicit `git fetch origin development:refs/remotes/origin/development` in the
  gate step before the auditor runs.
- **R3 — This PR blocks itself** (it touches `api/cmd/sentinel/`). Mitigated by
  this very proposal under `openspec/proposals/`, which the newly-taught auditor
  recognises — a live dogfood of the change.

## Verification Plan

1. `go test ./cmd/sentinel/` (run via the Docker test runner, AppLocker) — pass.
2. `go vet ./...` — clean.
3. Run `go run ./cmd/sentinel` against this branch — exits 0 (this proposal is the
   governance record for the `api/cmd/sentinel/` change).
4. Confirm in CI that the "Sentinel Audit — Governance Gate" step runs and passes
   on this PR.

## Origin

Surfaced 2026-06-08 during the unspecced-sprint review follow-ups: the ADR-010
gate that should have flagged `/for-partners` was built but never enforced, and
recognised only the `openspec/changes/` layout. This change wires it into CI and
broadens the record check to the `openspec/proposals/` layout the project uses.
