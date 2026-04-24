# Proposal: Staging + Production Deployment with Resend Alerting (feature-deploy-staging-prod-alerts)

## Why

VigilAfrica is approaching v1.0 — a credible, public-facing launch — but three operational prerequisites carried over from v0.5 are still open:

1. **No deployed environment exists.** The roadmap (§v0.5, line 182) requires a VPS deployment, and `docs/deployment/vps.md` documents it, but nothing is actually running.
2. **No email alerting.** ADR-011 and roadmap items (lines 177–178) mandate Resend-based alerts for failed ingestion runs and a staleness watchdog — neither is configured or wired up.
3. **No staging environment.** Every proposed production change currently has to be validated on a developer laptop or the demo compose, which does not mirror production topology (Caddy TLS, DNS, real Resend sending, live EONET ingest).

Without these, v1.0 cannot be tagged with confidence: there is no way to validate a release before it reaches users, no way to find out when ingestion breaks except by manual inspection, and no documented promotion path from code commit to live URL.

## What

Deliver a two-environment topology on a single VPS, an automated deploy pipeline driven by branches and tags, and a wired-up Resend alerting system.

### In scope

- **Hosting**: single Hetzner CX22 (€4.51/mo) running two isolated Docker Compose stacks — `staging` and `production` — behind one Caddy instance with automatic TLS via Let's Encrypt.
- **Domains**:
  - Production: `vigilafrica.org` (Vercel frontend), `api.vigilafrica.org` (VPS API)
  - Staging: `staging.vigilafrica.org` (Vercel frontend), `api.staging.vigilafrica.org` (VPS API)
- **Branching model**:
  - `development` — integration branch (existing, continues as daily PR target)
  - `main` — staging mirror; merge auto-deploys to staging
  - `release` — production mirror; merges stage code, annotated tag `vX.Y.Z` triggers production deploy
- **Deploy automation** via GitHub Actions + GitHub Environments:
  - `staging` environment: runs on push to `main`, SSH-deploys to VPS staging stack
  - `production` environment: runs on push of `v*` tag, requires manual reviewer approval, SSH-deploys to VPS prod stack
- **Tagging**: annotated SemVer tags on `release` branch; tag name becomes the `/health` `version` field via `docker build --build-arg VERSION`.
- **Resend alerting** (from zero): account creation, `vigilafrica.org` sending-domain verification (SPF/DKIM/DMARC), two API keys (one per env), wired into existing `alert` package to satisfy ADR-011:
  - Failed-ingestion alert on every failed `ingestion_runs` row
  - Staleness watchdog goroutine — alerts if no successful ingestion in `ALERT_STALENESS_THRESHOLD_HOURS` (default 2h)
- **Secrets**: GitHub Environments hold CI-side secrets (SSH key, host); runtime secrets (`RESEND_API_KEY`, `DATABASE_URL`, etc.) live in `/opt/vigilafrica/{env}/.env` on the VPS, `chmod 600`, root-owned. Master copies kept in the maintainer's password manager.
- **Documentation refresh** across 7 files (see spec).
- **Hosted demo URL wiring** — `DEMO.md` placeholder from v0.8 resolves to `staging.vigilafrica.org` (demo seed data path).

### Out of scope

- Multi-maintainer secrets management (SOPS, Doppler, HashiCorp Vault) — revisit when project onboards a co-maintainer.
- Blue-green / canary deployments — rollback is "deploy previous tag" for v1.0.
- Horizontal scaling, CDN in front of API, managed Postgres — not needed at current traffic.
- Uptime monitoring (UptimeRobot, BetterStack) — addressed in a follow-up change; this proposal delivers the alerting *for ingestion health*, not *for external reachability*.
- Full security audit of the VPS (intrusion detection, SIEM) — baseline hardening only (ufw, fail2ban, SSH key auth, unattended-upgrades).

### Analysis considered and rejected

- **Railway**: managed Postgres lacks PostGIS; running Postgres as a custom container defeats the managed-DB benefit and costs ~5× more than a VPS for equivalent capability.
- **Supabase (DB) + VPS (API)**: native PostGIS is attractive, but splitting DB and API across vendors introduces egress fees, latency on chatty bulk upserts from the ingest loop, two dashboards, and breaks the unified `docker-compose.*.yml` story contributors rely on.
- **Fly.io**: similar tradeoffs to Railway; no decisive advantage.
- **Branch-push-triggered production deploy**: rejected in favor of tag-triggered — lets maintainer merge to `release` without shipping immediately, and the tag becomes the deployed version identifier.

## Success Signals

- A commit merged to `main` is visible on `staging.vigilafrica.org` within 5 minutes, with a matching `/health` version reflecting the commit SHA.
- A tag `vX.Y.Z` pushed to `release`, after maintainer approval in GitHub, is visible on `vigilafrica.org` with `/health` reporting `version: vX.Y.Z`.
- Forcing a failed EONET ingestion (e.g., bogus API URL in staging) generates a Resend email to the maintainer within one ingestion cycle.
- The maintainer can roll back production by pushing an older tag (or re-deploying the prior tag via the GitHub Actions workflow `workflow_dispatch`), with no manual SSH required for the rollback itself.
- Contributor docs (README, CONTRIBUTING, DEMO) correctly describe the new branching + tagging + environment model; no contributor asks "which branch do I target?" after reading them.
