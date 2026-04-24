# Spec: Multiple Alert Recipients (feature-alert-multiple-recipients)

## Context

VigilAfrica sends operational ingestion alerts through Resend from
`api/internal/alert`. The current config shape exposes a single `ToEmail` value
loaded from `ALERT_EMAIL_TO`, and the Resend payload wraps that value in a
one-item `to` array. Resend supports multiple recipients, so the API should parse
and pass a recipient slice instead.

Runtime secrets and recipient lists must not be committed. The repository may
document variable names and placeholder examples in `.env.example`, while real
values are configured in ignored `.env` files:

- Local development: `<repo>/.env`, ignored by `.gitignore`
- Staging VPS: `/opt/vigilafrica/staging/.env`, deploy-owned `0600`
- Production VPS: `/opt/vigilafrica/production/.env`, deploy-owned `0600`

## Components to Touch

1. `api/internal/alert/resend.go` — replace single-recipient config handling
   with a parsed recipient slice.
2. `api/internal/alert/resend_test.go` — cover comma-separated parsing, empty
   entries, missing recipients, and Resend payload shape.
3. `api/cmd/server/main.go` — load preferred `ALERTS_TO`, falling back to
   `ALERT_EMAIL_TO` for compatibility.
4. `.env.example` — document `ALERTS_TO` with placeholder recipients only.
5. `docker-compose.prod.yml` and `docker-compose.staging.yml` — pass `ALERTS_TO`
   through to the API container while preserving `ALERT_EMAIL_TO` during
   migration if needed.
6. `docs/deployment/vps.md`, `docs/deployment/resend-setup.md`, and
   `docs/deployment/staging-production-topology.md` — document comma-separated
   recipients and the non-committed runtime `.env` location.

## Implementation Plan

1. Add a helper such as `ParseRecipients(value string) []string` that:
   - splits on commas,
   - trims whitespace,
   - removes empty entries,
   - preserves recipient order,
   - does not log full recipient values beyond existing operational logs.
2. Change `alert.Config` to hold `ToEmails []string` or equivalent.
3. Make `Client.Enabled()` require `RESEND_API_KEY` plus at least one parsed
   recipient.
4. Marshal Resend payloads with `"to": []string{...all recipients...}`.
5. Load config from `ALERTS_TO` first, then `ALERT_EMAIL_TO` if `ALERTS_TO` is
   empty.
6. Update docs and examples to use placeholders:

```env
ALERTS_TO=ops@example.com,maintainer@example.com
```

7. Make clear that real values are set on the VPS in
   `/opt/vigilafrica/<environment>/.env`, not in committed files.

## Acceptance Criteria

- `ALERTS_TO=ops@example.com,maintainer@example.com` sends one Resend request
  whose `to` field contains both addresses.
- Whitespace and empty entries are handled gracefully, for example
  `ALERTS_TO=" ops@example.com, , maintainer@example.com "` resolves to two
  recipients.
- If `ALERTS_TO` is empty and `ALERT_EMAIL_TO` is set, existing deployments keep
  working.
- If both recipient variables are empty, alerting logs a warning and skips
  delivery without crashing.
- `.env.example` and docs contain only placeholder addresses, not real
  operational recipient addresses.
- Deployment docs explain that real values are edited directly in the ignored
  VPS runtime `.env` files.

## Verification Plan

1. `cd api && go test ./internal/alert/...`
2. `cd api && go test ./cmd/server/...`
3. `docker compose -f docker-compose.staging.yml config --quiet`
4. `docker compose -f docker-compose.prod.yml config --quiet`
5. `npm run spec:validate`
