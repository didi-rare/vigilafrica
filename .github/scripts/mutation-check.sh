#!/usr/bin/env bash
# Ad-hoc: which checks in write-known-hosts.sh are INDEPENDENTLY load-bearing?
# Disable one check at a time and see whether the suite still passes.
set -uo pipefail
D="$(mktemp -d)"; trap 'rm -rf -- "${D}"' EXIT
SRC="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cp "${SRC}/write-known-hosts.sh" "${SRC}/test-write-known-hosts.sh" "${D}/"

probe() {
  local name="$1"; shift
  cp "${SRC}/write-known-hosts.sh" "${D}/write-known-hosts.sh"
  perl -0pi -e "$1" "${D}/write-known-hosts.sh"
  if ! bash -n "${D}/write-known-hosts.sh" 2>/dev/null; then
    printf '  ??   %-26s mutation broke syntax\n' "${name}"; return
  fi
  if bash "${D}/test-write-known-hosts.sh" > /dev/null 2>&1; then
    printf '  LEAK %-26s suite still passes without it\n' "${name}"
  else
    printf '  ok   %-26s suite fails without it\n' "${name}"
  fi
}

probe "exact-host compare"  's/\[ "\$\{host_field\}" = "\$\{VPS_HOST\}" \]/true/'
probe "key-material parse"  's/ssh-keygen -lf "\$\{tmpkey\}" > \/dev\/null 2>&1/true/'
probe "record-count > 0"    's/\[ "\$\{records\}" -gt 0 \]/true/'
probe "final ssh-keygen -F" 's/ssh-keygen -F "\$\{VPS_HOST\}" -f "\$\{KNOWN_HOSTS\}" > \/dev\/null 2>&1/true/'
probe "@marker reject"      's/\@\*\) die/\@ZZZZ) die/'
probe "wildcard\/negation"   's/\*\[\*\?\!\]\*\)/*[\@]*)/'
probe "hashed-record reject" "s/'\\|1\\|'\\*\\)/'ZZZZ'*)/"
