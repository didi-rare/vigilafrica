# Release Process

VigilAfrica promotes code through three long-lived branches:

```text
feature/* -> development -> main -> release -> annotated tag vX.Y.Z
```

## Branch Roles

| Branch | Role | Deployment |
|---|---|---|
| `development` | Integration target for feature PRs | none |
| `main` | Staging mirror | auto-deploys staging API and Vercel staging frontend |
| `release` | Production staging branch | production deploy only after an annotated SemVer tag |

## Staging Promotion

1. Merge feature PRs into `development`.
2. Open a PR from `development` to `main`.
3. Merge after CI passes.
4. Confirm `Deploy Staging` passes and `https://api.staging.vigilafrica.org/health` reports the expected commit SHA.
5. Verify the Vercel staging project points `VITE_API_BASE_URL` at `https://api.staging.vigilafrica.org`.

## Production Promotion

1. Open a PR from `main` to `release`.
2. Merge after staging verification.
3. **release-please** opens a Release PR on `release` containing the next SemVer bump and a `CHANGELOG.md` update derived from conventional-commit signal since the last tag.
4. Review the proposed version + changelog. Merge the Release PR.
5. **release-please** creates the annotated `vX.Y.Z` tag automatically (via `RELEASE_PLEASE_TOKEN`, so the tag push triggers downstream workflows).
6. Approve the `production` GitHub Environment gate when `Deploy Production` pauses.
7. Confirm `https://api.vigilafrica.org/health` reports `"version":"vX.Y.Z"`.
8. The **cascade-back-merge** workflow opens an auto-merge PR `release → main`. Once that merges, it opens a second auto-merge PR `main → development`. The new `CHANGELOG.md` (and any hotfix code) propagates to both branches with no manual action.

> **PR title convention**: PRs targeting `development` (feature/fix) or `release` (hotfix) must use [Conventional Commits](https://www.conventionalcommits.org/) format — `feat:` for minor bumps, `fix:` for patch, `feat!:` or `BREAKING CHANGE` for major. The `pr-title-check` CI status enforces this. Promotion PRs (`development → main`, `main → release`) are exempt — they become merge commits release-please ignores. See [CONTRIBUTING.md](../../CONTRIBUTING.md#pr-title--conventional-commits) for the full list.

## Break-Glass — Manual Tag

If release-please is unavailable (action regression, expired PAT, broken upstream), the manual tag flow still works as the operator backstop:

```bash
git checkout release
git pull --ff-only origin release
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

The tag push fires `Deploy Production` exactly as it did before automation, and the Environment gate still applies. The cascade back-merge workflow does NOT fire in this path — you must manually open the back-merge PRs (or run a redeploy from a previous tag if you're rolling back, see below).

## Rollback

Run `Deploy Production` manually with the previous known-good tag:

```text
Actions -> Deploy Production -> Run workflow -> tag = v0.9.1
```

This checks out that tag under `/opt/vigilafrica/production` and rebuilds the production API stack with `APP_VERSION` set to the rollback tag.

## Hotfix

1. Branch from `release`: `hotfix/<short-name>`.
2. Open a PR back to `release` with title `fix: <description>` (Conventional Commits — see CONTRIBUTING.md). Squash-merge.
3. release-please opens a patch Release PR (`vX.Y.Z+1`). Review and merge.
4. Tag is created automatically; `Deploy Production` fires; Environment gate prompts for approval; smoke test asserts the new version.
5. Cascade back-merge auto-propagates the hotfix code + CHANGELOG entry to `main` and `development`. **No manual backport step.**
