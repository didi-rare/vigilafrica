# Analytics Operations — Self-Hosted Umami

VigilAfrica measures usage with a self-hosted [Umami](https://umami.is) instance
running on the existing VPS, alongside the API and Postgres in the same Docker
Compose stack. This document covers deployment, the one-time database bootstrap,
the events captured, how to read the dashboard, and secret rotation.

Introduced by `chore-analytics-and-feedback` (v1.3). Privacy posture: no cookies,
no PII, no fingerprinting, no consent banner required — the tracker is loaded with
`data-do-not-track="true"` and Umami stores only aggregate pageview/event rows.

---

## 1. Architecture

| Piece | Where | Notes |
| --- | --- | --- |
| Umami container | `prod-umami` / `staging-umami` services in `docker-compose.prod.yml` / `docker-compose.staging.yml` | Image `ghcr.io/umami-software/umami:postgresql-latest`. Binds `127.0.0.1:3000` (prod) / `127.0.0.1:3001` (staging) — never exposed directly. |
| Database | A dedicated `umami` database + `umami` role inside the **existing** Postgres instance (`prod-db` / `staging-db`) | Isolated from the main `vigilafrica` database. One-time setup, see §3. |
| Public ingress | Caddy blocks `analytics.vigilafrica.org` / `analytics.staging.vigilafrica.org` in `deploy/Caddyfile.example` | TLS + security headers; reverse-proxies to the local Umami port. |
| Tracker script | `web/index.html`, `src="%VITE_ANALYTICS_URL%/script.js"` | URL + website-id substituted at build time from `VITE_ANALYTICS_URL` / `VITE_ANALYTICS_WEBSITE_ID`. Both are public identifiers. |
| Event wrapper | `web/src/analytics.ts` | Typed `track()` helper; tolerates the tracker being absent. |

Local dev uses the `umami` service in `docker-compose.yml` (port `127.0.0.1:3000`)
with weak fallback secrets — analytics is optional locally and the build does not
require `VITE_ANALYTICS_URL` when `VITE_ENV=local`.

---

## 2. First-time deployment checklist

Run in order on the VPS. **Step 1 (secrets) and the admin-password rotation in
step 5 are mandatory before the dashboard is reachable publicly.**

1. **Generate and set secrets** in the VPS `.env` (gitignored):

   ```bash
   openssl rand -hex 24      # → UMAMI_DB_PASSWORD  (hex — URL-safe)
   openssl rand -base64 32   # → UMAMI_APP_SECRET
   ```

   Add `UMAMI_DB_PASSWORD=…` and `UMAMI_APP_SECRET=…` to `.env`. These have no
   fallback defaults in the prod/staging compose files — a missing value fails
   the container start loudly, by design.

   > **`UMAMI_DB_PASSWORD` must be URL-safe.** Umami interpolates it raw into
   > `postgresql://umami:<password>@<db>:5432/umami`, so a `/`, `+`, `=`, `@`, or
   > `:` (all producible by `openssl rand -base64`) breaks Node's URL parser with
   > `TypeError: Invalid URL` and the container crash-loops on `check-db`. Use
   > `openssl rand -hex 24` (hex has none of those characters). `UMAMI_APP_SECRET`
   > is not URL-embedded, so base64 is fine there.

2. **Bootstrap the database** (one-time SQL — see §3).

3. **Add the DNS A record** for `analytics.vigilafrica.org` (and
   `analytics.staging.vigilafrica.org`) pointing at the VPS IP.

4. **Bring up the stack** so Caddy provisions TLS and the container starts:

   ```bash
   docker compose -f docker-compose.prod.yml up -d prod-umami
   ```

5. **Rotate the default admin login.** Umami ships with `admin` / `umami`.
   Log in at `https://analytics.vigilafrica.org`, change the password
   immediately (Settings → Profile), **before any other action.**

6. **Register the website** in Settings → Websites. Copy the generated
   **website ID** into `VITE_ANALYTICS_WEBSITE_ID` and set
   `VITE_ANALYTICS_URL=https://analytics.vigilafrica.org` in the frontend build
   env (Vercel project env vars), then redeploy the frontend.

7. **Verify**: visit `https://vigilafrica.org`, confirm a pageview row appears
   in the dashboard within ~5 minutes, then trigger each custom event (§4) and
   confirm it lands within ~30s.

---

## 3. One-time database bootstrap

The Umami container expects an existing `umami` database owned by an `umami`
role on the shared Postgres instance. Create them once, after Postgres is
healthy but before (or while) the Umami container retries its first connection:

```bash
# Open a psql shell inside the running Postgres container.
# Production: prod-db. Staging: staging-db. Local dev: postgres.
docker compose -f docker-compose.prod.yml exec prod-db \
  psql -U "$POSTGRES_USER" -v ON_ERROR_STOP=1 <<'SQL'
CREATE ROLE umami WITH LOGIN PASSWORD :'umami_db_password';
CREATE DATABASE umami OWNER umami;
SQL
```

> Replace `:'umami_db_password'` with the value you set for `UMAMI_DB_PASSWORD`,
> or pass it via `-v umami_db_password="$UMAMI_DB_PASSWORD"`. The role password
> **must** match `UMAMI_DB_PASSWORD` in `.env`, because the container connects as
> `postgresql://umami:${UMAMI_DB_PASSWORD}@<db>:5432/umami`.

Umami creates its own schema (tables, indexes) automatically on first boot once
it can connect. No manual migration step is required.

For **local dev**, run the same SQL against the `postgres` service using the
local fallback password (`umami_local_dev_only_not_for_prod`), or override
`UMAMI_DB_PASSWORD` in your local `.env`.

---

## 4. Events captured

Six custom events cover the value-moment funnel. All are fired through
`web/src/analytics.ts`; payloads are visible in the Umami dashboard under each
event.

| Event | Payload | Fired when |
| --- | --- | --- |
| `state_filter_selected` | `{ state }` | A specific state is chosen in the dashboard filter |
| `category_filter_selected` | `{ category }` | A specific category is chosen in the dashboard filter |
| `context_resolve` | `{ country, state }` | `/v1/context` resolves a non-null "near me" location |
| `event_detail_opened` | `{ event_id, category, state }` | An event detail page mounts |
| `map_marker_clicked` | `{ event_id, category }` | A map marker popup opens |
| `feedback_submitted` | `{ value, event_id }` | The "Was this useful?" Yes / No widget is clicked |

Resetting a filter back to "All …" deliberately does **not** fire an event —
only positive selections are counted.

---

## 5. Reading the dashboard

- **Overview** → sessions, pageviews, bounce, and geographic distribution.
  Treat absolute counts as an undercount: ad-blockers hide a fraction of real
  visitors (true of all privacy-respecting analytics). The **relative trend** is
  the signal, not the absolute number.
- **Events** tab → counts per custom event. The funnel to watch:
  `context_resolve` / `state_filter_selected` → `event_detail_opened` →
  `feedback_submitted`.
- **Realtime** → useful for the post-deploy smoke test in §2 step 7.

---

## 6. Secret rotation

If `UMAMI_DB_PASSWORD` or `UMAMI_APP_SECRET` is ever exposed (committed, logged,
shared):

1. **Rotate immediately** — this is the priority over any history rewrite.
   - `UMAMI_APP_SECRET`: generate a new value, update `.env`, restart the
     container. Existing dashboard sessions are invalidated (re-login required).
   - `UMAMI_DB_PASSWORD`: update the role password **and** `.env` together, then
     restart:

     ```bash
     docker compose -f docker-compose.prod.yml exec prod-db \
       psql -U "$POSTGRES_USER" -c "ALTER ROLE umami WITH PASSWORD 'NEW_VALUE';"
     # set the same NEW_VALUE as UMAMI_DB_PASSWORD in .env, then:
     docker compose -f docker-compose.prod.yml up -d prod-umami
     ```

2. **Then, optionally**, rewrite git history with `git filter-repo` if a secret
   reached a commit. Rotation makes the leaked value useless; history rewrite is
   secondary cleanup.

The default Umami admin password counts as a secret too — rotating it (§2 step 5)
is part of first deploy, not optional.

---

## 7. Notes & follow-ups

- The Umami image is pinned to a specific `postgresql-latest` digest in every
  compose file, and CI (`scripts/check-image-pins.js`) enforces immutable refs.
  Re-pin by running
  `docker buildx imagetools inspect ghcr.io/umami-software/umami:postgresql-latest`
  and updating the `@sha256:…` in each compose file.
- Postgres growth from analytics rows is negligible at v1.3 scale; revisit
  retention only if VPS disk pressure becomes a real signal.
- Restricting the `analytics.` subdomain to a known IP range is a reasonable
  hardening step for a solo operator (the dashboard already requires login).
