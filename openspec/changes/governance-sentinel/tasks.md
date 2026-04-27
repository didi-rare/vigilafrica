# governance-sentinel

## 1. Review Decisions

- [ ] 1.1 Confirm governance gate should block source changes without an OpenSpec change record
- [ ] 1.2 Confirm `[trivial]` commit-message bypass semantics
- [ ] 1.3 Confirm allow-list scope, including whether `api/db/migrations/` is exempt
- [ ] 1.4 Confirm local comparison target should be `origin/development`

## 2. Sentinel Auditor

- [ ] 2.1 Create `api/cmd/sentinel/main.go`
- [ ] 2.2 Detect changed files with `git diff`
- [ ] 2.3 Treat `api/internal/*` and `web/src/*` as critical package changes
- [ ] 2.4 Require an active `openspec/changes/<change-id>/` record for critical changes
- [ ] 2.5 Support `[trivial]` override for allowed small hygiene commits

## 3. CI Integration

- [ ] 3.1 Add the Sentinel Auditor to `.github/workflows/openspec-verify.yml`
- [ ] 3.2 Ensure the workflow checks out enough Git history for diff comparisons
- [ ] 3.3 Keep existing OpenSpec drift validation behavior intact

## 4. Governance Documentation

- [ ] 4.1 Register ADR-010 in `openspec/specs/vigilafrica/decisions.md`
- [ ] 4.2 Document the blocking behavior and exemption path for contributors
- [ ] 4.3 Keep the active change checklist synced with `openspec/changes/governance-sentinel/tasks.md`

## 5. Verification

- [ ] 5.1 Run the auditor against a critical change without an OpenSpec record and confirm failure
- [ ] 5.2 Run the auditor with a `[trivial]` override and confirm success
- [ ] 5.3 Run `npm run spec:validate`
- [ ] 5.4 Push `feat/gov-sentinel` and verify GitHub Actions on the PR
