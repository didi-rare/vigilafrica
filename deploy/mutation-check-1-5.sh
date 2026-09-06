#!/usr/bin/env bash
#
# Mutation check for the task 1.5 guards in vigil-deploy.
#
# A passing test suite proves nothing about a guard unless removing the guard
# makes the suite fail. This deliberately breaks each new check and asserts the
# suite NOTICES. It also fails loudly when a mutation does not apply at all --
# otherwise drift in vigil-deploy silently turns this into a no-op that reports
# success, which is the exact failure mode it exists to catch.
#
# Run under WSL, not Git Bash: the suite it drives contains POSIX-permission
# logic that does not behave on a Windows filesystem.
set -uo pipefail

SRC="$(cd "$(dirname "$0")" && pwd)"
W="$(mktemp -d)"
trap 'rm -rf "${W}"' EXIT

cp -r "${SRC}" "${W}/deploy"
cd "${W}"

pass=0
fail=0

mut() {  # mut <name> <sed-expr> <substring of the case that must now FAIL>
  local name="$1" expr="$2" must_fail="$3" out

  cp deploy/vigil-deploy deploy/.orig
  sed -i "${expr}" deploy/vigil-deploy

  if cmp -s deploy/vigil-deploy deploy/.orig; then
    printf '  BAD  mutation DID NOT APPLY: %s\n' "${name}"
    printf '       (the sed matched nothing — vigil-deploy has drifted and this\n'
    printf '        check has quietly become a no-op)\n'
    fail=$((fail + 1))
    mv deploy/.orig deploy/vigil-deploy
    return
  fi

  out="$(bash deploy/test-vigil-deploy.sh 2>&1)"

  if printf '%s' "${out}" | grep -q "FAIL.*${must_fail}"; then
    printf '  ok   mutation CAUGHT: %s\n' "${name}"
    pass=$((pass + 1))
  else
    printf '  BAD  mutation SURVIVED: %s\n' "${name}"
    printf '       suite still passed the case: %s\n' "${must_fail}"
    fail=$((fail + 1))
  fi

  mv deploy/.orig deploy/vigil-deploy
}

echo "Mutation check — every new 1.5 guard must be load-bearing:"

# 1. The cross-environment check itself.
mut "drop cross-environment check" \
    '/^\[ "\${environment}" = "\${allowed_env}" \]/,+1d' \
    "staging key wants production"

# 2. Fail-closed on a missing pin (a pre-1.5 forced command).
mut "accept an empty pin" \
    "s|^  '') deny \"no environment pinned in the forced command\" ;;|  '') allowed_env=staging ;;|" \
    "legacy forced command"

# 3. The pin whitelist.
#
# ⚠️ This one does NOT change any accept/reject outcome -- the cross-environment
# check subsumes it, which this harness proved by leaving the suite green when
# the whitelist was deleted. Its job is to name the fault, so the assertion is
# on the MESSAGE. Recorded explicitly because "the suite passes without check X"
# had previously been misread here as "check X is redundant" when the real
# answer was "the suite is incomplete".
mut "accept any pin value (message check)" \
    's|^  staging\|production) ;;|  *) ;;|' \
    "invalid pin names the fault"

echo
printf '%d mutations caught, %d problems\n' "${pass}" "${fail}"
[ "${fail}" -eq 0 ] || exit 1
