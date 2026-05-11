---
id: fix-public-trust-quick-wins
status: proposed
branch: tbd
---

# Spec: Public Trust Quick Wins (fix-public-trust-quick-wins)

## Context

The v3 external review report identified concrete trust/SEO gaps in the production frontend. This spec implements §16.1 "Quick Wins — Same Day" as a single tactical PR. The disclaimer copy is captured here because it's a positioning commitment, not just a code change — future copy edits should reference this spec before softening or removing it.

Companion artifact: [openspec/proposals/fix-public-trust-quick-wins.md](../proposals/fix-public-trust-quick-wins.md).

## Decision Log

| # | Decision | Alternatives | Why |
|---|---|---|---|
| D1 | Scope = §16.1 only (six same-day items) | §16.1 + §16.2; everything | `/about` page, 20-event audit, a11y audit each deserve their own focused spec |
| D2 | Staging differentiation via build-time `VITE_ENV` flag | Hostname detection at runtime; separate index.html files per environment | Matches the existing two-Vercel-project setup; allows Vite HTML transform to flip robots meta at build time |
| D3 | Disclaimer placement = dashboard banner + event-detail micro-line | Banner only; banner on every page | Trust message appears at every data-consumption surface without polluting marketing pages |
| D4 | Two-CTA hero (Explore + GitHub) | Single primary CTA (civic OR contributor) | Product serves both audiences; forcing one collapses the other |
| D5 | Make `FreshnessIndicator` always render | Keep null-when-healthy; add separate "last updated" widget | Silence and broken are visually identical to users; same component handles healthy + degraded states |

## Components to Touch

### Modified files

| File | Change |
|---|---|
| [web/index.html](web/index.html) | Add `og:image`, `twitter:card` (and supporting `twitter:title`/`twitter:description`), `<link rel="canonical">`, `<link rel="alternate" type="application/rss+xml">` pointing at the GitHub releases atom feed; switch the `robots` meta tag to a Vite HTML transform that emits `noindex, nofollow` when `VITE_ENV === 'staging'` |
| [web/src/components/EventsDashboard.tsx](web/src/components/EventsDashboard.tsx) | Add `DashboardDisclaimer` component at the top of the dashboard route. Modify `FreshnessIndicator` to always render — show "Last updated Xm ago" when healthy, warn/error banners when stale or degraded |
| [web/src/components/EventsDashboard.css](web/src/components/EventsDashboard.css) | Styles for the disclaimer banner (neutral tone, not alarm-red) and the always-on freshness state |
| [web/src/pages/EventDetail.tsx](web/src/pages/EventDetail.tsx) | Add a single-line micro-disclaimer below the event title: "Location may be approximate — confirm with local authorities before making safety decisions." |
| [web/src/App.tsx](web/src/App.tsx) | Replace the GitHub-primary hero CTA with two CTAs: primary "Explore latest events" → `/events`, secondary "Contribute on GitHub" → repo URL. Add a `StagingBanner` component that renders only when `import.meta.env.VITE_ENV === 'staging'`. Add a `Footer` component with disclaimer-anchored microcopy + links |
| [web/src/App.css](web/src/App.css) | Styles for the staging banner (subtle but unmistakable — yellow/amber tone, sticky-top), the dual CTA layout, and the footer |
| [web/vite.config.ts](web/vite.config.ts) | Add an HTML transform plugin that substitutes the `robots` meta value based on `process.env.VITE_ENV` at build time |

### New assets

| File | Purpose |
|---|---|
| `web/public/social-card.png` | OG image for link previews. Dimensions: 1200×630px (Open Graph standard). Content: VigilAfrica logo/wordmark + "Natural event awareness for Africa" tagline. **Fallback**: if no designer-produced asset is available at PR time, ship with [docs/screenshots/demo.png](docs/screenshots/demo.png) renamed/copied as a stopgap |

### Untouched

`api/`, database, ingestion, all CI workflows, deployment scripts. This is a pure frontend/content change.

## Locked Copy (do not soften without amending this spec)

### Dashboard disclaimer banner

```
VigilAfrica is an awareness and visualization tool, not an official emergency alert system.
Event locations and timing may be approximate. Always confirm with local authorities and
official emergency agencies before making safety decisions.
```

Banner is non-dismissable and visible on every dashboard render.

### Event-detail micro-disclaimer

```
Location may be approximate — confirm with local authorities before making safety decisions.
```

Rendered below the event title on `/events/:id`.

### Staging banner

```
Staging environment — pre-release/test data. Do not rely on this for operational decisions.
```

Rendered only when `VITE_ENV === 'staging'`. Sticky top, full-width.

### Two-CTA hero

- **Primary**: `Explore latest events` → `/events`
- **Secondary**: `Contribute on GitHub` → `https://github.com/didi-rare/vigilafrica`

Supporting line below CTAs:

```
Open source · Nigeria and Ghana live · Apache 2.0
```

### Footer microcopy

```
Awareness tool — not an official emergency alert system.
Sources: NASA EONET · v1.1.0 (release notes) · Roadmap · GitHub Issues · Apache 2.0
```

- "v1.1.0" links to the current GitHub release page
- "Roadmap" links to [openspec/specs/vigilafrica/roadmap.md](openspec/specs/vigilafrica/roadmap.md) on GitHub
- "GitHub Issues" links to the repo issues page
- Version string updates with each release (currently `v1.1.0`)

## Behaviour Contract

- **B1** — Dashboard route renders the disclaimer banner above the freshness indicator on every render
- **B2** — Event detail page renders the approximate-location micro-disclaimer below every event title
- **B3** — `https://staging.vigilafrica.org` renders the staging banner; `https://vigilafrica.org` does NOT
- **B4** — `https://staging.vigilafrica.org` serves `<meta name="robots" content="noindex, nofollow">`; `https://vigilafrica.org` serves `index, follow`
- **B5** — Hero renders two visually distinct CTAs; primary CTA routes to `/events`
- **B6** — `og:image`, `twitter:card`, and `<link rel="canonical">` are present in both production and staging HTML head
- **B7** — `FreshnessIndicator` always renders some visible state — never `null` — regardless of health status
- **B8** — Footer is rendered on every route (homepage, dashboard, event detail, future about page)

## Phase 1 — Implementation (this PR)

- [ ] Add the Vite HTML transform plugin for environment-dependent `robots` meta
- [ ] Add `og:image`, `twitter:card`, canonical, RSS-alternate to [web/index.html](web/index.html)
- [ ] Create / copy `web/public/social-card.png` (or use [docs/screenshots/demo.png](docs/screenshots/demo.png) as a stopgap)
- [ ] Implement `DashboardDisclaimer` component with locked copy
- [ ] Implement `EventDetailDisclaimer` (or inline equivalent) on `/events/:id`
- [ ] Implement `StagingBanner` component gated by `import.meta.env.VITE_ENV === 'staging'`
- [ ] Modify `FreshnessIndicator` to always render
- [ ] Replace hero CTA with two-CTA structure
- [ ] Implement `Footer` component with locked microcopy

## Phase 2 — Operator Action (separate from PR)

- [ ] Add `VITE_ENV=staging` to the `vigilafrica-staging` Vercel project environment variables. **Do NOT add `VITE_ENV` to `vigilafrica-production`** — its absence is the signal that this is production.
- [ ] (Optional) Replace stopgap social card with a designed 1200×630 asset before next deploy

## Phase 3 — Validation (after deploy)

- [ ] `https://staging.vigilafrica.org` displays the staging banner; production does not
- [ ] `curl https://staging.vigilafrica.org/` shows `noindex, nofollow` in HTML head
- [ ] `curl https://vigilafrica.org/` shows `index, follow` in HTML head
- [ ] Disclaimer banner visible on `https://vigilafrica.org/events`
- [ ] Event detail page (`/events/:id`) shows micro-disclaimer
- [ ] Pasting `https://vigilafrica.org` into Slack/X renders a card with the `og:image`
- [ ] Pasting `https://vigilafrica.org` into Twitter renders a large summary card
- [ ] Dashboard's freshness indicator shows "Last updated Xm ago" (or equivalent) when healthy

## Acceptance Criteria

- [ ] All B1–B8 behaviour contracts verified manually after deploy
- [ ] No new `npm` dependencies added (everything uses existing React + Vite stack)
- [ ] Phase 1 PR is mergeable through `development → main → release` with no application/backend behaviour change
- [ ] No regression in existing dashboard tests (`npm run test` in `web/`)
- [ ] Lighthouse score for `https://vigilafrica.org/` does not regress on Performance, Accessibility, Best Practices, or SEO (capture before/after)

## Failure Modes & Recovery

| Failure | Symptom | Recovery |
|---|---|---|
| `VITE_ENV` not set on staging Vercel project | Staging banner missing; staging is still indexable | Operator adds the env var to the Vercel staging project; redeploy |
| `social-card.png` missing or wrong dimensions | Link previews show broken image | Replace asset; OG image is a static file, no rebuild required |
| Disclaimer banner styled too aggressively, scares casual visitors | UX feedback signals overcorrection | Tone CSS down — but the COPY stays locked per this spec |
| Footer version string drifts | "v1.X.Y" link points at outdated release | Future spec: derive footer version from build-time env var instead of hardcoded string |

## Risks Acknowledged

- **R1**: Designer-quality OG image is out of scope; stopgap uses [docs/screenshots/demo.png](docs/screenshots/demo.png) which is not 1200×630. Visual quality of link previews will be sub-optimal until a proper asset is produced. Tracked separately.
- **R2**: Footer version string is hardcoded to `v1.1.0` for this PR. Without a build-time substitution, future releases will leave this stale until edited manually. Acceptable for now; revisit if release cadence increases.
- **R3**: `VITE_ENV` is a new environment contract. Future Vercel preview deployments will not have it set, so they'll behave like production (no staging banner, indexable). Documented; acceptable since previews are short-lived and unindexed by default.

## Verification Plan

Single PR through `development → main → release`. The release-please-managed CHANGELOG will categorize this as a `fix:` entry (patch bump → `v1.1.1`) once it lands on `release`. Validation is observational post-deploy — open the production and staging URLs side-by-side, walk through the B1–B8 checklist, and run `curl` for the robots-meta check.

No new automated tests required for this PR. A future `chore-mobile-and-a11y-audit` spec will introduce Lighthouse + axe runs as part of CI.

## Follow-up Specs (named so they don't get lost)

| Spec ID | Scope | When |
|---|---|---|
| `feat-public-about-page` | Single consolidated `/about` page covering mission, methodology, data sources, limitations, contact path, roadmap link | Next priority after this PR |
| `chore-validate-event-state-tagging` | 20-event sample audit, Nigeria + Ghana × floods + wildfires, against source EONET records and ADM1 boundary data. Acceptance: ≥95% country, ≥90% state | Half-day evidence work; parallel to /about page |
| `chore-mobile-and-a11y-audit` | Lighthouse + axe runs (CI-integrated), manual keyboard nav, screen reader smoke test, mobile screenshots at 375/412/768/1280px | Within 2 weeks of this PR |
| `feat-ssr-public-pages` | Prerender or SSR the marketing homepage for crawler-readable content. Design decision between Vite plugins | Within 1 month if SEO results are weak |
| `feat-event-provenance-display` | Surface `source`, `source_url`, `ingested_at`, `enriched_at` on event cards/detail pages with confidence/approximation labels | After validation audit produces evidence |
