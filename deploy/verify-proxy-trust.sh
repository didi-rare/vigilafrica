#!/usr/bin/env bash
# Verify the client-IP trust boundary against a RUNNING stack.
# chore-vps-access-hardening task 5.2.
#
# Why this exists rather than "extend the production smoke test":
# a GitHub runner presents ONE client address and no endpoint exposes the
# resolved bucket key, so a smoke test cannot tell "one global bucket" from
# "correct per-client buckets" — it passes straight through the regression it
# is meant to catch. This probe instead speaks to the API from two DIFFERENT
# peers, which is the variable that actually matters:
#
#   1. an untrusted peer  — a throwaway container on the same bridge, forging
#                           X-Forwarded-For. It must be DISBELIEVED.
#   2. the trusted peer   — through Caddy, which is what real traffic does.
#                           It must still be BELIEVED, or geolocation is dead.
#
# A check that only proves (1) would pass with TRUSTED_PROXY_CIDRS empty, which
# breaks every real client. Both directions are required.
#
# Usage:  sudo ./verify-proxy-trust.sh staging|production
# Exit:   0 all checks passed, 1 a check failed, 2 could not run the checks.

set -uo pipefail

environment="${1:-}"
case "${environment}" in
  staging)    container="vigilafrica-staging-api"; public="https://api.staging.vigilafrica.org" ;;
  production) container="vigilafrica-prod-api";    public="https://api.vigilafrica.org" ;;
  *) echo "usage: $0 staging|production" >&2; exit 2 ;;
esac

command -v docker >/dev/null 2>&1 || { echo "docker not available (need root?)" >&2; exit 2; }
docker inspect "${container}" >/dev/null 2>&1 || { echo "no such container: ${container}" >&2; exit 2; }

network=$(docker inspect -f '{{range $k,$v := .NetworkSettings.Networks}}{{$k}}{{end}}' "${container}")
gateway=$(docker inspect -f '{{range $v := .NetworkSettings.Networks}}{{$v.Gateway}}{{end}}' "${container}")
address=$(docker inspect -f '{{range $v := .NetworkSettings.Networks}}{{$v.IPAddress}}{{end}}' "${container}")
configured=$(docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' "${container}" | sed -n 's/^TRUSTED_PROXY_CIDRS=//p')

echo "environment          : ${environment}"
echo "container / network  : ${container} on ${network}"
echo "api address          : ${address}"
echo "bridge gateway       : ${gateway}"
echo "TRUSTED_PROXY_CIDRS  : ${configured:-<unset, API default applies>}"
echo

failed=0

# --- 1. An untrusted peer on the same bridge must not be able to forge -------
# Sent from a throwaway container, so the peer is a container address, NOT the
# gateway. Under the old 172.16.0.0/12 this peer was trusted and the forgery
# would have been believed.
echo "[1/3] untrusted peer forging X-Forwarded-For: 8.8.8.8 ..."
forged=$(docker run --rm --network "${network}" curlimages/curl:latest \
  -s --max-time 15 -H 'X-Forwarded-For: 8.8.8.8' -H 'X-Real-IP: 8.8.8.8' \
  "http://${address}:8080/v1/context" 2>/dev/null)

if [ -z "${forged}" ]; then
  echo "      COULD NOT RUN — no response from ${address}:8080" >&2
  failed=1
elif printf '%s' "${forged}" | grep -q '"location_source":"ip"'; then
  echo "      FAIL — the forged header was BELIEVED from an untrusted peer."
  echo "             ${forged:0:200}"
  failed=1
else
  echo "      pass — forgery refused (location_source is not 'ip')."
fi

# --- 2. The real proxy path must still resolve a location -------------------
# The failure mode of over-narrowing: forgery is refused, and so is Caddy, so
# every real user silently loses geolocation.
echo "[2/3] real request through Caddy ..."
real=$(curl -s --max-time 20 "${public}/v1/context")
if printf '%s' "${real}" | grep -q '"location_source":"ip"'; then
  echo "      pass — Caddy is still trusted and the client was geolocated."
else
  echo "      FAIL — Caddy is NOT trusted; real clients get no location."
  echo "             ${real:0:200}"
  failed=1
fi

# --- 3. The API's own startup drift check -----------------------------------
echo "[3/3] startup gateway check in the container log ..."
line=$(docker logs "${container}" 2>&1 | grep -F 'trusted-proxy check' | tail -1)
if [ -z "${line}" ]; then
  echo "      inconclusive — no 'trusted-proxy check' line found."
  echo "      The log may have rotated past startup; restart the API to re-emit it."
elif printf '%s' "${line}" | grep -q 'FAILED'; then
  echo "      FAIL — the API reports its own gateway is not trusted:"
  echo "             ${line}"
  failed=1
else
  echo "      pass — ${line}"
fi

echo
if [ "${failed}" -eq 0 ]; then
  echo "RESULT: PASS — forgery refused from an untrusted peer, and the real proxy path still works."
else
  echo "RESULT: FAIL — see above."
fi
exit "${failed}"
