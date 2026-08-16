# Task 1.5 — split the deploy account, and retire the old one

Closes task 1.5 of `chore-vps-access-hardening`: replace the single `deploy` account, which owns both
environments, with `deploy-staging` and `deploy-prod` — each holding its own key, its own forced
command, and a sudoers rule that permits only its own environment.

A **runbook**, not a script. The forced-command migration was rewritten as a runbook after two review
rounds found that a one-shot root script must be defensive about host state it cannot observe, and
every blind guard turned out vacuous or wrong. Same reasoning here.

---

## Why this task still matters after 1.3 and 1.4

Tasks 1.3/1.4 already took away the shell and the docker group, and the trees are root-owned. So the
remaining exposure is narrow but real, and it is **authority, not file ownership**:

> **One credential can deploy production.** `VPS_SSH_KEY` reaches the host without touching GitHub, so
> production's required-reviewer gate does not constrain it — and per task 1.7 that gate is a single
> self-approving reviewer anyway. The split is what makes staging and production separate.

⚠️ The task text says each account should own "its own path". After 1.3/1.4 **neither account owns
any path** — root does, and the helper does all the writing. Do not "restore" per-account ownership
of `/opt/vigilafrica/*`; that would undo 1.4. The split here is of **credentials and authority**.

---

## What is already done in the repo

| Change | File | Status |
|---|---|---|
| Environment pinned per key in the forced command | `deploy/vigil-deploy` | done, 36/36 tests |
| Per-account sudoers rules | `deploy/sudoers.d/vigil-deploy` | done |
| Two accounts, distinct-key guard, pin verification | `deploy/provision.sh` | done |
| Workflows | `.github/workflows/deploy-*.yml` | **no change needed** |

`VPS_USER` and `VPS_SSH_KEY` are already **per-environment GitHub secrets**, and the workflows already
send `${VPS_USER}@${VPS_HOST}`. Switching accounts is a secret value change, not a code change.

`vigil-deploy-run` is unchanged — it already takes the environment as an argument and re-validates it.

---

## The cutover model: fail closed, briefly

`vigil-deploy` now **requires** the environment argument. The moment the new script is installed, the
old `deploy` account's pre-1.5 `authorized_keys` (no argument) stops working.

**That is deliberate.** The alternative — a transitional "both environments" mode — is exactly the
authority this task exists to remove, and transitional modes have a way of becoming permanent.

The cost is a window where deploys fail. That is acceptable here because **a failed deploy is a no-op,
not an outage** — proven during the host-key work: SSH refuses before anything executes, and the
running containers are untouched. Staging and production keep serving throughout.

⚠️ Do not start this while a deploy is in flight, and do not merge to `main` or cut a tag until
step 7 passes.

---

## Before you start

- A **root session on the host**, kept open until the end.
- The `vigil-admin` rescue account (task 1.1) reachable in a second terminal.
- Ability to set GitHub environment secrets (`gh secret set --env`).
- ⚠️ If task **1.6** has already been applied, password auth is off — confirm your key login works
  *before* you begin, because the console is then your only fallback.

---

## Step 1 — generate one keypair per environment, on the workstation

```bash
ssh-keygen -t ed25519 -N '' -C 'vigilafrica deploy-staging' -f ~/.ssh/vigilafrica_deploy_staging
ssh-keygen -t ed25519 -N '' -C 'vigilafrica deploy-prod'    -f ~/.ssh/vigilafrica_deploy_prod
```

Two distinct keys. `provision.sh` compares fingerprints and refuses identical ones, but this runbook
does not run `provision.sh`, so **check it yourself**:

```bash
ssh-keygen -lf ~/.ssh/vigilafrica_deploy_staging.pub
ssh-keygen -lf ~/.ssh/vigilafrica_deploy_prod.pub
```

The two fingerprints must differ. Reusing one key across both accounts leaves the split cosmetic —
every other check in this runbook would still pass.

---

## Step 2 — record the current state, so you can prove the change later

On the host:

```bash
getent passwd deploy deploy-staging deploy-prod
cat /home/deploy/.ssh/authorized_keys
sudo -l -U deploy
docker ps --format '{{.Names}}' | sort > /root/containers-before-account-split.txt
```

Save the output. `sudo -l -U deploy` is the "before" for the boundary you are about to move.

---

## Step 3 — create the two accounts

```bash
for u in deploy-staging deploy-prod; do
  id "$u" >/dev/null 2>&1 || useradd --create-home --shell /bin/bash "$u"
done
getent passwd deploy-staging deploy-prod
```

Neither account needs any group beyond its own. **Do not add either to `docker`** — that is equivalent
to root and would undo task 1.4.

```bash
id -nG deploy-staging; id -nG deploy-prod
```

Each must list only its own group.

---

## Step 4 — install the updated forced command and per-account sudoers

Copy the repo's `deploy/` directory to the host (or `git fetch` in a root-owned checkout), then:

```bash
install -m 0755 -o root -g root deploy/vigil-deploy /usr/local/bin/vigil-deploy
```

⚠️ `vigil-deploy-run` is **unchanged** — do not reinstall it unnecessarily.

Sudoers, validated before installing, because an unparseable file in `/etc/sudoers.d` can break `sudo`
for everyone including the rescue account:

```bash
TMP=$(mktemp)
install -m 0440 -o root -g root deploy/sudoers.d/vigil-deploy "$TMP"
visudo -cf "$TMP" || { echo "REJECTED — do not install"; rm -f "$TMP"; }
```

Only if that passed:

```bash
install -m 0440 -o root -g root "$TMP" /etc/sudoers.d/.vigil-deploy.new
mv -f /etc/sudoers.d/.vigil-deploy.new /etc/sudoers.d/vigil-deploy
rm -f "$TMP"
visudo -c
```

Now the check that actually matters:

```bash
sudo -l -U deploy-staging
sudo -l -U deploy-prod
sudo -l -U deploy
```

Required:

- `deploy-staging` — the **staging** rule only
- `deploy-prod` — the **production** rule only
- `deploy` — **nothing** (`User deploy is not allowed to run sudo`)

⚠️ Use `sudo -l -U`, never `visudo -c`. The wildcard bug this file was written to fix parsed
perfectly; only `-l` shows what is actually permitted.

---

## Step 5 — install the pinned forced commands

```bash
for pair in deploy-staging:staging deploy-prod:production; do
  u="${pair%%:*}"; e="${pair##*:}"
  install -d -m 0755 -o root -g root "/home/$u/.ssh"
  # paste the matching PUBLIC key for this account
  printf 'restrict,command="/usr/local/bin/vigil-deploy %s" %s\n' "$e" "<PUBKEY-FOR-$u>" \
    > "/home/$u/.ssh/authorized_keys"
  chown root:root "/home/$u/.ssh/authorized_keys"
  chmod 0644 "/home/$u/.ssh/authorized_keys"
done
```

⚠️ Root-owned and **not** writable by the account. An account that owns its own `authorized_keys` can
delete the forced command, and the boundary lasts only until someone thinks to.

Verify the pin landed — a missing environment argument looks fine here and refuses every deploy at
runtime:

```bash
grep -H 'command=' /home/deploy-staging/.ssh/authorized_keys /home/deploy-prod/.ssh/authorized_keys
stat -c '%U:%G %a %n' /home/deploy-staging/.ssh/authorized_keys /home/deploy-prod/.ssh/authorized_keys
```

Expect `vigil-deploy staging` / `vigil-deploy production`, both `root:root 644`, one key each.

---

## Step 6 — point GitHub at the new accounts

From the workstation. `VPS_USER` and `VPS_SSH_KEY` are per-environment, so set each side separately:

```bash
gh secret set VPS_USER    --env staging    --body 'deploy-staging'
gh secret set VPS_SSH_KEY --env staging    < ~/.ssh/vigilafrica_deploy_staging

gh secret set VPS_USER    --env production --body 'deploy-prod'
gh secret set VPS_SSH_KEY --env production < ~/.ssh/vigilafrica_deploy_prod
```

⚠️ `VPS_SSH_KEY` takes the **private** key. `VPS_HOST` and `VPS_HOST_KEY` are unchanged.

⚠️ Secrets cannot be read back. If the staging deploy in step 7 fails at `Configure SSH`, assume a
paste error and set it again — that failure is a no-op on the host.

---

## Step 7 — prove it, from the workstation

Use the pinned known-hosts file throughout:

```bash
O="-o IdentitiesOnly=yes -o BatchMode=yes -o StrictHostKeyChecking=yes \
   -o UserKnownHostsFile=$HOME/.ssh/known_hosts_vigilafrica -o GlobalKnownHostsFile=/dev/null"
```

**(a) Each account still refuses everything it should** (task 1.3 behaviour, per account):

```bash
ssh $O -i ~/.ssh/vigilafrica_deploy_staging deploy-staging@178.104.104.122            # no shell
ssh $O -i ~/.ssh/vigilafrica_deploy_staging deploy-staging@178.104.104.122 'id'       # refused
ssh $O -i ~/.ssh/vigilafrica_deploy_staging deploy-staging@178.104.104.122 'docker ps' # refused
```

**(b) THE SPLIT — both must be REFUSED.** These are well-formed requests the *other* account would
accept, so a refusal is the separation working rather than a parsing accident:

```bash
ssh $O -i ~/.ssh/vigilafrica_deploy_staging deploy-staging@178.104.104.122 'deploy-production v1.5.0'
ssh $O -i ~/.ssh/vigilafrica_deploy_prod    deploy-prod@178.104.104.122    'deploy-staging 8a15152'
```

Expect `refused: key is pinned to …` and nonzero exit from both.

**(c) The old key reaches neither environment.** The task's central requirement:

```bash
ssh $O -i ~/.ssh/vigilafrica_deploy deploy@178.104.104.122 'deploy-staging 8a15152'
ssh $O -i ~/.ssh/vigilafrica_deploy deploy@178.104.104.122 'deploy-production v1.5.0'
```

Both must fail. At this point they fail because `vigil-deploy` gets no pin — step 8 removes the
account entirely.

**(d) A real staging deploy through the workflow.** Merge something small to `main`, or dispatch
`deploy-staging.yml`, then confirm as in `forced-command-migration.md` §11.2: the run is green,
`api.staging.vigilafrica.org/health` reports the SHA, and the journal shows

```text
ACCEPTED: deploy-staging <sha> | env=staging
```

⚠️ **`deploy` renders as `***` in Actions logs** — it matches a secret value. `***-staging` is the
correct request form.

**(e) Production.** Only after (d). Dispatch `deploy-production.yml` with the **current** tag
`v1.5.0` — a redeploy of what is already running, so the blast radius is a restart, not a version
change. Confirm `/health` still reports `v1.5.0` and the journal shows
`ACCEPTED: deploy-production v1.5.0 | env=production`.

---

## Step 8 — retire the old account

⚠️ **Only after every check in step 7 passed.** Creating the new principals does not remove the old
one: its `authorized_keys`, sudoers entry, group memberships and file ownership can all survive and
keep the exposure intact.

```bash
# 1. Remove its authority first, so a mistake below cannot leave a usable account.
rm -f /home/deploy/.ssh/authorized_keys
gpasswd -d deploy docker 2>/dev/null || true
id -nG deploy

# 2. Lock the account and disable its shell.
usermod -L -s /usr/sbin/nologin deploy
passwd -S deploy          # expect L (locked)

# 3. Confirm sudo grants it nothing.
sudo -l -U deploy         # expect: not allowed to run sudo
```

Prove the old key is dead, from the workstation:

```bash
ssh $O -i ~/.ssh/vigilafrica_deploy deploy@178.104.104.122 'deploy-staging 8a15152'
```

Expect `Permission denied (publickey)` — refused at authentication now, not by the forced command.

Then check nothing else on the host still refers to it:

```bash
grep -rn 'deploy' /etc/sudoers /etc/sudoers.d/ 2>/dev/null | grep -vE 'deploy-staging|deploy-prod|vigil-deploy'
find / -xdev -path /home/deploy -prune -o -user deploy -print 2>/dev/null | head
```

Both must print nothing.

⚠️ **An earlier draft had `find / -xdev -user deploy` and claimed it must print nothing — that was
wrong**, because `/home/deploy` is owned by `deploy` and would always match. Pruning its own home is
the check actually intended: *no `deploy`-owned files outside its home directory*.

⚠️ **Expect a hit under `/opt/vigilafrica/.forced-command-rollback-<timestamp>/` if you have not yet
retired that material** — it holds the pre-1.3/1.4 deploy-owned checkouts on purpose. That is
containment, not leakage, **provided the rollback directory is `root:root 0700`**, which makes its
contents unreachable regardless of ownership. Verify rather than assume:

```bash
stat -c '%U:%G %a %n' /opt/vigilafrica/.forced-command-rollback-*
```

**Deleting the account** (`userdel -r deploy`) is optional and irreversible. Locked-plus-no-key is
already sufficient, and keeping the home directory preserves any forensic material. If you do delete
it, do so only after the next successful production deploy.

---

## Rollback

Before step 8 the old path is intact: restore `/home/deploy/.ssh/authorized_keys` from step 2's
record, reinstall the previous `vigil-deploy`, revert the two GitHub secrets, and deploys work as
before.

After step 8, roll forward instead — recreate `authorized_keys` for whichever new account is failing
from the keys generated in step 1. The rollback material from the forced-command migration
(`/opt/vigilafrica/.forced-command-rollback-*`) is unrelated to this change and should be left alone.

---

## After

- Record the `sudo -l -U` output for all three accounts in the task notes.
- Tick task 1.5 in `openspec/changes/chore-vps-access-hardening/tasks.md`.
- Then task **1.7**, which documents 1.1–1.6 in `docs/deployment/vps.md` and must come last.
- ⚠️ `docs/deployment/vps.md` and the incident runbook still describe a single `deploy` account —
  update them, or the next incident is worked from instructions that no longer match the host.
