# Resend Alert Setup

VigilAfrica uses Resend for operational email alerts from the API. Alerts cover failed ingestion runs and data staleness.

## Account Setup

1. Create a Resend account.
2. Add `vigilafrica.org` as a sending domain.
3. Add the DNS records Resend provides:
   - SPF/TXT
   - DKIM/TXT records
   - DMARC/TXT, for example `_dmarc.vigilafrica.org`
4. Wait for Resend to mark the domain as verified.

## API Keys

Create two sending-only API keys:

| Key name | Environment |
|---|---|
| `vigilafrica-staging` | `/opt/vigilafrica/staging/.env` |
| `vigilafrica-production` | `/opt/vigilafrica/production/.env` |

Store master copies in the maintainer password manager. Do not commit them to the repository.

## Required Variables

```env
RESEND_API_KEY=re_...
ALERT_FROM_EMAIL=VigilAfrica Alerts <alerts@vigilafrica.org>
ALERTS_TO=ops@example.com,maintainer@example.com
ALERT_STALENESS_THRESHOLD_HOURS=2
ALERT_STALENESS_CHECK_INTERVAL_MIN=15
```

`ALERTS_TO` is comma-separated and may contain one or more recipients. Keep real
addresses only in ignored runtime env files:

- `/opt/vigilafrica/staging/.env`
- `/opt/vigilafrica/production/.env`
- local `.env` files for developer testing

`ALERT_EMAIL_TO` remains a single-recipient compatibility fallback for existing
deployments, but new deployments should use `ALERTS_TO`.

If `RESEND_API_KEY` and all recipient variables are missing, the API logs a
warning and skips email delivery. Local development remains unaffected.

## Verification

Staging failed-ingestion test:

1. Temporarily set an invalid EONET endpoint or block outbound EONET access in staging.
2. Restart the staging API stack.
3. Wait for one ingestion cycle.
4. Confirm an email arrives and `/health.status` reports `degraded`.

Staleness test:

1. Set `ALERT_STALENESS_THRESHOLD_HOURS=1` and `ALERT_STALENESS_CHECK_INTERVAL_MIN=1` in staging.
2. Stop successful ingestion long enough to exceed the threshold.
3. Confirm a staleness email arrives once for the stale reference time.
4. Restore normal values.
