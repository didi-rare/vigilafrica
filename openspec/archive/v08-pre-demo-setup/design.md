## Context

v0.7 delivered a stable two-country prototype (Nigeria + Ghana). The project is now at a quality gate: v1.0 requires a shareable demo URL, a screenshot, and a 30-second GIF before it can be publicly announced. The demo environment must be fully isolated from production so a live ingestion failure cannot corrupt the demo experience.

Currently there is no separate demo infrastructure — there is only the production Docker Compose config and a Nigeria-only seed file. A journalist or NGO partner sent a link today would either see live (potentially stale) data or encounter a downed service.

## Goals / Non-Goals

**Goals:**
- A single URL that can be shared cold — demo environment is always "on", always has data, always loads
- Demo database seeded with curated Nigeria + Ghana events (static, no live EONET polling)
- Local standup documented in under 30 minutes via `DEMO.md`
- Screenshot and GIF committed to the repo so README is self-explanatory without clicking a link

**Non-Goals:**
- A third country
- Any new API endpoints or frontend features
- Hosting/DNS setup (that is an operational task outside the codebase)
- SSR, edge functions, or Vercel-specific deployment config changes
- Any production infrastructure changes

## Decisions

### D-1: Separate Docker Compose file (`docker-compose.demo.yml`)

**Decision**: New `docker-compose.demo.yml` — completely separate service definitions, own postgres volume (`vigil-demo-data`), no `ingest` service.

**Rationale**: The production `docker-compose.yml` starts the ingestor/scheduler alongside the API. A demo environment must never run live ingestion — it would overwrite the curated seed data. A separate file makes the distinction clear and explicit, rather than an env var flag that could be forgotten.

**Alternative rejected**: A single Compose file with an `--profile demo` flag — too easy to accidentally run the wrong profile; the separate file approach is unambiguous.

### D-2: Extend the existing seed file rather than creating a new one

**Decision**: Extend `api/db/seeds/sample_events_nigeria.sql` (rename or add a companion `sample_events_ghana.sql` loaded by `docker-compose.demo.yml`) rather than creating a new monolithic seed.

**Rationale**: Keeps the Nigeria seed independently useful for pure Nigeria local dev. Ghana seed is additive. The demo compose init script runs both.

### D-3: GIF tooling — screen recording, not automated

**Decision**: The 30-second GIF/video is produced by the developer using any screen recording tool (OBS, QuickTime, LICEcap, etc.), then committed as `docs/screenshots/demo.gif`. A specific tool is not prescribed.

**Rationale**: Prescribing a headless browser capture tool (Playwright, Puppeteer) adds dependency complexity for a one-time artifact. A human-recorded GIF is more authentic and captures real UX timing.

## Risks / Trade-offs

| Risk | Mitigation |
|---|---|
| Demo seed data becomes stale (events reference past dates) | Seed uses relative dates (`NOW() - INTERVAL '3 days'`) or a fixed recent reference; documented in seed file header |
| GIF file is large (>5 MB) | Record at 10 fps, max 30 seconds, optimize with `gifsicle -O3` before committing; target <3 MB |
| Demo URL goes down | Out of scope for this milestone — infrastructure ops are separate |
| `DEMO.md` instructions drift from code | DEMO.md is version-controlled and reviewed on every demo-related PR |

## Migration Plan

1. Extend seed file with Ghana events
2. Write `docker-compose.demo.yml`
3. Write `DEMO.md` and link from `CONTRIBUTING.md`
4. Run demo compose locally, validate both Nigeria and Ghana events appear on map
5. Take screenshot → commit
6. Record GIF → optimize → commit
7. Update `README.md` with GIF embed and demo link placeholder

No rollback strategy required — all work is additive (new files, extended seed). No schema or API changes.

## Open Questions

- **Demo URL**: Will the demo be hosted at `demo.vigilafrica.org` or a Vercel preview URL? This milestone only prepares the repo; hosting is an ops decision. `DEMO.md` should document a placeholder and leave the URL as `TBD`.
- **GIF content**: Should the GIF show the Nigeria view, the Ghana view, or a country toggle? Recommendation: start on Nigeria (familiar), switch to Ghana, show enriched state name — demonstrates the multi-country value prop in ~20 seconds.
