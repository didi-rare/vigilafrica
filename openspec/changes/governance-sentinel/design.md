# Implementation Plan: VigilAfrica Sentinel (Semantic Gate)

This plan implements a "Governance Sentinel" to ensure that no code implementation (AI or Human) can bypass the OpenSpec `propose -> apply -> archive` lifecycle.

## User Review Required

> [!IMPORTANT]
> **Blocking Nature**: Once active, this CI check will FAIL any Pull Request that modifies source code without a corresponding change record in `openspec/changes/`.
> 
> **Exemption Path**: We are adding a `[trivial]` commit message flag and a directory allow-list (docs, migrations) to ensure the governance doesn't slow down small repo hygiene tasks.

## Proposed Changes

### [Component Name] Governance Auditor (Go)

#### [NEW] [main.go](file:///c:/Users/Didi/Documents/Projects/VigilAfrica/vigilafrica/api/cmd/sentinel/main.go)
- Create a Go utility that detects changed files using `git diff`.
- Check for "Critical Package" modifications: `api/internal/*`, `web/src/*`.
- Verify the presence of a change subdirectory: `openspec/changes/<any-folder>/`.
- Logic for `[trivial]` commit message override.

---

### [Component Name] Continuous Integration (CI)

#### [MODIFY] [openspec-verify.yml](file:///c:/Users/Didi/Documents/Projects/VigilAfrica/vigilafrica/.github/workflows/openspec-verify.yml)
- Add a job to run the Sentinel Auditor.
- Ensure the job has access to Go and Git history (fetch-depth).

---

### [Component Name] Documentation & Governance

#### [MODIFY] [decisions.md](file:///c:/Users/Didi/Documents/Projects/VigilAfrica/vigilafrica/openspec/specs/vigilafrica/decisions.md)
- Register **ADR-010: Automated Governance Enforcement (The Sentinel)**.

## Open Questions

- **Specific Allow-List**: Should we exempt `api/db/migrations/` from the check? (I recommend Yes, as migrations are often separate from feature-spec cycles).
- **Target Branch**: In local mode, should the tool compare against `origin/development` or just the local `development`? (I recommend `origin/development` for stability).

## Verification Plan

### Automated Tests
- Create a test branch `test/sentinel-failure`.
- Modify a file in `api/internal/` and commit without `openspec/changes/`.
- Run `go run ./api/cmd/sentinel/main.go` locally and verify it exits with `1`.
- Add `[trivial]` to a commit and verify it exits with `0`.

### Manual Verification
- Push the changes to `feat/gov-sentinel` and verify the GitHub Action status.
