---
id: chore-alert-env-label
status: proposed
branch: tbd
---

# Proposal: Environment Label in Alert Email Subjects (chore-alert-env-label)

## Why

Alert subjects in [api/internal/alert/resend.go:118](api/internal/alert/resend.go#L118) and [resend.go:148](api/internal/alert/resend.go#L148) are identical between staging and production:

```
[VigilAfrica] Ingestion failed for NG at 2026-05-11T10:00Z
[VigilAfrica] No successful ingestion in 4 hours
```

When a staging incident pages, the recipient can't distinguish it from a production page without opening the email and reading the VPS-check hint. Staging noise blends with production noise — which defeats the alert.

## What Changes

1. Add an `Environment` field to `alert.Config` (sourced from a new `ENVIRONMENT=staging|production` env var, defaulted to `unknown` if missing)
2. Prefix both alert subjects with `[VigilAfrica:<env>]`:
   - `[VigilAfrica:staging] Ingestion failed for NG at ...`
   - `[VigilAfrica:production] No successful ingestion in 4 hours`
3. Thread the env var through `docker-compose.staging.yml` and `docker-compose.prod.yml` (deploy/ scripts also need it if they bake env at provision time)
4. Update [api/internal/alert/resend_test.go:55](api/internal/alert/resend_test.go#L55) to assert the new prefix
5. Document `ENVIRONMENT` in `.env.example`

## Out of Scope

- Routing different environments to different recipient lists (the existing `ALERT_TO` env var already supports per-deployment overrides)
- Slack / SMS integration
- Severity levels in the subject

## Origin

Captured in memory as `project_ingestion_alerting_backlog.md` after a 2026-05-11 staging incident where the user could not tell at a glance whether the page was staging or production.
