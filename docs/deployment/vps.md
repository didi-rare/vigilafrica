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

## One-Time Provisioning

Run the provisioning script as root after creating the VPS:

```bash
sudo SSH_PUBLIC_KEY='ssh-ed25519 ...' ./deploy/provision.sh
```

The script installs Docker, Caddy, ufw, fail2ban, unattended upgrades, creates the `deploy` user, and prepares `/opt/vigilafrica/{staging,production}`.

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

Create separate deploy-owned env files. The deploy workflows run `docker compose`
as this user, so Compose must be able to read `.env` while the file remains
private to the deploy account:

```bash
sudo install -m 600 -o deploy -g deploy /dev/null /opt/vigilafrica/staging/.env
sudo install -m 600 -o deploy -g deploy /dev/null /opt/vigilafrica/production/.env
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

1. Generate a new keypair (`ssh-keygen -t ed25519 -C 'vigilafrica-deploy-<yyyy-mm>'`).
2. Append the new public key to the deploy account's `authorized_keys`.
3. Update `VPS_SSH_KEY` in **both** environments and run a staging deploy to confirm it works.
4. **Remove the old public key from `authorized_keys`** — this step is the rotation. Adding a new
   key without removing the old one leaves the original credential valid.
5. Confirm the retired key is refused: `ssh -i old_key -o IdentitiesOnly=yes "$VPS_USER@$VPS_HOST"`
   must fail.

⚠️ Scope of the `production` required-reviewer gate: the rule exists, but the reviewer set is a
single account (`didi-rare`) which is also the account that triggers releases, and
`prevent_self_review` is `false`. It is a deliberate confirmation step, **not** a
separation-of-duties boundary, and it constrains only the *workflow* path — anyone holding
`VPS_SSH_KEY` reaches the host without touching GitHub at all.

🚨 **There is no host-level boundary between staging and production today.** A single `deploy`
account owns both environment directories and is in the `docker` group
([`provision.sh:33-47`](../../deploy/provision.sh)), so one leaked `VPS_SSH_KEY` is root-equivalent
on the box and reaches **both** environments. Nothing in this document should be read as saying
production is isolated at the host level — it is not. Establishing that boundary is task 1.5
(split the deploy account) of `chore-vps-access-hardening`, which is **not yet implemented**.

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
