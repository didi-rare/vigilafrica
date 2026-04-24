---
change_id: roadmap-v05-operational-prototype
status: proposal
created_date: 2026-04-15
author: Claude Code
---

# Proposal: v0.5 · Operational Prototype

## Context

VigilAfrica v0.4 delivers a Useful Prototype — floods and wildfires localized to Nigerian
states, visible on an interactive MapLibre map with GeoIP-based "near-me" context. The product
is functionally complete but operationally fragile: ingestion is manual, duplicates can
accumulate, and the system cannot run unattended.

v0.5 makes the prototype operational. After v0.5, VigilAfrica can run on a VPS without manual
intervention, a contributor can onboard from scratch in under 30 minutes, and the system is
ready for public exposure.

---

## What v0.5 Builds

### F-012 — Scheduled Ingestion

Replace the current manual ingestion trigger with a **gocron-based background scheduler**
that automatically polls NASA EONET at a configurable interval (default: every 60 minutes).

- No manual trigger needed after server start
- Interval configurable via `INGEST_INTERVAL_MIN` env var
- Structured JSON logs for every run: start, end, events fetched, events stored, errors

### F-013 — Deduplication

Replace the current insert-only approach with an **upsert on `source_id`**:

- Closed events are updated (status, geometry, timestamps) rather than re-inserted
- Running ingestion twice yields the same event count (idempotency guarantee)
- No phantom duplicates in the events table after repeated runs

### Operational Requirements (milestone blockers)

These are not feature-tagged but are required for v0.5 sign-off:

| Requirement | Detail |
|---|---|
| Structured JSON logging | All ingestion runs log start, end, events fetched/stored, errors |
| API rate limiting | Configurable via `RATE_LIMIT_RPM` env var; default: 60 req/min |
| Response caching | `GET /v1/events` cached 5–15 min (configurable TTL) |
| CORS configuration | `CORS_ORIGIN` env var; configured for Vercel production domain |
| VPS deployment docs | Caddy config example + Docker Compose production config |
| `CONTRIBUTING.md` | Contributor setup tested end-to-end; reproducible in < 30 min |
| Seed dataset | `api/db/seeds/sample_events_nigeria.sql` committed (no EONET needed locally) |
| `CODE_OF_CONDUCT.md` | Added to repo root |

---

## Why This Scope

v0.4 proved the concept works. v0.5 proves it can run. Without scheduled ingestion and
deduplication, every deployment degrades over time — data goes stale, duplicates accumulate,
and manual intervention is required. The operational requirements ensure any contributor can
reproduce the system reliably and that the API is safe to expose publicly.

The alert engine (email subscriptions, webhooks) is a strong future feature but is
explicitly deferred post-v1.0 per `roadmap.md` governance — it requires auth infrastructure,
rate limiting per subscriber, and SLA considerations that are out of scope for the MVP.

---

## Success Signal

> The prototype runs on a VPS, automatically ingests new events every hour, and a contributor
> can reproduce the full local environment in under 30 minutes by following `CONTRIBUTING.md`.

---

## Out of Scope for v0.5

- Alert subscriptions / email notifications (post-v1.0 per roadmap governance)
- Multi-country expansion (v0.6)
- Public API authentication / OAuth
- Mobile-native features
