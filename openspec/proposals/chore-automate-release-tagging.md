# Proposal: Automate Release Tagging (chore-automate-release-tagging)

**Status:** Proposed — promotes the deferred Post-MVP backlog entry "Automated release tagging" from `openspec/specs/vigilafrica/roadmap.md`.

## Why

The manual `git tag -a vX.Y.Z` command is the only hand-typed step left in the production deploy path. It introduced two real costs during the v1.0 cut:

- **Toil + risk of fat-fingering the version** — under deploy pressure, picking `v1.0.1` vs `v1.1.0` is a judgment call done at the keyboard with no review.
- **No release notes or CHANGELOG.md** — both `v1.0.0` and `v1.0.1` shipped with no curated record of what changed. There is currently no machine- or human-readable artifact for "what shipped when."

The original deferral rationale was "SemVer choice is a human judgment; revisit with release-please." With release-please, that judgment becomes a reviewable Release PR (with the version + changelog visible as a diff) instead of a typed command, which preserves the human checkpoint while removing the toil and producing the missing CHANGELOG.

## What Changes

Introduce [release-please](https://github.com/googleapis/release-please) configured against the `release` branch. When conventional-commit signal accumulates on `release`, release-please opens a Release PR containing the next SemVer bump and a CHANGELOG.md update. Merging the Release PR creates the annotated `vX.Y.Z` tag, which fires the existing `deploy-production.yml` workflow and the existing `production` Environment approval gate.

Additional changes:

- **PR-title-level conventional-commit enforcement** on entry-point PRs (`feature/* → development`, `hotfix/* → release`) via `amannn/action-semantic-pull-request`. Promotion PRs (`development → main`, `main → release`) are exempt because they become merge commits release-please ignores.
- **Automated back-merge cascade** (`release → main → development`) after each Release PR merge, replacing today's manual "backport fix to main and development" step in the hotfix flow.
- A fine-grained PAT (`RELEASE_PLEASE_TOKEN`) so release-please's tag push triggers `deploy-production.yml` (the default `GITHUB_TOKEN` would silently break the chain).

## Out of Scope

- No strict `commitlint` on every commit; PR-title check only.
- No `package.json` version-field bumps (cosmetic — nothing reads them).
- No monorepo per-component versioning; api and web continue shipping under one tag.
- No removal of the existing `workflow_dispatch` deploy fallback — kept as break-glass.
- No rewriting of existing `v1.0.0` / `v1.0.1` tags.
- No change to the production Environment approval gate.
- No change to the `feature/* → development → main → release` branch topology.

## User Impact

End users and the public roadmap gain a versioned `CHANGELOG.md` and curated GitHub Release notes for every tagged release. Contributors gain an explicit, low-ceremony commit-message convention enforced where it matters (PR title at squash). The maintainer loses the manual `git tag` step and the manual hotfix backport step.

Internally, the v1.0 closeout list item "Rollback workflow verified by redeploying a previous production tag" remains valid — `workflow_dispatch` rollback continues to work the same way.
