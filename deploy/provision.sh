#!/usr/bin/env bash
set -euo pipefail

APP_ROOT="${APP_ROOT:-/opt/vigilafrica}"
DEPLOY_USER="${DEPLOY_USER:-deploy}"
SSH_PUBLIC_KEY="${SSH_PUBLIC_KEY:-}"

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
usermod -aG docker "${DEPLOY_USER}"

if [[ -n "${SSH_PUBLIC_KEY}" ]]; then
  install -d -m 700 -o "${DEPLOY_USER}" -g "${DEPLOY_USER}" "/home/${DEPLOY_USER}/.ssh"
  touch "/home/${DEPLOY_USER}/.ssh/authorized_keys"
  grep -qxF "${SSH_PUBLIC_KEY}" "/home/${DEPLOY_USER}/.ssh/authorized_keys" \
    || echo "${SSH_PUBLIC_KEY}" >> "/home/${DEPLOY_USER}/.ssh/authorized_keys"
  chown "${DEPLOY_USER}:${DEPLOY_USER}" "/home/${DEPLOY_USER}/.ssh/authorized_keys"
  chmod 600 "/home/${DEPLOY_USER}/.ssh/authorized_keys"
fi

install -d -m 0755 -o "${DEPLOY_USER}" -g "${DEPLOY_USER}" "${APP_ROOT}/staging" "${APP_ROOT}/production"

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
echo "Keep them readable by ${DEPLOY_USER} only, for example:"
echo "  install -m 600 -o ${DEPLOY_USER} -g ${DEPLOY_USER} /path/to/staging.env ${APP_ROOT}/staging/.env"
echo "  install -m 600 -o ${DEPLOY_USER} -g ${DEPLOY_USER} /path/to/production.env ${APP_ROOT}/production/.env"
echo "Then install deploy/Caddyfile.example as /etc/caddy/Caddyfile and reload Caddy."
