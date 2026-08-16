# VPS Deployment Guide

This guide describes the v1.0 single-VPS topology for VigilAfrica: one Hetzner/DigitalOcean-style VPS, two isolated Docker Compose stacks, one host-level Caddy reverse proxy, and Resend-backed ingestion alerting.

## Topology

| Environment | Frontend | API | Compose file | Host API port | Branch/ref |
|---|---|---|---|---|---|
| Staging | `staging.vigilafrica.org` | `api.staging.vigilafrica.org` | `docker-compose.staging.yml` | `127.0.0.1:8081` | `main` |
| Production | `vigilafrica.org` | `api.vigilafrica.org` | `docker-compose.prod.yml` | `127.0.0.1:8080` | SemVer tag from `release` |

Both stacks run on the same VPS but use separate containers, networks, and Docker volumes:

```text
/opt/vigilafrica/
  staging/       # clone checked out to main, .env for staging
  production/    # clone checked out to vX.Y.Z tag, .env for production

Caddy:
  api.staging.vigilafrica.org -> 127.0.0.1:8081
  api.vigilafrica.org         -> 127.0.0.1:8080
```

## Host Access Model

Current as of **2026-08-16**, after tasks 1.1–1.6 of `chore-vps-access-hardening`. Every row below was
verified against the running host, not inferred from the scripts.

| Account | Purpose | Auth | Privilege |
|---|---|---|---|
| `vigil-admin` | human administration, rescue | ed25519 key only | `sudo` (password required) |
| `deploy-staging` | staging deploys | ed25519 key, forced command | `vigil-deploy-run ^staging …` only |
| `deploy-prod` | production deploys | ed25519 key, forced command | `vigil-deploy-run ^production …` only |
| `deploy` | **retired** | none — locked, `nologin` | none |
| `root` | — | **no SSH login** | — |

**How a deploy actually runs.** The workflow sends a *request*, not a command:
`ssh deploy-staging@host "deploy-staging <sha>"`. The forced command `/usr/local/bin/vigil-deploy
<environment>` validates it unprivileged, then hands a fixed argv to the root helper
`/usr/local/sbin/vigil-deploy-run` through a per-account sudoers rule. The deploy accounts have no
shell, no port forwarding, no SFTP/SCP, no `docker` membership, and write nothing under
`/opt/vigilafrica`.

**Where each boundary lives** — worth knowing during an incident, because they fail independently:

| Control | Enforced by | Task |
|---|---|---|
| No shell / no forwarding / no SFTP | `restrict,command=` in root-owned `authorized_keys` | 1.3 |
| Only two verbs, argument shapes | `vigil-deploy` (unprivileged) | 1.3 |
| **staging ≠ production** | **per-account sudoers rule** | **1.5** |
| No root-equivalence via Docker | not in `docker`; trees root-owned | 1.4 |
| Argument re-validation | `vigil-deploy-run` re-checks its own argv | 1.3 |
| Host identity | `VPS_HOST_KEY` pin, `StrictHostKeyChecking=yes` | 1.2 |
| No password / root SSH | `sshd_config.d/00-vigilafrica-hardening.conf` | 1.6 |
| Recovery | `vigil-admin`, then provider console | 1.1 |

⚠️ **`sshd` hardening lives in a drop-in, and the ordering is counter-intuitive.** `Include` sits near
the top of `sshd_config` and OpenSSH uses the **first obtained value**, so drop-ins beat the main file
and the **lowest** filename beats later ones. `00-` cannot be overridden by a later cloud-init `50-`.
Rollback is removing that one file and `systemctl reload ssh`.

⚠️ **`sshd -t` checks syntax, not effective policy.** Always confirm with `sshd -T`, and per-account
with `sshd -T -C user=…,host=…,addr=…`.

⚠️ **Recovery depends on `vigil-admin`'s sudo password.** It logs in by key but `sudo` prompts. An
admin that cannot administer is not a rescue path — verify `sudo -v` before closing any root session.

## One-Time Provisioning

Run the provisioning script as root after creating the VPS:

```bash
sudo SSH_PUBLIC_KEY_STAGING='ssh-ed25519 ... deploy-staging' \
     SSH_PUBLIC_KEY_PROD='ssh-ed25519 ... deploy-prod' \
     ./deploy/provision.sh
```

The script installs Docker, Caddy, ufw, fail2ban and unattended upgrades, creates the **two** deploy
accounts (`deploy-staging` and `deploy-prod`), installs the forced-command protocol and per-account
sudoers rules, hardens `sshd`, and prepares root-owned `/opt/vigilafrica/{staging,production}`.

⚠️ **The two keys must be different.** The provisioner compares fingerprints and refuses identical
ones: a single key on both accounts would leave the staging/production split cosmetic while every
other check reported success.

⚠️ `DEPLOY_USER` and `SSH_PUBLIC_KEY` no longer exist. The provisioner **rejects** them rather than
ignoring them, so an old command line fails loudly instead of provisioning something unintended.

⚠️ `sshd` hardening is **skipped with a warning** if no non-deploy account has a key-based login —
the deploy accounts have no shell and cannot recover a host, so hardening before an admin account
exists would lock you out. Create the admin account first (task 1.1).

Clone the repo into both paths:

```bash
sudo -iu deploy
git clone https://github.com/didi-rare/vigilafrica.git /opt/vigilafrica/staging
git clone https://github.com/didi-rare/vigilafrica.git /opt/vigilafrica/production
```

Install `deploy/Caddyfile.example` as `/etc/caddy/Caddyfile`, then reload:

```bash
sudo cp deploy/Caddyfile.example /etc/caddy/Caddyfile
sudo caddy validate --config /etc/caddy/Caddyfile
sudo systemctl reload caddy
```

## Runtime `.env` Files

Create separate **root-owned** env files, one per environment:

```bash
sudo install -m 600 -o root -g root /dev/null /opt/vigilafrica/staging/.env
sudo install -m 600 -o root -g root /dev/null /opt/vigilafrica/production/.env
```

⚠️ **These were `deploy`-owned before task 1.4, and must not be any more.** `docker compose` is run
by the **root helper** (`vigil-deploy-run`), not by a deploy account, so nothing but root needs to
read them — and they hold database and API credentials. Making them readable by a deploy account
would hand a leaked deploy key the database password, which is the exposure 1.4 and 1.5 exist to
close.

Verify after any change:

```bash
sudo stat -c '%U:%G %a %n' /opt/vigilafrica/{staging,production}/.env   # expect root:root 600
```

Minimum variables per environment:

```env
POSTGRES_USER=vigilafrica
POSTGRES_PASSWORD=<strong-random-password>
POSTGRES_DB=vigilafrica
CORS_ORIGIN=https://staging.vigilafrica.org
LOG_LEVEL=info
INGEST_INTERVAL_MIN=60
RATE_LIMIT_RPM=60
CACHE_TTL_SECONDS=300
RESEND_API_KEY=re_...
ALERT_FROM_EMAIL=VigilAfrica Alerts <alerts@vigilafrica.org>
ALERTS_TO=ops@example.com,maintainer@example.com
ALERT_STALENESS_THRESHOLD_HOURS=2
ALERT_STALENESS_CHECK_INTERVAL_MIN=15
MAXMIND_ACCOUNT_ID=<maxmind-account-id>
MAXMIND_LICENSE_KEY=<per-environment-license-key>
```

The `MAXMIND_*` values are **required for `/v1/context` ("near me") to work**, not
truly optional: the `geoipupdate` sidecar needs them to download `GeoLite2-City.mmdb`.
Left unset, the compose files fall back to placeholders, the sidecar crash-loops,
the database is never written, and `/v1/context` returns empty locations while the
API still reports healthy — so the failure is silent. Use a **separate license key
per environment** (one shared AccountID) so staging and production can be revoked
independently. Register free at <https://www.maxmind.com/en/geolite2/signup>.

`APP_ENV` does not need to appear in these `.env` files: `docker-compose.staging.yml`
and `docker-compose.prod.yml` hardcode it (`staging` / `production` respectively) so
the alert subject prefix stays bound to the compose file the operator chose. Override
only when running the stack outside its intended compose file.

Production should use `CORS_ORIGIN=https://vigilafrica.org`.
Use placeholder addresses in committed docs only. Set real alert recipients
directly in `/opt/vigilafrica/staging/.env` and
`/opt/vigilafrica/production/.env`; these files are not committed and should
remain deploy-owned with mode `0600`.

## Manual Stack Commands

Staging:

```bash
cd /opt/vigilafrica/staging
git checkout main
git pull --ff-only origin main
APP_VERSION=$(git rev-parse --short HEAD) docker compose -f docker-compose.staging.yml up -d --build
curl -fsS https://api.staging.vigilafrica.org/health
```

Production:

```bash
cd /opt/vigilafrica/production
git fetch --all --tags
git checkout --force v1.0.0
APP_VERSION=v1.0.0 docker compose -f docker-compose.prod.yml up -d --build
curl -fsS https://api.vigilafrica.org/health
```

## GitHub Actions

- `.github/workflows/deploy-staging.yml`: push to `main` deploys the staging API stack.
- `.github/workflows/deploy-production.yml`: pushing a `v*.*.*` tag deploys production after GitHub Environment approval.
- Production also supports `workflow_dispatch` with a tag input for rollback.

Configure GitHub Environments:

| Environment | Required secrets | Protection |
|---|---|---|
| `staging` | `VPS_HOST`, `VPS_USER`, `VPS_SSH_KEY`, `VPS_HOST_KEY` | none |
| `production` | `VPS_HOST`, `VPS_USER`, `VPS_SSH_KEY`, `VPS_HOST_KEY` | required reviewer |

### Host-key pinning (`VPS_HOST_KEY`)

Both deploy workflows write `~/.ssh/known_hosts` from this secret and connect with
`StrictHostKeyChecking=yes`. **A missing or empty `VPS_HOST_KEY` fails the deploy** rather than
falling back — the workflows previously ran `ssh-keyscan` on every deploy, which trusts whatever
answers on port 22 and therefore cannot detect the host substitution it appears to guard against.

Read the key **from the VPS filesystem**, over the provider's web console or an already-trusted
session.

⚠️ **`ssh-keyscan` must never be the authority for this value.** It opens a network connection and
prints whatever answers, so its output is only as trustworthy as the network at that moment. It is
not forbidden outright — comparing its output against a fingerprint you verified independently is a
legitimate cross-check — but on its own, *including run on the VPS itself*, it cannot authenticate
anything, and using it to create the pin just repeats the trust-on-first-use step this secret exists
to eliminate.

First confirm which host keys are configured. A host can have several, and the `.pub` file sitting
next to a private key is a convenience copy that may be stale or unused:

```bash
# Run ON the VPS. Prints the effective CONFIGURED host-key paths.
# Anchored to "hostkey " so it does not also match hostkeyalgorithms/hostkeyagent.
sudo sshd -T | grep -i '^hostkey '
```

⚠️ `sshd -T` prints the **effective configuration on disk**, which is not the same as what the
**currently running** daemon offers — the config may have changed since the last reload. Confirm the
live handshake separately and check it agrees with the file you read:

```bash
# The fingerprint the running daemon actually presents.
ssh-keyscan -t ed25519 "$VPS_HOST" 2>/dev/null | ssh-keygen -lf -
# Must equal:
ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub
```

Used this way `ssh-keyscan` is a **cross-check against a value you already trust**, never the source
of the pin.

Then read the public half of the key that list names — `/etc/ssh/ssh_host_ed25519_key` on a default
Debian/Ubuntu install:

```bash
cat /etc/ssh/ssh_host_ed25519_key.pub               # -> "ssh-ed25519 AAAA... root@host"
ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub    # fingerprint, to compare at the console
```

If `sshd -T` names a path whose `.pub` is missing, regenerate it from the private key with
`ssh-keygen -y -f /etc/ssh/ssh_host_ed25519_key`.

Take the **first two fields only** (`ssh-ed25519 AAAA...`, dropping the trailing comment) and
prepend the hostname the workflow actually connects to, i.e. the exact value of `VPS_HOST`:

```
vps.example.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA...
```

Confirm the fingerprint you printed at the console matches the one SSH reports on first contact
before trusting it anywhere else.

The hostname in the line **must match `VPS_HOST`** exactly — `known_hosts` matches on the string SSH
was asked to connect to, so an IP in the secret will not match a DNS name in `VPS_HOST` (or vice
versa). `hostname -f` on the VPS is **not** reliably that string; use `VPS_HOST` itself. The
workflows verify this for you: they call `ssh-keygen -F "$VPS_HOST"` and fail if the pinned file
contains no entry for that exact host.

⚠️ **The workflows connect on the default port 22 only.** `known_hosts` syntax for a non-default
port is `[vps.example.com]:2222 ssh-ed25519 AAAA...`, but neither workflow passes `-p`/`Port`, so a
non-default port needs a workflow change (a `VPS_PORT` input threaded into both `ssh` calls and into
the `ssh-keygen -F` lookup) — it is not configurable by the secret alone.

The workflows also reject wildcard host patterns and `@cert-authority` records — both trust more
than the one host this pin is meant to fix — and `@revoked` records, which do the *opposite*
(they refuse a key) and so cannot serve as the pin either. All three are configuration errors in
this secret, and each is rejected with its own message so the cause is obvious.

Verify the pin is actually load-bearing by setting `VPS_HOST_KEY` on **staging** to a deliberately
wrong key once and confirming the deploy **fails** with a host-key mismatch instead of warning and
continuing. A control that has never been observed failing has not been tested.

Rotate `VPS_HOST_KEY` whenever the VPS is rebuilt, restored from a snapshot, or its SSH host keys
are regenerated — otherwise every deploy fails closed until it is updated.

### Deploy-credential rotation

`VPS_SSH_KEY` is a static secret with **no expiry**; nothing rotates it automatically and nothing
alerts when it ages. Rotate it on a fixed schedule and immediately on any suspected exposure:

🚨 **Never append a bare public key.** Each deploy account's `authorized_keys` is root-owned and its
single entry carries `restrict,command="/usr/local/bin/vigil-deploy <environment>"`. A plain
`>> authorized_keys` adds an **unrestricted** key that can open a shell, silently undoing the whole
forced-command boundary — and nothing would fail or warn, because deploys would keep working.

⚠️ **There are two accounts and two keys** (task 1.5). Rotate them **independently**; rotating one
does not touch the other, and using one key for both would collapse the split while every check still
passed.

| Account | Environment | GitHub environment |
|---|---|---|
| `deploy-staging` | staging | `staging` |
| `deploy-prod` | production | `production` |

Rotate by **re-running the provisioner**, which rewrites both files with exactly one restricted entry
each:

1. Generate a new keypair for the account being rotated
   (`ssh-keygen -t ed25519 -C 'vigilafrica deploy-staging <yyyy-mm>'`).
2. From an admin session on the VPS, re-run the provisioner. It replaces `authorized_keys` wholesale
   — old key gone, new key restricted — so there is no window with two valid credentials:

   ```bash
   sudo SSH_PUBLIC_KEY_STAGING='ssh-ed25519 AAAA... deploy-staging <yyyy-mm>' \
        SSH_PUBLIC_KEY_PROD='ssh-ed25519 AAAA... deploy-prod <yyyy-mm>' \
        ./deploy/provision.sh
   ```

   ⚠️ Both variables are required, and the provisioner **refuses two identical keys** by comparing
   fingerprints. Pass the current value for whichever account you are not rotating.

3. Update `VPS_SSH_KEY` in the **matching** GitHub environment, then deploy to confirm.
4. Confirm the retired key is refused — it must fail at **authentication**:
   `ssh -i old_key -o IdentitiesOnly=yes "$VPS_USER@$VPS_HOST"` → `Permission denied (publickey)`.
5. Confirm the new key is **restricted, not merely working** — a key that deploys successfully may
   still be unrestricted, and may still be pinned to the wrong environment:

   ```bash
   ssh -i new_key "$VPS_USER@$VPS_HOST"                            # refused, no shell
   ssh -i new_key "$VPS_USER@$VPS_HOST" 'id'                       # refused
   ssh -i new_staging_key deploy-staging@"$VPS_HOST" 'deploy-production v1.0.0'  # refused: pinned
   ```

   That last one is the important check: it is a **well-formed request the other account would
   accept**, so a refusal proves the separation rather than a parsing accident.

If you must edit `authorized_keys` by hand, the entry format is:

```
restrict,command="/usr/local/bin/vigil-deploy staging" ssh-ed25519 AAAA... comment
```

⚠️ The environment argument is **required**. `vigil-deploy` fails closed without it — an entry left in
the pre-1.5 form (`command="/usr/local/bin/vigil-deploy"`) authenticates and then refuses every
deploy with `no environment pinned in the forced command`.

⚠️ Scope of the `production` required-reviewer gate: the rule exists, but the reviewer set is a
single account (`didi-rare`) which is also the account that triggers releases, and
`prevent_self_review` is `false`. It is a deliberate confirmation step, **not** a
separation-of-duties boundary, and it constrains only the *workflow* path — anyone holding
`VPS_SSH_KEY` reaches the host without touching GitHub at all.

✅ **A host-level boundary between staging and production now exists** (task 1.5, applied and verified
2026-08-16). `deploy-staging` and `deploy-prod` hold **separate keys** and separate authority, so a
leaked staging credential cannot reach production.

**The boundary is sudoers, not the forced command.** Each account's sudoers rule permits only its own
argv, so `deploy-staging` cannot deploy production even if its key, its forced command and
`vigil-deploy` itself were all subverted. The environment pinned in `authorized_keys` is the first
filter; it would fall to anyone able to rewrite that file, and the sudo rule would still hold.

Verify with `sudo -l -U deploy-staging` and `sudo -l -U deploy-prod` — **not** `visudo -c`, which
checks syntax only. The wildcard bug these rules were written to fix parsed perfectly.

⚠️ **What this boundary does NOT do.** It does not make a leaked production key harmless — that key
still deploys production without touching GitHub, and per the note above the reviewer gate constrains
only the workflow path. What changed is blast radius: one credential now reaches **one** environment.

Related hardening in place: forced command with no shell and no port forwarding (1.3), neither
account in the `docker` group and both environment trees root-owned (1.4), and password/root SSH
login disabled (1.6). The old single `deploy` account is **retired** — locked, no key, no sudoers
entry, and refused at authentication.

## Operational Checks

```bash
docker compose -f /opt/vigilafrica/staging/docker-compose.staging.yml ps
docker compose -f /opt/vigilafrica/production/docker-compose.prod.yml ps
curl -fsS https://api.staging.vigilafrica.org/health
curl -fsS https://api.vigilafrica.org/health
```

`/health.version` is stamped from `APP_VERSION` during the Docker build. Staging should show the short commit SHA; production should show the SemVer tag.

## Backups

Add root cron jobs for both volumes:

```cron
0 2 * * * docker compose -f /opt/vigilafrica/staging/docker-compose.staging.yml exec -T staging-db pg_dump -U vigilafrica vigilafrica | gzip > /var/backups/vigilafrica-staging-$(date +\%F).sql.gz
0 3 * * * docker compose -f /opt/vigilafrica/production/docker-compose.prod.yml exec -T prod-db pg_dump -U vigilafrica vigilafrica | gzip > /var/backups/vigilafrica-prod-$(date +\%F).sql.gz
0 4 * * * find /var/backups -name 'vigilafrica-*.sql.gz' -mtime +14 -delete
```

Sync backups off-box before calling production resilient.
