# Daily Flood Digest — Operations

VigilAfrica produces a **daily flood digest**: the day's flood events grouped by
country → state, available two ways from the same source
(`internal/digest.BuildTodayDigest`, so they never drift):

1. **JSON** — `GET /v1/digest/today.json` (always available).
2. **Email** — one message per day to a fixed recipient list (opt-in via env).

Introduced by `feature-daily-flood-digest` as the NRCS pilot deliverable. This is
a **maintainer-operated** digest to a fixed list — *not* a self-serve
subscription system (no accounts, no subscribe/unsubscribe, no webhooks).

---

## 1. What's in it

- **Scope:** `category = floods` only.
- **"Today":** events whose `event_date` falls on the current **UTC** calendar
  day (`[00:00, 24:00) UTC`). Events with no `event_date` are excluded.
  Note: Nigeria is UTC+1, so a late-evening local event can land in the next UTC
  day. A configurable timezone is a possible follow-up.
- **Grouping:** country → state, both sorted alphabetically; events missing a
  country/state fall under "Unknown".
- **Empty days are normal:** the endpoint returns `total: 0` with an empty list
  (never a 404/500), and the email still sends with a "no flood events today"
  body — a predictable daily cadence for the pilot.

### JSON shape

```json
{
  "date": "2026-06-02",
  "generated_at": "2026-06-02T06:00:03Z",
  "total": 1,
  "by_country": [
    { "country_name": "Nigeria", "states": [
      { "state_name": "Benue", "events": [
        { "id": "…", "title": "Flooding in Makurdi", "event_date": "2026-06-02T04:11:00Z", "source_url": "https://…" }
      ]}
    ]}
  ]
}
```

---

## 2. Configuration (env)

| Var | Purpose | Default |
| --- | --- | --- |
| `DIGEST_TO` | Comma-separated recipient list. **Empty = digest email disabled** (scheduler logs and skips; the JSON endpoint still works). | _empty_ |
| `DIGEST_FROM` | Verified Resend sender for the digest. | `VigilAfrica Digest <digest@vigilafrica.org>` |
| `DIGEST_SCHEDULE_HOUR` | UTC hour-of-day (0–23) to send. | `6` |
| `RESEND_API_KEY` | Reused from the alerting setup; the digest sends through the same Resend account. | _empty_ |
| `APP_ENV` | Tags the subject `[VigilAfrica:<env>]`. | `unknown` |

The digest runs on its **own** Resend client scoped to `DIGEST_TO`, independent
of the operational-alert recipients (`ALERTS_TO`). Recipient addresses live only
in the VPS `.env` (gitignored) — never in source.

---

## 3. Operating it

- **Enable on the VPS:** set `DIGEST_TO` (and optionally `DIGEST_FROM`,
  `DIGEST_SCHEDULE_HOUR`) in `.env`, then restart the API container. On startup
  the log shows either `daily digest scheduler started` (with `hour_utc`) or
  `daily digest disabled (no recipients configured)`.
- **Verify the content without waiting for the schedule:** hit
  `GET /v1/digest/today.json` — it's the exact payload the email is built from.
- **Test a real send:** point `DIGEST_TO` at a test inbox and temporarily set
  `DIGEST_SCHEDULE_HOUR` to the next UTC hour, or trigger a one-off send in a
  Go scratch using `digest.SendDigest(...)`.

## 4. Assumptions & limits

- **Single replica.** VigilAfrica runs one API container per environment, so
  there is no cross-replica send-lock — every running instance would send.
  If the deployment ever scales out, add a DB lock
  (`database.TryAcquireSchedulerLock`) around the send before scaling.
- A failed send is logged and does **not** crash the scheduler; the next day's
  run is independent.
- Floods only; no historical/back-dated digests (`today` only).
