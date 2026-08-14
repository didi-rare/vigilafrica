#!/usr/bin/env bash
#
# Write ~/.ssh/known_hosts from the VPS_HOST_KEY secret, then validate it.
#
# The deploy workflows previously ran `ssh-keyscan` against the VPS on every
# run. That opens a connection and records whatever answers, so its output is
# only as trustworthy as the network at that moment -- it must never be the
# authority for the pin. See docs/deployment/vps.md for how the value is
# derived from the VPS's configured host key instead.
#
# Shared by deploy-staging.yml and deploy-production.yml so the two cannot
# drift, and so the validation is testable outside CI
# (see test-write-known-hosts.sh).
#
# Which checks below are actually load-bearing, measured by mutation-check.sh
# rather than assumed: only the EXACT-HOST COMPARE and the KEY-MATERIAL PARSE
# fail the suite when individually disabled. The @marker, wildcard/negation and
# hashed-record rejections are subsumed by the exact-host compare -- none of
# those host fields can ever equal VPS_HOST -- and the record count and the
# final `ssh-keygen -F` are redundant with each other. They are kept because
# they give an operator a specific, actionable message instead of a generic
# "does not pin VPS_HOST exactly", not because each is an independent control.
# Do not cite them as separate layers of defence.
set -euo pipefail

: "${VPS_HOST_KEY:=}"
: "${VPS_HOST:=}"

KNOWN_HOSTS="${HOME}/.ssh/known_hosts"

die() { printf '%s\n' "$@" >&2; exit 1; }

[ -n "${VPS_HOST_KEY}" ] || die \
  "VPS_HOST_KEY is unset; refusing to deploy without a pinned host key." \
  "Derive it from the VPS's configured host key -- see docs/deployment/vps.md."

[ -n "${VPS_HOST}" ] || die \
  "VPS_HOST is unset; cannot verify the pin matches the deploy target."

install -m 700 -d "${HOME}/.ssh"
printf '%s\n' "${VPS_HOST_KEY}" > "${KNOWN_HOSTS}"
chmod 600 "${KNOWN_HOSTS}"

# Validate every record individually.
#
# `ssh-keygen -F` alone is NOT sufficient: it reports that *some* record matches
# the host, not that the file is well-formed and not that it trusts nothing
# else. A file with valid-looking junk, or with an extra line trusting a
# different host, passes -F while widening what the deploy run will accept.
tmpkey="$(mktemp)"
trap 'rm -f "${tmpkey}"' EXIT

records=0
while IFS= read -r line || [ -n "${line}" ]; do
  line="${line%$'\r'}"                    # tolerate CRLF from a pasted secret
  line="${line#"${line%%[![:space:]]*}"}" # strip indentation: OpenSSH honours
                                          # indented records, so this parser
                                          # must not be stricter than ssh is
  case "${line}" in
    '' | '#'*) continue ;;
    @*) die "VPS_HOST_KEY must hold plain host-key records, not @cert-authority/@revoked markers." ;;
  esac

  host_field="${line%% *}"
  key_field="${line#* }"

  case "${host_field}" in
    '|1|'*)
      die "VPS_HOST_KEY must be a plain (unhashed) record so it can be checked against VPS_HOST." ;;
    *[*?!]*)
      die "VPS_HOST_KEY must pin an exact host, not a wildcard or negated pattern." ;;
  esac

  # Exact match only. A comma-separated list would trust this key for hosts
  # beyond the deploy target, and CheckHostIP is disabled, so an extra IP
  # alias buys nothing.
  [ "${host_field}" = "${VPS_HOST}" ] || die \
    "VPS_HOST_KEY has a record that does not pin VPS_HOST exactly." \
    "known_hosts matches the exact string ssh connects to, so an IP will not" \
    "match a DNS name, and extra hosts widen the pin beyond the deploy target."

  # The remainder must actually parse as a key, so malformed base64 fails here
  # rather than confusingly at connect time.
  printf '%s\n' "${key_field}" > "${tmpkey}"
  ssh-keygen -lf "${tmpkey}" > /dev/null 2>&1 || die \
    "VPS_HOST_KEY has a record whose key material does not parse as a public key."

  records=$((records + 1))
done < "${KNOWN_HOSTS}"

[ "${records}" -gt 0 ] || die "VPS_HOST_KEY contains no usable host-key records."

# Multiple records are allowed on purpose, so a host-key rotation can pin the
# outgoing and incoming keys at once. Every one of them is constrained above to
# this exact host with parseable key material.

# Belt and braces: SSH's own matcher must agree.
ssh-keygen -F "${VPS_HOST}" -f "${KNOWN_HOSTS}" > /dev/null 2>&1 || die \
  "VPS_HOST_KEY contains no entry matching VPS_HOST."

echo "Pinned host key verified for the deploy target (${records} record(s))."
