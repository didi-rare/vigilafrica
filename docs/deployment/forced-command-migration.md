# Forced-command deployment migration runbook

This procedure migrates `vigilafrica-prod` (`178.104.104.122`) to the reviewed forced-command deployment protocol without restarting Docker, Caddy, or any running container.

Do **not** run:

```text
deploy/provision.sh
```

`provision.sh` upgrades packages, changes firewall state, and restarts Docker and Caddy. (`deploy/migrate-forced-command.sh` was an abandoned attempt at automating this procedure; it has been deleted, and this runbook replaces it.)

## Before you start

Have all of the following ready:

- Console or SSH access as `vigil-admin`.
- Two concurrent `vigil-admin` sessions. Keep the rescue session open for the entire migration.
- Working sudo access in both sessions.
- The deploy private key on the operator workstation.
- The existing pinned `known_hosts` file containing the verified key for `178.104.104.122`.
- The full 40-character commit SHA containing the reviewed migration artifacts. This is referenced below as `<migration-sha>`.
- Permission to disable and enable the two GitHub deployment workflows.
- A prepared, mergeable `development → main` promotion. Be ready to follow it with `main → release`.
- A maintenance freeze: nobody may merge to `main` or `release`, create a release tag, manually deploy, or dispatch either deployment workflow until this runbook says otherwise.

All host commands below are run from a root shell opened through `vigil-admin`:

```bash
sudo -i
```

Expected output: no output and a root prompt.

Confirm the session identity:

```bash
whoami
hostname
```

Expected output:

```text
root
vigilafrica-prod
```

In the second rescue session, run:

```bash
sudo -v
```

Expected output: no output and exit status zero. Leave this session open.

---

## Phase 0: discover

Everything in this phase is read-only. If a decision rule says stop, do not proceed to the migration phases.

### 0.1 Confirm the live service baseline

Run:

```bash
docker ps --format 'table {{.ID}}\t{{.Names}}\t{{.Status}}'
```

Expected output: a nonempty table containing the running staging and production containers. No relevant container should be restarting or unhealthy.

Run:

```bash
curl -fsS https://api.staging.vigilafrica.org/health
```

Expected output: JSON containing `"status":"ok"` and a commit-derived version.

Run:

```bash
curl -fsS https://api.vigilafrica.org/health
```

Expected output: JSON containing `"status":"ok"` and the deployed production tag.

**Decision:** stop if either health request fails or any relevant container is unhealthy. This migration is not an incident-recovery procedure.

**Verification:** record both health responses and the current container IDs.

**Reversal:** none; this step changes nothing.

### 0.2 Confirm required tools and inspect partial migration state

Run:

```bash
command -v git docker sudo visudo sshd ssh-keygen findmnt find flock install stat sha256sum diff curl pgrep gpasswd
```

Expected output: one absolute executable path for every named command.

Inspect the destination artifacts:

```bash
ls -l /usr/local/bin/vigil-deploy /usr/local/sbin/vigil-deploy-run /etc/sudoers.d/vigil-deploy
```

Expected output on an untouched host: three `No such file or directory` messages.

**Decision:** if any destination already exists, stop and inspect it against the reviewed repository copy. Treat this as a partial earlier migration; do not overwrite it blindly.

**Verification:** all required commands exist, and the install destinations are either absent or their provenance has been established.

**Reversal:** none; this step changes nothing.

### 0.3 Establish the sudo version and existing deploy privileges

Run:

```bash
sudo -V | head -n 1
```

Expected output in this form:

```text
Sudo version X.Y.Z
```

Record the version.

- For sudo `1.9.10` or newer, use `deploy/sudoers.d/vigil-deploy`.
- For an older version, use `deploy/sudoers.d/vigil-deploy.pre-1.9.10`.

The older fallback knowingly uses wildcard matching. Because `*` matches whitespace, sudoers does not constrain the full argument list on those versions; the privileged helper’s exact argument count and validation are then the final enforcement point.

Inspect current sudo authorization:

```bash
sudo -l -U deploy
```

Expected output before installation: either a statement that `deploy` may not run sudo or a list containing no privileged commands.

**Decision:** stop if `deploy` already has `ALL`, a shell, `docker`, `env`, an editor, or any other unrelated sudo command. That privilege must be understood and removed before this migration can establish the intended boundary.

**Verification:** record the sudo version and confirm there is no conflicting deploy-user authorization.

**Reversal:** none; this step changes nothing.

### 0.4 Discover effective sshd policy for `deploy`

In the admin SSH session, run:

```bash
printf '%s\n' "$SSH_CONNECTION"
```

Expected output: four fields. The first field is the operator’s client IP. Record it as `<client-ip>`.

Inspect all relevant source configuration:

```bash
grep -RInE '^[[:space:]]*(Include|Match|AuthorizedKeysFile|AuthorizedKeysCommand)[[:space:]]' /etc/ssh/sshd_config /etc/ssh/sshd_config.d/
```

Expected output: the configured `Include`, `Match`, and authorized-key directives. “No such file or directory” is acceptable only for a configuration directory that does not exist.

Now ask sshd for the effective configuration that applies to the deploy account:

```bash
sshd -T -C user=deploy,host=vigilafrica-prod,addr=<client-ip>,laddr=178.104.104.122,lport=22 | grep -E '^(authorizedkeysfile|authorizedkeyscommand) '
```

Expected output should include:

```text
authorizedkeysfile .ssh/authorized_keys
authorizedkeyscommand none
```

The `authorizedkeysfile` line may contain additional paths.

**Decision:**

- Stop if `.ssh/authorized_keys` is not effective.
- Stop if `AuthorizedKeysCommand` is active.
- If additional key files are effective, inspect each one. Continue only if it is absent or cannot authorize the deploy key, and the `deploy` user cannot create or modify it.
- Reconcile the effective output with every relevant `Match` block. Do not rely only on the global section.

No sshd configuration is changed during this migration, and sshd must not be restarted or reloaded.

**Verification:** `/home/deploy/.ssh/authorized_keys` is an effective key source and there is no alternative unrestricted authorization path for the deploy key.

**Reversal:** none; this step changes nothing.

### 0.5 Inspect the current authorized key

Run:

```bash
test -f /home/deploy/.ssh/authorized_keys && test ! -L /home/deploy/.ssh/authorized_keys && echo 'authorized_keys is a regular file'
```

Expected output:

```text
authorized_keys is a regular file
```

Run:

```bash
stat -c '%F %U:%G %a %n' /home/deploy /home/deploy/.ssh /home/deploy/.ssh/authorized_keys
```

Expected output: three lines. No path may be group- or world-writable. Record the current owners and modes for rollback.

Count and inspect the key entries:

```bash
wc -l /home/deploy/.ssh/authorized_keys
```

Expected output:

```text
1 /home/deploy/.ssh/authorized_keys
```

Run:

```bash
sed -n '1p' /home/deploy/.ssh/authorized_keys
```

Expected output: one bare OpenSSH public key beginning with a key type such as `ssh-ed25519`, followed by its base64 data and optional comment. It must not begin with existing authorized-key options.

Validate its fingerprint:

```bash
ssh-keygen -lf /home/deploy/.ssh/authorized_keys
```

Expected output: one fingerprint corresponding to the deployment key.

**Decision:** stop if there is more than one entry, if the entry already carries options, if the file is a symlink, or if its fingerprint is not the expected deployment-key fingerprint. Do not silently delete or reinterpret another administrator’s key.

**Verification:** exactly one known, bare deployment key is present.

**Reversal:** none; this step changes nothing.

### 0.6 Inspect mounts, parent permissions, and disk headroom

Identify the backing filesystem:

```bash
findmnt -T /opt/vigilafrica -o TARGET,SOURCE,FSTYPE,OPTIONS
```

Expected output: a header and one row describing the filesystem containing `/opt/vigilafrica`.

Check for nested mounts inside either deployment tree:

```bash
findmnt -rn -o TARGET | grep -E '^/opt/vigilafrica/(staging|production)(/|$)'
```

Expected output: no output.

**Decision:** stop if either live tree or anything below it is a separate mount. Renaming a mount point is not the cutover described by this runbook.

Inspect parent ownership and permissions:

```bash
stat -c '%U:%G %a %n' /opt /opt/vigilafrica
```

Expected output: two lines. Record the current `/opt/vigilafrica` owner, group, and mode. `/opt` must be root-owned and not group- or world-writable.

Inspect available space and current tree sizes:

```bash
df -hT /opt/vigilafrica /root
```

Expected output: filesystem rows with nonzero available space.

```bash
du -sh /opt/vigilafrica/staging /opt/vigilafrica/production
```

Expected output: one size for each tree.

**Decision:** continue only if the filesystem can hold:

- one fresh migration-source clone;
- two fresh deployment clones;
- both original trees for rollback; and
- normal Docker build headroom.

As a practical minimum, available space should comfortably exceed three times the larger live checkout. Do not obtain space by pruning Docker or deleting live or rollback data during this migration.

**Verification:** no nested mount will obstruct an atomic rename, and sufficient headroom exists.

**Reversal:** none; this step changes nothing.

### 0.7 Capture the deployed revisions without triggering dubious ownership

Do **not** run root-side `git -C ... rev-parse` in the existing trees. It is confirmed to fail with `fatal: detected dubious ownership`. Do not add these trees to root’s global `safe.directory`.

Read staging HEAD as the tree owner:

```bash
sudo -u deploy -- git -C /opt/vigilafrica/staging rev-parse --verify HEAD
```

Expected output: exactly one 40-character lowercase hexadecimal commit SHA. Record it as `<staging-sha>`.

Read production HEAD the same way:

```bash
sudo -u deploy -- git -C /opt/vigilafrica/production rev-parse --verify HEAD
```

Expected output: exactly one 40-character lowercase hexadecimal commit SHA. Record it as `<production-sha>`.

Inspect tracked and untracked state separately:

```bash
sudo -u deploy -- git -C /opt/vigilafrica/staging status --porcelain=v1 --untracked-files=all
```

```bash
sudo -u deploy -- git -C /opt/vigilafrica/production status --porcelain=v1 --untracked-files=all
```

Expected output from both commands: no output.

Inspect ignored files:

```bash
sudo -u deploy -- git -C /opt/vigilafrica/staging ls-files --others --ignored --exclude-standard
```

```bash
sudo -u deploy -- git -C /opt/vigilafrica/production ls-files --others --ignored --exclude-standard
```

Expected output: `.env`, with no other operationally required file.

Inspect each environment file:

```bash
test -f /opt/vigilafrica/staging/.env && test ! -L /opt/vigilafrica/staging/.env && stat -c '%F %U:%G %a %h %n' /opt/vigilafrica/staging/.env
```

```bash
test -f /opt/vigilafrica/production/.env && test ! -L /opt/vigilafrica/production/.env && stat -c '%F %U:%G %a %h %n' /opt/vigilafrica/production/.env
```

Expected output: each is a regular file. Ownership will normally be `deploy`, but do not change it yet.

**Decision:**

- Stop if either tree has tracked modifications or unignored untracked files.
- Review every ignored file. Only `.env` will be copied by this procedure.
- If anything else is required at runtime, define and verify its separate migration before continuing.

If either `rev-parse` command cannot produce a full SHA, do not deploy the default branch. Obtain the deployed version from the corresponding health endpoint or the last successful GitHub deployment, then resolve it in a fresh trusted clone:

For a staging version:

```bash
git -C <trusted-fresh-clone> rev-parse <staging-version>^{commit}
```

Expected output: one full 40-character SHA.

```bash
git -C <trusted-fresh-clone> merge-base --is-ancestor <resolved-sha> origin/main
```

Expected output: no output and exit status zero.

For a production tag:

```bash
git -C <trusted-fresh-clone> rev-list -n 1 refs/tags/<production-tag>
```

Expected output: one full 40-character SHA.

```bash
git -C <trusted-fresh-clone> merge-base --is-ancestor <resolved-sha> origin/release
```

Expected output: no output and exit status zero.

If the deployed revision still cannot be proven, abort the migration. Silently accepting a clone’s default branch is not permitted.

**Verification:** both exact revisions are recorded, both trees are clean, and `.env` is the only state that must cross.

**Reversal:** none; this step changes nothing.

---

## Phase 1: pause deployment activity

### 1.1 Disable both GitHub deployment workflows

**Classification: reversible.**

From an authenticated operator workstation:

```bash
gh workflow disable deploy-staging.yml --repo didi-rare/vigilafrica
```

Expected output: no output and exit status zero.

```bash
gh workflow disable deploy-production.yml --repo didi-rare/vigilafrica
```

Expected output: no output and exit status zero.

Alternatively, use GitHub’s Actions interface and confirm each workflow shows **Disabled manually**.

Verify their state:

```bash
gh api repos/didi-rare/vigilafrica/actions/workflows/deploy-staging.yml --jq .state
```

```bash
gh api repos/didi-rare/vigilafrica/actions/workflows/deploy-production.yml --jq .state
```

Expected output from both:

```text
disabled_manually
```

Check staging activity:

```bash
gh run list --repo didi-rare/vigilafrica --workflow deploy-staging.yml --limit 20 --json databaseId,status --jq '.[] | select(.status != "completed")'
```

Expected output: no output.

Check production activity:

```bash
gh run list --repo didi-rare/vigilafrica --workflow deploy-production.yml --limit 20 --json databaseId,status --jq '.[] | select(.status != "completed")'
```

Expected output: no output.

On the host, check for deploy-user processes:

```bash
pgrep -a -u deploy
```

Expected output: no output.

**Decision:** do not continue while any run is queued, waiting, pending, or in progress, or while any process is running as `deploy`. Wait for it to finish or cancel it deliberately, then repeat the checks.

No release tag may be created while the workflows are disabled.

**Verification:** both workflows are disabled, no deploy run is active, and `deploy` has no surviving process or SSH session.

**Reversal:** before host cutover, re-enable the original workflows with:

```bash
gh workflow enable deploy-staging.yml --repo didi-rare/vigilafrica
gh workflow enable deploy-production.yml --repo didi-rare/vigilafrica
```

Expected output: no output. Do this only if `main` and `release` still contain workflows compatible with the current host.

---

## Phase 2: obtain trusted installation sources

### 2.1 Clone and pin the reviewed migration commit

**Classification: reversible and inert.**

Confirm the destination does not already exist:

```bash
test ! -e /root/vigilafrica-migration-src && echo 'source destination is unused'
```

Expected output:

```text
source destination is unused
```

Clone from the pinned repository URL without consulting system or root-global Git configuration:

```bash
GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null git clone --no-local https://github.com/didi-rare/vigilafrica.git /root/vigilafrica-migration-src
```

Expected output begins with:

```text
Cloning into '/root/vigilafrica-migration-src'...
```

There must be no fatal error.

Check out the reviewed commit:

```bash
git -C /root/vigilafrica-migration-src checkout --detach <migration-sha>
```

Expected output includes:

```text
HEAD is now at <abbreviated-sha>
```

Verify the exact commit:

```bash
git -C /root/vigilafrica-migration-src rev-parse HEAD
```

Expected output: exactly `<migration-sha>`.

Verify the required artifacts are regular files, not symlinks:

```bash
find /root/vigilafrica-migration-src/deploy -maxdepth 2 \( -path '*/vigil-deploy' -o -path '*/vigil-deploy-run' -o -path '*/sudoers.d/vigil-deploy' -o -path '*/sudoers.d/vigil-deploy.pre-1.9.10' \) -type f -print
```

Expected output: exactly the four required artifact paths.

Record their checksums:

```bash
sha256sum /root/vigilafrica-migration-src/deploy/vigil-deploy /root/vigilafrica-migration-src/deploy/vigil-deploy-run /root/vigilafrica-migration-src/deploy/sudoers.d/vigil-deploy /root/vigilafrica-migration-src/deploy/sudoers.d/vigil-deploy.pre-1.9.10
```

Expected output: four lines, each beginning with a 64-character SHA-256 digest.

**Verification:** the source is a fresh root-owned clone at the exact reviewed commit.

**Reversal:** the clone is inert and may be left in place. If removal is necessary before cutover, first confirm:

```bash
realpath /root/vigilafrica-migration-src
```

Expected output:

```text
/root/vigilafrica-migration-src
```

Then remove only that exact clone.

---

## Phase 3: install inert controls

The old deployment path remains usable throughout this phase.

### 3.1 Install the forced command and privileged helper

**Classification: reversible and inert until the key is restricted.**

Run:

```bash
install -m 0755 -o root -g root /root/vigilafrica-migration-src/deploy/vigil-deploy /usr/local/bin/vigil-deploy
```

Expected output: no output.

Run:

```bash
install -m 0700 -o root -g root /root/vigilafrica-migration-src/deploy/vigil-deploy-run /usr/local/sbin/vigil-deploy-run
```

Expected output: no output.

Verify ownership and modes:

```bash
stat -c '%U:%G %a %n' /usr/local/bin/vigil-deploy /usr/local/sbin/vigil-deploy-run
```

Expected output:

```text
root:root 755 /usr/local/bin/vigil-deploy
root:root 700 /usr/local/sbin/vigil-deploy-run
```

Verify the installed content:

```bash
cmp /root/vigilafrica-migration-src/deploy/vigil-deploy /usr/local/bin/vigil-deploy && echo 'forced command matches reviewed source'
```

```bash
cmp /root/vigilafrica-migration-src/deploy/vigil-deploy-run /usr/local/sbin/vigil-deploy-run && echo 'helper matches reviewed source'
```

Expected output:

```text
forced command matches reviewed source
helper matches reviewed source
```

Test only the helper’s non-mutating argument rejection:

```bash
/usr/local/sbin/vigil-deploy-run staging not-a-sha
```

Expected output:

```text
vigil-deploy-run: invalid staging ref
```

The exit status must be nonzero. No Git checkout or Docker command is reached.

**Reversal:**

```bash
rm -f /usr/local/bin/vigil-deploy /usr/local/sbin/vigil-deploy-run
```

Expected output: no output. This is safe only before `authorized_keys` references the forced command.

### 3.2 Validate and install the correct sudoers artifact

**Classification: reversible and inert until invoked.**

Choose exactly one source based on Phase 0:

```text
sudo >= 1.9.10: deploy/sudoers.d/vigil-deploy
sudo <  1.9.10: deploy/sudoers.d/vigil-deploy.pre-1.9.10
```

The commands below use `<sudoers-source>` for the selected absolute path under `/root/vigilafrica-migration-src`.

Validate the source:

```bash
visudo -cf <sudoers-source>
```

Expected output ends with:

```text
parsed OK
```

Stage the file under a name containing a dot, which sudo ignores:

```bash
install -m 0440 -o root -g root <sudoers-source> /etc/sudoers.d/.vigil-deploy.new
```

Expected output: no output.

Validate the installed bytes:

```bash
visudo -cf /etc/sudoers.d/.vigil-deploy.new
```

Expected output ends with:

```text
parsed OK
```

Publish it atomically:

```bash
mv /etc/sudoers.d/.vigil-deploy.new /etc/sudoers.d/vigil-deploy
```

Expected output: no output.

Validate the complete sudo configuration:

```bash
visudo -c
```

Expected output: `/etc/sudoers` and applicable included files report `parsed OK`, with no error.

Inspect the deploy-user authorization:

```bash
sudo -l -U deploy
```

Expected output includes only the two permitted helper forms:

```text
/usr/local/sbin/vigil-deploy-run ...staging...
/usr/local/sbin/vigil-deploy-run ...production...
```

It must also show the deploy-specific `env_reset` and `secure_path` defaults. It must not grant a shell, `docker`, `env`, arbitrary arguments under the regex rule, or `(ALL) ALL`.

Test alternate compose-path injection:

```bash
sudo -u deploy -- sudo -n /usr/local/sbin/vigil-deploy-run staging abc1234 /tmp/evil-compose.yml
```

Expected result:

- sudo `1.9.10+`: sudo refuses authorization; or
- older fallback: the helper prints `expected exactly 2 arguments, got 3`.

The exit status must be nonzero.

Test added-mount and extra-flag injection:

```bash
sudo -u deploy -- sudo -n /usr/local/sbin/vigil-deploy-run staging abc1234 --mount /:/host
```

Expected result: sudo or the helper refuses it, with a nonzero exit status.

Test path traversal:

```bash
sudo -u deploy -- sudo -n /usr/local/sbin/vigil-deploy-run staging ../../etc/passwd
```

Expected result: sudo or the helper refuses it, with a nonzero exit status.

Test environment preservation:

```bash
sudo -u deploy -- env DOCKER_HOST=tcp://127.0.0.1:2375 sudo -n --preserve-env=DOCKER_HOST /usr/local/sbin/vigil-deploy-run staging abc1234
```

Expected output indicates that `deploy` is not permitted to preserve the environment. The exit status must be nonzero.

**Verification:** full sudo configuration parses, the deploy user has only the reviewed helper authorization, and alternate paths, mounts, flags, traversal, and environment preservation are refused.

**Reversal:**

```bash
rm -f /etc/sudoers.d/vigil-deploy
visudo -c
```

Expected output: removal prints nothing; the final validation reports `parsed OK`.

---

## Phase 4: prepare fresh root-owned trees

No live deployment directory is replaced in this phase.

### 4.1 Create a migration timestamp and protected rollback directory

**Classification: reversible and inert.**

Run:

```bash
MIGRATION_STAMP=$(date -u +%Y%m%dT%H%M%SZ)
printf '%s\n' "$MIGRATION_STAMP" | tee /root/vigil-migration-stamp
```

Expected output: one UTC timestamp such as:

```text
20260815T230500Z
```

⚠️ **Write this value down, and keep it.** `$MIGRATION_STAMP` is a shell
variable used in roughly forty commands across Phases 4–9, including every
reversal. If this root shell dies — a dropped connection, a closed window —
the variable is gone, and the reversal paths silently become wrong
(`.forced-command-rollback-` instead of `.forced-command-rollback-<stamp>`).
Those commands fail rather than destroy anything, but they fail at the worst
possible moment.

Run this migration inside `tmux` or `screen` so the session survives a
disconnect. If you do lose the shell, recover the value before running any
further step:

```bash
cat /root/vigil-migration-stamp
# or, if that file is gone:
ls -d /opt/vigilafrica/.forced-command-rollback-*
```

Then re-export it in the new shell:

```bash
MIGRATION_STAMP=$(cat /root/vigil-migration-stamp)
printf '%s\n' "$MIGRATION_STAMP"
```

Expected output: the same timestamp recorded above. **Do not proceed with an
empty `$MIGRATION_STAMP`.**

Protect the application parent:

```bash
chown root:root /opt/vigilafrica
chmod 0755 /opt/vigilafrica
```

Expected output: no output.

Create a rollback directory that the deploy user cannot traverse:

```bash
install -d -m 0700 -o root -g root "/opt/vigilafrica/.forced-command-rollback-$MIGRATION_STAMP"
```

Expected output: no output.

Verify:

```bash
stat -c '%U:%G %a %n' /opt/vigilafrica "/opt/vigilafrica/.forced-command-rollback-$MIGRATION_STAMP"
```

Expected output:

```text
root:root 755 /opt/vigilafrica
root:root 700 /opt/vigilafrica/.forced-command-rollback-<timestamp>
```

**Verification:** the parent and rollback directory are root-controlled. The existing deploy user can still traverse and modify its current live trees, so the old path continues to work.

**Reversal:** before cutover, remove the empty rollback directory and restore `/opt/vigilafrica` to the owner and mode recorded in Phase 0:

```bash
rmdir "/opt/vigilafrica/.forced-command-rollback-$MIGRATION_STAMP"
chown <old-owner>:<old-group> /opt/vigilafrica
chmod <old-mode> /opt/vigilafrica
```

Expected output: no output.

### 4.2 Prepare the staging clone at the deployed revision

**Classification: reversible and inert.**

Confirm the staging destination is unused:

```bash
test ! -e "/opt/vigilafrica/.staging.new-$MIGRATION_STAMP" && echo 'staging destination is unused'
```

Expected output:

```text
staging destination is unused
```

Clone afresh:

```bash
GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null git clone --no-local https://github.com/didi-rare/vigilafrica.git "/opt/vigilafrica/.staging.new-$MIGRATION_STAMP"
```

Expected output begins with `Cloning into` and contains no fatal error.

Prove the old SHA exists in the fresh clone:

```bash
git -C "/opt/vigilafrica/.staging.new-$MIGRATION_STAMP" cat-file -e <staging-sha>^{commit} && echo 'staging commit exists'
```

Expected output:

```text
staging commit exists
```

Prove it is reachable from `origin/main`, as the helper will require:

```bash
git -C "/opt/vigilafrica/.staging.new-$MIGRATION_STAMP" merge-base --is-ancestor <staging-sha> origin/main && echo 'staging commit is reachable from origin/main'
```

Expected output:

```text
staging commit is reachable from origin/main
```

Check out the exact deployed revision:

```bash
git -C "/opt/vigilafrica/.staging.new-$MIGRATION_STAMP" checkout --quiet --detach <staging-sha>
```

Expected output: no output.

Copy only `.env` into a newly created inode:

```bash
install -m 0600 -o root -g root /opt/vigilafrica/staging/.env "/opt/vigilafrica/.staging.new-$MIGRATION_STAMP/.env"
```

Expected output: no output.

Remove group/world write permission from the new clone:

```bash
chmod -R go-w "/opt/vigilafrica/.staging.new-$MIGRATION_STAMP"
```

Expected output: no output.

Run the same safety predicate used by the helper:

```bash
find "/opt/vigilafrica/.staging.new-$MIGRATION_STAMP" \( ! -user root -o -perm /022 -o -type l \) -print -quit
```

Expected output: no output.

Verify `.env` content without printing secrets:

```bash
cmp /opt/vigilafrica/staging/.env "/opt/vigilafrica/.staging.new-$MIGRATION_STAMP/.env" && echo 'staging .env is identical'
```

Expected output:

```text
staging .env is identical
```

Verify the revision:

```bash
git -c safe.directory="/opt/vigilafrica/.staging.new-$MIGRATION_STAMP" -C "/opt/vigilafrica/.staging.new-$MIGRATION_STAMP" rev-parse HEAD
```

Expected output: exactly `<staging-sha>`.

**Verification:** the fresh staging clone is at the current deployed revision, contains only the carried `.env`, is root-owned, is not group/world-writable, and contains no symlink.

**Reversal:** before cutover, leave the staged clone in place for investigation or move it aside. Do not touch `/opt/vigilafrica/staging`.

### 4.3 Prepare the production clone at the deployed revision

**Classification: reversible and inert.**

Repeat the same preparation independently for production:

```bash
test ! -e "/opt/vigilafrica/.production.new-$MIGRATION_STAMP" && echo 'production destination is unused'
```

Expected output:

```text
production destination is unused
```

```bash
GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null git clone --no-local https://github.com/didi-rare/vigilafrica.git "/opt/vigilafrica/.production.new-$MIGRATION_STAMP"
```

Expected output begins with `Cloning into` and contains no fatal error.

```bash
git -C "/opt/vigilafrica/.production.new-$MIGRATION_STAMP" cat-file -e <production-sha>^{commit} && echo 'production commit exists'
```

Expected output:

```text
production commit exists
```

```bash
git -C "/opt/vigilafrica/.production.new-$MIGRATION_STAMP" merge-base --is-ancestor <production-sha> origin/release && echo 'production commit is reachable from origin/release'
```

Expected output:

```text
production commit is reachable from origin/release
```

```bash
git -C "/opt/vigilafrica/.production.new-$MIGRATION_STAMP" checkout --quiet --detach <production-sha>
```

Expected output: no output.

```bash
install -m 0600 -o root -g root /opt/vigilafrica/production/.env "/opt/vigilafrica/.production.new-$MIGRATION_STAMP/.env"
```

Expected output: no output.

```bash
chmod -R go-w "/opt/vigilafrica/.production.new-$MIGRATION_STAMP"
```

Expected output: no output.

```bash
find "/opt/vigilafrica/.production.new-$MIGRATION_STAMP" \( ! -user root -o -perm /022 -o -type l \) -print -quit
```

Expected output: no output.

```bash
cmp /opt/vigilafrica/production/.env "/opt/vigilafrica/.production.new-$MIGRATION_STAMP/.env" && echo 'production .env is identical'
```

Expected output:

```text
production .env is identical
```

```bash
git -c safe.directory="/opt/vigilafrica/.production.new-$MIGRATION_STAMP" -C "/opt/vigilafrica/.production.new-$MIGRATION_STAMP" rev-parse HEAD
```

Expected output: exactly `<production-sha>`.

**Verification:** the fresh production clone passes the same checks as staging and remains at the currently deployed production revision.

**Reversal:** before cutover, leave or move aside only the staged production clone. Do not touch `/opt/vigilafrica/production`.

### 4.4 Capture container start times immediately before cutover

**Classification: reversible observation record.**

Run:

```bash
docker inspect -f '{{.Name}} {{.Id}} {{.State.StartedAt}}' $(docker ps -q) | sort | tee /root/container-started-before-forced-command.txt
```

Expected output: one nonempty line per running container, containing its name, ID, and start timestamp.

Confirm no deploy process appeared while the clones were prepared:

```bash
pgrep -a -u deploy
```

Expected output: no output.

**Decision:** stop if any deployment started or any relevant container is no longer healthy.

**Verification:** a final start-time baseline is stored and the host is quiescent.

**Reversal:** remove the baseline file after the migration is complete; it does not affect the host.

---

# Cutover point

The next step is the deliberate cutover. Until now, the old host deployment path has remained usable.

From the first directory rename until workflow lineage is verified later, both deployment workflows must remain disabled. Do not create a tag or allow any manual deploy.

## Phase 5: replace the deploy-writable trees

### 5.1 Swap the staging tree

**Classification: CUTOVER — immediately reversible.**

Move the old staging tree into the protected rollback directory:

```bash
mv /opt/vigilafrica/staging "/opt/vigilafrica/.forced-command-rollback-$MIGRATION_STAMP/staging"
```

Expected output: no output.

Publish the fresh clone at the original path:

```bash
mv "/opt/vigilafrica/.staging.new-$MIGRATION_STAMP" /opt/vigilafrica/staging
```

Expected output: no output.

Verify:

```bash
git -c safe.directory=/opt/vigilafrica/staging -C /opt/vigilafrica/staging rev-parse HEAD
```

Expected output: exactly `<staging-sha>`.

```bash
find /opt/vigilafrica/staging \( ! -user root -o -perm /022 -o -type l \) -print -quit
```

Expected output: no output.

```bash
curl -fsS https://api.staging.vigilafrica.org/health
```

Expected output: the same healthy staging response recorded in Phase 0.

**Reversal:** while CI remains paused:

```bash
mv /opt/vigilafrica/staging "/opt/vigilafrica/.staging.failed-$MIGRATION_STAMP"
mv "/opt/vigilafrica/.forced-command-rollback-$MIGRATION_STAMP/staging" /opt/vigilafrica/staging
```

Expected output: no output. Recheck staging health. The old deploy path is restored for staging.

### 5.2 Swap the production tree

**Classification: CUTOVER — immediately reversible.**

Move the old production tree into the protected rollback directory:

```bash
mv /opt/vigilafrica/production "/opt/vigilafrica/.forced-command-rollback-$MIGRATION_STAMP/production"
```

Expected output: no output.

Publish the fresh clone:

```bash
mv "/opt/vigilafrica/.production.new-$MIGRATION_STAMP" /opt/vigilafrica/production
```

Expected output: no output.

Verify:

```bash
git -c safe.directory=/opt/vigilafrica/production -C /opt/vigilafrica/production rev-parse HEAD
```

Expected output: exactly `<production-sha>`.

```bash
find /opt/vigilafrica/production \( ! -user root -o -perm /022 -o -type l \) -print -quit
```

Expected output: no output.

```bash
curl -fsS https://api.vigilafrica.org/health
```

Expected output: the same healthy production response recorded in Phase 0.

Confirm no container restarted because of the directory swaps:

```bash
docker inspect -f '{{.Name}} {{.Id}} {{.State.StartedAt}}' $(docker ps -q) | sort | diff -u /root/container-started-before-forced-command.txt -
```

Expected output: no output.

**Decision:** stop and reverse the tree swaps if any container ID or start timestamp changed, either health endpoint fails, or the new trees fail their ownership checks. Do not run Compose to “fix” a filesystem-swap problem.

**Reversal:** reverse production first, then staging:

```bash
mv /opt/vigilafrica/production "/opt/vigilafrica/.production.failed-$MIGRATION_STAMP"
mv "/opt/vigilafrica/.forced-command-rollback-$MIGRATION_STAMP/production" /opt/vigilafrica/production
```

Then use the staging reversal above. Expected output: no output. Recheck both health endpoints.

---

## Phase 6: restrict the deployment key

### 6.1 Construct and validate the forced authorized-key entry

**Classification: reversible preparation.**

Copy the existing bare public key to a root-only file:

```bash
install -m 0600 -o root -g root /home/deploy/.ssh/authorized_keys "/root/deploy-key.raw-$MIGRATION_STAMP"
```

Expected output: no output.

Construct the reviewed forced-command record:

```bash
sed '1s|^|restrict,command="/usr/local/bin/vigil-deploy" |' "/root/deploy-key.raw-$MIGRATION_STAMP" > "/root/authorized_keys.forced-$MIGRATION_STAMP"
```

Expected output: no output.

Confirm it is exactly one line:

```bash
wc -l "/root/authorized_keys.forced-$MIGRATION_STAMP"
```

Expected output:

```text
1 /root/authorized_keys.forced-<timestamp>
```

Validate the constructed record:

```bash
ssh-keygen -lf "/root/authorized_keys.forced-$MIGRATION_STAMP"
```

Expected output: the same deployment-key fingerprint recorded in Phase 0.

Display the record for visual inspection:

```bash
sed -n '1p' "/root/authorized_keys.forced-$MIGRATION_STAMP"
```

Expected output begins exactly with:

```text
restrict,command="/usr/local/bin/vigil-deploy" ssh-
```

**Decision:** stop if the fingerprint changes, more than one line exists, or the prefix differs.

**Verification:** the exact record to be installed parses and contains one known key.

**Reversal:** remove the two root-only candidate files; the live key is unchanged at this point.

### 6.2 Install the forced key atomically

**Classification: CUTOVER — reversible through the open admin session.**

Back up the current key inside the protected rollback directory:

```bash
install -m 0600 -o root -g root /home/deploy/.ssh/authorized_keys "/opt/vigilafrica/.forced-command-rollback-$MIGRATION_STAMP/authorized_keys.pre-forced"
```

Expected output: no output.

Make the key directory root-controlled:

```bash
chown root:root /home/deploy/.ssh
chmod 0755 /home/deploy/.ssh
```

Expected output: no output.

Stage the new key file:

```bash
install -m 0644 -o root -g root "/root/authorized_keys.forced-$MIGRATION_STAMP" /home/deploy/.ssh/.authorized_keys.new
```

Expected output: no output.

Validate the staged file:

```bash
ssh-keygen -lf /home/deploy/.ssh/.authorized_keys.new
```

Expected output: the known deployment-key fingerprint.

Publish it atomically:

```bash
mv /home/deploy/.ssh/.authorized_keys.new /home/deploy/.ssh/authorized_keys
```

Expected output: no output.

Verify ownership, mode, and entry count:

```bash
stat -c '%U:%G %a %n' /home/deploy/.ssh /home/deploy/.ssh/authorized_keys
wc -l /home/deploy/.ssh/authorized_keys
```

Expected output:

```text
root:root 755 /home/deploy/.ssh
root:root 644 /home/deploy/.ssh/authorized_keys
1 /home/deploy/.ssh/authorized_keys
```

**Verification:** the effective key file contains exactly one root-owned forced-command entry.

**Reversal:** from `vigil-admin`, while workflows remain disabled:

```bash
install -m 0600 -o deploy -g deploy "/opt/vigilafrica/.forced-command-rollback-$MIGRATION_STAMP/authorized_keys.pre-forced" /home/deploy/.ssh/.authorized_keys.rollback
mv /home/deploy/.ssh/.authorized_keys.rollback /home/deploy/.ssh/authorized_keys
```

Expected output: no output. If performing a complete rollback, also restore the key-directory owner and mode recorded in Phase 0 and restore both original trees.

---

## Phase 7: pre-removal verification

The deploy account is still in the `docker` group during this phase. Do not remove it until every check below passes.

### 7.1 Prove shell, `id`, and `docker ps` are refused

Run these commands from the operator workstation using the deploy private key and the already-pinned host-key file.

Attempt an interactive shell:

```bash
ssh -T -i <deploy-private-key> -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes -o UserKnownHostsFile=<pinned-known-hosts-file> -o GlobalKnownHostsFile=/dev/null deploy@178.104.104.122
```

Expected output includes:

```text
vigil-deploy: refused: this key is restricted to the deploy protocol; no shell
```

Expected exit status: nonzero. No shell prompt may appear.

Attempt `id`:

```bash
ssh -T -i <deploy-private-key> -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes -o UserKnownHostsFile=<pinned-known-hosts-file> -o GlobalKnownHostsFile=/dev/null deploy@178.104.104.122 'id'
```

Expected output includes:

```text
vigil-deploy: refused: unknown verb 'id'
```

Expected exit status: nonzero. There must be no `uid=` output.

Attempt `docker ps`:

```bash
ssh -T -i <deploy-private-key> -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes -o UserKnownHostsFile=<pinned-known-hosts-file> -o GlobalKnownHostsFile=/dev/null deploy@178.104.104.122 'docker ps'
```

Expected output includes:

```text
vigil-deploy: refused: unknown verb 'docker'
```

Expected exit status: nonzero. There must be no container listing.

On the host, inspect the refusal log:

```bash
journalctl -t vigil-deploy --since "-15 minutes" --no-pager
```

Expected output: `REFUSED` records for no command, `id`, and `docker ps`, including the operator’s peer address.

**Decision:** if any attempt obtains a prompt, prints identity information, or lists containers, restore the previous key immediately and reverse the tree swaps. Do not remove the Docker group membership.

**Reversal:** use the authorized-key reversal in Phase 6.2, then restore the original trees if returning to the old protocol.

### 7.2 Prove the protocol discards client stdin

Run from the operator workstation:

```bash
printf 'id\n' | ssh -T -i <deploy-private-key> -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes -o UserKnownHostsFile=<pinned-known-hosts-file> -o GlobalKnownHostsFile=/dev/null deploy@178.104.104.122 'deploy-staging not-a-sha'
```

Expected output:

```text
vigil-deploy: refused: staging ref must be a hex commit sha
```

Expected exit status: nonzero. There must be no `uid=` output and no evidence that the stdin text was executed.

**Verification:** unexpected stdin does not become a command or reach a shell.

**Reversal:** none; this request is rejected before privileged work.

### 7.3 Confirm a same-revision deploy will not restart containers

Use the first seven characters of `<staging-sha>` as `<staging-short-sha>`.

From the host, ask Compose for a dry-run of the exact same staging revision:

```bash
cd /opt/vigilafrica/staging
```

Expected output: no output.

```bash
APP_VERSION=<staging-short-sha> docker compose --dry-run -f docker-compose.staging.yml up -d --build | tee "/root/staging-deploy-dry-run-$MIGRATION_STAMP.txt"
```

Expected output: a dry-run build/deployment plan. It must not report that any running container will be recreated, restarted, removed, or newly created.

Read the plan for actions against **containers**. This is advisory — it tells
you where to look, and it is expected to produce output:

```bash
grep -Ei 'container' "/root/staging-deploy-dry-run-$MIGRATION_STAMP.txt"
```

Expected output: one line per staging container, each describing a no-op such
as `Running`. Compose legitimately prints `Created`/`Creating` for networks and
volumes that already exist, so a bare word match is not evidence of a problem —
what matters is whether a line names a **running container** being recreated,
restarted, removed or created.

⚠️ Do not treat this grep as the gate. Its exact output format varies by
Compose version and has not been validated on this host. The decisive check is
the start-time comparison below: it observes what actually happened rather than
predicting it, and does not depend on Compose's wording.

**This is the gate.** Confirm the dry-run itself changed no start times:

```bash
docker inspect -f '{{.Name}} {{.Id}} {{.State.StartedAt}}' $(docker ps -q) | sort | diff -u /root/container-started-before-forced-command.txt -
```

Expected output: no output, and exit status zero.

If this produces a diff, a container was touched. Stop and treat it as a
failed step regardless of what the plan text said.

**Decision:** if this Compose version does not support `--dry-run`, or the dry-run predicts any container change, do not use a live deploy request as a test. Reverse the host cutover and reschedule for a window in which a staging restart is explicitly allowed. Production must not be restarted as a side effect of testing.

**Verification:** a request for the currently deployed staging revision is predicted to be a no-op for running containers.

**Reversal:** none; a valid dry-run changes nothing.

### 7.4 Prove a valid deploy request works

Run from the operator workstation. The deliberate stdin payload also verifies that accepted requests discard client input:

```bash
printf 'id\n' | ssh -T -i <deploy-private-key> -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes -o UserKnownHostsFile=<pinned-known-hosts-file> -o GlobalKnownHostsFile=/dev/null deploy@178.104.104.122 'deploy-staging <staging-short-sha>'
```

Expected output ends with:

```text
vigil-deploy-run: staging now at <staging-short-sha>
```

Expected exit status: zero. There must be no `uid=` output.

Confirm the revision:

```bash
git -c safe.directory=/opt/vigilafrica/staging -C /opt/vigilafrica/staging rev-parse HEAD
```

Expected output: exactly `<staging-sha>`.

Confirm no container restarted:

```bash
docker inspect -f '{{.Name}} {{.Id}} {{.State.StartedAt}}' $(docker ps -q) | sort | diff -u /root/container-started-before-forced-command.txt -
```

Expected output: no output.

Confirm both services remain healthy:

```bash
curl -fsS https://api.staging.vigilafrica.org/health
curl -fsS https://api.vigilafrica.org/health
```

Expected output: healthy JSON responses. Staging must still report `<staging-short-sha>`; production must report its unchanged tag.

Inspect accepted-request logs:

```bash
journalctl -t vigil-deploy -t vigil-deploy-run --since "-15 minutes" --no-pager
```

Expected output includes:

```text
ACCEPTED: deploy-staging <staging-short-sha>
DEPLOY staging <staging-short-sha>
```

**Decision:** if the request fails, changes a container start time, or affects production health, do not remove Docker group membership. Restore the previous key and trees.

**Verification:** the deploy key can run the validated protocol while remaining unable to execute a shell command.

**Reversal:** use the Phase 6.2 and Phase 5 reversal commands while CI remains disabled.

---

## Phase 8: promote the workflow lineage while deployment stays paused

### 8.1 Promote `development → main`

**Classification: reversible by a forward revert; workflows remain disabled.**

Merge the prepared `development → main` PR. Do not re-enable staging yet.

Fetch the updated branch into the trusted clone:

```bash
git -C /root/vigilafrica-migration-src fetch --force origin main:refs/remotes/origin/main
```

Expected output: a fetch update or no output, with no error.

Verify that `main` sends the forced-command request:

```bash
git -C /root/vigilafrica-migration-src show origin/main:.github/workflows/deploy-staging.yml | grep -F 'deploy-staging ${GITHUB_SHA::7}'
```

Expected output contains:

```text
"deploy-staging ${GITHUB_SHA::7}" < /dev/null
```

**Decision:** if `main` still sends the old remote shell program, keep the workflow disabled and do not continue.

**Reversal:** if the host must be rolled back to the old protocol, keep staging disabled and revert the workflow change through a reviewed PR before re-enabling it.

### 8.2 Promote `main → release`

**Classification: reversible by a forward revert; workflows remain disabled.**

Merge `main → release`. Do not create or merge a Release PR that creates a tag during this maintenance window.

Fetch the updated release branch:

```bash
git -C /root/vigilafrica-migration-src fetch --force origin release:refs/remotes/origin/release
```

Expected output: a fetch update or no output, with no error.

Verify that `release` sends the production request:

```bash
git -C /root/vigilafrica-migration-src show origin/release:.github/workflows/deploy-production.yml | grep -F 'deploy-production ${DEPLOY_TAG}'
```

Expected output contains:

```text
"deploy-production ${DEPLOY_TAG}" < /dev/null
```

**Decision:** do not proceed until the new production workflow is in the `release` lineage.

A production tag runs the workflow from the tag’s commit. If a tag is cut from a release commit carrying the old workflow after the host cutover, that workflow sends a shell program. The forced command will reject it and production deployment will fail, although the currently running production containers should remain up.

Conversely, if the new request-form workflow runs before the host cutover, the unrestricted shell tries to execute `deploy-staging` or `deploy-production` as an ordinary command and fails because no such shell command exists. Keeping both workflows disabled is what bridges this compatibility gap safely.

**Reversal:** if the host is rolled back, keep production disabled and revert the production workflow through a reviewed PR before creating any tag or re-enabling it.

---

## Phase 9: remove Docker group membership

### 9.1 Final quiescence check

**Classification: read-only gate.**

Run:

```bash
pgrep -a -u deploy
```

Expected output: no output.

Run:

```bash
id deploy
```

Expected output still includes the `docker` group at this point.

**Decision:** do not continue while any process runs as `deploy`. An already-running process retains its supplementary groups after the group database is changed.

**Verification:** no existing deploy process can retain Docker access.

**Reversal:** none; this gate changes nothing.

### 9.2 Remove `deploy` from `docker`

**Classification: LAST IRREVERSIBLE CUTOVER STEP.**

Run:

```bash
gpasswd -d deploy docker
```

Expected output is equivalent to:

```text
Removing user deploy from group docker
```

Verify:

```bash
id deploy
```

Expected output: the group list does not contain `docker`.

Verify the group database:

```bash
getent group docker
```

Expected output: the Docker group entry does not contain `deploy` in its member list.

Confirm no stale process appeared:

```bash
pgrep -a -u deploy
```

Expected output: no output.

**Verification:** the deploy account no longer receives Docker group membership and no pre-existing process retains it.

**Reversal:** only from `vigil-admin`:

```bash
gpasswd -a deploy docker
```

Expected output is equivalent to:

```text
Adding user deploy to group docker
```

The restored membership applies only to a newly created deploy process. For a complete rollback, also restore the original authorized key and deployment trees before testing the old path.

---

## Verification

Run these final tests after Docker group removal.

### 10.1 Repeat the three refusal tests

From the operator workstation, repeat:

```bash
ssh -T -i <deploy-private-key> -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes -o UserKnownHostsFile=<pinned-known-hosts-file> -o GlobalKnownHostsFile=/dev/null deploy@178.104.104.122
```

Expected: `no shell` refusal, nonzero exit, and no prompt.

```bash
ssh -T -i <deploy-private-key> -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes -o UserKnownHostsFile=<pinned-known-hosts-file> -o GlobalKnownHostsFile=/dev/null deploy@178.104.104.122 'id'
```

Expected: unknown-verb refusal, nonzero exit, and no `uid=` output.

```bash
ssh -T -i <deploy-private-key> -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes -o UserKnownHostsFile=<pinned-known-hosts-file> -o GlobalKnownHostsFile=/dev/null deploy@178.104.104.122 'docker ps'
```

Expected: unknown-verb refusal, nonzero exit, and no container listing.

### 10.2 Confirm final permissions and health

Run on the host:

```bash
stat -c '%U:%G %a %n' /usr/local/bin/vigil-deploy /usr/local/sbin/vigil-deploy-run /etc/sudoers.d/vigil-deploy /opt/vigilafrica /opt/vigilafrica/staging /opt/vigilafrica/production /home/deploy/.ssh /home/deploy/.ssh/authorized_keys
```

Expected ownership and modes:

```text
root:root 755 /usr/local/bin/vigil-deploy
root:root 700 /usr/local/sbin/vigil-deploy-run
root:root 440 /etc/sudoers.d/vigil-deploy
root:root 755 /opt/vigilafrica
root:root ... /opt/vigilafrica/staging
root:root ... /opt/vigilafrica/production
root:root 755 /home/deploy/.ssh
root:root 644 /home/deploy/.ssh/authorized_keys
```

The tree modes may vary below the top directory, but nothing may be group/world-writable and no path may be owned by `deploy`.

Run:

```bash
find /opt/vigilafrica/staging /opt/vigilafrica/production \( ! -user root -o -perm /022 -o -type l \) -print -quit
```

Expected output: no output.

Run:

```bash
visudo -c
```

Expected output: all sudoers files report `parsed OK`.

Run:

```bash
curl -fsS https://api.staging.vigilafrica.org/health
curl -fsS https://api.vigilafrica.org/health
```

Expected output: healthy JSON from both environments, with the same versions recorded before migration.

Run:

```bash
docker inspect -f '{{.Name}} {{.Id}} {{.State.StartedAt}}' $(docker ps -q) | sort | diff -u /root/container-started-before-forced-command.txt -
```

Expected output: no output.

Run:

```bash
journalctl -t vigil-deploy -t vigil-deploy-run --since "-30 minutes" --no-pager
```

Expected output contains the deliberate `REFUSED` records and one accepted same-revision staging deployment. There should be no unexplained request.

---

## After

### 11.1 Re-enable workflows in deployment order

The required code promotion order is:

```text
development → main → release → annotated production tag
```

It must already have been completed through `release` while deployments were disabled.

Enable staging first:

```bash
gh workflow enable deploy-staging.yml --repo didi-rare/vigilafrica
```

Expected output: no output.

Verify:

```bash
gh api repos/didi-rare/vigilafrica/actions/workflows/deploy-staging.yml --jq .state
```

Expected output:

```text
active
```

Do not manually dispatch a new staging revision merely to test enablement; the direct same-revision protocol test already proved the host path, and deploying a different SHA could restart staging containers.

Enable production only after confirming the release branch contains the request-form workflow:

```bash
gh workflow enable deploy-production.yml --repo didi-rare/vigilafrica
```

Expected output: no output.

Verify:

```bash
gh api repos/didi-rare/vigilafrica/actions/workflows/deploy-production.yml --jq .state
```

Expected output:

```text
active
```

**Reversal:** disable either workflow again if its branch lineage or host compatibility becomes uncertain. Do not create a production tag while production deployment is disabled or mismatched.

### 11.2 Monitor the next normal deployments

For the next intentional staging deployment, confirm:

- the workflow sends `deploy-staging <7-character-sha>`;
- the workflow finishes successfully;
- the staging health endpoint reports that SHA;
- `journalctl -t vigil-deploy -t vigil-deploy-run` records the accepted request;
- no unexplained refusal appears.

Before the next production tag, verify once more that the tag will be cut from `release` and that the release commit contains the new production workflow. After deployment, confirm the health endpoint reports the tag and the host log records `deploy-production <tag>`.

### 11.3 Retain rollback material

Keep all of the following until at least one subsequent normal staging deployment and one tag-triggered production deployment have succeeded through the new workflows:

```text
/opt/vigilafrica/.forced-command-rollback-<timestamp>/
/root/vigilafrica-migration-src/
/root/deploy-key.raw-<timestamp>
/root/authorized_keys.forced-<timestamp>
/root/container-started-before-forced-command.txt
```

The rollback directory contains old deploy-owned checkouts and `.env` files. Keep its parent mode at `0700`; never promote its Git configuration, hooks, symlinks, or other contents into a root-trusted checkout.

Remove or archive the rollback material only after the normal observation period and after confirming that current environment secrets are backed up through the approved secure process.

---

## If it goes wrong

### Failure before Phase 5

No live tree or SSH authorization has changed.

- Leave production running.
- Remove the inert helper or sudoers files if necessary using their inline reversals.
- Move aside the fresh clones.
- Re-enable the original workflows only if `main` and `release` still contain workflows compatible with the unchanged host.
- Do not run `provision.sh`.

### Failure after one or both tree swaps, before the forced key

The deploy key is still unrestricted, but the old shell workflow cannot update a root-owned live checkout.

- Keep both workflows disabled.
- Use the Phase 5 reversal commands to move the original trees back.
- Confirm both health endpoints.
- Confirm the old trees are again at their recorded SHAs.
- Re-enable deployments only after confirming workflow/host compatibility.

### Failure after installing the forced key, before Docker-group removal

Use the open `vigil-admin` session.

1. Restore `authorized_keys` using the Phase 6.2 reversal.
2. Restore production and staging trees using the Phase 5 reversal commands.
3. Restore the previously recorded `.ssh` owner and mode if performing a complete rollback.
4. Confirm the old deploy key can connect.
5. Keep workflows disabled until their old remote-shell form is restored through reviewed revert PRs.

The deploy account still has Docker group membership at this point; do not close the admin rescue session.

### Failure after workflow promotion

If the host remains on the forced-command protocol, keep the promoted workflows and resolve the host issue through `vigil-admin`.

If the host is rolled back to the old protocol:

- keep both deployment workflows disabled;
- revert the request-form workflow changes through reviewed forward commits or PRs;
- verify `main` and `release` once again send the old protocol;
- only then re-enable deployments.

Do not rewrite published branch history to recover during the migration.

### Failure after Docker-group removal

Use `vigil-admin`; the deploy key is not the rescue path.

1. Restore membership:

   ```bash
   gpasswd -a deploy docker
   ```

   Expected output: `Adding user deploy to group docker`.

2. Restore the original authorized key.
3. Restore the original production and staging trees.
4. Confirm no stale deploy process exists, then establish a new deploy connection so restored group membership takes effect.
5. Keep workflows disabled until their lineage matches the restored host protocol.
6. Verify both health endpoints before ending the maintenance window.

### Any production health failure

Filesystem renames and authorized-key changes do not require container restarts. Do not respond by restarting Docker, Caddy, Compose, or the production containers.

From `vigil-admin`:

- confirm the production container is still running;
- compare its ID and `StartedAt` value with the baseline;
- restore the original production tree if the failure began at the tree swap;
- restore the old key only if deployment access itself must be rolled back;
- keep CI disabled while investigating.

Never run `deploy/provision.sh` as a recovery action.
