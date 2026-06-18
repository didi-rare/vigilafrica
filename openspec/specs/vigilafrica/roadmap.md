# VigilAfrica — MVP Roadmap

**Version**: 1.0
**Status**: LOCKED — Approved 2026-04-12
**Maintained by**: @didi-rare

> **Governance rule**: Milestone scope is locked. Adding a feature to a milestone, removing one, or changing milestone ordering requires explicit maintainer approval and an updated ADR if a technology decision is involved. Feature IDs reference `product.md` — all acceptance criteria live there.

---

## Milestone Index

| Milestone | Theme | Features | Status |
| --- | --- | --- | --- |
| v0.1 | Something real exists | F-001, F-009 | ✅ Complete |
| v0.2 | First real data flow | F-002, F-003, F-004 | ✅ Complete |
| v0.3 | Localization engine | F-005, F-006, F-007, F-010, F-016 | ✅ Complete |
| v0.4 | Useful prototype | F-008, F-011, F-014, F-015, F-017 | ✅ Complete |
| v0.5 | Operational prototype | F-012, F-013 + operational hygiene | ✅ Complete |
| v0.6 | Country expansion model | Process template (no new F-IDs) | ✅ Complete |
| v0.7 | Second country stable | Enrichment quality validation | ✅ Complete |
| v0.8 | Pre-demo setup | Demo environment + curated seed data | ✅ Complete |
| v1.0 | Credible public launch | Quality gate (no new F-IDs) | ✅ Complete |
| v1.1 | Release automation activation | Release-please CI infrastructure (no new F-IDs) | ✅ Complete |
| v1.2 | Post-launch quality sweep | HDX boundaries + chore-post-v11-quality-sweep roll-up | ✅ Complete |
| v1.3 | Category expansion + design-system tokens | Impact categories, risk intelligence, type/spacing/z-index tokens, stylelint suppression audit | Proposed |


> **Release-state note**: The milestone index above is the authoritative release-state source for milestone tracking. Historical checklist boxes below are preserved as delivery records and are not retroactively rewritten when a milestone is marked release-complete.

---

## Pre-Milestone: Repo Foundation

These are blockers that must be resolved before v0.1 development begins. They are not features but repo hygiene fixes.

- [ ] Fix `api/go.mod` Go version: `go 1.26.2` → `go 1.26` (ADR-008)
- [ ] Align CI `go-version` to `'1.26'` (ADR-008)
- [ ] Create Go entry point: `api/cmd/server/main.go` (ADR-007)
- [ ] Update root `package.json` scripts to reference `./api/cmd/server/` (ADR-007)
- [ ] Fix `package.json` devDependency: `@openspec/cli` → `@fission-ai/openspec`
- [ ] Fix `openspec-verify.yml`: install `@fission-ai/openspec`, not `@openspec/cli`
- [ ] Replace current `README.md` with the approved draft (no overclaiming, prototype-stage tone)
- [ ] Confirm `LICENSE` file is present (Apache 2.0) ✅ Done 2026-04-12
- [ ] Create `.env.example` with all required variables

---

## v0.1 — Something Real Exists

**Goal**: Transform the repository from a scaffold/template into a recognisable VigilAfrica project. Any visitor can understand what the project does and run both the API and frontend locally.

**Features in this milestone**:

| Feature | ID    | Description                                          |
|---------|-------|------------------------------------------------------|
| Health endpoint | F-001 | `GET /health` returns `{"status":"ok","version":"0.1.0"}` |
| Landing page | F-009 | Branded VigilAfrica page replaces Vite starter template |

**Milestone acceptance criteria** (all must pass to call v0.1 complete):
- [ ] `GET /health` returns HTTP 200 with correct JSON body
- [ ] Frontend renders VigilAfrica name, tagline, and "early prototype" notice
- [ ] Zero Vite template content (no counter button, no React/Vite logos)
- [ ] Page is responsive at 375px and 1280px
- [ ] README accurately describes the project at prototype stage (no overclaiming)
- [ ] `LICENSE` file is present
- [ ] Both `npm run web:dev` and `npm run api:dev` work from the repo root and are documented in README
- [ ] Go version aligned between `go.mod` and CI

**Success signal**: A first-time repo visitor understands the project in under 60 seconds and can run the app locally in under 15 minutes by following the README.

---

## v0.2 — First Real Data Flow

**Goal**: Prove real ingestion. A real upstream EONET event enters the system, is normalized, stored in PostgreSQL, and can be queried directly from the database.

**Features in this milestone**:

| Feature | ID    | Description                                               |
|---------|-------|-----------------------------------------------------------|
| EONET ingestion | F-002 | Fetch Floods + Wildfires for Nigeria bounding box |
| Event normalization | F-003 | Raw EONET payload → internal Event model       |
| PostgreSQL storage | F-004 | Events persisted with PostGIS geometry support  |

**Milestone acceptance criteria** (all must pass):
- [ ] `docker-compose.yml` starts PostgreSQL 15 with PostGIS 3 extension enabled
- [ ] Migration `001_create_events.sql` creates the events table matching `data-model.md` §2
- [ ] Manual ingestion can be triggered (CLI flag or HTTP endpoint)
- [ ] At least one real EONET event is stored in PostgreSQL after running ingestion
- [ ] Events with geometry are stored with PostGIS `geom` column populated
- [ ] Running ingestion twice yields the same event count (idempotent — no duplicates)
- [ ] `database/url` reads from `DATABASE_URL` environment variable only
- [ ] `.env.example` documents `DATABASE_URL` with a working localhost example
- [ ] Ingestion failure (EONET unreachable) is logged; service does not crash

**Success signal**: Developer runs `make ingest` (or equivalent), connects to PostgreSQL with `psql`, and queries real Nigerian flood/wildfire events with coordinates.

---

## v0.3 — Localization Engine

**Goal**: Deliver the project's unique value. Events are displayed with Nigerian state names instead of raw coordinates.

**Features in this milestone**:

| Feature | ID    | Description                                                         |
|---------|-------|---------------------------------------------------------------------|
| PostGIS enrichment | F-005 | Match events to Nigeria ADM0 + ADM1 state boundaries  |
| API: events list | F-006 | `GET /v1/events` with category, state, status filters   |
| API: event detail | F-007 | `GET /v1/events/:id` returns single event               |
| Frontend: event list | F-010 | Live event list with loading, empty, and error states |
| Frontend: category filter | F-016 | Filter events by Floods / Wildfires                |

**Milestone acceptance criteria** (all must pass):
- [ ] Nigeria ADM1 boundary data (36 states + FCT) loaded from HDX source via migration
- [ ] Events geographically within Nigerian states are tagged with correct `state_name`
- [ ] Events outside Nigeria receive `country_name = null`, `state_name = null` — no error
- [ ] `GET /v1/events?category=floods` returns only flood events
- [ ] `GET /v1/events?state=Benue` returns only events for Benue State
- [ ] `GET /v1/events?status=closed` returns only closed events
- [ ] Pagination works: `?limit=10&offset=0` returns max 10 events
- [ ] Frontend event list shows events with state name alongside title
- [ ] Category filter controls render and filter the event list correctly
- [ ] Empty state and error state render correctly in the frontend

**"Before / After" proof** — this milestone is not complete until this demonstration is possible:
> An event that EONET describes as "coordinates: [8.13, 7.33]" is displayed in the frontend as **"Flood · Benue State, Nigeria"**.

**Success signal**: A non-technical person can read the event list and identify what is happening in which Nigerian state without understanding what a coordinate is.

---

## v0.4 — Useful Prototype for Real Users

**Goal**: A small external audience (NGO, journalist, civic responder) can meaningfully use the product. The "near you" experience is live.

**Features in this milestone**:

| Feature | ID    | Description                                                |
|---------|-------|------------------------------------------------------------|
| API: context | F-008 | `GET /v1/context` — IP geolocation + nearby events   |
| Frontend: map | F-011 | MapLibre GL JS map with coloured event markers        |
| IP geolocation | F-014 | MaxMind GeoLite2 local .mmdb lookup                 |
| Frontend: event detail | F-015 | Full event page at `/events/:id`              |
| Frontend: state filter | F-017 | Filter events by Nigerian state               |

**Milestone acceptance criteria** (all must pass):
- [ ] Map renders centred on Nigeria with event markers at correct coordinates
- [ ] Flood markers are blue (`#3B82F6`), Wildfire markers are orange (`#F97316`)
- [ ] Marker popup shows: title, state name, category, status
- [ ] `GET /v1/context` resolves caller IP to country + state using local `.mmdb` (no network call)
- [ ] Context endpoint returns state-matched events with response time < 200ms
- [ ] Context endpoint returns `{"location": null, "events": []}` if IP can't be resolved — never an HTTP error
- [ ] Event detail page loads at `/events/:id` and displays all fields from `api-contract.md` §3
- [ ] State filter dropdown renders unique state names and filters the event list
- [ ] State and category filters work in combination (AND logic)
- [ ] At least one external tester (NGO rep, journalist, or civic-tech contact) has reviewed the prototype and provided feedback

**Success signal**: A person visiting the site from a Nigerian IP address sees flood/wildfire events for their state without typing anything.

---

## v0.5 — Operational Prototype

**Goal**: The prototype can run reliably without manual intervention. It ingests automatically and handles repeated runs without data pollution.

**Features in this milestone**:

| Feature | ID    | Description                                         |
|---------|-------|-----------------------------------------------------|
| Scheduled ingestion | F-012 | gocron-based automatic EONET polling (default: every 60 min) |
| Deduplication | F-013 | Upsert on `source_id` — closed events updated, no duplicates |

**Operational requirements** (milestone blockers, not F-tagged):
- [x] Structured JSON logging for all ingestion runs (start, end, events fetched, events stored, errors)
- [x] `ingestion_runs` table — one row per run recording: started_at, completed_at, status, events_fetched, events_stored, error message
- [x] `/health` endpoint extended with `last_ingestion` block and `status: degraded` when last run failed (ADR-011)
- [x] Frontend "last updated" freshness indicator — reads `last_ingestion.completed_at` from `/health`; warns if > 2 hours stale
- [x] Resend email alert on every failed ingestion run — `RESEND_API_KEY` env var required (ADR-011)
- [x] Staleness watchdog goroutine — emails via Resend if no successful ingestion in > `ALERT_STALENESS_THRESHOLD_HOURS` (default: 2) (ADR-011)
- [x] API rate limiting (configurable via `RATE_LIMIT_RPM` env var; default: 60 requests/minute)
- [x] Response caching for `GET /v1/events` (5–15 min TTL, configurable)
- [x] CORS correctly configured for Vercel production domain via `CORS_ORIGIN` env var
- [x] VPS deployment fully documented (Caddy config example, Docker Compose production config)
- [x] Contributor setup instructions are complete, tested, and documented in `CONTRIBUTING.md`
- [x] Seed dataset committed at `api/db/seeds/sample_events_nigeria.sql` (local dev, no EONET connection needed — Nigeria data only at this stage)
- [x] `CODE_OF_CONDUCT.md` added to repo

**Success signal**: The prototype runs on a VPS, automatically ingests new events every hour, and a contributor can reproduce the full local environment in under 30 minutes by following `CONTRIBUTING.md`. A failed or stalled ingestion triggers an email alert without manual log inspection.

---

## v0.6 — Country Expansion Model

**Goal**: Adding a second African country to VigilAfrica becomes a repeatable, documented process that any contributor can follow.

**Deliverables** (process documentation, not user-facing features):
- [ ] Country Onboarding Template document in `openspec/specs/vigilafrica/country-onboarding-template.md`
- [ ] Tier classification criteria documented:
  - Tier 1: Countries with high event frequency, good HDX boundary data, existing NGO demand signal
  - Tier 2: Countries with partial data or lower priority
  - Tier 3: Backlog
- [ ] Boundary dataset standards (ADM level, required source, GeoJSON format, naming convention)
- [ ] Enrichment validation rules (what passes for a "successfully enriched" event per country)
- [ ] Fallback logic for events near borders or outside all boundaries
- [ ] Second country added as proof of template (recommended: **Ghana** or **Kenya**)

**Success signal**: Adding the second country takes fewer than 2 days of engineering effort using the template.

---

## v0.7 — Second Country Stable

**Goal**: The second country added in v0.6 meets the same production quality bar as Nigeria before v1.0 is within reach.

**Acceptance criteria** (all must pass):
- [ ] Second country enrichment achieves the same "before/after" proof as Nigeria: raw coordinates → state/province name displayed in the frontend
- [ ] Enrichment success rate documented — percentage of ingested events successfully matched to ADM1 (target: ≥ 85%)
- [ ] Border and edge cases documented: events near country borders, events outside all admin boundaries, geometry gaps in HDX source data
- [ ] Any deviations from the country onboarding template recorded in a country-specific notes file or new ADR
- [ ] EONET bounding box for second country validated — no significant overlap with Nigeria bounding box, no events incorrectly captured
- [ ] Frontend state/province filter works for second country without UI changes
- [ ] API `?country=` filter returns correct results for both Nigeria and second country independently

**What this milestone is not:**
- Not adding a third country
- Not new event categories
- Not UI redesign

**Success signal**: A non-technical user visiting the site from the second country's IP sees correctly localised flood/wildfire events for their state — the same experience Nigeria delivers today.

---

## v0.8 — Pre-Demo Setup

**Goal**: A stable, curated demo environment exists before v1.0 is attempted. The demo tells the project's story without depending on live EONET data or production infrastructure.

**Acceptance criteria** (all must pass):
- [ ] Demo deployment is separate from production — own Docker Compose config, own database, own Vercel project or preview URL
- [ ] Demo database seeded with curated static data from `api/db/seeds/sample_events_nigeria.sql` (extended at this milestone to include second country events) — live ingestion does not overwrite demo data
- [ ] Demo subdomain or URL is stable and shareable (e.g. `demo.vigilafrica.org`)
- [ ] Demo environment setup documented — a contributor can stand it up independently from `CONTRIBUTING.md` or a dedicated `DEMO.md`
- [ ] At least one screenshot committed to the repository showing the demo state
- [ ] 30-second demo GIF committed to the repository

**What this milestone is not:**
- Not a new feature
- Not a third country
- Not production hardening

**Success signal**: A single URL can be sent to an NGO contact, journalist, or potential contributor and they can explore the product immediately — no setup, no "it's down right now."

**Delivered** (2026-04-22):
- `docker-compose.demo.yml` with Ghana + Nigeria seed data, migrations on first boot, and `INGEST_INTERVAL_MIN=0` to prevent live ingestion overwriting demo data
- `DEMO.md` documenting start/stop/reset
- `docs/screenshots/demo.png` + `docs/screenshots/demo.gif` committed
- `README.md §Demo` section with embedded GIF
- **Sub-feature**: Graceful EONET rate limiting (`feature-eonet-rate-limiting`) — adaptive retry loop (max 3 attempts), dynamic `retry_after + 5s` backoff, exponential fallback, and frontend error surfacing. Archived: `openspec/archive/spec-feature-eonet-rate-limiting.md`.

---

## v1.0 — Credible Public Launch

**Goal**: A version that is genuinely useful, publicly defensible, and ready for community contributors, NGO partners, and potential funders. This milestone is a quality gate — all v0.x work must be complete and stable before v1.0 is tagged.

**Quality gate criteria** (all must pass before tagging v1.0):
- [x] At least 2 African countries supported in depth (enriched to ADM1 level) — Nigeria complete, second country validated at v0.7
- [x] At least 2 event categories supported (Floods + Wildfires minimum)
- [x] Localized enrichment working consistently for all supported countries
- [x] REST API is stable — documented in `api-contract.md` with no breaking changes since v0.3
- [x] Frontend is usable without technical knowledge by personas P-01 through P-03
- [x] Demo environment live and stable (delivered at v0.8)
- [x] Staging API and frontend deployed from `main` and validated before production tagging
- [ ] Production API and frontend deployed from `release` via annotated SemVer tag with GitHub Environment approval
- [ ] `/health.version` reports the deployed commit SHA in staging and the SemVer tag in production — staging verified; production pending
- [x] Failed-ingestion and staleness Resend alerts verified in staging
- [ ] Rollback workflow verified by redeploying a previous production tag
- [x] `CONTRIBUTING.md` is complete and tested
- [x] `CODE_OF_CONDUCT.md` is in place
- [x] Screenshot and 30-second demo GIF committed to repository (delivered at v0.8)
- [x] Public roadmap linked from `README.md`
- [x] GitHub Issues contact path exists per ADR-006 and `README.md`

**Suggested v1.0 launch message**:
> "Localized natural event awareness for [2+ African countries] — floods and wildfires shown by state, not coordinates. Open-source and free to use."

---

## v1.1 — Release Automation Activation

**Goal**: First release-please-managed cut of VigilAfrica. No functional changes; this milestone exists to prove the automated release pipeline before v1.2's larger functional roll-up.

**Status**: ✅ Shipped 2026-04-26.

**What shipped**:

- Release-please workflow on the `release` branch with `target-branch` correctly wired as an action input (not just config) — see `chore-automate-release-tagging`.
- First automated SemVer tag (`v1.1.0`) and changelog entry.

---

## v1.2 — Post-Launch Quality Sweep

**Goal**: Roll up two weeks of post-v1.0 audit followups and country-onboarding work into a single functional release.

**Status**: ✅ Shipped 2026-05-26.

**What shipped** (across PRs #84 → #98):

- HDX COD ADM1 polygon enrichment replacing v0.6 rectangles (100% / 100% success for Nigeria + Ghana).
- Alert subject env labelling (`[VigilAfrica:<env>]`) driven by `APP_ENV`.
- Staging frontend `noindex, nofollow` + visible banner upgrade.
- EONET transient-error retry (network + 5xx non-503).
- Country filter API now accepts ISO `country_code` with 400 on unknown.
- Watchdog dedupe-vs-send order fix — Resend hiccups no longer permanently drop alerts.
- React error boundary restored via `react-error-boundary`.
- "Project Status" landing-page copy refresh + footer link fix.
- ~10 other smaller cleanups from `chore-post-v11-quality-sweep` (B6 eonet.go Ingestor-struct refactor deferred to a focused follow-up).

---

## v1.3 — Category Expansion + Design-System Tokens

**Goal**: Broaden hazard coverage with four new NASA EONET categories, close out the design-system token gap on the frontend, and instrument the production deployment with privacy-respecting analytics so partnership and grant conversations can cite real data. All seven proposals below ship before the v1.3.0 tag.

**Status**: Proposed (planning).

**Feature scope** (sequenced — feature-impact-categories lands first, feature-v13-risk-intelligence lands second on top of it):

- **`feature-impact-categories`** ([changes/feature-impact-categories](../../changes/feature-impact-categories/proposal.md)) — introduces the shared category registry and adds `landslides` + `tempExtremes`.
- **`feature-v13-risk-intelligence`** ([proposals/feature-v13-risk-intelligence.md](../../proposals/feature-v13-risk-intelligence.md)) — extends the registry with `severeStorms` + `drought`.

**Hygiene scope** (parallel design-system tokens — order between these is flexible, can ship before or after the feature pair):

- **`chore-type-tokens`** ([proposals/chore-type-tokens.md](../../proposals/chore-type-tokens.md)) — extract hardcoded typography values into design tokens.
- **`chore-spacing-tokens`** ([proposals/chore-spacing-tokens.md](../../proposals/chore-spacing-tokens.md)) — extract hardcoded spacing values into design tokens.
- **`chore-z-index-tokens`** ([proposals/chore-z-index-tokens.md](../../proposals/chore-z-index-tokens.md)) — extract hardcoded z-index values into design tokens.
- **`chore-stylelint-suppressions-review`** ([proposals/chore-stylelint-suppressions-review.md](../../proposals/chore-stylelint-suppressions-review.md)) — periodic audit of stylelint rule suppressions; rides alongside the token chores since most suppressions trace back to the same drift.
- **`chore-analytics-and-feedback`** ([proposals/chore-analytics-and-feedback.md](../../proposals/chore-analytics-and-feedback.md)) — self-hosted Umami analytics on the existing VPS plus a 1-click "Was this useful?" feedback widget. Closes the traction-data gap surfaced in the 2026-05-27 business / market review. Lands first within v1.3 because every downstream partnership / grant conversation benefits from having real numbers.

**Acceptance criteria** (all must pass before v1.3 is tagged):

- [ ] EONET ingestion requests `floods`, `wildfires`, `landslides`, `tempExtremes`, `severeStorms`, and `drought`.
- [ ] Normalization maps each supported category explicitly; unsupported categories do not silently default to floods.
- [ ] Database category constraint accepts the full v1.3 category set through a reversible migration path.
- [ ] `GET /v1/events?category=<id>` returns only matching events for every category in the v1.3 set; rejects unknown values with 400.
- [ ] Frontend category filter, event cards, detail views, and map markers render all six categories distinctly.
- [ ] Nigeria and Ghana seed/demo data include representative events for all four newly-added categories.
- [ ] API contract, architecture, and product/spec references reflect the v1.3 supported set.
- [ ] All hardcoded typography, spacing, and z-index values in `web/src/` reference design tokens; stylelint rules enforce this going forward.
- [ ] Stylelint suppression list audited; each remaining suppression has a documented reason.
- [ ] Self-hosted Umami analytics live at `analytics.vigilafrica.org`, six custom events firing on production, no literal secrets committed to the repo (verified via `git grep`).
- [ ] `<FeedbackPrompt />` component live on `/events/:id`; `feedback_submitted` event captured in the Umami dashboard.

**What this milestone is not:**

- Not adding all NASA EONET categories — only the four listed.
- Not a secondary data oracle.
- Not a dark-mode toggle (`feat-dark-mode-toggle` is a separate post-v1.3 proposal).
- Not a Vercel SPA fallback fix (`fix-vercel-spa-fallback` is a separate proposal — sequence at maintainer discretion).
- Not the deferred B6 eonet.go `Ingestor`-struct refactor (its own focused follow-up).
- Not pre-commit secret scanning (e.g., `gitleaks` as a `pre-commit` hook). Surfaced as an adjacent concern during `chore-analytics-and-feedback` (which introduces new secrets), but tracked as a separate follow-up chore so the analytics work can ship without bundling tooling-discipline changes.

---

## Post-MVP Backlog (Not Before v1.0)

The following items are explicitly deferred. They must not be built before v1.0 is tagged, unless a new ADR explicitly promotes them with maintainer sign-off:

| Item                              | Reason Deferred                                                |
|-----------------------------------|----------------------------------------------------------------|
| Historical timeline (12–24 months) | Storage cost + UI complexity; prove value with live data first |
| Alert webhooks / subscriptions    | Requires auth, rate limiting, SLA — post-v1.0 complexity       |
| LGA (ADM2) enrichment             | ADM1 proves value first; LGA boundary data quality varies       |
| SMS / push notifications          | Out of scope for web-first MVP                                  |
| Coverage beyond 1–3 countries     | Expansion model (v0.6) gates this                              |
| User accounts / authentication    | No user data collected in MVP                                  |
| Parametric insurance data export  | Enterprise feature                                             |
| Fundraising / sustainability UI   | Deferred (ADR-005) — post v1.0 launch                         |
| Multi-language support            | Post-v1.0                                                      |
| Mobile native app                 | Post-v1.0                                                      |
| AI digest narrative (`feature-ai-digest-narrative`) | Standalone proposal + spec, crystallized 2026-06-18; builds on the daily flood digest. Sits here until the flood-digest pilot is live. First LLM integration (Anthropic via stdlib `net/http`) — promoting it into a milestone needs the usual ADR per the governance rule. |

