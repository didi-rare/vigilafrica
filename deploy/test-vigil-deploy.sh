#!/usr/bin/env bash
#
# Tests the forced-command protocol's argument handling.
#
# This is the part an attacker holding a leaked VPS_SSH_KEY interacts with
# directly, so it is worth testing hard and worth testing OFFLINE -- none of it
# needs a VPS. What is NOT covered here is the git/compose behaviour of
# vigil-deploy-run, which needs a real host; that is called out at the end
# rather than left as a silent gap.
#
# Every rejection case is paired with acceptance controls, so a script that
# fails for an unrelated reason cannot pass as working validation.
#
# Run on Linux/WSL.
set -uo pipefail

SRC="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
D="$(mktemp -d)"; trap 'rm -rf -- "${D}"' EXIT

# Stub sudo: record argv instead of elevating.
cat > "${D}/sudo-stub" <<'STUB'
#!/usr/bin/env bash
# drop the leading -n that vigil-deploy passes
[ "${1:-}" = "-n" ] && shift
printf '%s\n' "$*" > "${SUDO_ARGV_FILE}"
# Record whether stdin still carries the client's data. vigil-deploy must have
# replaced it with /dev/null, so a read here must hit EOF immediately.
# ⚠️ Without this the "discards stdin" case was VACUOUS: the stub never read
# stdin, so deleting `exec < /dev/null` from vigil-deploy still passed 29/29.
if IFS= read -r -t 1 leaked; then
  printf '%s\n' "${leaked}" > "${SUDO_STDIN_FILE}"
else
  : > "${SUDO_STDIN_FILE}"
fi
exit 0
STUB
chmod +x "${D}/sudo-stub"

# Copy the real script, pointing SUDO at the stub. Nothing else is changed --
# the validation under test is the shipped code.
sed "s|^SUDO=/usr/bin/sudo$|SUDO=${D}/sudo-stub|" "${SRC}/vigil-deploy" > "${D}/vigil-deploy"
chmod +x "${D}/vigil-deploy"
grep -q "sudo-stub" "${D}/vigil-deploy" || { echo "FATAL: sudo stub not injected"; exit 1; }

pass=0; fail=0

run() {  # run <accept|reject> <desc> <SSH_ORIGINAL_COMMAND> [expected-argv] [pin]
  local want="$1" desc="$2" cmd="$3" expect_argv="${4:-}" pin="${5:-}"
  local rc argv
  export SUDO_ARGV_FILE="${D}/argv" SUDO_STDIN_FILE="${D}/stdin"
  : > "${SUDO_ARGV_FILE}"; : > "${SUDO_STDIN_FILE}"

  # ⚠️ Default the pinned environment to the one the VERB implies, so each case
  # still tests what its name claims. Pinning everything to staging would make
  # the four production cases below pass because the pin mismatched -- never
  # reaching the ref validation they exist to exercise. That is the vacuous-test
  # failure this suite has been bitten by twice. Pass an explicit 5th argument
  # only for the deliberate cross-environment cases.
  if [ -z "${pin}" ]; then
    case "${cmd}" in
      deploy-production*) pin=production ;;
      *)                  pin=staging ;;
    esac
  fi

  # Feed stdin deliberately: the script must discard it.
  SSH_ORIGINAL_COMMAND="${cmd}" SSH_CLIENT="203.0.113.7 1234 22" \
    bash "${D}/vigil-deploy" "${pin}" > /dev/null 2>&1 <<< "echo pwned; rm -rf /"
  rc=$?
  argv="$(cat "${SUDO_ARGV_FILE}" 2>/dev/null || true)"

  local ok=1
  if [ "${want}" = "accept" ]; then
    [ "${rc}" -eq 0 ] || ok=0
    [ -n "${expect_argv}" ] && [ "${argv}" != "${expect_argv}" ] && ok=0
  else
    [ "${rc}" -ne 0 ] || ok=0
    # A rejected request must never have reached the privileged helper.
    [ -n "${argv}" ] && ok=0
  fi

  if [ "${ok}" -eq 1 ]; then
    printf '  ok   %-34s (%s)\n' "${desc}" "${want}"; pass=$((pass+1))
  else
    printf '  FAIL %-34s want=%s rc=%d argv=[%s]\n' "${desc}" "${want}" "${rc}" "${argv}"; fail=$((fail+1))
  fi
}

echo "Refusals — a leaked key must get none of these:"
run reject "interactive shell (no command)" ""
run reject "bare shell verb"               "bash"
run reject "unknown verb"                  "deploy-everything abc1234"
run reject "command chaining ;"            "deploy-staging abc1234; id"
run reject "command chaining &&"           "deploy-staging abc1234 && id"
run reject "pipe"                          "deploy-staging abc1234 | id"
run reject "backtick substitution"         'deploy-staging `id`'
run reject "dollar substitution"           'deploy-staging $(id)'
run reject "redirection"                   "deploy-staging abc1234 > /etc/passwd"
run reject "newline injection"             "deploy-staging abc1234
id"
run reject "too many arguments"            "deploy-staging abc1234 extra"
run reject "missing ref"                   "deploy-staging"
run reject "flag as ref"                   "deploy-staging --build"
run reject "path traversal ref"            "deploy-staging ../../etc/passwd"
run reject "staging ref too short"         "deploy-staging abc12"
run reject "staging ref not hex"           "deploy-staging zzzzzzz"
run reject "staging given a tag"           "deploy-staging v1.4.0"
run reject "production given a sha"        "deploy-production abc1234"
run reject "production tag malformed"      "deploy-production 1.4.0"
run reject "production tag with slash"     "deploy-production v1.4.0/../../x"

echo "Task 1.5 — one account per environment; a key may not cross over:"
# These are the whole point of the split. Both requests are WELL-FORMED and
# would be accepted by the other account -- they are refused solely because the
# key is pinned elsewhere.
run reject "staging key wants production"  "deploy-production v1.4.0"  "" staging
run reject "prod key wants staging"        "deploy-staging abc1234"    "" production

# ⚠️ The pin WHITELIST is not an authorization gate, and saying otherwise would
# be a false claim in a security test.
#
# `environment` is only ever `staging` or `production` (the verb case sets it),
# so the cross-environment check above already refuses every malformed pin. The
# whitelist cannot change any accept/reject outcome -- proven by mutation:
# deleting it leaves the suite green. What it DOES do is refuse earlier and name
# the actual fault, which is what an operator reads at 3am.
#
# So assert the REASON, not the verdict. Asserting only the verdict is how this
# suite previously certified a check that did nothing.
expect_msg() {  # expect_msg <desc> <pin> <cmd> <expected stderr substring>
  local desc="$1" pin="$2" cmd="$3" want="$4" err rc
  export SUDO_ARGV_FILE="${D}/argv" SUDO_STDIN_FILE="${D}/stdin"
  : > "${SUDO_ARGV_FILE}"
  err="$(SSH_ORIGINAL_COMMAND="${cmd}" SSH_CLIENT="203.0.113.7 1234 22" \
          bash "${D}/vigil-deploy" "${pin}" 2>&1 >/dev/null <<< "")"
  rc=$?
  if [ "${rc}" -ne 0 ] && [ ! -s "${SUDO_ARGV_FILE}" ] && [[ "${err}" == *"${want}"* ]]; then
    printf '  ok   %-34s (said: %s)\n' "${desc}" "${want}"; pass=$((pass+1))
  else
    printf '  FAIL %-34s rc=%d err=[%s]\n' "${desc}" "${rc}" "${err}"; fail=$((fail+1))
  fi
}

expect_msg "invalid pin names the fault"  "both"             "deploy-staging abc1234" "invalid pinned environment"
expect_msg "path-shaped pin names it too" "../../etc/passwd" "deploy-staging abc1234" "invalid pinned environment"

# Fail closed: a forced command left at the pre-1.5 form passes NO argument.
# The account must lose its authority rather than keep both environments.
export SUDO_ARGV_FILE="${D}/argv" SUDO_STDIN_FILE="${D}/stdin"
: > "${SUDO_ARGV_FILE}"
SSH_ORIGINAL_COMMAND="deploy-staging abc1234" SSH_CLIENT="203.0.113.7 1234 22" \
  bash "${D}/vigil-deploy" > /dev/null 2>&1 <<< ""
rc=$?
if [ "${rc}" -ne 0 ] && [ ! -s "${SUDO_ARGV_FILE}" ]; then
  printf '  ok   %-34s (refused, helper never reached)\n' "legacy forced command, no pin"
  pass=$((pass+1))
else
  printf '  FAIL %-34s rc=%d argv=[%s]\n' "legacy forced command, no pin" "${rc}" "$(cat "${SUDO_ARGV_FILE}")"
  fail=$((fail+1))
fi

echo "Controls — the real protocol must work:"
# The stub consumes sudo's leading -n, so expectations start at the helper path.
run accept "staging short sha"   "deploy-staging abc1234"                  "/usr/local/sbin/vigil-deploy-run staging abc1234"
run accept "staging full sha"    "deploy-staging $(printf 'a%.0s' {1..40})" "/usr/local/sbin/vigil-deploy-run staging $(printf 'a%.0s' {1..40})"
run accept "production tag"      "deploy-production v1.4.0"                "/usr/local/sbin/vigil-deploy-run production v1.4.0"
run accept "production prerelease" "deploy-production v1.4.0-rc.1"         "/usr/local/sbin/vigil-deploy-run production v1.4.0-rc.1"

# Stdin must not survive into the privileged helper. `run` always pipes a
# payload in; assert the helper saw EOF rather than that payload.
export SUDO_ARGV_FILE="${D}/argv" SUDO_STDIN_FILE="${D}/stdin"
: > "${SUDO_STDIN_FILE}"
SSH_ORIGINAL_COMMAND="deploy-staging abc1234" SSH_CLIENT="203.0.113.7 1234 22" \
  bash "${D}/vigil-deploy" staging > /dev/null 2>&1 <<< "echo pwned; rm -rf /"
if [ -s "${SUDO_STDIN_FILE}" ]; then
  printf '  FAIL %-24s client stdin reached the helper: %s\n' "stdin discarded" "$(head -1 "${SUDO_STDIN_FILE}")"
  fail=$((fail+1))
else
  printf '  ok   %-24s (helper saw EOF)\n' "stdin discarded"
  pass=$((pass+1))
fi

# ---- privileged helper: argument re-validation (root check stubbed out) ----
echo "Privileged helper re-validates its own arguments:"
# NB: use perl and a trailing .* -- the line contains `||`, which terminates a
# sed s|...| expression early and silently produced a no-op mutation here once.
perl -pe 's/^\[ "\$\(id -u\)" -eq 0 \].*$/:/' \
  "${SRC}/vigil-deploy-run" > "${D}/helper"
chmod +x "${D}/helper"
grep -qE '^\[ "\$\(id -u\)"' "${D}/helper" && { echo "FATAL: root check not stubbed"; exit 1; }

helper() {  # helper <reject> <desc> <args...>
  local desc="$1"; shift
  # Point APP_ROOT at a dir that does not exist, so anything reaching the git
  # stage fails anyway; we only assert on the validation messages here.
  out="$(APP_ROOT="${D}/nope" bash "${D}/helper" "$@" 2>&1)"; rc=$?
  if [ "${rc}" -ne 0 ] && printf '%s' "${out}" | grep -qiE 'invalid|unknown|expected exactly'; then
    printf '  ok   %-34s (rejected)\n' "${desc}"; pass=$((pass+1))
  else
    printf '  FAIL %-34s rc=%d out=%s\n' "${desc}" "${rc}" "$(printf '%s' "${out}" | head -1)"; fail=$((fail+1))
  fi
}
helper "unknown environment"    foo v1.4.0
helper "wrong arg count"        staging
helper "staging given a tag"    staging v1.4.0
helper "production given a sha" production abc1234
helper "production bad tag"     production "v1.4.0;id"

# ---- failures AFTER the lock must still be visible -------------------------
#
# ⚠️ Regression test for a live defect. The lock was originally acquired with
#
#     exec 9> "/run/lock/..." 2>/dev/null || exec 9> "/tmp/..."
#
# and `exec` with no command applies its redirections to the whole shell, so
# that `2>/dev/null` silenced stderr for the REST OF THE SCRIPT. Every failure
# past that point exited 1 with no output. It reached the production host and
# was only found by running it there.
#
# The cases above cannot catch it: they all die during argument validation,
# BEFORE the lock. This one deliberately gets past the lock and asserts a
# diagnostic still reaches stderr.
echo "Diagnostics survive past the lock:"
fakeroot="${D}/approot"
mkdir -p "${fakeroot}/staging"
: > "${fakeroot}/staging/.env"
chmod 0777 "${fakeroot}/staging"          # group/world writable -> guard must fire
sed -e 's|^APP_ROOT=/opt/vigilafrica$|APP_ROOT='"${fakeroot}"'|' \
    "${D}/helper" > "${D}/helper-approot"
grep -q "APP_ROOT=${fakeroot}" "${D}/helper-approot" || { echo "  FATAL: APP_ROOT not rewritten"; fail=$((fail+1)); }

out="$(bash "${D}/helper-approot" staging 0123456789abcdef0123456789abcdef01234567 2>&1)"; rc=$?
if [ "${rc}" -ne 0 ] && [ -n "${out}" ]; then
  printf '  ok   %-24s (exit %d, said: %s)\n' "post-lock failure speaks" "${rc}" "$(printf '%s' "${out}" | head -1 | cut -c1-58)"
  pass=$((pass + 1))
else
  printf '  FAIL %-24s exit=%d but produced NO diagnostic -- stderr is being swallowed\n' "post-lock failure speaks" "${rc}"
  fail=$((fail + 1))
fi

echo
echo "${pass} passed, ${fail} failed"
echo
echo "NOT covered here (needs a real host): git fetch/checkout behaviour, the"
echo "server-side origin/release ancestry check, the root-ownership guard on"
echo "APP_ROOT, sudoers enforcement, and compose execution. Those are the"
echo "staging rollout steps in tasks 1.3/1.4."
[ "${fail}" -eq 0 ]
