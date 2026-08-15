#!/usr/bin/env bash
#
# Exercise migrate-forced-command.sh preflight guards.
#
# 🚨 THIS HARNESS IS DESTRUCTIVE. It creates and DELETES the `deploy` and
# `vigil-admin` accounts and the whole of /opt/vigilafrica, because the script
# under test hardcodes those paths and names. Run it ONLY on a disposable
# machine -- a container or a throwaway WSL instance.
#
# Independent review rated the unguarded version P0: run on the VPS it would
# overwrite both .env files, delete both deployment trees, and delete the
# rescue account. The guards below exist so that cannot happen by accident.
set -u

REQUIRED_OPT_IN="i-understand-this-deletes-things"
if [ "${VIGIL_DESTRUCTIVE_TEST:-}" != "${REQUIRED_OPT_IN}" ]; then
  cat >&2 <<EOF
REFUSING TO RUN.

This harness deletes /opt/vigilafrica and the deploy / vigil-admin accounts.
On a real host that destroys the deployment and the rescue account.

Run it only on a disposable machine, then opt in explicitly:

  VIGIL_DESTRUCTIVE_TEST=${REQUIRED_OPT_IN} sudo -E bash \$0
EOF
  exit 2
fi

# Refuse on anything that looks like a real host, even with the opt-in set.
if [ -e /opt/vigilafrica ] && [ -n "$(ls -A /opt/vigilafrica 2>/dev/null)" ]; then
  echo "REFUSING: /opt/vigilafrica already exists and is non-empty -- this looks like a real host." >&2
  exit 2
fi
for u in deploy vigil-admin; do
  if id -u "$u" >/dev/null 2>&1; then
    echo "REFUSING: user '$u' already exists -- this harness would delete it. Not on a real host." >&2
    exit 2
  fi
done
if [ ! -f /.dockerenv ] && ! grep -qiE 'microsoft|wsl' /proc/version 2>/dev/null; then
  echo "REFUSING: not obviously a container or WSL instance." >&2
  echo "Set VIGIL_TEST_FORCE_HOST=1 as well if you are certain this machine is disposable." >&2
  [ "${VIGIL_TEST_FORCE_HOST:-}" = "1" ] || exit 2
fi
S="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/migrate-forced-command.sh"
R=/opt/vigilafrica
pass=0; fail=0

setup() {
  id -u deploy       >/dev/null 2>&1 || useradd -M -s /bin/bash deploy
  id -u vigil-admin  >/dev/null 2>&1 || useradd -M -s /bin/bash vigil-admin
  usermod -aG sudo vigil-admin
  mkdir -p "$R/staging" "$R/production"
  echo x > "$R/staging/.env"; echo x > "$R/production/.env"
  ssh-keygen -q -t ed25519 -N '' -C '' -f /tmp/mk >/dev/null 2>&1 || true
  KEY="$(cut -d' ' -f1,2 /tmp/mk.pub)"
}

check() {  # check <want:refuse|allow> <desc> <env-assignments...>
  local want="$1" desc="$2"; shift 2
  local out rc
  out="$(env "$@" bash "$S" --check 2>&1)"; rc=$?
  if { [ "$want" = refuse ] && [ "$rc" -ne 0 ]; } || { [ "$want" = allow ] && [ "$rc" -eq 0 ]; }; then
    printf '  ok   %-38s (%s, exit %d)\n' "$desc" "$want" "$rc"; pass=$((pass+1))
  else
    printf '  FAIL %-38s want=%s rc=%d\n' "$desc" "$want" "$rc"
    printf '       %s\n' "$(printf '%s' "$out" | grep -E 'FAILED|ABORTED' | head -1)"; fail=$((fail+1))
  fi
}

setup

echo "Argument parsing:"
printf '  '; bash "$S" --chek >/dev/null 2>&1; [ $? -eq 2 ] \
  && { echo "ok   --chek rejected (exit 2, not a live run)"; pass=$((pass+1)); } \
  || { echo "FAIL --chek not rejected"; fail=$((fail+1)); }

echo "Rescue-account guard:"
check refuse "ADMIN_USER unset"            SSH_PUBLIC_KEY="$KEY"
check refuse "ADMIN_USER = deploy"         ADMIN_USER=deploy      SSH_PUBLIC_KEY="$KEY"
check refuse "ADMIN_USER does not exist"   ADMIN_USER=ghost99     SSH_PUBLIC_KEY="$KEY"

echo "Key validation:"
check refuse "options-prefixed key"        ADMIN_USER=vigil-admin SSH_PUBLIC_KEY="no-pty $KEY"
check refuse "multi-line key"              ADMIN_USER=vigil-admin SSH_PUBLIC_KEY="$(printf 'ssh-ed25519 AAAA\nssh-ed25519 BBBB')"
check refuse "not a key at all"            ADMIN_USER=vigil-admin SSH_PUBLIC_KEY="hello world"

echo "Tree guards:"
ln -sfn /tmp/elsewhere "$R/staging/.env.link" 2>/dev/null
rm -f "$R/staging/.env"; ln -s /tmp/target-env "$R/staging/.env"; echo x > /tmp/target-env
check refuse ".env is a symlink"           ADMIN_USER=vigil-admin SSH_PUBLIC_KEY="$KEY"
rm -f "$R/staging/.env"; echo x > "$R/staging/.env"

echo "Control (must be ALLOWED):"
check allow  "all guards satisfied"        ADMIN_USER=vigil-admin SSH_PUBLIC_KEY="$KEY"

userdel vigil-admin 2>/dev/null; userdel deploy 2>/dev/null
find "$R" -mindepth 1 -delete 2>/dev/null; rmdir "$R" 2>/dev/null
find /tmp -maxdepth 1 -name 'mk*' -delete 2>/dev/null; rm -f /tmp/target-env
echo
echo "$pass passed, $fail failed"
[ "$fail" -eq 0 ]
