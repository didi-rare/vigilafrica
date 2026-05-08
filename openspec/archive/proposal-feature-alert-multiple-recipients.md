# Proposal: Multiple Alert Recipients (feature-alert-multiple-recipients)

## Why

The deployment alerting path currently accepts one recipient via `ALERT_EMAIL_TO`.
Operationally, ingestion failure and staleness alerts may need to reach more than
one maintainer inbox without hardcoding any addresses in the public repository.

## What Changes

- Add `ALERTS_TO` as the preferred runtime environment variable for alert
  recipients.
- Parse `ALERTS_TO` as a comma-separated list, trimming whitespace and ignoring
  empty entries.
- Keep `ALERT_EMAIL_TO` as a backward-compatible alias during migration.
- Send Resend payloads with all parsed recipients in the `to` array.
- Update deployment docs to show placeholder examples only and explain that real
  recipient values belong in ignored local `.env` files or VPS runtime `.env`
  files, never committed to the repo.

## Out of Scope

- Mailing-list or group management outside Resend.
- Per-alert routing rules, escalation policies, or PagerDuty-style schedules.
- Storing recipients in GitHub Actions secrets for the deployed API; the VPS
  runtime `.env` remains the source of runtime alert configuration.

## User Impact

Operators can set one or many alert recipients without changing code:

```env
ALERTS_TO=ops@example.com,maintainer@example.com
```

If no recipients are configured, alert delivery remains disabled gracefully for
local and development environments.
