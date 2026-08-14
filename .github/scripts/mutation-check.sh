#!/usr/bin/env bash
#
# Which checks in write-known-hosts.sh are INDEPENDENTLY load-bearing?
# Disable one at a time and see whether test-write-known-hosts.sh still passes.
#
# A "LEAK" means the suite passed without that check -- i.e. the suite does not
# prove the check is needed. That is a statement about THE SUITE, not proof the
# check is useless: another input outside the suite may still require it.
#
# ⚠️ Every probe asserts its mutation actually applied. Without that, source
# drift silently turns a probe into a no-op and the check gets misreported as
# not load-bearing -- the harness would then quietly stop testing anything.
set -uo pipefail

D="$(mktemp -d)"; trap 'rm -rf -- "${D}"' EXIT
SRC="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cp "${SRC}/test-write-known-hosts.sh" "${D}/"
ORIG="${SRC}/write-known-hosts.sh"

probe() {
  local name="$1" expr="$2"
  cp "${ORIG}" "${D}/write-known-hosts.sh"
  perl -0pi -e "${expr}" "${D}/write-known-hosts.sh"

  if cmp -s "${ORIG}" "${D}/write-known-hosts.sh"; then
    printf '  ??   %-26s MUTATION DID NOT APPLY -- probe is stale, fix it\n' "${name}"
    return
  fi
  if ! bash -n "${D}/write-known-hosts.sh" 2>/dev/null; then
    printf '  ??   %-26s mutation broke syntax -- probe is wrong\n' "${name}"
    return
  fi

  if bash "${D}/test-write-known-hosts.sh" > /dev/null 2>&1; then
    printf '  LEAK %-26s suite still passes without it\n' "${name}"
  else
    printf '  ok   %-26s suite fails without it\n' "${name}"
  fi
}

probe "exact-host compare"   's/\[ "\$\{host_field\}" = "\$\{VPS_HOST\}" \]/true/'
probe "key-material parse"   's/ssh-keygen -lf "\$\{tmpkey\}" > \/dev\/null 2>&1/true/'
probe "final ssh-keygen -F"  's/ssh-keygen -F "\$\{VPS_HOST\}" -f "\$\{KNOWN_HOSTS\}" > \/dev\/null 2>&1/true/'
probe "wildcard\/negation"    's/\*\[\*\?\!\]\*\)/*[\x01]*)/'
probe "record-count > 0"     's/\[ "\$\{records\}" -gt 0 \]/true/'
probe "\@marker reject"       's/\@\*\) die/\@\x01*) die/'
probe "hashed-record reject" "s/'\\|1\\|'\\*\\)/'\\x01'*)/"
probe "leading-space trim"   's/line="\$\{line#"\$\{line%%\[!\[:space:\]\]\*\}"\}"/:/'
probe "secret non-empty"     's/\[ -n "\$\{VPS_HOST_KEY\}" \]/true/'
probe "VPS_HOST non-empty"   's/\[ -n "\$\{VPS_HOST\}" \]/true/'
