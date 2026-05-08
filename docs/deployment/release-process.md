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
3. Create an annotated tag from `release`:

```bash
git checkout release
git pull --ff-only origin release
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

4. Approve the `production` GitHub Environment gate.
5. Confirm `https://api.vigilafrica.org/health` reports `"version":"v1.0.0"`.

## Rollback

Run `Deploy Production` manually with the previous known-good tag:

```text
Actions -> Deploy Production -> Run workflow -> tag = v0.9.1
```

This checks out that tag under `/opt/vigilafrica/production` and rebuilds the production API stack with `APP_VERSION` set to the rollback tag.

## Hotfix

1. Branch from `release`: `hotfix/<short-name>`.
2. Open a PR back to `release`.
3. After merge, tag the patch version.
4. Backport or merge the fix into `main` and `development`.
