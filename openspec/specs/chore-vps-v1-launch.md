---
id: chore-vps-v1-launch
status: in-progress
branch: feat/v1.0-quality-gate
---

# Spec: VPS Staging + Production Launch (chore-vps-v1-launch)

## Context

All code milestones (v0.1–v0.8) are complete. The CI/CD workflows (`deploy-staging.yml`, `deploy-production.yml`), Docker Compose configs (`docker-compose.staging.yml`, `docker-compose.prod.yml`), Caddy config (`deploy/Caddyfile.example`), and provision script (`deploy/provision.sh`) are committed and ready. This spec tracks the operational steps required to clear the v1.0 quality gate.

### Additional fixes made during this work

The following issues were discovered during execution and resolved on `fix/staging-docs-caddy` (merged to `development`):

- **Caddyfile log blocks removed** — `deploy/Caddyfile.example` had `log { output file ... }` blocks that required `/var/log/caddy/` to exist before Caddy could start. Blocks removed; Caddy logs to journald instead.
- **OpenAPI spec embedded in binary** — `/docs` was broken in Docker because the spec file was outside the build context. Fixed with `//go:embed openapi.yaml` in `api/internal/handlers/docs.go`.
- **OpenAPI spec version bumped to 1.0.0** — was `0.7.0`.
- **Staging server URL corrected** — was `api-staging.vigilafrica.org`, corrected to `api.staging.vigilafrica.org`.
- **`npm run sync:openapi` script added** — prevents the source-of-truth spec (`openspec/specs/vigilafrica/openapi.yaml`) drifting from the embedded copy (`api/internal/handlers/openapi.yaml`). CI gate added to `ci-cd.yml`.

---

## Phase 1 — GitHub Configuration ✅ Complete

**1.1 Create GitHub Environments**

| Environment | Protection rule |
|---|---|
| `staging` | None (auto-deploys on push to `main`) |
| `production` | Required reviewer: `@didi-rare` |

**1.2 Add Secrets to each Environment**

| Secret | Value |
|---|---|
| `VPS_SSH_KEY` | Private key for the `deploy` user |
| `VPS_HOST` | `178.104.104.122` |
| `VPS_USER` | `deploy` |

---

## Phase 2 — VPS Provisioning ✅ Complete

Hetzner CX22, Nuremberg, Ubuntu 22.04 LTS. Provisioned via:

```bash
git clone https://github.com/didi-rare/vigilafrica.git /tmp/vigilafrica
cd /tmp/vigilafrica
SSH_PUBLIC_KEY='ssh-ed25519 ...' bash deploy/provision.sh
```

The script installed Docker, Caddy, ufw, fail2ban, created the `deploy` user, and prepared `/opt/vigilafrica/staging` and `/opt/vigilafrica/production`.

**2.1 Staging `.env`** — written to `/opt/vigilafrica/staging/.env` (mode 600, owned by `deploy`):

```env
POSTGRES_USER=vigilafrica
POSTGRES_PASSWORD=<redacted>
POSTGRES_DB=vigilafrica
CORS_ORIGIN=https://staging.vigilafrica.org
LOG_LEVEL=info
INGEST_INTERVAL_MIN=60
RATE_LIMIT_RPM=60
CACHE_TTL_SECONDS=300
RESEND_API_KEY=re_...
ALERT_FROM_EMAIL=VigilAfrica Alerts <alerts@send.vigilafrica.org>
ALERTS_TO=ops@example.com
ALERT_STALENESS_THRESHOLD_HOURS=2
ALERT_STALENESS_CHECK_INTERVAL_MIN=15
```

**2.2 Production `.env`** — same structure, `CORS_ORIGIN=https://vigilafrica.org`, separate `POSTGRES_PASSWORD`.

**2.3 Caddy** — `deploy/Caddyfile.example` installed as `/etc/caddy/Caddyfile`. Log blocks removed (see additional fixes above). Caddy active and running.

**DNS A records added in Namecheap:**

| Host | Value |
|---|---|
| `api` | `178.104.104.122` |
| `api.staging` | `178.104.104.122` |

---

## Phase 3 — Staging Deployment ✅ Complete

**3.1 Merge development → main** ✅ — `Deploy Staging` passed in 17s. Commit `ffe0bb7`.

**3.2 Smoke test API** ✅ — `https://api.staging.vigilafrica.org/health` returns:
```json
{"status":"ok","version":"ffe0bb7","last_ingestion":{"country_code":"GH","status":"success",...}}
```

**3.3 Set up Vercel staging project** ✅

The frontend staging deployment is separate from the API and runs on Vercel.

1. Create a new Vercel project linked to this repository, targeting the `main` branch.
2. Set the following environment variable in the Vercel staging project:
   ```
   VITE_API_BASE_URL=https://api.staging.vigilafrica.org
   ```
3. Add the custom domain `staging.vigilafrica.org` in the Vercel project settings.
4. **DNS records (CNAME for `staging`, apex for production) are owned by `fix-staging-soft-launch` §8** — operator action tracked there.
5. Verified 2026-04-26: `https://staging.vigilafrica.org` loads the frontend and renders events from `https://api.staging.vigilafrica.org` end-to-end.

**3.4 Verify Resend failure alert** ✅

1. SSH into VPS, temporarily break the EONET endpoint in staging `.env`.
2. Set `INGEST_INTERVAL_MIN=1` temporarily.
3. Restart: `docker compose -f docker-compose.staging.yml up -d`
4. Wait one cycle. Confirm email arrives at the configured staging recipient.
5. Restore correct env and restart.
6. Verified in staging; see `openspec/specs/vigilafrica/roadmap.md` v1.0 quality gate.

**3.5 Verify Resend staleness alert** ✅

1. Set `ALERT_STALENESS_THRESHOLD_HOURS=1` and `ALERT_STALENESS_CHECK_INTERVAL_MIN=1`.
2. Stop ingestion long enough to exceed the threshold.
3. Confirm exactly one staleness email arrives (deduplication check).
4. Restore normal values and restart.
5. Verified in staging; see `openspec/specs/vigilafrica/roadmap.md` v1.0 quality gate.

---

## Phase 4 — Rollback Verification ⏳

After v1.0.0 is tagged and deployed, verify the rollback path by redeploying the same tag via `workflow_dispatch`:

```
Actions → Deploy Production → Run workflow → tag = v1.0.0
```

Confirm the workflow completes cleanly and `https://api.vigilafrica.org/health` still reports `"version":"v1.0.0"`. This exercises the full rollback mechanism before any real rollback is ever needed.

---

## Phase 5 — Production Deployment ⏳

**5.1 Merge main → release**

Open a PR from `main` to `release`. Merge after staging is fully verified (Phases 3.3, 3.4 complete).

**5.2 Tag v1.0.0**

```bash
git checkout release
git pull --ff-only origin release
git tag -a v1.0.0 -m "Release v1.0.0 — Credible public launch"
git push origin v1.0.0
```

**5.3 Approve the production Environment gate**

The `Deploy Production` workflow pauses for reviewer approval. Approve in GitHub Actions.

**5.4 Smoke test production**

```bash
curl https://api.vigilafrica.org/health | jq .
```

Expected: `{"status":"ok","version":"v1.0.0", ...}`

---

## Phase 6 — Closeout ⏳

- [ ] Update `openspec/specs/vigilafrica/roadmap.md` — mark v1.0 complete with delivery date.
- [ ] Update `openspec/specs/vigilafrica/decisions.md` if any deployment decisions deviate from ADR-011.
- [ ] Archive this spec to `openspec/archive/spec-chore-vps-v1-launch.md`.

---

## Acceptance Criteria

- [x] `staging` and `production` GitHub Environments exist with correct secrets
- [x] `deploy/provision.sh` executed on the VPS without errors
- [x] `https://api.staging.vigilafrica.org/health` returns `status: ok` and correct commit SHA
- [x] Vercel staging project live at `https://staging.vigilafrica.org` with `VITE_API_BASE_URL` pointing to staging API (DNS sign-off lives in `fix-staging-soft-launch` §8)
- [x] Resend failure alert email received on staging
- [x] Resend staleness alert email received on staging (exactly once)
- [ ] Rollback workflow exercised via `workflow_dispatch` after v1.0.0 tag
- [ ] `https://api.vigilafrica.org/health` returns `status: ok` and `version: v1.0.0`
- [ ] v1.0 marked complete in `roadmap.md`

## Verification Plan

All acceptance criteria have an observable output (HTTP response, email in inbox, GitHub Actions pass/fail). No automated test additions are required — the `Deploy Staging` smoke test in `deploy-staging.yml` is the automated gate.
