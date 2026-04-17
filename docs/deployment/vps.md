# VPS Deployment Guide

This guide describes how to deploy VigilAfrica to a self-managed VPS (Hetzner, DigitalOcean, Contabo, etc.) using Docker Compose for the API + database and Caddy as a reverse proxy with automatic HTTPS.

Target audience: single-VPS production deployment for the v0.5 operational prototype.

---

## Table of Contents

- [Architecture](#architecture)
- [Prerequisites](#prerequisites)
- [Server Setup](#server-setup)
- [Docker Compose (Production)](#docker-compose-production)
- [Caddy Reverse Proxy](#caddy-reverse-proxy)
- [Environment Variables Reference](#environment-variables-reference)
- [Deployment Steps](#deployment-steps)
- [Database Backups](#database-backups)
- [Operational Checks](#operational-checks)
- [Troubleshooting](#troubleshooting)

---

## Architecture

```
          ┌──────────────────────────────────────────────┐
          │                    VPS                       │
          │                                              │
Internet ─┼─► Caddy :443 ──► Go API :8080 ──► Postgres :5432
          │   (TLS, HTTP/2)  (Docker)         (Docker + PostGIS)
          │                                              │
          └──────────────────────────────────────────────┘

Frontend (Vercel) ── fetch() ──► https://api.vigilafrica.io
```

- **Caddy** terminates TLS, serves a free Let's Encrypt cert, and proxies to the API container.
- **API** runs inside Docker, binds to `127.0.0.1:8080` (not exposed externally).
- **Postgres + PostGIS** runs inside Docker, reachable only from the API container.
- **Frontend** is hosted on Vercel and calls the API over HTTPS.

---

## Prerequisites

| Tool | Version | Notes |
|---|---|---|
| A VPS | 2 vCPU / 2 GB RAM min | Ubuntu 22.04 LTS or 24.04 LTS recommended |
| DNS record | A/AAAA | Point `api.yourdomain.com` at the VPS IP before requesting a cert |
| Docker Engine | 24+ | `docker compose` subcommand |
| Caddy | 2.7+ | Installed on the host (not in Docker — simpler cert renewal) |
| Non-root user | — | With `sudo` and Docker group membership |

---

## Server Setup

### 1. Base hardening

```bash
sudo apt update && sudo apt upgrade -y
sudo apt install -y ufw fail2ban
sudo ufw allow OpenSSH
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable
```

### 2. Install Docker

```bash
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker $USER
newgrp docker
```

### 3. Install Caddy

```bash
sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' \
  | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' \
  | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt update
sudo apt install -y caddy
```

### 4. Clone the repository

```bash
sudo mkdir -p /opt/vigilafrica
sudo chown $USER:$USER /opt/vigilafrica
cd /opt/vigilafrica
git clone https://github.com/didi-rare/vigilafrica.git .
```

---

## Docker Compose (Production)

Create `/opt/vigilafrica/docker-compose.prod.yml`:

```yaml
services:
  db:
    image: postgis/postgis:15-3.4
    restart: unless-stopped
    environment:
      POSTGRES_USER: ${POSTGRES_USER}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
      POSTGRES_DB: ${POSTGRES_DB}
    volumes:
      - pgdata:/var/lib/postgresql/data
    networks:
      - internal
    # Do NOT expose 5432 publicly — internal network only

  api:
    build:
      context: ./api
      dockerfile: Dockerfile
    restart: unless-stopped
    env_file: .env
    depends_on:
      - db
    ports:
      - "127.0.0.1:8080:8080"   # bound to localhost only; Caddy proxies in
    networks:
      - internal

volumes:
  pgdata:

networks:
  internal:
```

Create `/opt/vigilafrica/.env` from `.env.example` and fill in production values (see reference below).

---

## Caddy Reverse Proxy

Edit `/etc/caddy/Caddyfile`:

```caddy
api.yourdomain.com {
    encode zstd gzip

    # Forward the real client IP so the API's per-IP rate limiter works correctly
    reverse_proxy 127.0.0.1:8080 {
        header_up X-Real-IP {remote_host}
        header_up X-Forwarded-For {remote_host}
        header_up X-Forwarded-Proto {scheme}
    }

    # Standard security headers
    header {
        Strict-Transport-Security "max-age=31536000; includeSubDomains"
        X-Content-Type-Options "nosniff"
        Referrer-Policy "strict-origin-when-cross-origin"
        -Server
    }

    log {
        output file /var/log/caddy/api.log
        format json
    }
}
```

Reload:

```bash
sudo systemctl reload caddy
sudo journalctl -u caddy -f   # watch cert issuance
```

Caddy fetches and renews Let's Encrypt certs automatically.

---

## Environment Variables Reference

The production `.env` should set at minimum:

| Variable | Purpose | Example |
|---|---|---|
| `DATABASE_URL` | Postgres connection string (use the Docker service name `db`) | `postgres://vigilafrica:<pw>@db:5432/vigilafrica` |
| `POSTGRES_USER` | DB user for the `db` service | `vigilafrica` |
| `POSTGRES_PASSWORD` | DB password | strong random value |
| `POSTGRES_DB` | DB name | `vigilafrica` |
| `API_PORT` | API listen port | `8080` |
| `CORS_ORIGIN` | Allowed CORS origin — your Vercel frontend | `https://vigilafrica.vercel.app` |
| `LOG_LEVEL` | slog level | `info` |
| `GEOIP_DB_PATH` | Path to MaxMind `.mmdb` (mount into container) | `/data/GeoLite2-City.mmdb` |
| `INGEST_INTERVAL_MIN` | EONET poll interval in minutes | `60` |
| `RATE_LIMIT_RPM` | Per-IP rate limit (req/min) | `60` |
| `CACHE_TTL_SECONDS` | `/v1/events` response cache TTL | `300` |
| `RESEND_API_KEY` | Email alerting (optional) | `re_...` |
| `ALERT_EMAIL_TO` | Alert recipient | `ops@yourdomain.com` |
| `ALERT_FROM_EMAIL` | Verified Resend sender | `alerts@yourdomain.com` |
| `ALERT_STALENESS_THRESHOLD_HOURS` | Staleness alert threshold | `2` |

**Never commit `.env`.** Store a copy in a secrets manager (1Password, Bitwarden, etc.).

---

## Deployment Steps

From `/opt/vigilafrica`:

```bash
# 1. Pull the latest code
git pull origin main

# 2. Build and start
docker compose -f docker-compose.prod.yml up -d --build

# 3. Tail logs
docker compose -f docker-compose.prod.yml logs -f api
```

Migrations run automatically on API startup.

### Updating

```bash
cd /opt/vigilafrica
git pull
docker compose -f docker-compose.prod.yml up -d --build api
```

The DB container is not rebuilt — only the API.

---

## Database Backups

Add a nightly cron (root crontab):

```bash
sudo crontab -e
```

```cron
0 3 * * * docker compose -f /opt/vigilafrica/docker-compose.prod.yml exec -T db \
  pg_dump -U vigilafrica vigilafrica | gzip > /var/backups/vigilafrica-$(date +\%F).sql.gz
0 4 * * * find /var/backups -name 'vigilafrica-*.sql.gz' -mtime +14 -delete
```

Ensure `/var/backups` exists and is root-owned. Sync off-box (rsync, S3, Backblaze B2) for disaster recovery.

---

## Operational Checks

```bash
# API health (includes last_ingestion block)
curl https://api.yourdomain.com/health

# Expected on a healthy system:
# {"status":"ok","version":"0.5.0","last_ingestion":{"status":"success",...}}

# Rate limiter check — burst a few requests and confirm 429 appears only past RATE_LIMIT_RPM
for i in $(seq 1 80); do curl -s -o /dev/null -w "%{http_code}\n" https://api.yourdomain.com/v1/events; done | sort | uniq -c

# Check container state
docker compose -f docker-compose.prod.yml ps

# Stream API logs (JSON slog output)
docker compose -f docker-compose.prod.yml logs -f api
```

If `/health` returns `"status":"degraded"`, the last ingestion run failed — inspect logs and the `ingestion_runs` table.

---

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| `/health` returns `degraded` | Last ingestion failed | Check API logs for the EONET error; confirm outbound HTTPS works |
| Rate limiter blocks all traffic | Caddy not forwarding client IP | Verify `X-Forwarded-For` / `X-Real-IP` headers are set in the Caddyfile |
| 502 Bad Gateway from Caddy | API container down or not listening on `127.0.0.1:8080` | `docker compose logs api`; confirm `ports: 127.0.0.1:8080:8080` |
| CORS errors in browser | `CORS_ORIGIN` mismatch | Set to the exact Vercel origin including scheme, no trailing slash |
| Cert issuance fails | DNS not propagated or port 80 blocked | Confirm A record resolves to VPS IP; `ufw allow 80/tcp` |
| `ingestion_runs` table missing | Migrations didn't run | Restart the API container; migrations run on startup |

---

## Related Documents

- [ADR-011 — Ingestion Observability](../../openspec/specs/vigilafrica/decisions.md)
- [Roadmap v0.5](../../openspec/specs/vigilafrica/roadmap.md)
- [CONTRIBUTING.md](../../CONTRIBUTING.md)
