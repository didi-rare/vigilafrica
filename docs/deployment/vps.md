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
ALERT_EMAIL_TO=maintainer@example.com
ALERT_STALENESS_THRESHOLD_HOURS=2
ALERT_STALENESS_CHECK_INTERVAL_MIN=15
MAXMIND_ACCOUNT_ID=<optional>
MAXMIND_LICENSE_KEY=<optional>
```

Production should use `CORS_ORIGIN=https://vigilafrica.org`.

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
| `staging` | `VPS_HOST`, `VPS_USER`, `VPS_SSH_KEY` | none |
| `production` | `VPS_HOST`, `VPS_USER`, `VPS_SSH_KEY` | required reviewer |

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
