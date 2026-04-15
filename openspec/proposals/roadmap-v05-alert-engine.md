---
change_id: roadmap-v05-alert-engine
status: proposal
created_date: 2026-04-15
author: Claude Code
---

# Proposal: v0.5 · Alert Engine

## Context

VigilAfrica v0.4 delivers the Useful Prototype — floods and wildfires localized to Nigerian
states, visible on an interactive MapLibre map with GeoIP-based "near-me" context.

The product currently shows events **on demand** (user visits the site, sees what's happening).
The next step is to push events **proactively** to people in affected areas — turning VigilAfrica
from a dashboard into an early-warning system.

---

## The Problem v0.5 Solves

| Persona | Current pain | With v0.5 |
|---|---|---|
| NGO Field Team | Must remember to check the site | Gets an alert when a flood hits their operational state |
| Local Journalist | Discovers events after the fact | Receives a webhook/email the moment an event is detected |
| Logistics Planner | Manually checks routes before dispatch | Automated alert if a wildfire or flood is detected on a key corridor |
| Civic Responder | Relies on word-of-mouth | Receives a verified NASA-sourced alert for their LGA |

---

## What v0.5 Builds

### Core: Alert Subscription Engine

Users register a **subscription**:
- **Location scope** — State (e.g. "Benue State") or Country (Nigeria)
- **Event type filter** — Floods, Wildfires, or All
- **Delivery channel** — Email (v0.5), Webhook URL (v0.5), SMS (v0.6+)
- **Threshold** — Notify on new events only, or also on event updates

### How It Works

```
NASA EONET poll (existing, every 15 min)
        ↓
Event normaliser + localiser (existing)
        ↓
  New event detected? ──→ Match against active subscriptions
                                      ↓
                            Dispatch alert (email / webhook)
                                      ↓
                            Log delivery in audit table
```

### API Surface

New endpoints added to the existing Go API:

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/v1/subscriptions` | Create a subscription |
| `GET`  | `/api/v1/subscriptions/:id` | Retrieve subscription |
| `DELETE` | `/api/v1/subscriptions/:id` | Cancel subscription |
| `POST` | `/api/v1/subscriptions/:id/verify` | Confirm email |

### Frontend

- Subscription form embedded on the landing page (after the EventsDashboard)
- "Notify me about events in [State ▾] — [Event type ▾]" form
- Confirmation email / webhook verification flow
- Unsubscribe link in every alert email

---

## Scope Options Considered

| Option | Fit | Verdict |
|---|---|---|
| Multi-country expansion (Ghana, Kenya) | High future impact, but data source coverage varies | v0.6 |
| Offline PWA / Service Worker cache | Critical for field use, but complex build | v0.6 |
| **Alert Engine (email + webhook)** | Highest immediate value for all 4 personas | ✅ v0.5 |
| Public API v1 (OAuth + rate limits) | Enables ecosystem, but needs auth infra first | v0.7 |
| Historical trend dashboard | Nice-to-have, low urgency | Backlog |
| Severity scoring | Useful but no upstream data for this yet | Backlog |

---

## Why Alert Engine is the Right v0.5

1. **Directly serves all 4 target personas** — every audience card on the landing page benefits
2. **Natural extension of Near-Me (v0.4)** — "you told us where you are, now we'll tell you when something happens near you"
3. **Drives retention** — the product stops requiring a visit to deliver value
4. **Nigeria-first focus preserved** — subscriptions are scoped to Nigerian states initially
5. **Achievable** — email dispatch (SendGrid/Resend) + webhook delivery is a well-understood stack

---

## Success Metrics

- Subscriptions created in first 30 days
- Alert delivery rate (target > 99%)
- Unsubscribe rate (target < 15%)
- Time from NASA event detection to alert dispatch (target < 5 min)

---

## Out of Scope for v0.5

- SMS delivery (requires USSD/SMS gateway — v0.6)
- Subscription management UI beyond create/cancel
- Multi-country events
- Paid tiers or rate limiting
