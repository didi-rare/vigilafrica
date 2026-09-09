#!/usr/bin/env bash
# Run `npm audit` at the given level, retrying only TRANSPORT failures.
#
# Why this exists (2026-09-04): three consecutive PRs were blocked by a red
# "Run Root Dependency Audit" while status.npm.com reported all systems
# operational. The same commit passed on one trigger and failed on another.
#
# The mechanism, established by comparing the two runs:
#   passing run  43s     -> bulk advisory endpoint answered, "found 0 vulnerabilities"
#   failing run  5m07s   -> bulk endpoint stalled, npm fell back to the LEGACY
#                           /-/npm/v1/security/audits/quick endpoint, which npm is
#                           decommissioning and which now answers 400 with the
#                           misleading body "Invalid package tree, run npm install
#                           to rebuild your package-lock.json".
#
# The lockfile is NOT the problem — it was checked: 74 entries, every one with a
# version and a registry `resolved` URL, no link/file/git deps, lockfileVersion 3.
# Do not go rebuilding lockfiles because of that message.
#
# ⚠️ This retries transport failures ONLY. A real advisory still fails the build
# on the first attempt, and if every attempt fails to reach the registry the step
# FAILS rather than passing quietly — a security gate that cannot run must not be
# reported as a gate that passed.

set -uo pipefail

level="${1:-moderate}"
attempts="${2:-3}"
delay=10

# Bound each attempt. npm's default fetch timeout is minutes long and it retries
# internally, so an unbounded wrapper turns a degraded endpoint into a 20-minute
# step before it even fails — one observed CI run took 28 minutes. We do the
# retrying ourselves, so npm should give up quickly and hand back control.
FETCH_TIMEOUT_MS=45000

for attempt in $(seq 1 "${attempts}"); do
  out="$(npm audit --audit-level="${level}" --fetch-timeout="${FETCH_TIMEOUT_MS}" --fetch-retries=0 2>&1)"
  code=$?

  if ! grep -qE "audit endpoint returned an error|ECONNRESET|network timeout|audits/quick" <<<"${out}"; then
    # Reached the registry: this exit code reflects advisories, not transport.
    printf '%s\n' "${out}"
    exit "${code}"
  fi

  echo "npm-audit-retry: attempt ${attempt}/${attempts} could not reach the advisory endpoint."
  printf '%s\n' "${out}" | grep -E "npm warn audit|statusCode|npm error" | head -4
  [ "${attempt}" -lt "${attempts}" ] && { echo "  retrying in ${delay}s"; sleep "${delay}"; delay=$((delay * 2)); }
done

echo
echo "npm-audit-retry: FAILED — the npm advisory endpoint was unreachable on all ${attempts} attempts."
echo "This is a transport failure, NOT a clean audit. The gate did not run, so it is"
echo "reported as a failure rather than a pass. Re-run once npm's bulk advisory"
echo "endpoint recovers; do not rebuild the lockfile on the strength of npm's"
echo "\"Invalid package tree\" message, which is the retired endpoint's generic 400 body."
exit 1
