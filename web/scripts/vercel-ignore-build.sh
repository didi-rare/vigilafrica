#!/usr/bin/env bash
# Vercel Ignored Build Step — skip builds for refs that are not the project's deploy branch.
#
# Wired in each Vercel project under Settings > Build and Deployment > Ignored Build Step:
#     bash scripts/vercel-ignore-build.sh
#
# Per-project env var (Settings > Environments > All Environments):
#     vigilafrica-production  DEPLOY_BRANCH=release
#     vigilafrica-staging     DEPLOY_BRANCH=main
#
# Vercel exit-code convention:
#     1 = proceed with build
#     0 = skip build

set -euo pipefail

ref="${VERCEL_GIT_COMMIT_REF:-}"
deploy_branch="${DEPLOY_BRANCH:-}"

if [ -z "$deploy_branch" ]; then
  echo "vercel-ignore-build: DEPLOY_BRANCH is unset; failing open and building." >&2
  exit 1
fi

if [ "$ref" = "$deploy_branch" ]; then
  echo "vercel-ignore-build: ref '$ref' matches DEPLOY_BRANCH; building."
  exit 1
fi

echo "vercel-ignore-build: ref '${ref:-<none>}' != DEPLOY_BRANCH '$deploy_branch'; skipping."
exit 0
