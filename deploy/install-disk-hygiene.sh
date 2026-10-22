#!/usr/bin/env bash
# Installs the VPS disk-hygiene guards: a weekly Docker build-cache prune and a
# journald size cap. Called by provision.sh; also run on its own to apply them
# to an already-provisioned host (see docs/deployment/vps.md → Disk Hygiene).
# Idempotent, and neither step restarts Docker, so it is safe on a live host.
#
# Every deploy runs `docker compose up -d --build`, and nothing pruned the
# BuildKit cache that leaves behind: on 2026-10-22 it held 23.8 GB of the 31 GB
# in use (none of it referenced), and the journal another 3.3 GB.
set -euo pipefail

[[ ${EUID} -eq 0 ]] || { echo "install-disk-hygiene: must run as root" >&2; exit 1; }

# The prune drops build cache unused for a week, so routine deploys keep their
# recent layers. It never touches images, containers or volumes.
#
# It skips rather than starts Docker: `Requires=` would pull docker.service up
# if an operator had stopped it, bringing the `restart: unless-stopped` stack
# back mid-maintenance. Bare command names are resolved from systemd's PATH, so
# the unit does not depend on where Docker was installed.
cat > /etc/systemd/system/vigil-docker-builder-prune.service <<'EOF'
[Unit]
Description=Prune Docker build cache unused for a week
After=docker.service

[Service]
Type=oneshot
ExecCondition=systemctl is-active --quiet docker.service
ExecStart=docker builder prune --all --force --filter until=168h
EOF
cat > /etc/systemd/system/vigil-docker-builder-prune.timer <<'EOF'
[Unit]
Description=Weekly Docker build cache prune

[Timer]
OnCalendar=weekly
RandomizedDelaySec=1h
Persistent=true

[Install]
WantedBy=timers.target
EOF
chmod 0644 /etc/systemd/system/vigil-docker-builder-prune.{service,timer}

# Caps the PERSISTENT journal (/var/log/journal). journald deletes only archived
# files to meet it, so usage can run slightly over while the active files fill.
#
# ⚠️ OPPOSITE precedence to the sshd drop-in in provision.sh: systemd sorts
# journald.conf.d drop-ins from /etc, /run and /usr/lib together by filename and
# the LAST value wins. `90-` is a convention, not a guarantee, so the effective
# value is checked below. The cap also bounds how far back the vigil-deploy /
# vigil-deploy-run audit lines go; export them first if you need a longer record.
install -d -m 0755 -o root -g root /etc/systemd/journald.conf.d
cat > /etc/systemd/journald.conf.d/90-vigilafrica.conf <<'EOF'
[Journal]
SystemMaxUse=1G
EOF
chmod 0644 /etc/systemd/journald.conf.d/90-vigilafrica.conf

systemctl daemon-reload
systemctl enable --now vigil-docker-builder-prune.timer
systemctl restart systemd-journald

# Fail closed: confirm what took effect rather than what was written.
systemctl is-enabled --quiet vigil-docker-builder-prune.timer \
  || { echo "install-disk-hygiene: prune timer is not enabled" >&2; exit 1; }
systemctl is-active --quiet vigil-docker-builder-prune.timer \
  || { echo "install-disk-hygiene: prune timer is not active" >&2; exit 1; }
effective="$(systemd-analyze cat-config systemd/journald.conf | grep -E '^SystemMaxUse=' | tail -n 1 || true)"
if [[ "${effective}" != "SystemMaxUse=1G" ]]; then
  echo "install-disk-hygiene: journald cap not in effect (last value: '${effective:-none}')." >&2
  echo "install-disk-hygiene: a later-sorting drop-in is overriding 90-vigilafrica.conf." >&2
  exit 1
fi
echo "install-disk-hygiene: weekly build-cache prune timer active; journald SystemMaxUse=1G in effect."
