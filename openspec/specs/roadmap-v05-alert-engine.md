---
change_id: roadmap-v05-alert-engine
status: spec
created_date: 2026-04-15
author: Claude Code
---

# Spec: v0.5 · Alert Engine

## Objective

Build an alert subscription engine that dispatches email and webhook notifications to
registered users when VigilAfrica detects a new flood or wildfire event in their subscribed
state. The engine integrates with the existing NASA EONET polling pipeline and Nigerian
state localisation layer.

---

## System Design

### Data Model (PostgreSQL)

```sql
-- Subscriptions table
CREATE TABLE subscriptions (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email         TEXT,                          -- nullable if webhook-only
  webhook_url   TEXT,                          -- nullable if email-only
  state_filter  TEXT NOT NULL,                 -- e.g. "Benue State" | "ALL"
  event_filter  TEXT NOT NULL DEFAULT 'ALL',   -- "FLOOD" | "WILDFIRE" | "ALL"
  verified      BOOLEAN NOT NULL DEFAULT false,
  verify_token  TEXT,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  cancelled_at  TIMESTAMPTZ
);

-- Alert dispatch log (idempotency + audit)
CREATE TABLE alert_dispatches (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  subscription_id UUID NOT NULL REFERENCES subscriptions(id),
  event_id        TEXT NOT NULL,               -- EONET event ID
  channel         TEXT NOT NULL,               -- "email" | "webhook"
  dispatched_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  status          TEXT NOT NULL,               -- "sent" | "failed"
  UNIQUE (subscription_id, event_id)           -- prevent duplicate alerts
);
```

### Go API — New Package: `api/internal/alerts/`

```
api/internal/alerts/
  handler.go       — HTTP handlers for POST/GET/DELETE /api/v1/subscriptions
  service.go       — Business logic: subscription CRUD, dispatch matching
  dispatcher.go    — Email + webhook delivery (interface, two impls)
  email.go         — Resend (or SendGrid) email delivery implementation
  webhook.go       — HTTP POST delivery implementation
  templates/
    alert_email.html  — HTML email template for event alerts
    verify_email.html — HTML email for subscription verification
```

### Dispatcher Interface

```go
type Dispatcher interface {
    Send(ctx context.Context, sub Subscription, event Event) error
}
```

Two implementations: `EmailDispatcher` (Resend API) and `WebhookDispatcher` (HTTP POST).
Both are selected at service layer based on subscription fields.

### Alert Matching — integrated into existing poller

In `api/internal/poller/` (existing), after a new event is stored:

```go
// After event.Save()
matches, err := alerts.FindMatchingSubscriptions(ctx, db, event)
for _, sub := range matches {
    dispatcher.Send(ctx, sub, event)
}
```

Matching logic:
- `sub.state_filter == "ALL"` OR `sub.state_filter == event.State`
- `sub.event_filter == "ALL"` OR `sub.event_filter == event.Type`
- `sub.verified == true`
- `sub.cancelled_at IS NULL`
- No existing `alert_dispatches` row for `(sub.id, event.id)` (idempotency)

### REST Handlers

**POST `/api/v1/subscriptions`**
```json
// Request
{ "email": "user@example.com", "state": "Benue State", "event_type": "FLOOD" }

// Response 201
{ "id": "uuid", "message": "Check your email to confirm your subscription." }
```

**GET `/api/v1/subscriptions/:id`** — returns subscription metadata (no PII)

**DELETE `/api/v1/subscriptions/:id`** — soft-delete (sets `cancelled_at`)

**POST `/api/v1/subscriptions/:id/verify?token=...`** — sets `verified = true`

### Frontend — New Component: `web/src/components/AlertSubscribe.tsx`

Embedded after `<EventsDashboard />` in `App.tsx`:

```tsx
<AlertSubscribe />
```

Form fields:
- State dropdown (all 36 Nigerian states + FCT + "All Nigeria")
- Event type radio: Floods / Wildfires / Both
- Email input (required for email delivery)
- Optional: Webhook URL toggle

Success state: "Check your inbox for a confirmation link."
Error state: inline field validation, server error message.

### Email Templates

**Verification email subject:** `Confirm your VigilAfrica alert for [State]`
**Alert email subject:** `🌊 Flood detected in [State], Nigeria` / `🔥 Wildfire detected in [State], Nigeria`

Alert email body:
- Event title + localised state name
- Event category + date detected
- Link to VigilAfrica map centred on event coordinates
- Unsubscribe link: `DELETE /api/v1/subscriptions/:id`

---

## Environment Variables (new)

| Variable | Description |
|---|---|
| `RESEND_API_KEY` | Resend email delivery API key |
| `ALERT_FROM_EMAIL` | Sender address (e.g. `alerts@vigilafrica.com`) |
| `APP_BASE_URL` | Base URL for links in emails (e.g. `https://vigilafrica.com`) |

---

## CI/CD Impact

- No new workflow files required
- `go test ./...` in `ci-cd.yml` will cover the new `alerts` package
- New migration SQL file committed to `api/db/migrations/`
- `DATABASE_URL` env var must be set in production Vercel environment (already required by v0.4)

---

## Acceptance Criteria

### API
- [ ] `POST /api/v1/subscriptions` creates a subscription and sends a verification email
- [ ] `POST /api/v1/subscriptions/:id/verify` sets `verified = true`
- [ ] `DELETE /api/v1/subscriptions/:id` soft-deletes the subscription
- [ ] Verified subscribers receive an alert email when a matching event is detected
- [ ] Duplicate alerts for the same `(subscription_id, event_id)` are prevented
- [ ] Unverified subscriptions receive no alerts
- [ ] Cancelled subscriptions receive no alerts
- [ ] Webhook subscribers receive a POST to their registered URL with event JSON

### Frontend
- [ ] Alert subscription form renders on the landing page below the events dashboard
- [ ] State dropdown includes all 36 states + FCT + "All Nigeria"
- [ ] Form submits successfully and shows confirmation message
- [ ] Form shows inline validation errors (invalid email, empty state)
- [ ] Form is responsive at 375px and 1280px

### Database
- [ ] `subscriptions` table created via migration
- [ ] `alert_dispatches` table created with unique constraint on `(subscription_id, event_id)`

### Observability
- [ ] Each alert dispatch logged to `alert_dispatches` with status
- [ ] Failed dispatches logged with error reason (not silently dropped)

---

## Out of Scope

- SMS / USSD delivery
- Subscription management UI (list / edit subscriptions)
- Multi-country events
- Rate limiting or paid tiers
- Push notifications (browser)
