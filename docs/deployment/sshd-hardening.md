# Task 1.6 — harden `sshd_config`

Closes task 1.6 of `chore-vps-access-hardening`: disable password authentication, keyboard-interactive
authentication, and root login on `vigilafrica-prod` (178.104.104.122).

This is a **runbook**, not a script. Every step is copy-paste, and every change is verified before the
rescue session is closed.

---

## Host state — measured 2026-08-16, not assumed

| Fact | Value | How it was established |
|---|---|---|
| OS | Ubuntu 24.04.4 LTS | `/etc/os-release` |
| sshd | OpenSSH 9.6p1 Ubuntu-3ubuntu13.18 | `sshd -V` |
| `Include` directive | **line 12**, before all auth directives | `grep -n` on `/etc/ssh/sshd_config` |
| `/etc/ssh/sshd_config.d/` | **empty** | `ls -la` |
| `KbdInteractiveAuthentication` | **already `no`** (line 71) and effective | config + auth-method probe |
| `PasswordAuthentication` | **absent from config** → compiled-in default `yes` | config grep |
| `PermitRootLogin` | **absent from config** → compiled-in default | config grep |
| Offered auth methods | **`publickey,password`** | `ssh -v` probe from the workstation |

**Password authentication is live on this host.** The probe returned
`Authentications that can continue: publickey,password` for both an unknown user and `root`.

⚠️ The probe **cannot** distinguish `PermitRootLogin yes` from `prohibit-password` — sshd advertises
the method list per connection, not per user. Only `sshd -T` (root) settles it. Either way 1.6 wants
`no`, so this does not change the work.

---

## ⚠️ The trap: drop-in files WIN, and the lowest filename wins

`sshd_config` line 12 is `Include /etc/ssh/sshd_config.d/*.conf`, and **OpenSSH uses the first
obtained value for each keyword** (`sshd_config(5)`). Two consequences, both counter-intuitive:

1. A directive in a drop-in **overrides** the same directive later in the main file.
2. Among drop-ins, files are read in glob (alphabetical) order, so **`00-…` beats `99-…`**. The
   familiar "99 = last = wins" convention from other config systems is **backwards here.**

The directory is empty today, but Ubuntu cloud images and `unattended-upgrades` are known to drop
files such as `50-cloud-init.conf` in later. **Therefore put the hardening in a low-sorted drop-in,
not in the main file** — that way a future cloud-init file cannot silently re-enable password auth.

⚠️ `sshd -t` checks **syntax, not effective policy.** It will pass happily while password login
remains enabled. Only `sshd -T` reports effective values.

---

## Before you start

**Keep the root/console session open for the entire procedure.** Every verification below must pass
*before* you close it. If anything is wrong, you fix it from that still-open session.

Rescue paths, in order of preference:

1. The **still-open** root session you are running this from.
2. `vigil-admin` key login from the workstation (task 1.1, verified working 2026-08-14):
   ```bash
   ssh -i ~/.ssh/vigil_admin -o IdentitiesOnly=yes \
     -o UserKnownHostsFile=~/.ssh/known_hosts_vigilafrica -o GlobalKnownHostsFile=/dev/null \
     vigil-admin@178.104.104.122
   ```
3. The **provider console** (out-of-band, survives total SSH lockout).

⚠️ `vigil-admin`'s `sudo` requires its password. Have it to hand — without it the rescue account can
log in but cannot administer.

---

## Step 1 — record the current effective policy

```bash
sshd -T | grep -iE '^(passwordauthentication|kbdinteractiveauthentication|permitrootlogin|pubkeyauthentication|usepam)'
```

Save the output. This is the "before" record and the rollback reference.

Expected, based on the measured state: `passwordauthentication yes`,
`kbdinteractiveauthentication no`, `permitrootlogin` either `yes` or `prohibit-password`.

---

## Step 2 — confirm the rescue account can actually log in with a key

Do this **now**, before changing anything. From the workstation, in a **second** terminal:

```bash
ssh -i ~/.ssh/vigil_admin -o IdentitiesOnly=yes -o BatchMode=yes \
  -o PreferredAuthentications=publickey \
  -o UserKnownHostsFile=~/.ssh/known_hosts_vigilafrica -o GlobalKnownHostsFile=/dev/null \
  vigil-admin@178.104.104.122 'id'
```

Expected: `uid=1001(vigil-admin) … groups=…,27(sudo)`.

**If this fails, STOP.** Disabling password auth without a working key login is how you lose the box.

---

## Step 3 — write the hardening drop-in

```bash
install -d -m 0755 -o root -g root /etc/ssh/sshd_config.d

cat > /etc/ssh/sshd_config.d/00-vigilafrica-hardening.conf <<'EOF'
# Task 1.6, chore-vps-access-hardening.
# Deliberately low-sorted: sshd uses the FIRST obtained value for each keyword and
# reads this directory in glob order, so 00- wins over any later cloud-init drop-in.
PasswordAuthentication no
KbdInteractiveAuthentication no
PermitRootLogin no
PubkeyAuthentication yes
EOF

chown root:root /etc/ssh/sshd_config.d/00-vigilafrica-hardening.conf
chmod 0644 /etc/ssh/sshd_config.d/00-vigilafrica-hardening.conf
```

`KbdInteractiveAuthentication` is already `no` in the main file; it is restated here so the whole
policy lives in one place and survives edits to the main file.

---

## Step 4 — syntax check, then verify EFFECTIVE policy before reloading

```bash
sshd -t && echo "syntax OK"
```

Then the check that actually matters:

```bash
sshd -T | grep -iE '^(passwordauthentication|kbdinteractiveauthentication|permitrootlogin)'
```

Required output — all three, exactly:

```text
passwordauthentication no
kbdinteractiveauthentication no
permitrootlogin no
```

Per-account confirmation, because a `Match` block could differ (none exist today, but verify rather
than trust):

```bash
for u in vigil-admin deploy root; do
  echo "--- $u"
  sshd -T -C "user=$u,host=vigilafrica-prod,addr=178.104.104.122" \
    | grep -iE '^(passwordauthentication|permitrootlogin)'
done
```

All three accounts must report `passwordauthentication no`.

⚠️ **If any value is still `yes`, do NOT reload.** Something earlier in the include order is winning;
find it with `sshd -T` and `grep -rn` over `/etc/ssh/sshd_config.d/` before continuing.

---

## Step 5 — reload

```bash
systemctl reload ssh
systemctl is-active ssh
```

Use `reload`, not `restart` — reload does not drop established connections.

⚠️ Ubuntu 24.04 socket-activates SSH. If `reload` reports the unit is inactive, the service is
`ssh.socket`-driven; `systemctl reload ssh` still applies to new connections. Confirm by opening a
**new** connection in step 6 rather than trusting the unit state.

---

## Step 6 — prove it empirically, from the workstation, BEFORE closing the rescue session

All three from a second terminal.

**(a) Key login still works** — the one that must succeed:

```bash
ssh -i ~/.ssh/vigil_admin -o IdentitiesOnly=yes -o BatchMode=yes \
  -o UserKnownHostsFile=~/.ssh/known_hosts_vigilafrica -o GlobalKnownHostsFile=/dev/null \
  vigil-admin@178.104.104.122 'echo KEY_LOGIN_OK'
```

Expected: `KEY_LOGIN_OK`.

**(b) Password auth is refused** — the offered-method list must no longer contain `password`:

```bash
ssh -o PubkeyAuthentication=no -o PreferredAuthentications=password,keyboard-interactive \
  -o BatchMode=yes -o ConnectTimeout=15 \
  -o UserKnownHostsFile=~/.ssh/known_hosts_vigilafrica -o GlobalKnownHostsFile=/dev/null \
  -v vigil-admin@178.104.104.122 true 2>&1 | grep -i 'Authentications that can continue'
```

Expected: `Authentications that can continue: publickey` — **`password` absent.**
Before the change this read `publickey,password`.

**(c) Root login is refused:**

```bash
ssh -o BatchMode=yes -o ConnectTimeout=15 \
  -o UserKnownHostsFile=~/.ssh/known_hosts_vigilafrica -o GlobalKnownHostsFile=/dev/null \
  -v root@178.104.104.122 true 2>&1 | grep -iE 'Permission denied|Authentications that can continue'
```

Expected: permission denied, and no method list that would let root in.

**(d) The deploy path still works** — the forced command is unaffected by these directives, but prove
it rather than assume:

```bash
ssh -i ~/.ssh/vigilafrica_deploy -o IdentitiesOnly=yes -o BatchMode=yes \
  -o UserKnownHostsFile=~/.ssh/known_hosts_vigilafrica -o GlobalKnownHostsFile=/dev/null \
  deploy@178.104.104.122 'id'
```

Expected: a **refusal** from `vigil-deploy`, nonzero exit, and **no `uid=` output** — i.e. exactly the
same behaviour as before this task.

⚠️ The exact message depends on the shape of the request, and an earlier draft of this runbook got it
wrong. `vigil-deploy` checks for a missing ref *before* it looks at the verb, so:

| Request | Refusal |
|---|---|
| `id` (one word) | `refused: missing ref argument` |
| `docker ps` (two words) | `refused: unknown verb 'docker'` |

Both are correct. What matters is the refusal and the absence of `uid=`, not which guard fired.

**Only when (a), (b), (c) and (d) all pass may you close the rescue session.**

---

## Rollback

From the still-open root session:

```bash
rm -f /etc/ssh/sshd_config.d/00-vigilafrica-hardening.conf
sshd -t && systemctl reload ssh
sshd -T | grep -iE '^(passwordauthentication|permitrootlogin)'
```

That restores the exact pre-change state, because the change is one additive file and the main config
is untouched.

If SSH is already unreachable, use the **provider console**, then run the same three commands.

---

## After

- Record the before/after `sshd -T` output in the task notes.
- Tick task 1.6 in `openspec/changes/chore-vps-access-hardening/tasks.md`.
- Note that `provision.sh` still does not manage `sshd_config`, so a **rebuild from
  `provision.sh` would not reproduce this hardening.** Either fold the drop-in into `provision.sh`
  or record it as a known gap — otherwise the next fresh VPS silently regresses.
