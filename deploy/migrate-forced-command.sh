#!/usr/bin/env bash
#
# Migrate a LIVE VigilAfrica host to the forced-command deploy protocol.
# Rollout half of chore-vps-access-hardening tasks 1.3 and 1.4.
#
#   sudo ./deploy/migrate-forced-command.sh --check
#   sudo ADMIN_USER=vigil-admin SSH_PUBLIC_KEY='ssh-ed25519 AAAA...' \
#        ./deploy/migrate-forced-command.sh
#
# WHY NOT provision.sh
#   provision.sh is INITIAL provisioning. Re-running it on a live host also
#   runs `apt-get upgrade -y`, reinstalls packages, re-applies ufw rules and
#   restarts Docker and Caddy -- restarting production. None of that is needed
#   by 1.3/1.4. This script does only the migration: no apt, no firewall, no
#   service restarts.
#
# ⚠️ WHY THE TREES ARE RE-CLONED, NOT RE-OWNED
#   An earlier revision ran `chown -R root:root` over the existing checkouts.
#   Independent review caught that this PROMOTES UNTRUSTED STATE TO ROOT: the
#   trees were deploy-writable, so .git/hooks/*, .git/config (core.hooksPath,
#   include.path, url.*.insteadOf) and any symlink placed there become
#   root-trusted, and the root helper then runs git in that tree. Re-owning a
#   directory does not sanitise its contents. So we clone fresh as root from a
#   pinned URL and carry over only .env, after validating it is a regular file.
#   That also removes the symlink / hard-link / ACL / mount-crossing concerns
#   the same review raised, rather than trying to enumerate them.
#
# ORDER. Steps 1-3 are inert -- an abort leaves the existing deploy path fully
# working. Step 4 is the cutover; step 6 the last irreversible one.
#
#   1  preflight              nothing modified
#   2  install helpers        inert
#   3  install sudoers        inert
#   4  fresh root clones      CUTOVER: old path can no longer deploy
#   5  forced authorized_keys new protocol live
#   6  drop the docker group  last
set -euo pipefail

APP_ROOT=/opt/vigilafrica
DEPLOY_USER=deploy
REPO_URL="${REPO_URL:-https://github.com/didi-rare/vigilafrica.git}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SSH_PUBLIC_KEY="${SSH_PUBLIC_KEY:-}"
ADMIN_USER="${ADMIN_USER:-}"
KEYDIR="/home/${DEPLOY_USER}/.ssh"
PHASE="startup"
CHECK_ONLY=0

# Argument parsing fails closed: an earlier revision treated ANY unrecognised
# argument as a real run, so `--chek` performed the live cutover.
case "$#" in
  0) ;;
  1) [ "$1" = "--check" ] || { echo "usage: $0 [--check]" >&2; exit 2; }; CHECK_ONLY=1 ;;
  *) echo "usage: $0 [--check]" >&2; exit 2 ;;
esac

step() { PHASE="$1"; printf '\n=== %s ===\n' "$1"; }
ok()   { printf '  ok   %s\n' "$1"; }
warn() { printf '  WARN %s\n' "$1"; }
die()  { printf '\nFAILED: %s\n' "$1" >&2; exit 1; }

print_recovery() {
  cat <<EOF

--- RECOVERY (run from your ${ADMIN_USER:-admin} session) ---
Phase reached: ${PHASE}

Nothing needs undoing if the phase above is 'preflight', 'install helpers' or
'install sudoers' -- those are inert and the old deploy path still works.

Otherwise restore the previous deploy path:

  sudo cp -p ${KEYDIR}/authorized_keys.pre-forced ${KEYDIR}/authorized_keys
  sudo chown ${DEPLOY_USER}:${DEPLOY_USER} ${KEYDIR}/authorized_keys
  sudo chmod 600 ${KEYDIR}/authorized_keys
  sudo gpasswd -a ${DEPLOY_USER} docker
  # restore the pre-migration checkouts (kept alongside the new ones):
  sudo rm -rf ${APP_ROOT}/staging ${APP_ROOT}/production
  sudo mv ${APP_ROOT}/staging.pre-migration    ${APP_ROOT}/staging
  sudo mv ${APP_ROOT}/production.pre-migration ${APP_ROOT}/production

Then confirm a deploy works before investigating further.
EOF
}

# Any unexpected failure must say WHERE it stopped and HOW to get back. An
# earlier revision printed the rollback only on total success -- i.e. never
# when it was actually needed.
on_err() {
  local rc=$? line="${1:-?}"
  printf '\n!!! ABORTED in phase "%s" (line %s, exit %s)\n' "${PHASE}" "${line}" "${rc}" >&2
  [ "${CHECK_ONLY}" -eq 1 ] || print_recovery >&2
}
trap 'on_err $LINENO' ERR

# --------------------------------------------------------------------------
step "1. Preflight (nothing is modified)"
# --------------------------------------------------------------------------
[ "$(id -u)" -eq 0 ] || die "run as root (sudo)"

for f in vigil-deploy vigil-deploy-run sudoers.d/vigil-deploy sudoers.d/vigil-deploy.pre-1.9.10; do
  [ -f "${SCRIPT_DIR}/${f}" ] || die "missing ${SCRIPT_DIR}/${f} -- run from a checkout of the repo"
done
ok "source files present"

for c in flock visudo ssh-keygen git docker gpasswd find stat sudo sed sort head install chown chmod cp mv mktemp getent pgrep; do
  command -v "$c" >/dev/null || die "required command not found: $c"
done
ok "required commands present"

id "${DEPLOY_USER}" >/dev/null 2>&1 || die "user ${DEPLOY_USER} does not exist"
ok "deploy user exists"

# A NAMED rescue account, verified to be usable. An earlier revision accepted
# any username listed in the sudo group -- a stale entry for a deleted or
# locked account read as a working rescue path.
[ -n "${ADMIN_USER}" ] || die "ADMIN_USER is required, e.g. ADMIN_USER=vigil-admin.
       It must be an administrative account OTHER than ${DEPLOY_USER} that you
       have just used from a second, concurrent session. Task 1.1 exists to
       create and prove it; without it a mistake here is a lockout."
[ "${ADMIN_USER}" != "${DEPLOY_USER}" ] || die "ADMIN_USER must not be ${DEPLOY_USER}"
id "${ADMIN_USER}" >/dev/null 2>&1 || die "ADMIN_USER '${ADMIN_USER}' does not exist"
admin_shell="$(getent passwd "${ADMIN_USER}" | cut -d: -f7)"
case "${admin_shell}" in
  */nologin | */false | "") die "ADMIN_USER '${ADMIN_USER}' has a non-login shell (${admin_shell:-none})" ;;
esac
# sudo AUTHORIZATION is the thing that must exist; this is the real gate.
sudo -l -U "${ADMIN_USER}" >/dev/null 2>&1 \
  || die "ADMIN_USER '${ADMIN_USER}' has no sudo privileges"
# ⚠️ Password state is a WARNING, not a refusal. `passwd -S` reports "L" both
# for a locked account and for one that simply has no password -- and a
# key-authenticated admin with NOPASSWD sudo is perfectly valid. Refusing on
# "L" falsely blocks a correctly configured host, which is worse than the
# thing it guards against, since sudo authorization is already proven above.
admin_pw="$(passwd -S "${ADMIN_USER}" 2>/dev/null | awk '{print $2}' || true)"
if [ "${admin_pw}" = "L" ]; then
  warn "'${ADMIN_USER}' has no usable password (passwd -S reports L)."
  warn "That is fine for key login with NOPASSWD sudo, but if sudo prompts you"
  warn "for a password it will fail. Confirm 'sudo -v' works in your session NOW."
fi
ok "rescue account '${ADMIN_USER}' exists, can log in, and is authorized for sudo"

if ! SUDO_VER="$(sudo -V 2>/dev/null | sed -n '1s/.*version \([0-9.]*\).*/\1/p')"; then
  die "could not run 'sudo -V' to determine the sudo version"
fi
[ -n "${SUDO_VER}" ] || die "cannot parse the sudo version; refusing to guess which rule to install"
SUDOERS_SRC="${SCRIPT_DIR}/sudoers.d/vigil-deploy"
if [ "$(printf '%s\n1.9.10\n' "${SUDO_VER}" | sort -V | head -1)" != "1.9.10" ] && [ "${SUDO_VER}" != "1.9.10" ]; then
  SUDOERS_SRC="${SCRIPT_DIR}/sudoers.d/vigil-deploy.pre-1.9.10"
  warn "sudo ${SUDO_VER} < 1.9.10: no regex argv matching. Installing the"
  warn "wildcard fallback, which does NOT constrain argv -- sudoers(5) '*'"
  warn "matches across white space. Enforcement then rests entirely on"
  warn "vigil-deploy-run's own re-validation. Upgrade sudo when you can."
else
  ok "sudo ${SUDO_VER} supports regex argv matching"
fi
visudo -cf "${SUDOERS_SRC}" >/dev/null || die "sudoers source $(basename "${SUDOERS_SRC}") is invalid"
ok "sudoers source validates"

for d in "${APP_ROOT}/staging" "${APP_ROOT}/production"; do
  [ -d "${d}" ] || die "missing ${d}"
  [ ! -L "${d}" ] || die "${d} is a symlink; refusing to migrate"
  [ -f "${d}/.env" ] || die "missing ${d}/.env -- the root helper needs it"
  [ ! -L "${d}/.env" ] || die "${d}/.env is a symlink; chown/chmod would act on its target"
done
ok "deploy trees present, .env files are regular files"

# OpenSSH refuses a key file if any path component is writable by others.
# Root-owned authorized_keys is fine, but /home/deploy itself must be sane.
home_bad="$(find "/home/${DEPLOY_USER}" -maxdepth 0 -perm /022 -o -maxdepth 0 ! -user root ! -user "${DEPLOY_USER}" 2>/dev/null || true)"
[ -z "${home_bad}" ] || die "/home/${DEPLOY_USER} is group/world writable or oddly owned; sshd StrictModes would reject the key file"
akf="$(sshd -T 2>/dev/null | sed -n 's/^authorizedkeysfile //p' || true)"
if [ -n "${akf}" ]; then
  case "${akf}" in
    *".ssh/authorized_keys"*) ok "sshd AuthorizedKeysFile includes .ssh/authorized_keys" ;;
    *) die "sshd AuthorizedKeysFile is '${akf}'; this script writes ${KEYDIR}/authorized_keys" ;;
  esac
else
  warn "could not read effective sshd config; confirm AuthorizedKeysFile manually"
fi

# Quiescing. A deploy running concurrently can race the swap below, and an
# already-open session keeps its docker supplementary group after gpasswd.
if pgrep -u "${DEPLOY_USER}" >/dev/null 2>&1; then
  procs="$(pgrep -a -u "${DEPLOY_USER}" | head -5 || true)"
  die "processes are running as ${DEPLOY_USER}; pause CI and let them finish first:
${procs}"
fi
ok "no processes running as ${DEPLOY_USER}"

if [ -n "${SSH_PUBLIC_KEY}" ]; then
  [ "$(printf '%s' "${SSH_PUBLIC_KEY}" | grep -c '')" -eq 1 ] \
    || die "SSH_PUBLIC_KEY must be exactly one line; a stray newline would add a
       second, UNRESTRICTED authorized_keys entry"
  # Must be a RAW key. ssh-keygen -l accepts a full authorized_keys record
  # including options, so `no-pty ssh-ed25519 ...` passes validation and then
  # produces an unparsable line once our own options are prepended.
  case "${SSH_PUBLIC_KEY}" in
    ssh-ed25519\ * | ssh-rsa\ * | ecdsa-sha2-nistp256\ * | ecdsa-sha2-nistp384\ * | ecdsa-sha2-nistp521\ * | sk-ssh-ed25519@openssh.com\ * ) ;;
    *) die "SSH_PUBLIC_KEY must begin with a key type (ssh-ed25519, ssh-rsa, ...).
       A record carrying its own options is rejected: prepending the forced
       command to it produces an invalid authorized_keys line." ;;
  esac
  printf '%s\n' "${SSH_PUBLIC_KEY}" | ssh-keygen -l -f /dev/stdin >/dev/null 2>&1 \
    || die "SSH_PUBLIC_KEY is not a valid OpenSSH public key"
  # Validate the EXACT line we will install, not just the key.
  probe="$(mktemp)"
  printf 'restrict,command="/usr/local/bin/vigil-deploy" %s\n' "${SSH_PUBLIC_KEY}" > "${probe}"
  ssh-keygen -l -f "${probe}" >/dev/null 2>&1 \
    || { rm -f "${probe}"; die "the constructed authorized_keys line does not parse"; }
  rm -f "${probe}"
  ok "public key is a raw single-line key, and the constructed line parses"
elif [ "${CHECK_ONLY}" -eq 1 ]; then
  warn "SSH_PUBLIC_KEY not set -- required for the real run"
else
  die "SSH_PUBLIC_KEY is required. Without it this script would leave the
       existing unrestricted key in place and report success."
fi

# The release lineage supplies the workflow for tag-triggered production
# deploys, so promoting development->main alone does not fix production.
if git ls-remote --heads "${REPO_URL}" release >/dev/null 2>&1; then
  ok "origin reachable for a fresh clone"
else
  die "cannot reach ${REPO_URL} to clone; check network/DNS from this host"
fi

printf '\n  Current state:\n'
printf '    %-30s %s\n' "docker group membership:" \
  "$(id -nG "${DEPLOY_USER}" | tr ' ' '\n' | grep -qx docker && echo 'YES (will be removed)' || echo 'no (already cut over)')"
printf '    %-30s %s\n' "authorized_keys:" \
  "$(grep -q 'command="/usr/local/bin/vigil-deploy"' "${KEYDIR}/authorized_keys" 2>/dev/null \
     && echo 'already forced' || echo 'UNRESTRICTED (will be replaced)')"
for d in "${APP_ROOT}/staging" "${APP_ROOT}/production"; do
  printf '    %-30s %s\n' "$(basename "${d}") tree:" \
    "$(stat -c '%U' "${d}") -- will be replaced by a fresh root clone"
done

if [ "${CHECK_ONLY}" -eq 1 ]; then
  printf '\n--check: no changes made.\n'
  printf 'Re-run without --check to migrate. Keep your %s session open.\n' "${ADMIN_USER:-admin}"
  exit 0
fi

# --------------------------------------------------------------------------
step "2. install helpers"
# --------------------------------------------------------------------------
install -m 0755 -o root -g root "${SCRIPT_DIR}/vigil-deploy"     /usr/local/bin/vigil-deploy
install -m 0700 -o root -g root "${SCRIPT_DIR}/vigil-deploy-run" /usr/local/sbin/vigil-deploy-run
ok "/usr/local/bin/vigil-deploy (0755 root), /usr/local/sbin/vigil-deploy-run (0700 root)"

# --------------------------------------------------------------------------
step "3. install sudoers"
# --------------------------------------------------------------------------
# sudo reads this directory continuously; a half-written file there can break
# sudo for EVERY account, including the rescue admin. Validate, then rename.
TMP_SUDOERS=/etc/sudoers.d/.vigil-deploy.new
install -m 0440 -o root -g root "${SUDOERS_SRC}" "${TMP_SUDOERS}"
visudo -cf "${TMP_SUDOERS}" >/dev/null || { rm -f "${TMP_SUDOERS}"; die "sudoers rejected after install"; }
mv -f "${TMP_SUDOERS}" /etc/sudoers.d/vigil-deploy
visudo -c >/dev/null || die "FATAL: /etc/sudoers is now invalid -- fix from ${ADMIN_USER} immediately"
ok "/etc/sudoers.d/vigil-deploy installed atomically and validated"

# --------------------------------------------------------------------------
step "4. CUTOVER -- fresh root-owned clones"
# --------------------------------------------------------------------------
for env in staging production; do
  d="${APP_ROOT}/${env}"
  staged="${APP_ROOT}/.${env}.new.$$"

  # Clone into a temporary path first, so a failure never leaves the live path
  # missing. GIT_CONFIG_* pins the clone's configuration: the old tree's
  # config and hooks are deliberately not consulted at all.
  rm -rf "${staged}"
  GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null \
    git clone --quiet --no-local "${REPO_URL}" "${staged}" \
    || die "clone into ${staged} failed"

  # Keep the deployed revision identical, so this migration does not also
  # change what is running. The SHA is read from the old tree but validated
  # against the fresh clone before use.
  old_sha="$(GIT_CONFIG_GLOBAL=/dev/null git -C "${d}" rev-parse HEAD 2>/dev/null || true)"
  if printf '%s' "${old_sha}" | grep -qE '^[0-9a-f]{40}$' \
     && git -C "${staged}" cat-file -e "${old_sha}^{commit}" 2>/dev/null; then
    git -C "${staged}" checkout --quiet --force "${old_sha}"
    ok "${env}: fresh clone checked out at ${old_sha:0:7} (unchanged from before)"
  else
    warn "${env}: could not reuse the previous revision; left on the default branch."
    warn "${env}: the next deploy will set the correct ref."
  fi

  # Carry over only .env, and only if it is a regular file.
  [ -f "${d}/.env" ] && [ ! -L "${d}/.env" ] || die "${d}/.env vanished or is a symlink"
  cp --no-dereference --preserve=timestamps "${d}/.env" "${staged}/.env"
  chown root:root "${staged}/.env"; chmod 0600 "${staged}/.env"

  chown -R root:root "${staged}"
  chmod -R go-w "${staged}"

  # Verify with the SAME predicate vigil-deploy-run enforces, including
  # symlinks -- otherwise migration can "succeed" on a tree the helper then
  # refuses, breaking every future deploy.
  bad="$(find "${staged}" \( ! -user root -o -perm /022 -o -type l \) -print -quit 2>/dev/null || true)"
  [ -z "${bad}" ] || die "fresh clone is not safe: ${bad}"

  # Swap: keep the old tree for rollback rather than deleting it.
  rm -rf "${d}.pre-migration"
  mv "${d}" "${d}.pre-migration"
  mv "${staged}" "${d}"
  ok "${env}: swapped in; previous tree kept at $(basename "${d}").pre-migration"
done
chown root:root "${APP_ROOT}"; chmod 0755 "${APP_ROOT}"

# --------------------------------------------------------------------------
step "5. forced authorized_keys"
# --------------------------------------------------------------------------
# ROOT-owned: if ${DEPLOY_USER} owns this file it can simply delete the forced
# command, and the boundary lasts only until someone does.
install -d -m 0755 -o root -g root "${KEYDIR}"

# Back up the CURRENT file every run, into a fresh timestamped name. An
# earlier revision wrote one backup and thereafter trusted any file that
# happened to exist -- a stale one would be restored as the "recovery" key.
if [ -f "${KEYDIR}/authorized_keys" ] && [ ! -L "${KEYDIR}/authorized_keys" ]; then
  bk="${KEYDIR}/authorized_keys.pre-forced"
  if [ -e "${bk}" ]; then
    mv -f "${bk}" "${bk}.$(date -u +%Y%m%dT%H%M%SZ)"
  fi
  cp --no-dereference --preserve=timestamps "${KEYDIR}/authorized_keys" "${bk}"
  chown root:root "${bk}"; chmod 0600 "${bk}"
  ok "backed up the previous authorized_keys to $(basename "${bk}")"
else
  warn "no previous authorized_keys to back up"
fi

# mktemp, not a predictable name: this directory was deploy-controlled, and a
# pre-existing symlink at a guessable path would be followed by a redirect.
NEWKEYS="$(mktemp "${KEYDIR}/.ak.XXXXXXXX")"
printf 'restrict,command="/usr/local/bin/vigil-deploy" %s\n' "${SSH_PUBLIC_KEY}" > "${NEWKEYS}"
ssh-keygen -l -f "${NEWKEYS}" >/dev/null 2>&1 || { rm -f "${NEWKEYS}"; die "constructed authorized_keys does not parse; nothing replaced"; }
chown root:root "${NEWKEYS}"; chmod 0644 "${NEWKEYS}"
mv -f "${NEWKEYS}" "${KEYDIR}/authorized_keys"
ok "authorized_keys: exactly 1 restricted entry, root-owned, validated"

# --------------------------------------------------------------------------
step "6. drop the docker group"
# --------------------------------------------------------------------------
if id -nG "${DEPLOY_USER}" | tr ' ' '\n' | grep -qx docker; then
  gpasswd -d "${DEPLOY_USER}" docker >/dev/null
  ok "removed ${DEPLOY_USER} from the docker group"
else
  ok "${DEPLOY_USER} was not in the docker group"
fi

PHASE="complete"
trap - ERR

cat <<EOF

=== Migration complete. VERIFY from your workstation. ===

The first three MUST be refused. A key that deploys successfully can still be
unrestricted, so testing only the happy path proves nothing:

  ssh -i <deploy_key> ${DEPLOY_USER}@<host>                     # no shell
  ssh -i <deploy_key> ${DEPLOY_USER}@<host> 'id'                # refused
  ssh -i <deploy_key> ${DEPLOY_USER}@<host> 'docker ps'         # refused

Then the protocol itself:

  ssh -i <deploy_key> ${DEPLOY_USER}@<host> 'deploy-staging <7-hex-sha>'

Refusals are logged:  journalctl -t vigil-deploy

⚠️ STAGING deploys fail until the new workflows reach 'main'. Promote
   development -> main now to close that window.

🚨 PRODUCTION needs more than that. Tag-triggered deploys run the workflow
   from the TAG'S commit, and tags are cut on 'release'. Until the new
   workflow is in the release lineage, the next production tag will send the
   old shell program and be refused. Promote through to 'release' BEFORE the
   next release, or use workflow_dispatch from the updated default branch.

Previous state kept for rollback:
  ${APP_ROOT}/staging.pre-migration
  ${APP_ROOT}/production.pre-migration
  ${KEYDIR}/authorized_keys.pre-forced
EOF
print_recovery
