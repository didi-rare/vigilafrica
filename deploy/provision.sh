#!/usr/bin/env bash
set -euo pipefail

APP_ROOT="${APP_ROOT:-/opt/vigilafrica}"
DEPLOY_USER="${DEPLOY_USER:-deploy}"
SSH_PUBLIC_KEY="${SSH_PUBLIC_KEY:-}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# ⚠️ These two LOOK configurable but are not, and a mismatch produces a broken
# install that reports success: vigil-deploy-run hardcodes APP_ROOT, and the
# sudoers rules hardcode the `deploy` user name. Fail loudly rather than
# provision something that cannot deploy.
if [[ "${APP_ROOT}" != "/opt/vigilafrica" ]]; then
  echo "APP_ROOT is pinned to /opt/vigilafrica (hardcoded in vigil-deploy-run)." >&2
  echo "Change it there and in deploy/sudoers.d/* together, or leave it unset." >&2
  exit 1
fi
if [[ "${DEPLOY_USER}" != "deploy" ]]; then
  echo "DEPLOY_USER is pinned to 'deploy' (hardcoded in deploy/sudoers.d/*)." >&2
  echo "Change it there too, or leave it unset." >&2
  exit 1
fi

if [[ "${EUID}" -ne 0 ]]; then
  echo "Run as root: sudo APP_ROOT=/opt/vigilafrica SSH_PUBLIC_KEY='ssh-ed25519 ...' ./deploy/provision.sh" >&2
  exit 1
fi

apt-get update
DEBIAN_FRONTEND=noninteractive apt-get upgrade -y
DEBIAN_FRONTEND=noninteractive apt-get install -y \
  ca-certificates curl debian-keyring debian-archive-keyring apt-transport-https \
  fail2ban ufw unattended-upgrades git gnupg

if ! command -v docker >/dev/null 2>&1; then
  curl -fsSL https://get.docker.com | sh
fi

if ! command -v caddy >/dev/null 2>&1; then
  install -m 0755 -d /usr/share/keyrings
  curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' \
    | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
  curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' \
    > /etc/apt/sources.list.d/caddy-stable.list
  apt-get update
  DEBIAN_FRONTEND=noninteractive apt-get install -y caddy
fi

if ! id "${DEPLOY_USER}" >/dev/null 2>&1; then
  useradd --create-home --shell /bin/bash "${DEPLOY_USER}"
fi

# ⚠️ Docker-group removal is deliberately NOT here. It is the last cutover
# step, at the end of this script. Removing it before the helper, sudoers,
# forced key and ownership migration are all in place means any later failure
# leaves the OLD deploy path unable to reach Docker and the NEW one not yet
# working -- i.e. no way to deploy at all.
#
# Preflight instead: fail before mutating anything if the inputs are wrong.
if [[ -z "${SSH_PUBLIC_KEY}" ]]; then
  echo "SSH_PUBLIC_KEY is required: without it this script would leave the deploy" >&2
  echo "account's existing unrestricted key in place and report success." >&2
  exit 1
fi
if [[ "$(printf '%s' "${SSH_PUBLIC_KEY}" | grep -c '')" -ne 1 ]]; then
  echo "SSH_PUBLIC_KEY must be exactly one line; a pasted newline would add a" >&2
  echo "second, unrestricted authorized_keys entry." >&2
  exit 1
fi
if ! printf '%s\n' "${SSH_PUBLIC_KEY}" | ssh-keygen -l -f /dev/stdin >/dev/null 2>&1; then
  echo "SSH_PUBLIC_KEY is not a valid OpenSSH public key" >&2
  exit 1
fi
for f in vigil-deploy vigil-deploy-run sudoers.d/vigil-deploy sudoers.d/vigil-deploy.pre-1.9.10; do
  [[ -f "${SCRIPT_DIR}/${f}" ]] || { echo "missing ${SCRIPT_DIR}/${f}" >&2; exit 1; }
done
command -v flock  >/dev/null || { echo "flock is required by vigil-deploy-run" >&2; exit 1; }
command -v visudo >/dev/null || { echo "visudo is required" >&2; exit 1; }

# --- forced-command deploy protocol (chore-vps-access-hardening 1.3 / 1.4) ---
#
# These ship together. Installing the forced command without the scripts breaks
# every deploy; installing the scripts without the forced command leaves the
# key able to open a shell.
install -m 0755 -o root -g root "${SCRIPT_DIR}/vigil-deploy"     /usr/local/bin/vigil-deploy
install -m 0700 -o root -g root "${SCRIPT_DIR}/vigil-deploy-run" /usr/local/sbin/vigil-deploy-run

# Regex argument matching needs sudo >= 1.9.10. On older sudo the `^...$` rule
# parses but never matches, so every deploy would fail -- safe, but broken and
# confusing. Pick the form this host can actually enforce.
SUDO_VER="$(sudo -V 2>/dev/null | sed -n '1s/.*version \([0-9.]*\).*/\1/p')"
SUDOERS_SRC="${SCRIPT_DIR}/sudoers.d/vigil-deploy"
# Refuse to guess. An unreadable version must not silently select the regex
# rule, which on old sudo parses but never matches -- every deploy would fail.
[[ -n "${SUDO_VER}" ]] || { echo "cannot determine sudo version; refusing to guess" >&2; exit 1; }
if [[ -n "${SUDO_VER}" ]] &&
   [[ "$(printf '%s\n1.9.10\n' "${SUDO_VER}" | sort -V | head -1)" != "1.9.10" ]] &&
   [[ "${SUDO_VER}" != "1.9.10" ]]; then
  SUDOERS_SRC="${SCRIPT_DIR}/sudoers.d/vigil-deploy.pre-1.9.10"
  echo "WARNING: sudo ${SUDO_VER} predates regex argument matching (1.9.10)." >&2
  echo "WARNING: installing the wildcard fallback, which does NOT restrict argv --" >&2
  echo "WARNING: sudoers(5) '*' matches across white space. The argv restriction is" >&2
  echo "WARNING: then enforced only by vigil-deploy-run. Upgrade sudo when you can." >&2
fi

# Validate BEFORE installing: an unparseable file in /etc/sudoers.d can break
# sudo entirely, and this host's only rescue path is sudo via vigil-admin.
TMP_SUDOERS="$(mktemp)"
install -m 0440 -o root -g root "${SUDOERS_SRC}" "${TMP_SUDOERS}"
if ! visudo -cf "${TMP_SUDOERS}"; then
  rm -f "${TMP_SUDOERS}"
  echo "sudoers file rejected by visudo; refusing to install it" >&2
  exit 1
fi
# Atomic same-filesystem rename, so there is never a window where
# /etc/sudoers.d/vigil-deploy is half-written -- sudo reads the directory
# continuously, and a truncated file there can break sudo for everyone,
# including the vigil-admin rescue account.
install -m 0440 -o root -g root "${TMP_SUDOERS}" /etc/sudoers.d/.vigil-deploy.new
mv -f /etc/sudoers.d/.vigil-deploy.new /etc/sudoers.d/vigil-deploy
rm -f "${TMP_SUDOERS}"
visudo -c >/dev/null || { echo "FATAL: /etc/sudoers is now invalid" >&2; exit 1; }

# `restrict` disables port/agent/X11/tunnel forwarding, PTY allocation and
# ~/.ssh/rc; `command=` forces every session -- including sftp and scp, which
# arrive as subsystem/exec requests -- through vigil-deploy.
#
# ⚠️ ROOT-owned and NOT writable by deploy. If the deploy account owns its own
# authorized_keys it can simply delete the forced command, and the whole
# boundary lasts only until someone thinks to do that.
install -d -m 0755 -o root -g root "/home/${DEPLOY_USER}/.ssh"
FORCED="restrict,command=\"/usr/local/bin/vigil-deploy\" ${SSH_PUBLIC_KEY}"
printf '%s\n' "${FORCED}" > "/home/${DEPLOY_USER}/.ssh/authorized_keys.new"
chown root:root "/home/${DEPLOY_USER}/.ssh/authorized_keys.new"
chmod 0644 "/home/${DEPLOY_USER}/.ssh/authorized_keys.new"
mv -f "/home/${DEPLOY_USER}/.ssh/authorized_keys.new" \
      "/home/${DEPLOY_USER}/.ssh/authorized_keys"
echo "Installed forced-command authorized_keys (root-owned, 1 key)."

# ⚠️ ROOT-owned, and the deploy user must not be able to write ANYTHING here.
#
# The compose files build the API from a Dockerfile inside this tree. If the
# deploy user could write it, running compose as root would let them edit that
# Dockerfile and execute arbitrary code as root -- so root-owning only the
# compose file would be theatre. vigil-deploy-run does the git work itself and
# refuses to run if this ownership has drifted.
#
# ⚠️ ORDER MATTERS, and getting it wrong made this a silent no-op.
# An earlier revision ran `install -d -o root` first and then re-owned only
# `if` the directory was not already root. `install -d` flips the TOP directory
# immediately, so the test always saw root and the recursive chown never ran --
# on a host that already had a deploy-owned tree, the Dockerfile and .env
# stayed deploy-writable while the directory *looked* migrated. Verified: the
# deploy account could still append `RUN id` to the Dockerfile that root builds.
# Re-own unconditionally and recursively, BEFORE creating anything.
install -d -m 0755 -o root -g root "${APP_ROOT}"
for d in "${APP_ROOT}/staging" "${APP_ROOT}/production"; do
  if [[ -e "${d}" ]]; then
    chown -R root:root "${d}"
    # Strip group/other write from everything, not just the top directory.
    chmod -R go-w "${d}"
    echo "Re-owned ${d} to root:root and removed group/other write."
  fi
  install -d -m 0755 -o root -g root "${d}"
done

# Prove the migration actually took, rather than assuming it did.
for d in "${APP_ROOT}/staging" "${APP_ROOT}/production"; do
  if bad="$(find "${d}" \( ! -user root -o -perm /022 \) -print -quit 2>/dev/null)" && [[ -n "${bad}" ]]; then
    echo "FATAL: ${bad} is not root-owned or is group/world writable after migration" >&2
    exit 1
  fi
done

ufw allow OpenSSH
ufw allow 80/tcp
ufw allow 443/tcp
ufw --force enable

systemctl enable --now docker
systemctl enable --now caddy
systemctl enable --now fail2ban
dpkg-reconfigure -f noninteractive unattended-upgrades

# ---------------------------------------------------------------------------
# FINAL CUTOVER STEP. Everything above must have succeeded first.
#
# Membership of `docker` is equivalent to root, so leaving it would make the
# forced command pointless. But removing it is the one irreversible step for
# the OLD deploy path, so it goes last: if anything above failed, the script
# has already exited and the previous deploy mechanism still works.
# ---------------------------------------------------------------------------
if id -nG "${DEPLOY_USER}" | tr ' ' '\n' | grep -qx docker; then
  gpasswd -d "${DEPLOY_USER}" docker
  echo "CUTOVER: removed ${DEPLOY_USER} from the docker group."
else
  echo "${DEPLOY_USER} is not in the docker group (already cut over)."
fi

echo "Provisioning complete. Copy env files to:"
echo "  ${APP_ROOT}/staging/.env"
echo "  ${APP_ROOT}/production/.env"
echo "They are read by the ROOT helper, and ${DEPLOY_USER} must NOT be able to read them"
echo "(they hold database and API credentials):"
echo "  install -m 600 -o root -g root /path/to/staging.env ${APP_ROOT}/staging/.env"
echo "  install -m 600 -o root -g root /path/to/production.env ${APP_ROOT}/production/.env"
echo
echo "Verify the forced command actually restricts the deploy key:"
echo "  ssh -i <deploy_key> ${DEPLOY_USER}@<host>                 # must be refused, no shell"
echo "  ssh -i <deploy_key> ${DEPLOY_USER}@<host> 'id'            # must be refused"
echo "  ssh -i <deploy_key> ${DEPLOY_USER}@<host> 'deploy-staging <sha>'   # must work"
echo "  ssh -i <deploy_key> ${DEPLOY_USER}@<host> 'docker ps'     # must be refused"
echo "Then install deploy/Caddyfile.example as /etc/caddy/Caddyfile and reload Caddy."
