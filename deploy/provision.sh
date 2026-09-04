#!/usr/bin/env bash
set -euo pipefail

APP_ROOT="${APP_ROOT:-/opt/vigilafrica}"

# Task 1.5: ONE ACCOUNT PER ENVIRONMENT. A single account owning both is the
# exposure -- production's reviewer gate constrains only the workflow path, so
# with one credential a leaked key reaches production regardless of it.
STAGING_USER="${STAGING_USER:-deploy-staging}"
PROD_USER="${PROD_USER:-deploy-prod}"
LEGACY_USER="${LEGACY_USER:-deploy}"

# Each account gets its OWN key. Reusing one key across both would leave the
# accounts split on paper and joined in practice.
SSH_PUBLIC_KEY_STAGING="${SSH_PUBLIC_KEY_STAGING:-}"
SSH_PUBLIC_KEY_PROD="${SSH_PUBLIC_KEY_PROD:-}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# ⚠️ These LOOK configurable but are not, and a mismatch produces a broken
# install that reports success: vigil-deploy-run hardcodes APP_ROOT, and the
# sudoers rules hardcode both account names. Fail loudly rather than provision
# something that cannot deploy.
if [[ "${APP_ROOT}" != "/opt/vigilafrica" ]]; then
  echo "APP_ROOT is pinned to /opt/vigilafrica (hardcoded in vigil-deploy-run)." >&2
  echo "Change it there and in deploy/sudoers.d/* together, or leave it unset." >&2
  exit 1
fi
if [[ "${STAGING_USER}" != "deploy-staging" || "${PROD_USER}" != "deploy-prod" ]]; then
  echo "STAGING_USER/PROD_USER are pinned to deploy-staging/deploy-prod" >&2
  echo "(hardcoded in deploy/sudoers.d/*). Change them there too, or leave unset." >&2
  exit 1
fi

# ⚠️ Reject the pre-1.5 variable outright instead of ignoring it. Someone
# re-running an old command line with DEPLOY_USER=deploy set would otherwise get
# a successful-looking provision that silently ignored their intent.
if [[ -n "${DEPLOY_USER:-}" ]]; then
  echo "DEPLOY_USER is no longer used (task 1.5 split the account in two)." >&2
  echo "Set SSH_PUBLIC_KEY_STAGING and SSH_PUBLIC_KEY_PROD instead." >&2
  exit 1
fi
if [[ -n "${SSH_PUBLIC_KEY:-}" ]]; then
  echo "SSH_PUBLIC_KEY is no longer used (task 1.5 gave each environment its own key)." >&2
  echo "Set SSH_PUBLIC_KEY_STAGING and SSH_PUBLIC_KEY_PROD instead." >&2
  exit 1
fi

if [[ "${EUID}" -ne 0 ]]; then
  echo "Run as root:" >&2
  echo "  sudo SSH_PUBLIC_KEY_STAGING='ssh-ed25519 ...' \\" >&2
  echo "       SSH_PUBLIC_KEY_PROD='ssh-ed25519 ...' ./deploy/provision.sh" >&2
  echo "The two keys must be DIFFERENT -- one key on both accounts defeats the split." >&2
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

for u in "${STAGING_USER}" "${PROD_USER}"; do
  if ! id "${u}" >/dev/null 2>&1; then
    useradd --create-home --shell /bin/bash "${u}"
  fi
done

# ⚠️ Docker-group removal is deliberately NOT here. It is the last cutover
# step, at the end of this script. Removing it before the helper, sudoers,
# forced key and ownership migration are all in place means any later failure
# leaves the OLD deploy path unable to reach Docker and the NEW one not yet
# working -- i.e. no way to deploy at all.
#
# Preflight instead: fail before mutating anything if the inputs are wrong.
validate_key() {  # validate_key <var-name> <value>
  local name="$1" value="$2"
  if [[ -z "${value}" ]]; then
    echo "${name} is required: without it this script would leave the account's" >&2
    echo "existing unrestricted key in place and report success." >&2
    exit 1
  fi
  if [[ "$(printf '%s' "${value}" | grep -c '')" -ne 1 ]]; then
    echo "${name} must be exactly one line; a pasted newline would add a" >&2
    echo "second, unrestricted authorized_keys entry." >&2
    exit 1
  fi
  if ! printf '%s\n' "${value}" | ssh-keygen -l -f /dev/stdin >/dev/null 2>&1; then
    echo "${name} is not a valid OpenSSH public key" >&2
    exit 1
  fi
}
validate_key SSH_PUBLIC_KEY_STAGING "${SSH_PUBLIC_KEY_STAGING}"
validate_key SSH_PUBLIC_KEY_PROD    "${SSH_PUBLIC_KEY_PROD}"

# ⚠️ THE CHECK THAT MAKES THE SPLIT REAL.
#
# Two accounts sharing one key is separation on paper only: whoever holds that
# key still reaches both environments, which is precisely the exposure task 1.5
# exists to remove -- and every other check here would still report success.
#
# Compare FINGERPRINTS, not the raw strings: the same key differs textually by
# its trailing comment, so a string compare would pass for two copies of one key
# labelled `deploy@staging` and `deploy@prod`.
fp_staging="$(printf '%s\n' "${SSH_PUBLIC_KEY_STAGING}" | ssh-keygen -l -f /dev/stdin | awk '{print $2}')"
fp_prod="$(printf '%s\n' "${SSH_PUBLIC_KEY_PROD}"    | ssh-keygen -l -f /dev/stdin | awk '{print $2}')"
if [[ "${fp_staging}" == "${fp_prod}" ]]; then
  echo "SSH_PUBLIC_KEY_STAGING and SSH_PUBLIC_KEY_PROD are the SAME key (${fp_staging})." >&2
  echo "That defeats the account split: one credential would reach both environments." >&2
  echo "Generate a separate keypair per environment." >&2
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
#
# ⚠️ The environment is pinned IN the forced command (task 1.5). vigil-deploy
# refuses to run without it, so an authorized_keys left in the pre-1.5 form
# fails closed rather than granting both environments.
install_forced_key() {  # install_forced_key <user> <environment> <pubkey>
  local user="$1" env="$2" key="$3" ssh_dir="/home/$1/.ssh"
  install -d -m 0755 -o root -g root "${ssh_dir}"
  printf '%s\n' "restrict,command=\"/usr/local/bin/vigil-deploy ${env}\" ${key}" \
    > "${ssh_dir}/authorized_keys.new"
  chown root:root "${ssh_dir}/authorized_keys.new"
  chmod 0644 "${ssh_dir}/authorized_keys.new"
  mv -f "${ssh_dir}/authorized_keys.new" "${ssh_dir}/authorized_keys"
  echo "Installed forced-command authorized_keys for ${user} (root-owned, 1 key, env=${env})."
}
install_forced_key "${STAGING_USER}" staging    "${SSH_PUBLIC_KEY_STAGING}"
install_forced_key "${PROD_USER}"    production "${SSH_PUBLIC_KEY_PROD}"

# Prove the pin actually landed, rather than trusting the write. A forced
# command missing its environment argument is the one failure that would look
# fine here and refuse every deploy at runtime.
for pair in "${STAGING_USER}:staging" "${PROD_USER}:production"; do
  u="${pair%%:*}"; e="${pair##*:}"
  grep -q "command=\"/usr/local/bin/vigil-deploy ${e}\"" "/home/${u}/.ssh/authorized_keys" \
    || { echo "FATAL: ${u} authorized_keys does not pin environment ${e}" >&2; exit 1; }
done

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

# --- sshd hardening (chore-vps-access-hardening 1.6) ------------------------
#
# Folded in so a rebuild cannot silently regress it. Without this, provision.sh
# leaves password auth and root login at image defaults and reports success.
#
# ⚠️ DROP-IN, and deliberately LOW-sorted. sshd_config's `Include
# /etc/ssh/sshd_config.d/*.conf` sits near the TOP of the file and OpenSSH uses
# the FIRST obtained value for each keyword, so drop-ins beat the main file and
# the lowest filename beats later ones. `00-` therefore wins over a cloud-init
# `50-`; the familiar "99 = wins" convention is backwards here.
#
# ⚠️ Refuse rather than lock the operator out. Disabling password auth is only
# safe if some human account can already log in with a key. The deploy accounts
# do not count -- their forced command gives no shell, so they cannot be used to
# recover a host.
harden_sshd() {
  local admin_ok=0 home u
  while IFS=: read -r u _ uid _ _ home _; do
    [[ "${uid}" -ge 1000 ]] || continue
    [[ "${u}" == "${STAGING_USER}" || "${u}" == "${PROD_USER}" || "${u}" == "${LEGACY_USER}" ]] && continue
    [[ -s "${home}/.ssh/authorized_keys" ]] || continue
    grep -qE '^[^#].*ssh-(ed25519|rsa)|^[^#].*ecdsa-' "${home}/.ssh/authorized_keys" && admin_ok=1
  done < /etc/passwd

  if [[ "${admin_ok}" -ne 1 ]]; then
    echo "WARNING: skipping sshd hardening -- no non-deploy account has a key-based" >&2
    echo "WARNING: login, so disabling password auth would lock this host out." >&2
    echo "WARNING: create an admin account with an authorized_keys (task 1.1), then" >&2
    echo "WARNING: apply docs/deployment/sshd-hardening.md by hand." >&2
    return 0
  fi

  install -d -m 0755 -o root -g root /etc/ssh/sshd_config.d
  cat > /etc/ssh/sshd_config.d/00-vigilafrica-hardening.conf <<'EOF'
# chore-vps-access-hardening task 1.6. Low-sorted on purpose: sshd takes the
# FIRST obtained value and reads this directory in glob order, so 00- wins over
# any later cloud-init drop-in.
PasswordAuthentication no
KbdInteractiveAuthentication no
PermitRootLogin no
PubkeyAuthentication yes
EOF
  chown root:root /etc/ssh/sshd_config.d/00-vigilafrica-hardening.conf
  chmod 0644 /etc/ssh/sshd_config.d/00-vigilafrica-hardening.conf

  # ⚠️ `sshd -t` checks SYNTAX, not effective policy -- it passes happily while
  # password login stays enabled. Assert the effective values before reloading.
  sshd -t || { echo "FATAL: sshd config invalid; not reloading" >&2; exit 1; }
  local effective
  effective="$(sshd -T 2>/dev/null | grep -ciE '^(passwordauthentication no|permitrootlogin no|kbdinteractiveauthentication no)$')"
  if [[ "${effective}" -ne 3 ]]; then
    echo "FATAL: sshd hardening did not take effect (${effective}/3 directives)." >&2
    echo "FATAL: something earlier in the include order is winning. Not reloading." >&2
    sshd -T | grep -iE '^(passwordauthentication|permitrootlogin|kbdinteractiveauthentication)' >&2
    exit 1
  fi
  systemctl reload ssh
  echo "Hardened sshd: password auth, keyboard-interactive and root login disabled."
}
harden_sshd

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
for u in "${STAGING_USER}" "${PROD_USER}" "${LEGACY_USER}"; do
  id "${u}" >/dev/null 2>&1 || continue
  if id -nG "${u}" | tr ' ' '\n' | grep -qx docker; then
    gpasswd -d "${u}" docker
    echo "CUTOVER: removed ${u} from the docker group."
  else
    echo "${u} is not in the docker group (already cut over)."
  fi
done

# ⚠️ The legacy account is NOT retired here. Creating the new principals does
# not remove the old one -- its authorized_keys, sudoers entry and group
# memberships can all survive and keep the exposure intact. Retiring it is a
# deliberate, verified step in docs/deployment/deploy-account-split.md, because
# doing it from this script would cut the only working deploy path on a host
# where the new one has not yet been proven.
if id "${LEGACY_USER}" >/dev/null 2>&1 \
   && [[ -s "/home/${LEGACY_USER}/.ssh/authorized_keys" ]]; then
  echo "WARNING: legacy account '${LEGACY_USER}' still exists WITH an authorized_keys." >&2
  echo "WARNING: task 1.5 is not complete until it is retired and its key is proven" >&2
  echo "WARNING: to reach neither environment. See docs/deployment/deploy-account-split.md." >&2
fi

echo "Provisioning complete. Copy env files to:"
echo "  ${APP_ROOT}/staging/.env"
echo "  ${APP_ROOT}/production/.env"
echo "They are read by the ROOT helper, and neither deploy account may read them"
echo "(they hold database and API credentials):"
echo "  install -m 600 -o root -g root /path/to/staging.env ${APP_ROOT}/staging/.env"
echo "  install -m 600 -o root -g root /path/to/production.env ${APP_ROOT}/production/.env"
echo
echo "Verify the forced command restricts each key (task 1.3):"
echo "  ssh -i <staging_key> ${STAGING_USER}@<host>                        # refused, no shell"
echo "  ssh -i <staging_key> ${STAGING_USER}@<host> 'id'                   # refused"
echo "  ssh -i <staging_key> ${STAGING_USER}@<host> 'docker ps'            # refused"
echo "  ssh -i <staging_key> ${STAGING_USER}@<host> 'deploy-staging <sha>' # must work"
echo
echo "Then verify the SPLIT itself (task 1.5) -- both must be REFUSED:"
echo "  ssh -i <staging_key> ${STAGING_USER}@<host> 'deploy-production v1.0.0'"
echo "  ssh -i <prod_key>    ${PROD_USER}@<host> 'deploy-staging <sha>'"
echo "These are well-formed requests that the other account would accept, so a"
echo "refusal here is the separation working rather than a parsing accident."
echo
echo "And confirm sudo agrees, since sudoers -- not the forced command -- is the"
echo "actual boundary (use -l, NOT visudo -c):"
echo "  sudo -l -U ${STAGING_USER}   # staging rule only"
echo "  sudo -l -U ${PROD_USER}      # production rule only"
echo
echo "Then install deploy/Caddyfile.example as /etc/caddy/Caddyfile and reload Caddy."
