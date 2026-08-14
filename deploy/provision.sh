#!/usr/bin/env bash
set -euo pipefail

APP_ROOT="${APP_ROOT:-/opt/vigilafrica}"
DEPLOY_USER="${DEPLOY_USER:-deploy}"
SSH_PUBLIC_KEY="${SSH_PUBLIC_KEY:-}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

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

# The deploy account is deliberately NOT in the `docker` group: membership is
# equivalent to root, so a leaked VPS_SSH_KEY would be host root. It reaches
# Docker only through the audited forced command below.
# gpasswd is idempotent and also removes the membership on an existing host.
if id -nG "${DEPLOY_USER}" | tr ' ' '\n' | grep -qx docker; then
  gpasswd -d "${DEPLOY_USER}" docker
  echo "Removed ${DEPLOY_USER} from the docker group."
fi

# --- forced-command deploy protocol (chore-vps-access-hardening 1.3 / 1.4) ---
#
# These ship together. Installing the forced command without the scripts breaks
# every deploy; installing the scripts without the forced command leaves the
# key able to open a shell.
install -m 0755 -o root -g root "${SCRIPT_DIR}/vigil-deploy"     /usr/local/bin/vigil-deploy
install -m 0700 -o root -g root "${SCRIPT_DIR}/vigil-deploy-run" /usr/local/sbin/vigil-deploy-run

install -m 0440 -o root -g root "${SCRIPT_DIR}/sudoers.d/vigil-deploy" /etc/sudoers.d/vigil-deploy
# Never leave an unparseable sudoers file behind: it can lock out sudo entirely.
if ! visudo -cf /etc/sudoers.d/vigil-deploy; then
  rm -f /etc/sudoers.d/vigil-deploy
  echo "sudoers file rejected by visudo; removed it rather than risk breaking sudo" >&2
  exit 1
fi

if [[ -n "${SSH_PUBLIC_KEY}" ]]; then
  install -d -m 700 -o "${DEPLOY_USER}" -g "${DEPLOY_USER}" "/home/${DEPLOY_USER}/.ssh"
  # `restrict` disables port/agent/X11 forwarding, PTY allocation and user rc;
  # `command=` forces every session through vigil-deploy regardless of what the
  # client asks to run.
  FORCED="restrict,command=\"/usr/local/bin/vigil-deploy\" ${SSH_PUBLIC_KEY}"
  printf '%s\n' "${FORCED}" > "/home/${DEPLOY_USER}/.ssh/authorized_keys"
  chown "${DEPLOY_USER}:${DEPLOY_USER}" "/home/${DEPLOY_USER}/.ssh/authorized_keys"
  chmod 600 "/home/${DEPLOY_USER}/.ssh/authorized_keys"
fi

# ⚠️ ROOT-owned, and the deploy user must not be able to write here.
#
# The compose files build the API from a Dockerfile inside this tree. If the
# deploy user could write it, running compose as root would let them edit that
# Dockerfile and execute arbitrary code as root -- so root-owning only the
# compose file would be theatre. vigil-deploy-run does the git work itself and
# refuses to run if this ownership has drifted.
install -d -m 0755 -o root -g root "${APP_ROOT}/staging" "${APP_ROOT}/production"
for d in "${APP_ROOT}/staging" "${APP_ROOT}/production"; do
  if [[ -e "${d}" ]] && [[ "$(stat -c '%U' "${d}")" != "root" ]]; then
    chown -R root:root "${d}"
    echo "Re-owned ${d} to root (was deploy-writable)."
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
