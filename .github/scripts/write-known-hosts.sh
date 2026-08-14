#!/usr/bin/env bash
#
# Write ~/.ssh/known_hosts from the VPS_HOST_KEY secret, then validate it.
#
# The deploy workflows previously ran `ssh-keyscan` against the VPS on every
# run. That opens a connection and records whatever answers, so it cannot
# authenticate the result and cannot detect the host substitution it appears to
# guard against. That is true wherever it runs -- including on the VPS itself,
# which is why the runbook derives the pin from /etc/ssh/ssh_host_*_key.pub
# rather than from a scan.
#
# Shared by deploy-staging.yml and deploy-production.yml so the two cannot
# drift, and so the validation is testable outside CI.
set -euo pipefail

: "${VPS_HOST_KEY:=}"
: "${VPS_HOST:=}"

KNOWN_HOSTS="${HOME}/.ssh/known_hosts"

if [ -z "${VPS_HOST_KEY}" ]; then
  echo "VPS_HOST_KEY is unset; refusing to deploy without a pinned host key." >&2
  echo "Read it from /etc/ssh/ssh_host_ed25519_key.pub on the VPS console," >&2
  echo "not with ssh-keyscan. See docs/deployment/vps.md." >&2
  exit 1
fi

if [ -z "${VPS_HOST}" ]; then
  echo "VPS_HOST is unset; cannot verify the pinned key matches the deploy target." >&2
  exit 1
fi

install -m 700 -d "${HOME}/.ssh"
printf '%s\n' "${VPS_HOST_KEY}" > "${KNOWN_HOSTS}"
chmod 600 "${KNOWN_HOSTS}"

# @cert-authority delegates trust to a CA for anything it matches, and @revoked
# changes the meaning of the record. Neither is an exact-host pin.
if grep -qE '^[[:space:]]*@(cert-authority|revoked)' "${KNOWN_HOSTS}"; then
  echo "VPS_HOST_KEY must hold plain host-key lines, not @cert-authority/@revoked records." >&2
  exit 1
fi

# The first field is a host pattern list; a wildcard there trusts this key for
# more hosts than the one we mean to pin.
# NB: set a flag rather than `exit 0` here -- an exit inside the main block still
# runs END, and an exit there would override the status.
if awk '!/^[[:space:]]*(#|$)/ { if ($1 ~ /[*?!]/) found = 1 } END { exit(found ? 0 : 1) }' "${KNOWN_HOSTS}"; then
  echo "VPS_HOST_KEY must pin an exact host, not a wildcard or negated pattern." >&2
  exit 1
fi

# Use SSH's own matcher rather than a homemade string compare. This catches the
# hostname/IP mismatch case, which would otherwise fail at connect time with a
# far less obvious message.
if ! ssh-keygen -F "${VPS_HOST}" -f "${KNOWN_HOSTS}" > /dev/null 2>&1; then
  echo "VPS_HOST_KEY contains no entry matching VPS_HOST." >&2
  echo "known_hosts matches the exact string ssh connects to, so an IP in the" >&2
  echo "secret will not match a DNS name in VPS_HOST (or vice versa)." >&2
  exit 1
fi

echo "Pinned host key verified for the deploy target."
