#!/usr/bin/env bash
#
# Assert both deploy workflows are still wired to the host-key pin.
#
# test-write-known-hosts.sh proves the VALIDATOR works. It says nothing about
# whether the workflows still call it, or still pass the options that make the
# pinned file the exclusive trust source. Either could be deleted and CI would
# stay green -- which is how a security control quietly stops existing.
set -uo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/../.." || exit 1

WORKFLOWS=(.github/workflows/deploy-staging.yml .github/workflows/deploy-production.yml)

# Every option that must be present on the ssh invocation, with why it matters.
REQUIRED_OPTS=(
  "StrictHostKeyChecking=yes"        # refuse an unknown/changed host key
  "UserKnownHostsFile"               # read the pin we just wrote
  "GlobalKnownHostsFile=/dev/null"   # ignore /etc/ssh/ssh_known_hosts*
  "KnownHostsCommand=none"           # no external key provider
  "VerifyHostKeyDNS=no"              # no SSHFP records from DNS
  "CheckHostIP=no"                   # no separate IP-keyed entries
  "UpdateHostKeys=no"                # server cannot rotate our pin under us
)

fail=0
for wf in "${WORKFLOWS[@]}"; do
  if [ ! -f "${wf}" ]; then
    echo "FAIL ${wf}: missing"; fail=1; continue
  fi

  if ! grep -q 'write-known-hosts\.sh' "${wf}"; then
    echo "FAIL ${wf}: does not call write-known-hosts.sh"; fail=1
  fi

  for opt in "${REQUIRED_OPTS[@]}"; do
    if ! grep -qF -- "${opt}" "${wf}"; then
      echo "FAIL ${wf}: missing ssh option ${opt}"; fail=1
    fi
  done

  # The whole point of the change: never relearn the key at deploy time.
  if grep -q 'ssh-keyscan' "${wf}"; then
    echo "FAIL ${wf}: ssh-keyscan reintroduced"; fail=1
  fi

  [ "${fail}" -eq 0 ] && echo "ok   ${wf}"
done

if [ ! -x .github/scripts/write-known-hosts.sh ] && [ ! -f .github/scripts/write-known-hosts.sh ]; then
  echo "FAIL .github/scripts/write-known-hosts.sh is missing"; fail=1
fi

if [ "${fail}" -ne 0 ]; then
  echo "Deploy host-key wiring is broken. The validator can be perfect and still never run."
  exit 1
fi
echo "Deploy host-key wiring intact."
