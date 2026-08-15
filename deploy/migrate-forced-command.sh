#!/usr/bin/env bash
#
# Migrate a LIVE VigilAfrica host to the forced-command deploy protocol.
# Implements the rollout half of chore-vps-access-hardening tasks 1.3 and 1.4.
#
#   sudo ./deploy/migrate-forced-command.sh --check     # preflight only, changes nothing
#   sudo SSH_PUBLIC_KEY='ssh-ed25519 AAAA...' ./deploy/migrate-forced-command.sh
#
# WHY THIS EXISTS SEPARATELY FROM provision.sh
#
# provision.sh is an INITIAL provisioning script. Re-running it on a live host
# also runs `apt-get upgrade -y`, reinstalls packages, re-applies ufw rules and
# restarts Docker and Caddy -- i.e. it restarts production and may leave the
# box wanting a reboot. None of that is required by 1.3/1.4. This script does
# only the security migration: no apt, no firewall, no service restarts.
#
# ORDER IS THE WHOLE DESIGN. Steps 1-3 are inert: they install files nothing
# uses yet, so an abort leaves the existing deploy path fully working. Step 4
# is the cutover.
#
#   1  preflight               nothing is modified
#   2  install helper scripts  inert until authorized_keys points at them
#   3  install sudoers         inert until the helper is invoked
#   4  re-own the deploy trees CUTOVER: the old path can no longer git-checkout
#   5  forced authorized_keys  new protocol live
#   6  drop the docker group   last, and irreversible for the old path
#
# ⚠️ Between step 4 and promoting the new workflows to `main`, NO deploy can
# succeed: the old workflow sends a shell program, which the forced command
# overrides. That window is expected. Keep it short, and keep an admin session
# open throughout -- if this host has no second administrative account, stop
# now and do task 1.1 first.
set -euo pipefail

APP_ROOT=/opt/vigilafrica
DEPLOY_USER=deploy
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SSH_PUBLIC_KEY="${SSH_PUBLIC_KEY:-}"
CHECK_ONLY=0
[ "${1:-}" = "--check" ] && CHECK_ONLY=1

die()  { printf '\nFAILED: %s\n' "$1" >&2; exit 1; }
step() { printf '\n=== %s ===\n' "$1"; }
ok()   { printf '  ok   %s\n' "$1"; }
warn() { printf '  WARN %s\n' "$1"; }

# --------------------------------------------------------------------------
step "1. Preflight (nothing is modified)"
# --------------------------------------------------------------------------
[ "$(id -u)" -eq 0 ] || die "run as root (sudo)"

for f in vigil-deploy vigil-deploy-run sudoers.d/vigil-deploy sudoers.d/vigil-deploy.pre-1.9.10; do
  [ -f "${SCRIPT_DIR}/${f}" ] || die "missing ${SCRIPT_DIR}/${f} -- run this from a checkout of the repo"
done
ok "source files present"

for c in flock visudo ssh-keygen git docker gpasswd find stat; do
  command -v "$c" >/dev/null || die "required command not found: $c"
done
ok "required commands present"

id "${DEPLOY_USER}" >/dev/null 2>&1 || die "user ${DEPLOY_USER} does not exist"
ok "deploy user exists"

# A second administrative account is the rescue path if this goes wrong.
# Do not rely on the deploy account: this script is about to restrict it.
# ⚠️ `|| true` is load-bearing. `getent group admin` exits non-zero on hosts
# with no `admin` group -- which is most modern Ubuntu -- and under `set -e`
# that aborts the preflight with exit 2 and NO message, before the lockout
# check below ever runs. Caught in testing; it would have looked like a broken
# script rather than a safety refusal.
admins="$(getent group sudo 2>/dev/null | cut -d: -f4 | tr ',' ' ' || true)"
admins="${admins} $(getent group admin 2>/dev/null | cut -d: -f4 | tr ',' ' ' || true)"
found_admin=0
for a in ${admins}; do
  [ -n "${a}" ] && [ "${a}" != "${DEPLOY_USER}" ] && found_admin=1
done
if [ "${found_admin}" -eq 1 ]; then
  ok "administrative account(s) exist besides ${DEPLOY_USER}:$(printf ' %s' ${admins})"
else
  die "no administrative account other than ${DEPLOY_USER} found in the sudo/admin groups.
       Do task 1.1 first -- create an admin user, verify it from a SECOND concurrent
       session, and only then run this. Without it a mistake here is a lockout."
fi

SUDO_VER="$(sudo -V 2>/dev/null | sed -n '1s/.*version \([0-9.]*\).*/\1/p')"
[ -n "${SUDO_VER}" ] || die "cannot determine sudo version; refusing to guess which rule to install"
SUDOERS_SRC="${SCRIPT_DIR}/sudoers.d/vigil-deploy"
if [ "$(printf '%s\n1.9.10\n' "${SUDO_VER}" | sort -V | head -1)" != "1.9.10" ] && [ "${SUDO_VER}" != "1.9.10" ]; then
  SUDOERS_SRC="${SCRIPT_DIR}/sudoers.d/vigil-deploy.pre-1.9.10"
  warn "sudo ${SUDO_VER} < 1.9.10: no regex argv matching."
  warn "Installing the wildcard fallback, which does NOT constrain argv --"
  warn "sudoers(5) '*' matches across white space. Enforcement then rests"
  warn "entirely on vigil-deploy-run's own re-validation. Upgrade sudo."
else
  ok "sudo ${SUDO_VER} supports regex argv matching"
fi
visudo -cf "${SUDOERS_SRC}" >/dev/null || die "sudoers source ${SUDOERS_SRC} is invalid"
ok "sudoers source validates ($(basename "${SUDOERS_SRC}"))"

for d in "${APP_ROOT}/staging" "${APP_ROOT}/production"; do
  [ -d "${d}" ] || die "missing ${d}"
  [ -f "${d}/.env" ] || die "missing ${d}/.env -- the root helper needs it"
done
ok "deploy trees and .env files present"

if [ -z "${SSH_PUBLIC_KEY}" ]; then
  if [ "${CHECK_ONLY}" -eq 1 ]; then
    warn "SSH_PUBLIC_KEY not set -- required for the real run"
  else
    die "SSH_PUBLIC_KEY is required. Without it this script would leave the
       existing unrestricted key in place and report success."
  fi
else
  [ "$(printf '%s' "${SSH_PUBLIC_KEY}" | grep -c '')" -eq 1 ] \
    || die "SSH_PUBLIC_KEY must be exactly one line; a stray newline would add a
       second, UNRESTRICTED authorized_keys entry"
  printf '%s\n' "${SSH_PUBLIC_KEY}" | ssh-keygen -l -f /dev/stdin >/dev/null 2>&1 \
    || die "SSH_PUBLIC_KEY is not a valid OpenSSH public key"
  ok "public key is valid and single-line"
fi

# Report what will change, so --check is genuinely informative.
printf '\n  Current state:\n'
printf '    %-28s %s\n' "docker group membership:" \
  "$(id -nG "${DEPLOY_USER}" | tr ' ' '\n' | grep -qx docker && echo 'YES (will be removed)' || echo 'no (already cut over)')"
for d in "${APP_ROOT}/staging" "${APP_ROOT}/production"; do
  n="$(find "${d}" \( ! -user root -o -perm /022 \) -print 2>/dev/null | wc -l)"
  printf '    %-28s %s\n' "$(basename "${d}") non-root/writable:" "${n} path(s)"
done
printf '    %-28s %s\n' "authorized_keys:" \
  "$(grep -q 'command="/usr/local/bin/vigil-deploy"' "/home/${DEPLOY_USER}/.ssh/authorized_keys" 2>/dev/null \
     && echo 'already forced' || echo 'UNRESTRICTED (will be replaced)')"

if [ "${CHECK_ONLY}" -eq 1 ]; then
  printf '\n--check: no changes made.\n'
  exit 0
fi

# --------------------------------------------------------------------------
step "2. Install helper scripts (inert -- nothing invokes them yet)"
# --------------------------------------------------------------------------
install -m 0755 -o root -g root "${SCRIPT_DIR}/vigil-deploy"     /usr/local/bin/vigil-deploy
install -m 0700 -o root -g root "${SCRIPT_DIR}/vigil-deploy-run" /usr/local/sbin/vigil-deploy-run
ok "/usr/local/bin/vigil-deploy (0755 root)"
ok "/usr/local/sbin/vigil-deploy-run (0700 root)"

# --------------------------------------------------------------------------
step "3. Install sudoers rule (inert -- deploy does not invoke it yet)"
# --------------------------------------------------------------------------
# Validate, then rename atomically. sudo reads this directory continuously and
# a half-written file there can break sudo for EVERY account, including the
# rescue admin.
TMP_SUDOERS=/etc/sudoers.d/.vigil-deploy.new
install -m 0440 -o root -g root "${SUDOERS_SRC}" "${TMP_SUDOERS}"
visudo -cf "${TMP_SUDOERS}" >/dev/null || { rm -f "${TMP_SUDOERS}"; die "sudoers rejected after install"; }
mv -f "${TMP_SUDOERS}" /etc/sudoers.d/vigil-deploy
visudo -c >/dev/null || die "FATAL: /etc/sudoers is now invalid -- fix from the admin session immediately"
ok "/etc/sudoers.d/vigil-deploy installed atomically and validated"

# --------------------------------------------------------------------------
step "4. CUTOVER -- re-own the deploy trees to root"
# --------------------------------------------------------------------------
# From here the OLD deploy path can no longer git-checkout as ${DEPLOY_USER}.
#
# ⚠️ Unconditional and recursive. An earlier revision did this only `if` the
# top directory was not already root -- but `install -d -o root` had already
# flipped it, so the chown never ran and the Dockerfile stayed deploy-writable
# while the directory looked migrated. Root then builds from that Dockerfile.
for d in "${APP_ROOT}/staging" "${APP_ROOT}/production"; do
  chown -R root:root "${d}"
  chmod -R go-w "${d}"
  ok "re-owned ${d} (root:root, no group/other write)"
done
chown root:root "${APP_ROOT}"; chmod 0755 "${APP_ROOT}"

# .env holds database and API credentials; deploy no longer needs to read it.
for d in "${APP_ROOT}/staging" "${APP_ROOT}/production"; do
  chmod 0600 "${d}/.env"; chown root:root "${d}/.env"
done
ok ".env files root-owned, 0600"

# Verify rather than assume. This is the check whose absence hid the bug above.
for d in "${APP_ROOT}/staging" "${APP_ROOT}/production"; do
  bad="$(find "${d}" \( ! -user root -o -perm /022 \) -print -quit 2>/dev/null || true)"
  [ -z "${bad}" ] || die "migration did not take: ${bad} is not root-owned or is group/world writable"
done
ok "verified: no non-root or group/world-writable path remains"

# --------------------------------------------------------------------------
step "5. Install the forced-command authorized_keys"
# --------------------------------------------------------------------------
# ROOT-owned: if ${DEPLOY_USER} owns this file it can simply delete the forced
# command, and the boundary lasts only until someone does.
KEYDIR="/home/${DEPLOY_USER}/.ssh"
install -d -m 0755 -o root -g root "${KEYDIR}"
if [ -f "${KEYDIR}/authorized_keys" ] && [ ! -f "${KEYDIR}/authorized_keys.pre-forced" ]; then
  cp -p "${KEYDIR}/authorized_keys" "${KEYDIR}/authorized_keys.pre-forced"
  chown root:root "${KEYDIR}/authorized_keys.pre-forced"
  chmod 0600 "${KEYDIR}/authorized_keys.pre-forced"
  ok "kept the previous authorized_keys as authorized_keys.pre-forced (root, 0600)"
fi
printf 'restrict,command="/usr/local/bin/vigil-deploy" %s\n' "${SSH_PUBLIC_KEY}" \
  > "${KEYDIR}/.authorized_keys.new"
chown root:root "${KEYDIR}/.authorized_keys.new"
chmod 0644 "${KEYDIR}/.authorized_keys.new"
mv -f "${KEYDIR}/.authorized_keys.new" "${KEYDIR}/authorized_keys"
ok "authorized_keys: exactly 1 restricted entry, root-owned"

# --------------------------------------------------------------------------
step "6. Final cutover -- remove ${DEPLOY_USER} from the docker group"
# --------------------------------------------------------------------------
# Membership of `docker` is equivalent to root, so leaving it would make
# everything above pointless. Last, because it is the irreversible step for the
# old path: if anything above had failed we would have exited already.
if id -nG "${DEPLOY_USER}" | tr ' ' '\n' | grep -qx docker; then
  gpasswd -d "${DEPLOY_USER}" docker >/dev/null
  ok "removed ${DEPLOY_USER} from the docker group"
else
  ok "${DEPLOY_USER} was not in the docker group"
fi

cat <<EOF

=== Migration complete. Now VERIFY, from your workstation. ===

The first three MUST be refused. A key that deploys successfully can still be
unrestricted, so testing only the happy path proves nothing:

  ssh -i <deploy_key> ${DEPLOY_USER}@<host>                    # no shell
  ssh -i <deploy_key> ${DEPLOY_USER}@<host> 'id'               # refused
  ssh -i <deploy_key> ${DEPLOY_USER}@<host> 'docker ps'        # refused

Then confirm the protocol itself works:

  ssh -i <deploy_key> ${DEPLOY_USER}@<host> 'deploy-staging <7-hex-sha>'

Refusals are logged: journalctl -t vigil-deploy

⚠️ Staging deploys will FAIL until the new workflows reach 'main', because the
old workflow sends a shell program that the forced command overrides. Promote
development -> main now to close that window.

Rollback, from the admin session, if the deploy path must be restored:
  cp ${KEYDIR}/authorized_keys.pre-forced ${KEYDIR}/authorized_keys
  chown ${DEPLOY_USER}:${DEPLOY_USER} ${KEYDIR}/authorized_keys && chmod 600 \$_
  gpasswd -a ${DEPLOY_USER} docker
  chown -R ${DEPLOY_USER}:${DEPLOY_USER} ${APP_ROOT}/staging ${APP_ROOT}/production
EOF
