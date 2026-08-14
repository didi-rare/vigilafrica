#!/usr/bin/env bash
#
# Exercises write-known-hosts.sh.
#
# Two rules this suite is built around, both learned the hard way:
#
#   1. Every rejection case is paired with an acceptance CONTROL, so a script
#      that fails for an unrelated reason cannot masquerade as working
#      validation. (An earlier run under Git Bash "passed" only because
#      `install -m 700` failed before any check ran.)
#   2. Each rejection case must be rejectable by exactly ONE check. An earlier
#      "negated pattern" case used `!vps.example.org,*` -- which also contains
#      `*`, so deleting `!` from the pattern left the suite still reporting
#      13/13. Independent review found that by mutation. Cases below are kept
#      minimal so a mutation to one check fails one case.
#
# ⚠️ Honest limit of this suite: run mutation-check.sh and you will see that
# only the exact-host compare and the key-material parse fail it when disabled
# individually. The @marker, wildcard and hashed-record cases below are really
# re-testing the exact-host compare, because none of those host fields can
# equal VPS_HOST. They are still worth keeping -- they pin the ERROR MESSAGE an
# operator gets -- but this suite does not prove those checks are independent
# controls, and nothing here should be quoted as if it did.
#
# Run on Linux/WSL -- it asserts on POSIX file modes, which do not survive a
# Windows filesystem.
set -uo pipefail

SCRIPT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/write-known-hosts.sh"
HOST="vps.example.org"
pass=0
fail=0

# Real key material. A hand-written "AAAA..." blob does not parse, which would
# make every acceptance control vacuous -- they would prove host-field lookup
# and nothing about whether the file is usable.
keydir="$(mktemp -d)"
trap 'rm -rf -- "${keydir}"' EXIT
ssh-keygen -q -t ed25519 -N '' -C '' -f "${keydir}/k"
KEY="$(cut -d' ' -f1,2 "${keydir}/k.pub")"
ssh-keygen -q -t rsa -b 2048 -N '' -C '' -f "${keydir}/r"
KEY2="$(cut -d' ' -f1,2 "${keydir}/r.pub")"

run_case() {
  local want="$1" desc="$2" host="$3" secret="$4"
  local home out rc
  home="$(mktemp -d)"
  out="$(HOME="${home}" VPS_HOST="${host}" VPS_HOST_KEY="${secret}" bash "${SCRIPT}" 2>&1)"
  rc=$?
  rm -rf -- "${home}"

  if { [ "${want}" = "accept" ] && [ "${rc}" -eq 0 ]; } ||
     { [ "${want}" = "reject" ] && [ "${rc}" -ne 0 ]; }; then
    printf '  ok   %-24s (%s, exit %d)\n' "${desc}" "${want}" "${rc}"
    pass=$((pass + 1))
  else
    printf '  FAIL %-24s expected %s, got exit %d\n' "${desc}" "${want}" "${rc}"
    printf '       %s\n' "${out}" | tail -2
    fail=$((fail + 1))
  fi
}

echo "Rejections:"
run_case reject "empty secret"        "${HOST}" ""
run_case reject "whitespace only"     "${HOST}" "   "
run_case reject "no VPS_HOST"         ""        "${HOST} ${KEY}"
run_case reject "hostname mismatch"   "${HOST}" "other.example.org ${KEY}"
run_case reject "wildcard *"          "${HOST}" "* ${KEY}"
run_case reject "wildcard suffix"     "${HOST}" "vps.* ${KEY}"
run_case reject "negation only"       "${HOST}" "!other.example.org ${KEY}"
run_case reject "cert-authority"      "${HOST}" "@cert-authority ${HOST} ${KEY}"
run_case reject "revoked record"      "${HOST}" "@revoked ${HOST} ${KEY}"
run_case reject "hashed record"       "${HOST}" "|1|abc=|def= ${KEY}"
run_case reject "host,extra-host"     "${HOST}" "${HOST},other.example.org ${KEY}"
run_case reject "extra host line"     "${HOST}" "$(printf '%s %s\nother.example.org %s' "${HOST}" "${KEY}" "${KEY}")"
run_case reject "malformed key blob"  "${HOST}" "${HOST} ssh-ed25519 AAAAnotvalidbase64!!"
run_case reject "key type only"       "${HOST}" "${HOST} ssh-ed25519"
run_case reject "host only, no key"   "${HOST}" "${HOST}"
run_case reject "mismatched type"     "${HOST}" "${HOST} ssh-rsa $(echo "${KEY}" | cut -d' ' -f2)"

echo "Controls (must be accepted):"
run_case accept "exact match"         "${HOST}" "${HOST} ${KEY}"
run_case accept "comment + blank"     "${HOST}" "$(printf '# pinned\n\n%s %s' "${HOST}" "${KEY}")"
run_case accept "CRLF line endings"   "${HOST}" "$(printf '%s %s\r' "${HOST}" "${KEY}")"
run_case accept "rotation: 2 keys"    "${HOST}" "$(printf '%s %s\n%s %s' "${HOST}" "${KEY}" "${HOST}" "${KEY2}")"
run_case accept "leading whitespace"  "${HOST}" "  ${HOST} ${KEY}"

# The accepted file must actually be usable by ssh, not merely well-formed.
home="$(mktemp -d)"
HOME="${home}" VPS_HOST="${HOST}" VPS_HOST_KEY="${HOST} ${KEY}" bash "${SCRIPT}" > /dev/null 2>&1
mode="$(stat -c '%a' "${home}/.ssh/known_hosts" 2>/dev/null || echo missing)"
if [ "${mode}" = "600" ]; then
  echo "  ok   known_hosts mode         (600)"; pass=$((pass + 1))
else
  echo "  FAIL known_hosts mode         expected 600, got ${mode}"; fail=$((fail + 1))
fi
if ssh-keygen -F "${HOST}" -f "${home}/.ssh/known_hosts" > /dev/null 2>&1; then
  echo "  ok   ssh can find the host    (ssh-keygen -F)"; pass=$((pass + 1))
else
  echo "  FAIL ssh cannot find the host"; fail=$((fail + 1))
fi
rm -rf -- "${home}"

echo
echo "${pass} passed, ${fail} failed"
[ "${fail}" -eq 0 ]
