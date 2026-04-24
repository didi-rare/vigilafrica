# feature-deploy-staging-prod-alerts

**Branch:** `feat/feature-deploy-staging-prod-alerts`
**Status:** Archived on 2026-04-24
**Proposal:** `openspec/archive/proposal-feature-deploy-staging-prod-alerts.md`
**Spec:** `openspec/archive/spec-feature-deploy-staging-prod-alerts.md`

## 1. Stale Cleanup

- [x] 1.1 Replace stale v0.8 pre-demo checklist with this feature-specific OpenSpec apply checklist
- [x] 1.2 Confirm active OpenSpec docs and implementation branch are aligned

## 2. Resend Alerting

- [x] 2.1 Move Resend delivery into `api/internal/alert`
- [x] 2.2 Keep alert configuration explicit and documented in `.env.example`
- [x] 2.3 Send failed-ingestion alerts from scheduled ingestion failures
- [x] 2.4 Start a configurable staleness watchdog from the API server
- [x] 2.5 Add unit tests for Resend delivery and watchdog decision logic

## 3. Deployment Topology

- [x] 3.1 Add production and staging Docker Compose files with distinct ports and volumes
- [x] 3.2 Add a Caddyfile example for `api.vigilafrica.org` and `api.staging.vigilafrica.org`
- [x] 3.3 Add an idempotent VPS provisioning script
- [x] 3.4 Stamp API `/health.version` from Docker build args

## 4. GitHub Actions

- [x] 4.1 Keep CI focused on build/test for `development`, `main`, and `release`
- [x] 4.2 Add staging deploy workflow for pushes to `main`
- [x] 4.3 Add gated production deploy workflow for SemVer tags and rollback dispatch

## 5. Documentation

- [x] 5.1 Refresh `docs/deployment/vps.md` for two-stack VPS topology
- [x] 5.2 Add release-process, Resend setup, and topology deployment docs
- [x] 5.3 Update README, CONTRIBUTING, DEMO, and web README for environments and branch flow
- [x] 5.4 Update roadmap, architecture, and decisions with the v1.0 deployment model

## 6. Verification

- [x] 6.1 Run Go alert unit tests
- [x] 6.2 Run broader Go tests
- [x] 6.3 Run OpenSpec validation
- [x] 6.4 Note any environment-only verification that still requires live VPS, DNS, Resend, Vercel, or GitHub Environment setup

Environment-only checks still required after merge/deploy:

- Configure live DNS and Caddy on the VPS
- Configure GitHub `staging` and `production` Environments with deploy secrets
- Configure Vercel staging/production projects and `VITE_API_BASE_URL`
- Verify a staging failed-ingestion email with real Resend credentials
- Verify staleness alerting and production rollback against live infrastructure
