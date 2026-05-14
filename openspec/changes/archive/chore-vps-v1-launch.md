---
id: chore-vps-v1-launch
status: proposed
branch: feat/v1.0-quality-gate
---

# Proposal: VPS Staging + Production Launch (chore-vps-v1-launch)

## Why

All v0.x milestones are complete. The codebase, CI workflows, Docker Compose configs, Caddy config, and release process documentation are fully in place. The only remaining blocker for tagging v1.0.0 is getting the VPS provisioned, staging validated, and the production deploy gate exercised end-to-end. Until a real deployment exists, the v1.0 quality gate criteria around `/health.version`, Resend alert verification, and rollback cannot be signed off.

## What Changes

This is a deployment and configuration chore, not a code feature. The deliverables are:

1. **GitHub Secrets and Environments** — `staging` and `production` GitHub Environments created with the required secrets (`VPS_SSH_KEY`, `VPS_HOST`, `VPS_USER`) and protection rules (required reviewer on `production`).
2. **VPS provisioned** — `deploy/provision.sh` executed on the VPS; `/opt/vigilafrica/staging` and `/opt/vigilafrica/production` directories initialised with correct `.env` files including Resend vars.
3. **Staging deployed and smoke-tested** — `development → main` PR merged; `Deploy Staging` workflow passes; `https://api.staging.vigilafrica.org/health` reports the expected commit SHA.
4. **Resend alerts verified on staging** — both failure and staleness alert paths confirmed end-to-end.
5. **Rollback verified on staging** — `Deploy Production` manually triggered with a prior tag to confirm the workflow works before touching production.
6. **Production deployed** — `main → release` PR merged; `v1.0.0` annotated tag pushed; `production` Environment gate approved; `https://api.vigilafrica.org/health` reports `"version":"v1.0.0"`.
7. **Roadmap updated** — v1.0 marked complete in `openspec/specs/vigilafrica/roadmap.md`.

## Out of Scope

- No new API endpoints or frontend changes.
- No new countries or event categories.
- Secondary oracle (feature-secondary-oracle) remains deferred to v1.1+.
