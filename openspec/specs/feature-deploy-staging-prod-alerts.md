# Spec: Staging + Production Deployment with Resend Alerting (feature-deploy-staging-prod-alerts)

## Context

Delivers the operational layer required to tag v1.0: two environments (staging, production) on a single VPS, automated deploys, Resend email alerting, and a documented release process.

Builds on pre-existing scaffolding:
- `docs/deployment/vps.md` — single-environment VPS walkthrough (to be rewritten for two-stack layout)
- ADR-011 — ingestion observability contract the alert system must satisfy
- `docker-compose.demo.yml` — stays as-is for local contributor use; not replaced
- `/health` endpoint with `last_ingestion` block — already emits the signal Resend alerting consumes

## Architecture

### Single-VPS two-stack layout

```
┌────────────────────────────── Hetzner CX22 (Ubuntu 24.04 LTS) ──────────────────────────────┐
│                                                                                              │
│  Caddy (host-installed, auto-TLS via Let's Encrypt, :80/:443 exposed)                        │
│    ├── api.staging.vigilafrica.org     → reverse_proxy 127.0.0.1:8081                        │
│    └── api.vigilafrica.org             → reverse_proxy 127.0.0.1:8080                        │
│                                                                                              │
│  /opt/vigilafrica/staging/                    /opt/vigilafrica/production/                   │
│    ├── docker-compose.yml (api:8081,            ├── docker-compose.yml (api:8080,            │
│    │   db with named volume vigil-staging-data) │   db with named volume vigil-prod-data)   │
│    ├── .env (RESEND key for staging,            ├── .env (RESEND key for prod,               │
│    │   ALERT_EMAIL_TO, CORS_ORIGIN, ...)        │   ALERT_EMAIL_TO, CORS_ORIGIN, ...)        │
│    └── (git clone, checked out to main)         └── (git clone, checked out to tag vX.Y.Z)   │
│                                                                                              │
│  Firewall (ufw): 22/tcp (SSH), 80/tcp, 443/tcp only                                          │
│  fail2ban: SSH brute-force protection                                                        │
│  unattended-upgrades: security patches                                                       │
│                                                                                              │
└──────────────────────────────────────────────────────────────────────────────────────────────┘

Vercel (Hobby tier, two projects):
  ├── Project "vigilafrica-staging"    → staging.vigilafrica.org ← auto-deploy from `main`
  └── Project "vigilafrica-production" → vigilafrica.org          ← auto-deploy from `release`

Resend (one account, two API keys):
  ├── Sending domain: vigilafrica.org (SPF + DKIM + DMARC verified)
  ├── Sender: alerts@vigilafrica.org
  ├── Key "vigilafrica-staging"    → used by staging stack
  └── Key "vigilafrica-production" → used by production stack
```

### Branching & release model

```
feature/*  ──PR──▶  development  ──PR──▶  main  ──PR──▶  release  ──tag──▶  vX.Y.Z
                    (integration)    (staging)     (prod stage)        (prod deploy, gated)
```

Rollback: push the previous tag through the production workflow (`workflow_dispatch` with tag input), or re-run the prior successful run.

Hotfix: branch off `release` → `hotfix/thing` → PR back to `release` (cherry-pick/merge to `main` + `development` after) → tag patch version.

## Components to Touch

### New files

1. `.github/workflows/deploy-staging.yml` — on push to `main`, SSH-deploy staging stack
2. `.github/workflows/deploy-production.yml` — on push of `v*` tag, gated by `production` environment approval, SSH-deploy production stack
3. `docker-compose.prod.yml` — production compose (base file committed; `.env` lives on VPS only). API bound to `127.0.0.1:8080`, PostGIS in internal network.
4. `docker-compose.staging.yml` — staging compose, identical to prod except API on `127.0.0.1:8081`, separate named volume, separate `.env` path.
5. `deploy/Caddyfile.example` — two-vhost Caddyfile for the VPS (checked into repo for reference; live file is at `/etc/caddy/Caddyfile` on VPS).
6. `deploy/provision.sh` — idempotent bootstrap script for the VPS (installs Docker, Caddy, ufw, fail2ban, unattended-upgrades, creates `/opt/vigilafrica/{staging,production}` dirs).
7. `api/internal/alert/resend.go` — Resend client wrapping `POST https://api.resend.com/emails`, respects `RESEND_API_KEY`, `ALERT_FROM_EMAIL`, `ALERT_EMAIL_TO`. No-ops if `RESEND_API_KEY` unset (dev environments).
8. `api/internal/alert/resend_test.go` — unit tests with `httptest.NewServer` stub.
9. `api/internal/alert/watchdog.go` — staleness watchdog goroutine started from `cmd/server/main.go`, polls `ingestion_runs` on `ALERT_STALENESS_CHECK_INTERVAL_MIN` (default 15 min), alerts when `NOW() - last_success > ALERT_STALENESS_THRESHOLD_HOURS`.
10. `api/internal/alert/watchdog_test.go` — uses fake clock + repository stub.
11. `docs/deployment/release-process.md` — end-to-end release walkthrough (tag, approval, rollback, hotfix).
12. `docs/deployment/resend-setup.md` — from-zero Resend account + domain setup.
13. `docs/deployment/staging-production-topology.md` — diagram + DNS record table + env-var comparison matrix.

### Modified files

14. `docs/deployment/vps.md` — rewrite for two-stack layout on single VPS (replaces single-env walkthrough).
15. `README.md` — add "Environments" section (staging + production URLs, status), update deploy section to link new docs, update branching model.
16. `CONTRIBUTING.md` — new branching model (`development` → `main` → `release`), tag protocol, hotfix flow, reference `docs/deployment/release-process.md`.
17. `DEMO.md` — clarify demo vs staging vs production distinction; resolve the v0.8 placeholder to `staging.vigilafrica.org`.
18. `web/README.md` — document Vercel env mapping (`main` → staging project, `release` → prod project) and `VITE_API_BASE_URL` per env.
19. `openspec/specs/vigilafrica/roadmap.md` — check off v0.5 items 177, 178, 182; add a note under v1.0 quality gate that staging validation is required before tagging.
20. `openspec/specs/vigilafrica/architecture.md` — add deployment topology diagram + environment table.
21. `openspec/specs/vigilafrica/decisions.md` — append ADR-012 "Single-VPS two-stack deployment model" with the Railway/Supabase rejection rationale from the proposal.
22. `api/cmd/server/main.go` — start the staleness watchdog goroutine alongside the scheduled ingest; wire failed-ingestion path to call `alert.SendIngestFailure`.
23. `api/internal/ingest/scheduler.go` (or equivalent) — after writing a failed `ingestion_runs` row, call `alert.SendIngestFailure(ctx, run)`.
24. `api/.env.example` — add `RESEND_API_KEY`, `ALERT_FROM_EMAIL`, `ALERT_EMAIL_TO`, `ALERT_STALENESS_THRESHOLD_HOURS`, `ALERT_STALENESS_CHECK_INTERVAL_MIN` with safe defaults and comments.
25. `api/Dockerfile` — add `--build-arg VERSION` and stamp it into a Go `-ldflags "-X main.version=$VERSION"` variable exposed via `/health`.

## Implementation Plan

### Phase 1 — Resend alerting (can be built + tested locally before any VPS exists)

1. Create `api/internal/alert` package per §1.1 (developers-go.md).
2. Implement `Client` with `SendIngestFailure(ctx, IngestionRun)` and `SendStalenessAlert(ctx, lastSuccessAt, thresholdHours)`:
   - Both build an HTML+text email via `html/template` and POST to `https://api.resend.com/emails`.
   - Use `context.Context` (§3), wrap errors with `fmt.Errorf("resend: %w", err)` (§4), log via `slog` (§8).
   - If `RESEND_API_KEY` is empty, log a warning and return `nil` — prevents dev/test environments from erroring.
3. Implement staleness watchdog goroutine:
   - Started from `main.go` with the root `context.Context`; cancels cleanly on shutdown (§7).
   - Ticker fires every `ALERT_STALENESS_CHECK_INTERVAL_MIN`; queries `SELECT MAX(completed_at) FROM ingestion_runs WHERE status = 'success'` via the repository pattern (§5.1).
   - Emits one alert per staleness event; de-dup state kept in-process (resets on restart). Document the de-dup limitation in code comment.
4. Wire `alert.SendIngestFailure` into the ingest scheduler's post-run failure path.
5. Write unit tests (§9): `httptest` stub for Resend, fake clock for watchdog, repository stub for `ingestion_runs` query.
6. Update `api/.env.example` and `docs/deployment/vps.md` env var reference (already has the Resend rows — verify still accurate).

### Phase 2 — VPS provisioning

7. Write `deploy/provision.sh` — idempotent bash: apt updates, Docker, Caddy, ufw/fail2ban rules, `unattended-upgrades`, creates `/opt/vigilafrica/{staging,production}`, creates `deploy` user with docker group and SSH key.
8. Manual (one-time): provision Hetzner CX22, run `provision.sh`, copy `.env` files from password manager into `/opt/vigilafrica/staging/.env` and `/opt/vigilafrica/production/.env` (chmod 600).
9. Install `deploy/Caddyfile.example` → `/etc/caddy/Caddyfile`, replacing placeholders; reload Caddy.
10. DNS setup (manual at registrar): A records for `vigilafrica.org`, `staging.vigilafrica.org`, `api.vigilafrica.org`, `api.staging.vigilafrica.org` + Vercel CNAMEs for the frontend subdomains.

### Phase 3 — Compose files

11. Create `docker-compose.prod.yml` based on the vps.md example — `postgis/postgis:15-3.4` for db, `./api` build for api, named volume, internal network, `127.0.0.1:8080` binding.
12. Create `docker-compose.staging.yml` — same but port `8081` and a distinct named volume `vigil-staging-data`.
13. Verify idempotency: `docker compose up -d` on each stack twice produces no errors, no data loss.
14. Update API Dockerfile for `VERSION` build arg; verify `/health` returns it.

### Phase 4 — Resend account setup

15. Create Resend account; add `vigilafrica.org` as sending domain.
16. Add MX/SPF/DKIM/DMARC records at the DNS registrar; wait for Resend verification.
17. Create two API keys scoped "sending access only": `vigilafrica-staging`, `vigilafrica-production`.
18. Record keys in password manager; insert into respective VPS `.env` files.

### Phase 5 — GitHub Actions

19. Create `production` and `staging` GitHub Environments:
    - Both: secrets `VPS_HOST`, `VPS_USER`, `VPS_SSH_KEY`.
    - `production`: add required reviewer (the maintainer).
20. Write `.github/workflows/deploy-staging.yml`:
    - Trigger: `push` to `main`.
    - Job steps: checkout, set up SSH agent, `ssh $USER@$HOST 'cd /opt/vigilafrica/staging && git fetch --all && git checkout main && git pull && docker compose -f docker-compose.staging.yml up -d --build --build-arg VERSION=$(git rev-parse --short HEAD) api'`.
    - Post-deploy smoke test: `curl -fsS https://api.staging.vigilafrica.org/health | jq -e '.status == "ok"'`.
21. Write `.github/workflows/deploy-production.yml`:
    - Trigger: `push` of tag matching `v*.*.*`; also `workflow_dispatch` accepting a tag input (for rollback).
    - Environment: `production` (manual approval required).
    - Steps: same as staging but uses the tag ref, checks out the tag, builds with `--build-arg VERSION=<tag>`, runs against `/opt/vigilafrica/production`.
22. Verify workflows end-to-end: merge a trivial change to `main` → staging auto-deploys; tag `v0.8.1` on `release` → requires approval, then prod deploys.

### Phase 6 — Vercel

23. Create two Vercel projects from the same repo.
24. Project "vigilafrica-staging": production branch `main`, domain `staging.vigilafrica.org`, env `VITE_API_BASE_URL=https://api.staging.vigilafrica.org`.
25. Project "vigilafrica-production": production branch `release`, domain `vigilafrica.org`, env `VITE_API_BASE_URL=https://api.vigilafrica.org`.
26. Verify each Vercel project's CORS origin matches the respective API's `CORS_ORIGIN` env var.

### Phase 7 — Documentation refresh

27. Rewrite `docs/deployment/vps.md` for two-stack layout.
28. Write `docs/deployment/release-process.md` (full flow: develop → main → release → tag → deploy → rollback → hotfix).
29. Write `docs/deployment/resend-setup.md` (from-zero walkthrough).
30. Write `docs/deployment/staging-production-topology.md` (diagram + DNS + env matrix).
31. Update `README.md` with "Environments" section and link deploy docs.
32. Update `CONTRIBUTING.md` with new branching model and tag protocol.
33. Update `DEMO.md` to distinguish demo (local compose) vs staging (`staging.vigilafrica.org`) vs production.
34. Update `web/README.md` with Vercel env mapping.
35. Update `openspec/specs/vigilafrica/roadmap.md` — check off §v0.5 items 177, 178, 182.
36. Update `openspec/specs/vigilafrica/architecture.md` with deployment diagram.
37. Append ADR-012 to `openspec/specs/vigilafrica/decisions.md`.

### Phase 8 — Validation

38. Tag `v0.9.0-rc.1` on `release`; verify gated prod deploy works end-to-end.
39. Force a failed ingest in staging (invalid EONET URL via env override) → confirm Resend email arrives.
40. Stop staging ingest for >2h in a test scenario (or temporarily lower `ALERT_STALENESS_THRESHOLD_HOURS=0.01`) → confirm watchdog emails arrive.
41. Verify rollback: `workflow_dispatch` with previous tag successfully redeploys.

## Acceptance Criteria

### Alerting (ADR-011 compliance)
- [ ] `RESEND_API_KEY`, `ALERT_FROM_EMAIL`, `ALERT_EMAIL_TO`, `ALERT_STALENESS_THRESHOLD_HOURS`, `ALERT_STALENESS_CHECK_INTERVAL_MIN` are documented in `.env.example` with safe defaults.
- [ ] A failed ingestion run triggers a Resend email within one ingestion cycle (verified in staging by injecting a failure).
- [ ] The staleness watchdog sends an alert when `NOW() - MAX(ingestion_runs.completed_at WHERE status='success') > threshold`.
- [ ] `RESEND_API_KEY` being empty logs a warning and does not crash the API (dev/local runs unaffected).
- [ ] Unit tests cover: happy path, Resend 5xx response, missing API key no-op, watchdog fires exactly once per staleness event.

### Deployment topology
- [ ] `docker-compose.prod.yml` + `docker-compose.staging.yml` exist, each idempotent (`up -d` twice = no errors).
- [ ] `/etc/caddy/Caddyfile` on the VPS serves both subdomains with valid Let's Encrypt certs.
- [ ] `ufw`, `fail2ban`, `unattended-upgrades` are active on the VPS; SSH root login disabled; SSH key auth only.
- [ ] Production and staging databases use distinct named volumes — destroying one does not affect the other.
- [ ] The API Dockerfile accepts `--build-arg VERSION` and `/health` returns that value.

### Deploy automation
- [ ] Push to `main` triggers staging deploy within 5 minutes; `/health` on staging reflects the commit SHA.
- [ ] Tag `v*.*.*` on `release` triggers a production workflow that **requires maintainer approval** before running.
- [ ] Production deploy on approval updates `api.vigilafrica.org/health` to report `version: vX.Y.Z` within 5 minutes of approval.
- [ ] `workflow_dispatch` on the production workflow accepts a tag input and redeploys that tag (rollback path).
- [ ] Post-deploy smoke test (`curl /health | jq -e '.status == "ok"'`) fails the workflow run if the API is unreachable or unhealthy.

### Frontend (Vercel)
- [ ] `staging.vigilafrica.org` serves the frontend built from `main`, with `VITE_API_BASE_URL` pointing at `api.staging.vigilafrica.org`.
- [ ] `vigilafrica.org` serves the frontend built from `release`, with `VITE_API_BASE_URL` pointing at `api.vigilafrica.org`.
- [ ] Browser console shows no CORS errors on either domain.

### Branching & tagging
- [ ] `main` and `release` branches both exist and are protected (no force-push, PR-only updates).
- [ ] Release process doc describes: promote `development`→`main`→`release`, then `git tag -a vX.Y.Z`, then `git push origin vX.Y.Z`.
- [ ] Hotfix process documented: branch off `release`, PR back, tag patch version, backport.

### Documentation
- [ ] All 7 docs listed in Components to Touch (§14–§21) updated.
- [ ] A new contributor can read `CONTRIBUTING.md` and correctly identify which branch to PR against without asking.
- [ ] `DEMO.md` no longer contains the `TBD — see project README once deployed` placeholder; resolves to `staging.vigilafrica.org`.
- [ ] ADR-012 recorded in `decisions.md` documenting the hosting choice + Railway/Supabase rejection rationale.

### Security
- [ ] No secrets committed to the repo (verified by grepping the diff for `re_`, `postgres://`, private-key headers).
- [ ] `.env` files on VPS are `chmod 600`, root-owned.
- [ ] GitHub Environment `production` has a required reviewer configured.
- [ ] Only the maintainer's SSH public key is in `~/.ssh/authorized_keys` on the VPS; password auth disabled in `sshd_config`.

## Verification Plan

1. **Phase 1 verification (alerting, local)**:
   - `cd api && go test ./internal/alert/...` — unit tests green.
   - Run API locally with `RESEND_API_KEY=re_...` and `ALERT_EMAIL_TO=maintainer@...`; simulate failure; confirm email arrives.

2. **Phase 2 verification (VPS)**:
   - `ssh deploy@vps 'sudo ufw status'` shows only 22, 80, 443 open.
   - `ssh deploy@vps 'docker ps'` shows both stacks running.
   - `curl -I https://api.staging.vigilafrica.org/health` and `curl -I https://api.vigilafrica.org/health` both return 200 + valid TLS cert.

3. **Phase 5 verification (CI)**:
   - Trivial docs-only commit to `main` triggers the staging workflow and passes the smoke test.
   - Tag `v0.9.0-rc.1` on `release` triggers production workflow; maintainer receives approval prompt in GitHub UI; approval proceeds to a successful deploy; `/health` reports `version: v0.9.0-rc.1`.

4. **End-to-end (v1.0 readiness)**:
   - Merge a feature through `development` → `main` → `release`, tag `v1.0.0-rc.1`, approve prod deploy.
   - Verify staleness alert by temporarily setting `ALERT_STALENESS_THRESHOLD_HOURS=0` in staging.
   - Verify rollback path by running `workflow_dispatch` with a prior tag and confirming `/health` reverts.

## Risks & Mitigations

| Risk | Mitigation |
|---|---|
| Single VPS = single point of failure | Accepted at v1.0 scale; documented in ADR-012; off-box `pg_dump` backups (existing cron in vps.md) cover data; rebuild-from-script path via `provision.sh`. |
| Resend free tier exhausted (100/day) | Failure-only alerts — realistic volume <5/day. De-dup state in watchdog prevents repeated fires for one incident. |
| SSH key compromise = full prod access | GitHub Environment SSH keys are separate from maintainer's personal key; revoke by removing from `authorized_keys`. |
| Tag pushed by mistake → prod deploy | Production environment requires manual approval click; tag push alone does not deploy. |
| `.env` on VPS drifts from documented example | `api/.env.example` is the source of truth; a quarterly diff check is a CONTRIBUTING.md reminder. |
| DNS misconfiguration blocks cert issuance | `deploy/provision.sh` verifies DNS resolves to VPS IP before reloading Caddy; Caddy logs cert issuance. |
