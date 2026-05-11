# chore-automate-release-tagging

**Branch:** `chore/automate-release-tagging`
**Spec:** [openspec/specs/chore-automate-release-tagging.md](openspec/specs/chore-automate-release-tagging.md)
**Proposal:** [openspec/proposals/chore-automate-release-tagging.md](openspec/proposals/chore-automate-release-tagging.md)

## Phase 1 — CI Wiring (Dry-Run) — this PR

- [x] Add `release-please-config.json` (release-type: simple, target-branch: release)
- [x] Add `.release-please-manifest.json` seeded `{ ".": "1.0.1" }`
- [x] Add `.github/workflows/release-please.yml` — dry-run gated, references `RELEASE_PLEASE_TOKEN`
- [x] Add `.github/workflows/cascade-back-merge.yml` — two-stage, second leg gated on first-leg merge
- [x] Add `.github/workflows/pr-title-check.yml` — active on PRs to `development` and `release`
- [x] Update [CONTRIBUTING.md](CONTRIBUTING.md) with conventional-commit PR-title section
- [x] Update [docs/deployment/release-process.md](docs/deployment/release-process.md) to describe Release PR + cascade flow
- [x] Pin all third-party actions by full commit SHA (project convention)
- [x] Set explicit least-privilege `permissions:` block on each workflow
- [x] Set `concurrency:` and `timeout-minutes:` on each workflow

## Phase 2 — Enable Release-Please — this PR

- [x] Create `RELEASE_PLEASE_TOKEN` repo secret (fine-grained PAT, `contents: write` + `pull-requests: write`, 12-month expiry)
- [x] Flip dry-run guard in [release-please.yml](.github/workflows/release-please.yml) from `if: false` to `if: true`
- [x] Bug-fix carried in same PR: exempt `main`-headed PRs from `pr-title-check.yml` (spec contract B5)

## Phase 3 — First Auto Release — operator follow-up

- [ ] First auto Release PR opens against `release`
- [ ] Maintainer reviews + merges
- [ ] Tag created automatically; `deploy-production.yml` fires; Environment gate prompts for approval
- [ ] `/health.version` reports the new tag after deploy
- [ ] Cascade back-merge auto-merges `release → main` then `main → development`

## Phase 4 — Closeout — operator follow-up

- [ ] Remove "Automated release tagging" from Post-MVP Backlog in [openspec/specs/vigilafrica/roadmap.md](openspec/specs/vigilafrica/roadmap.md)
- [ ] Calendar reminder created for `RELEASE_PLEASE_TOKEN` rotation 11 months out
- [ ] Archive the spec to `openspec/archive/spec-chore-automate-release-tagging.md`
