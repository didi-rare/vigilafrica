# feature-alert-multiple-recipients

**Branch:** `feat/feature-deploy-staging-prod-alerts`
**Proposal:** `openspec/proposals/feature-alert-multiple-recipients.md`
**Spec:** `openspec/specs/feature-alert-multiple-recipients.md`

## 1. Configuration

- [x] 1.1 Confirm active OpenSpec docs and Go standards
- [x] 1.2 Add comma-separated alert recipient parsing
- [x] 1.3 Keep `ALERT_EMAIL_TO` fallback for existing deployments

## 2. Resend Delivery

- [x] 2.1 Store parsed recipients as a slice in alert config
- [x] 2.2 Send all recipients in Resend `to` payload
- [x] 2.3 Keep missing recipients as a graceful no-op

## 3. Documentation

- [x] 3.1 Update `.env.example` with placeholder-only `ALERTS_TO`
- [x] 3.2 Update compose files to pass `ALERTS_TO`
- [x] 3.3 Update deployment docs with VPS-only runtime `.env` guidance

## 4. Verification

- [x] 4.1 Run alert package tests
- [x] 4.2 Run server package tests
- [x] 4.3 Validate compose configs
- [x] 4.4 Run OpenSpec validation
