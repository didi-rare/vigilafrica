---
id: feature-daily-flood-digest
status: proposed
branch: feat/daily-flood-digest
---

# Proposal: Daily Flood Digest (feature-daily-flood-digest)

## Why

The partnership-readiness sprint (2026-05-29 → 2026-06-04) promised the Nigerian Red Cross Society (NRCS) a concrete pilot deliverable: a **daily digest of the day's flood events, by admin name**, that a focal point can read without touching the API or the map. The NRCS follow-up conversation is the first place this lands; the same artefact strengthens the Code for Africa pitch and any grant application that needs to show an operational output, not just a data API.

VigilAfrica already ingests, enriches, and serves flood events. What it does **not** have is a *push* surface — everything today is pull (the website, `GET /v1/events`). A field coordinator will not check a website every morning, but they will read an email. This proposal adds the lightest credible push: one scheduled daily email plus a machine-readable JSON view of the same content.

This is deliberately a **prototype** sized for the pilot — not the deferred "Alert webhooks / subscriptions" system (see *Out of Scope*).

## What Changes

### Backend / API (Go)

1. **New shared digest builder** — `api/internal/digest` package exposing `BuildTodayDigest(ctx, repo) (Digest, error)`. Single source of truth used by both the HTTP endpoint and the scheduled email, so the JSON and the email can never drift. The `Digest` struct carries: `date` (UTC day), `generated_at`, `total`, and `events` grouped by `country_name` → `state_name`, each event reduced to `{ id, title, state_name, country_name, event_date, source_url }`.

2. **"Today" definition** — floods (`category = 'floods'`) whose `event_date` falls on the current UTC calendar day (`event_date >= start-of-today-UTC AND event_date < start-of-tomorrow-UTC`). Events with a null `event_date` are excluded from the daily view. Rationale: the digest answers "what flooded *today*"; open-but-old events belong on the map, not the daily push.

3. **Repository date filtering** — extend `database.EventFilters` (api/internal/database/queries.go) with `DateFrom *time.Time` / `DateTo *time.Time`, and update `buildEventFilterClause()` to append `AND event_date >= $n AND event_date < $n+1` when set. This is the one change to the governed repository layer (developers-go.md §5); all event access stays inside `internal/database`.

4. **New endpoint** — `GET /v1/digest/today.json`, registered on the rate-limited `v1Mux` in `api/cmd/server/main.go`, behind the existing `RateLimit` + `SecurityHeaders` + `CORS` middleware chain. Returns `200` with `Content-Type: application/json` and the `Digest` payload — including when there are zero events for the day (an empty digest is a valid answer, never a 404 or 500). Optionally wrapped in the existing `cache.CacheMiddleware` with a short TTL (the underlying query is cheap; caching is a nicety, not a requirement).

5. **Scheduled daily email** — a `StartDigestScheduler(ctx, repo, mailer, cfg, logger)` goroutine in `api/internal/digest`, mirroring the existing watchdog/ingestor pattern (`time`-based, listens on `ctx.Done()`, spawned from `cmd/server/main.go`). It wakes once per day at `DIGEST_SCHEDULE_HOUR` (UTC, default `6`), calls `BuildTodayDigest`, renders an HTML + text email, and sends it to the configured recipients via the existing Resend client.

### Email delivery (reuse, not rebuild)

6. Reuse the existing `api/internal/alert` Resend infrastructure (`sendEmail`, `html/template` render pattern, `[VigilAfrica:<env>]` subject labelling via `APP_ENV`). Add `digestHTMLTemplate` + `digestTextTemplate`. The HTML is a simple, readable list grouped by state — no images, no tracking pixels, no external CSS — so it renders in any client and matches the project's privacy posture.

7. **Recipients via env, not source.** Add `DIGEST_TO` (comma-separated), parsed with the existing `alert.ParseRecipients` helper, plus `DIGEST_FROM` (falls back to the alert `FromEmail`). The sprint called for "hardcoded 3 test recipients"; this is satisfied by a fixed list in the VPS `.env` (gitignored) rather than literals in source — consistent with secrets/config discipline (developers-go.md §2) and avoids committing real addresses. The scheduler is a no-op (logs and skips) when `DIGEST_TO` is empty, so local dev and CI never send mail.

### Documentation

8. `docs/operations/daily-digest.md` — what the digest contains, the schedule, the env vars (`DIGEST_TO`, `DIGEST_FROM`, `DIGEST_SCHEDULE_HOUR`), how to trigger a one-off send for testing, and how to read `GET /v1/digest/today.json`.
9. `.env.example` — document the three new vars with comments and safe placeholders.
10. `api-contract.md` — add the `GET /v1/digest/today.json` response shape.

## Digest shape (JSON)

```json
{
  "date": "2026-06-02",
  "generated_at": "2026-06-02T06:00:03Z",
  "total": 2,
  "by_country": [
    {
      "country_name": "Nigeria",
      "states": [
        { "state_name": "Benue", "events": [
          { "id": "…", "title": "Flooding in Makurdi", "event_date": "2026-06-02T04:11:00Z", "source_url": "https://…" }
        ]}
      ]
    }
  ]
}
```

The HTML email is the same content rendered as grouped headings + bullet lists, with a one-line header (`Daily Flood Digest — 2 events — 2 Jun 2026`) and the standard "awareness tool, confirm with authorities" disclaimer footer that already appears on the site.

## Scheduling

- A single goroutine started in `main`, following developers-go.md §7 (spawned in `main`, exits on `ctx.Done()`).
- Computes the duration until the next `DIGEST_SCHEDULE_HOUR` (UTC), sleeps, sends, then loops every 24h.
- **No multi-replica dedup** for the prototype: VigilAfrica runs a single API container per environment, so a DB send-lock (like the watchdog's `TryRecordStalenessAlert`) is unnecessary now and explicitly deferred. A note in the code + doc records this assumption so it is revisited if the deployment ever scales out.

## Security & Secrets

| Secret / config | Where it lives | Where it does NOT live |
| --- | --- | --- |
| `RESEND_API_KEY` | VPS `.env` (gitignored) — already present for alerting | The repo, ever |
| `DIGEST_TO` / `DIGEST_FROM` | VPS `.env` | Source files; `.env.example` carries placeholders only |

No new secret class is introduced — the digest reuses the alerting Resend key. Recipient addresses are configuration, kept out of git.

## Out of Scope

- **The deferred "Alert webhooks / subscriptions" system** (roadmap Post-MVP Backlog). This proposal is explicitly *not* that: no user accounts, no auth, no self-service subscribe/unsubscribe, no webhooks, no per-user preferences. It is a maintainer-operated digest to a fixed, env-configured recipient list for a specific pilot. If/when self-serve subscriptions are built, they supersede this and require their own ADR + maintainer sign-off.
- **A subscriber-management UI** — not in this sprint (v1.5+ at the earliest).
- **Per-recipient or per-state customisation** — every recipient gets the same national digest.
- **Other categories** — floods only. A "wildfire digest" or combined digest can follow once the flood pilot proves the format.
- **SMS / push / WhatsApp** — email only.
- **Historical digests / an archive of past days** — only `today` is exposed. A `?date=` parameter is a possible follow-up, not part of the prototype.
- **Modifying the LOCKED roadmap milestone scope** — see *Open question* below; this proposal does not edit `roadmap.md`.

## Capabilities

### New Capabilities

- `digest-today-json`: `GET /v1/digest/today.json` returns the day's flood events grouped by country/state as JSON.
- `digest-daily-email`: a scheduled daily email of the same content to a fixed recipient list.

### Modified Capabilities

- `events-repository`: gains optional `event_date` range filtering (additive; existing callers unaffected).

## Acceptance Criteria

- [ ] `GET /v1/digest/today.json` returns `200` + `application/json` with the documented shape; an empty day returns `total: 0` and an empty list, never 4xx/5xx.
- [ ] The endpoint contains only `category = floods` events whose `event_date` is within the current UTC day.
- [ ] `EventFilters` date range is additive — every existing `ListEvents` caller and test passes unchanged.
- [ ] `BuildTodayDigest` is the sole source for both the endpoint and the email (no duplicated query/grouping logic).
- [ ] With `DIGEST_TO` set, the scheduler sends one HTML+text email at `DIGEST_SCHEDULE_HOUR` UTC; subject carries `[VigilAfrica:<env>]`; the body groups events by state and includes the disclaimer footer.
- [ ] With `DIGEST_TO` empty, the scheduler logs that it is disabled and sends nothing (verified in local dev / CI).
- [ ] No literal recipient address or secret appears in committed files (`git grep`).
- [ ] `docs/operations/daily-digest.md` and `.env.example` document `DIGEST_TO`, `DIGEST_FROM`, `DIGEST_SCHEDULE_HOUR`; `api-contract.md` documents the endpoint.
- [ ] `go test ./...`, `npm run build`, and the existing lints pass; `govulncheck` and image-pin CI gates stay green.
- [ ] No regression in `GET /v1/events` p95 latency (digest query is separate and bounded).

## Risks

- **R1 — Empty-day email fatigue.** Sending a "no flood events today" email daily could read as noise. *Decision:* send every day regardless (predictable cadence builds pilot trust; the disclaimer-footer email clearly states "no flood events today"). If NRCS later prefers a quieter inbox, a `DIGEST_SKIP_WHEN_EMPTY` toggle is a trivial follow-up — not built now.
- **R2 — Timezone confusion.** "Today" is UTC; Nigeria is UTC+1. An event late on the Nigerian evening could land in the next UTC day. *Mitigation:* document the UTC basis; a configurable `DIGEST_TZ` is a follow-up if it matters to the pilot.
- **R3 — Resend dependency / send failure.** A failed send should log loudly (reuse alert logging) and not crash the scheduler goroutine; the next day's run is independent.
- **R4 — Scope creep toward the deferred subscription system.** *Mitigation:* the Out-of-Scope section draws the line explicitly; any move toward self-serve subscriptions triggers a new ADR.
- **R5 — Single-replica scheduling assumption.** If the API ever scales to multiple replicas, every replica would send. *Mitigation:* documented assumption; add a DB send-lock (watchdog pattern) before any horizontal scale-out.

## Verification Plan

1. Unit: `BuildTodayDigest` groups correctly and respects the UTC-day + floods-only filter (table tests with fixture events spanning category, date, and null `event_date`).
2. Unit: empty-day path returns a valid empty digest.
3. Handler test: `GET /v1/digest/today.json` returns the expected JSON for a seeded set.
4. Scheduler: a small, injectable `now`/clock + a fake mailer to assert one send per scheduled tick and a no-op when `DIGEST_TO` is empty — no real network.
5. Local manual: seed flood events dated today, hit the endpoint, and run a one-off send against a Resend test recipient; confirm the HTML renders and groups by state.
6. Secrets: `git grep` shows no recipient/secret literals.

## Decisions (maintainer, 2026-06-02)

- **Roadmap placement.** Kept a **standalone proposal**, sequenced at maintainer discretion (like `feat-dark-mode-toggle` / `fix-vercel-spa-fallback`). The LOCKED v1.3 milestone scope in `roadmap.md` is **not** edited by this proposal.
- **Empty-day behaviour.** The daily email **always sends** at the scheduled hour, with a clear "no flood events today" body on empty days (see R1). No `DIGEST_SKIP_WHEN_EMPTY` toggle for the prototype.

## Origin

Day 3–4 deliverable of the partnership-readiness sprint (`sprint-2026-05-29-partnership-readiness`), the NRCS pilot artefact promised ahead of the 2026-06-04 follow-up. Builds directly on the v0.5 Resend alerting + scheduler infrastructure (ADR-011) and the v0.3 events repository.
