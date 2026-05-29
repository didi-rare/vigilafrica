---
id: chore-analytics-and-feedback
status: proposed
branch: tbd
---

# Proposal: Self-Hosted Analytics + 1-Click Feedback Widget (chore-analytics-and-feedback)

## Why

VigilAfrica has been in production since April 2026 with zero user-side instrumentation. No web analytics, no funnel data, no documented user feedback. Every product, partnership, and grant-application conversation currently relies on hand-waving about who uses the site and how. The 2026-05-27 business / market review flagged this as the single biggest information deficit in the project (risk R-02, "no usage data → no PMF signal").

This chore closes the gap with the lightest credible instrumentation:

1. **Self-hosted Umami** for traffic + custom-event analytics.
2. **A 1-click "Was this useful?" feedback widget** on the event detail page.

Both run on the existing VPS (Umami container added to the existing Docker Compose stack, sharing the existing Postgres instance with its own isolated database). No new paid services. Privacy posture matches the project's open / civic positioning — no cookies, no PII, no fingerprinting, no consent banner required in most jurisdictions.

The timing also lines up with the active anchor-partnership outreach (NRCS contact form + `info@redcrossnigeria.org` sent 2026-05-28; CfA outreach pending). Every partnership conversation from the 2026-06-04 follow-up onward should be able to cite real numbers, not assertions.

## What Changes

### Backend / Infrastructure

1. Add `umami` service to `docker-compose.yml` using `ghcr.io/umami-software/umami:postgresql-latest`. No exposed ports — Caddy handles ingress.
2. Add `analytics.vigilafrica.org` reverse-proxy block to the production Caddy config.
3. Create a `umami` database and a dedicated `umami` Postgres role inside the existing Postgres instance (one-time SQL setup, run on the VPS).
4. Add `UMAMI_DB_PASSWORD` and `UMAMI_APP_SECRET` to `.env.example` with placeholder values and clear comments. The real values live only in the VPS `.env`, which remains gitignored as it is today.
5. Add a DNS A record for `analytics.vigilafrica.org` pointing at the existing VPS IP.

### Frontend (React / TypeScript)

6. Add the Umami tracker script to `web/index.html` with `data-website-id` referencing the website registered in the Umami dashboard. The website-id is a public identifier, safe to commit.
7. Add a thin `web/src/analytics.ts` helper exposing typed `track(eventName, data?)` calls. Tolerates the tracker being absent (e.g. blocked by an ad-blocker) — `window.umami?.track(...)` pattern.
8. Wire six custom events (see "Events to track" below) at the existing event-handler call-sites in the React components.
9. Add a `<FeedbackPrompt />` component on the event detail page (`/events/:id`) — a single inline row reading "Was this useful?" with Yes / No buttons. On click, fire a `feedback_submitted` event with `{ value, event_id }` and (optionally) an open-text reason. Component must use existing design tokens (no hardcoded colours / spacing).

### Documentation

10. Add a `docs/operations/analytics.md` page documenting: how Umami is deployed, what events are captured, how to interpret the dashboard, and how to rotate the `UMAMI_DB_PASSWORD` / `UMAMI_APP_SECRET` if either is ever exposed.
11. Update [README.md](README.md) to mention privacy posture ("no cookies, no PII, self-hosted analytics") for transparency. One sentence.

## Events to track

Six events cover the value-moment funnel without over-instrumenting:

| Event name | Data payload | Fired when | Maps to BA-review KPI |
|---|---|---|---|
| `state_filter_selected` | `{ state }` | State filter changes via the dropdown | North Star: weekly active state-views |
| `category_filter_selected` | `{ category }` | Category filter changes | Tier 1 value moment |
| `context_resolve` | `{ country, state }` | `/v1/context` returns a non-null location | The "what's near me" answer landed |
| `event_detail_opened` | `{ event_id, category, state }` | Event detail page mounts | Tier 1 funnel conversion |
| `map_marker_clicked` | `{ event_id, category }` | MapLibre popup opens | Engagement depth |
| `feedback_submitted` | `{ value: 'yes' \| 'no', event_id, reason? }` | User clicks Yes/No on the feedback widget | Direct qualitative signal |

Anything beyond these six is over-instrumentation for v1.

## Security & Secrets

This section is explicit because the chore touches secrets-management discipline directly.

| Secret | Where it lives | Where it does NOT live |
|---|---|---|
| `UMAMI_DB_PASSWORD` | VPS `.env` (gitignored); referenced as `${UMAMI_DB_PASSWORD}` in `docker-compose.yml` | The committed repo, ever. `.env.example` carries a placeholder string only. |
| `UMAMI_APP_SECRET` | VPS `.env` (gitignored); referenced as `${UMAMI_APP_SECRET}` in `docker-compose.yml` | The committed repo, ever. |
| Default Umami admin login (`admin` / `umami`) | Replaced on first dashboard login, before any other action. | Persistent state on the VPS — must be rotated on Day 1. |
| Umami website-id | Frontend HTML (public). | N/A — this is a public identifier and not a secret. |

Generation commands (run once on the VPS, not in any committed script):

```
openssl rand -base64 24  # for UMAMI_DB_PASSWORD
openssl rand -base64 32  # for UMAMI_APP_SECRET
```

If either secret is ever accidentally committed: rotate immediately in `.env` + Postgres, then optionally rewrite git history with `git filter-repo`. Rotation is the priority; history rewrite is secondary.

## Out of Scope

- **Pre-commit secret scanning** (e.g., `gitleaks` hook). A reasonable follow-up chore but not bundled here.
- **A subscriber-management UI** for the daily flood digest (covered in a separate proposal once the digest itself exists).
- **Cross-domain tracking** beyond vigilafrica.org (we do not own or operate any other domain that needs analytics).
- **A/B testing infrastructure**. Umami does not natively support experiments and the project does not need them at v1.3 scale.
- **Heatmaps / session replay**. Privacy posture explicitly excludes these; they require cookies and PII.
- **Migrating any of the existing alerting / observability stack** (Resend, `/health`, ingestion logging). Out of scope for analytics work.

## Capabilities

### New Capabilities

- `web-analytics`: Privacy-respecting, self-hosted tracking of pageviews + custom events on vigilafrica.org. Reachable at `analytics.vigilafrica.org` (admin-only).
- `event-feedback`: A user can mark a specific event detail as useful or not useful in one click.

### Modified Capabilities

- `frontend-event-detail`: Adds a feedback row at the bottom of the detail content area.
- `frontend-build`: Adds the Umami tracker script reference in `index.html`.

## Acceptance Criteria

- [ ] `docker-compose.yml` contains a `umami` service that references `${UMAMI_DB_PASSWORD}` and `${UMAMI_APP_SECRET}` — no literal secrets in the committed file.
- [ ] `.env.example` documents both `UMAMI_DB_PASSWORD` and `UMAMI_APP_SECRET` with placeholder values and clear comments. `.env` itself remains gitignored.
- [ ] `analytics.vigilafrica.org` serves the Umami dashboard over HTTPS via Caddy, with the default admin password already rotated.
- [ ] The Umami tracker loads on https://vigilafrica.org and produces at least one pageview row in the dashboard within five minutes of a manual visit.
- [ ] All six custom events listed above fire on the correct user actions, verified by triggering each one manually and confirming it appears in the Umami dashboard within ~30s.
- [ ] The `<FeedbackPrompt />` component renders on `/events/:id`, uses existing design tokens (no hardcoded colours / spacing per §7.5), and fires `feedback_submitted` with the correct payload on Yes / No clicks.
- [ ] `docs/operations/analytics.md` exists and documents deployment, events, dashboard interpretation, and secret rotation.
- [ ] `README.md` carries a one-sentence privacy-posture statement referencing the self-hosted analytics.
- [ ] `npm run build` (frontend) and `go test ./...` (backend) both pass.
- [ ] No regression in p95 latency on `GET /v1/events` (the analytics work touches the frontend, not the API, but verify).

## Risks

- **R1 — Default admin password forgotten on first login.** Mitigation: deployment checklist in `docs/operations/analytics.md` step 1 is "rotate admin password". Verifier checks this before declaring the chore done.
- **R2 — CORS misconfiguration between Vercel frontend and VPS-hosted Umami.** Umami v2 handles this automatically once the website is registered, but verify in the deployment checklist.
- **R3 — Postgres growth from analytics rows.** Umami's default retention is reasonable; revisit if VPS disk pressure ever becomes a real signal (currently negligible).
- **R4 — Ad-blockers will hide a fraction of real visitors.** Acceptable — undercount is a known property of all privacy-respecting analytics. The relative trend is what matters, not the absolute count.
- **R5 — Tracker availability becomes a render dependency.** Mitigation: the helper in `web/src/analytics.ts` uses `window.umami?.track(...)` so tracker absence never throws.
- **R6 — Public dashboard could leak data if Caddy is misconfigured.** Mitigation: Umami dashboard requires login by default; verify the Caddy block does not bypass auth. Optionally restrict the `analytics.` subdomain to a known IP range if running solo.

## Verification Plan

1. Local dev: bring up Umami via docker-compose, register a local website, confirm a manual visit appears in the dashboard.
2. Staging: deploy via the existing `development → main` flow. DNS for `analytics.vigilafrica.org` points to the same VPS. Verify tracker loads from `staging.vigilafrica.org`.
3. Custom-event smoke test: trigger each of the six events manually on staging, confirm each appears in the dashboard within 30s.
4. Privacy verification: open the production site in a fresh incognito window, inspect network traffic, confirm no cookies set by Umami and no PII (no email / no precise location / no device fingerprint) leaves the browser.
5. Secrets verification: `git grep -i "UMAMI_DB_PASSWORD\|UMAMI_APP_SECRET"` finds matches only in `docker-compose.yml` (as `${VAR}` references) and `.env.example` (as placeholders). No literal secret in any committed file.
6. After 1 week of production data: confirm the dashboard shows a non-empty session count, a non-empty geographic distribution, and at least one `state_filter_selected` and `feedback_submitted` event.

## Origin

Surfaced in the 2026-05-27 business / market review (`/business-analyst` + `/startup-business-analyst-market-opportunity`) as the single highest-impact next move. Recommended as the Day 1–2 deliverable of the partnership-readiness sprint (2026-05-29 → 2026-06-04). Self-hosted Umami chosen over Plausible after user flagged that the VPS already carries the operational cost — no need for a second paid SaaS.
