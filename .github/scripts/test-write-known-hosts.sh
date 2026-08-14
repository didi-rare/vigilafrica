#!/usr/bin/env bash
#
# Exercises write-known-hosts.sh. Every rejection case is paired with a control
# that must be ACCEPTED, so a script that fails for an unrelated reason (a bad
# path, an unsupported chmod) cannot be mistaken for working validation.
#
# Run on Linux/WSL -- it asserts on POSIX file modes, which do not survive a
# Windows filesystem.
set -uo pipefail

SCRIPT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/write-known-hosts.sh"
KEY="ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExampleExampleExampleExampleExampleXX"
pass=0
fail=0

run_case() {
  local want="$1" desc="$2" host="$3" secret="$4"
  local home out rc
  home="$(mktemp -d)"
  out="$(HOME="${home}" VPS_HOST="${host}" VPS_HOST_KEY="${secret}" bash "${SCRIPT}" 2>&1)"
  rc=$?
  rm -rf -- "${home}"

  if { [ "${want}" = "accept" ] && [ "${rc}" -eq 0 ]; } ||
     { [ "${want}" = "reject" ] && [ "${rc}" -ne 0 ]; }; then
    printf '  ok   %-22s (%s, exit %d)\n' "${desc}" "${want}" "${rc}"
    pass=$((pass + 1))
  else
    printf '  FAIL %-22s expected %s, got exit %d\n' "${desc}" "${want}" "${rc}"
    printf '       %s\n' "${out}" | tail -2
    fail=$((fail + 1))
  fi
}

echo "Rejections:"
run_case reject "empty secret"      "vps.example.org" ""
run_case reject "whitespace only"   "vps.example.org" "   "
run_case reject "no VPS_HOST"       ""                "vps.example.org ${KEY}"
run_case reject "hostname mismatch" "vps.example.org" "other.example.org ${KEY}"
run_case reject "wildcard *"        "vps.example.org" "* ${KEY}"
run_case reject "wildcard vps.*"    "vps.example.org" "vps.* ${KEY}"
run_case reject "negated pattern"   "vps.example.org" "!vps.example.org,* ${KEY}"
run_case reject "cert-authority"    "vps.example.org" "@cert-authority vps.example.org ${KEY}"
run_case reject "revoked record"    "vps.example.org" "@revoked vps.example.org ${KEY}"

echo "Controls (must be accepted):"
run_case accept "exact match"       "vps.example.org" "vps.example.org ${KEY}"
run_case accept "host,ip list"      "vps.example.org" "vps.example.org,203.0.113.9 ${KEY}"
run_case accept "comment + blank"   "vps.example.org" "$(printf '# pinned\n\nvps.example.org %s' "${KEY}")"

# The mode assertion needs its own accepted run to inspect.
home="$(mktemp -d)"
HOME="${home}" VPS_HOST="vps.example.org" VPS_HOST_KEY="vps.example.org ${KEY}" \
  bash "${SCRIPT}" > /dev/null 2>&1
mode="$(stat -c '%a' "${home}/.ssh/known_hosts" 2>/dev/null || echo missing)"
rm -rf -- "${home}"
if [ "${mode}" = "600" ]; then
  echo "  ok   known_hosts mode      (600)"
  pass=$((pass + 1))
else
  echo "  FAIL known_hosts mode      expected 600, got ${mode}"
  fail=$((fail + 1))
fi

echo
echo "${pass} passed, ${fail} failed"
[ "${fail}" -eq 0 ]
