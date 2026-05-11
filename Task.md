# fix-public-trust-quick-wins

**Branch:** `fix/public-trust-quick-wins`
**Spec:** [openspec/specs/fix-public-trust-quick-wins.md](openspec/specs/fix-public-trust-quick-wins.md)
**Proposal:** [openspec/proposals/fix-public-trust-quick-wins.md](openspec/proposals/fix-public-trust-quick-wins.md)

## Phase 1 — Implementation (this PR)

- [x] Add `web/src/vite-env.d.ts` typing `VITE_ENV` and other Vite env vars
- [x] Add Vite HTML transform plugin in [web/vite.config.ts](web/vite.config.ts) to flip `robots` meta to `noindex, nofollow` when `VITE_ENV === 'staging'`
- [x] Add `og:image`, `twitter:card`, `<link rel="canonical">`, `<link rel="alternate" type="application/rss+xml">` to [web/index.html](web/index.html)
- [x] Copy `docs/screenshots/demo.png` to `web/public/social-card.png` as stopgap OG asset
- [x] Repurpose existing `prototype-banner` as conditional `StagingBanner` in [web/src/App.tsx](web/src/App.tsx) — only renders when `import.meta.env.VITE_ENV === 'staging'`
- [x] Add `DashboardDisclaimer` component above `FreshnessIndicator` in [web/src/components/EventsDashboard.tsx](web/src/components/EventsDashboard.tsx) with locked disclaimer copy
- [x] Modify `FreshnessIndicator` to always render — "Last updated Xm ago" when healthy (`role="status"`), keep `role="alert"` for warn/error states
- [x] Replace single-CTA hero with two-CTA structure: primary "Explore latest events" → `#dashboard` anchor, secondary "Contribute on GitHub" → repo URL
- [x] Update footer microcopy in [web/src/App.tsx](web/src/App.tsx) to spec's locked footer text (disclaimer-anchored, with version link, roadmap, GitHub Issues, license)
- [x] Add micro-disclaimer line below event title in [web/src/pages/EventDetail.tsx](web/src/pages/EventDetail.tsx)
- [x] Add CSS for staging banner, disclaimer banner, freshness OK state to [web/src/components/EventsDashboard.css](web/src/components/EventsDashboard.css) and [web/src/App.css](web/src/App.css)
- [x] Update [web/src/components/EventsDashboard.test.tsx](web/src/components/EventsDashboard.test.tsx) to assert new behaviour (always-render freshness, disclaimer present, no a11y regressions)
- [x] `npm run lint` passes
- [x] `npm run test` passes
- [x] `npm run build` succeeds

## Phase 2 — Operator Action (after PR is on `release`)

- [ ] Add `VITE_ENV=staging` to the `vigilafrica-staging` Vercel project environment variables (Project Settings → Environment Variables)
- [ ] Confirm `vigilafrica-production` has NO `VITE_ENV` value (its absence is the signal that this is production)
- [ ] Optional: replace stopgap `social-card.png` with a designed 1200×630 asset

## Phase 3 — Validation (after deploy)

- [ ] `https://staging.vigilafrica.org` shows the staging banner; `https://vigilafrica.org` does NOT
- [ ] `curl -s https://staging.vigilafrica.org/ | grep robots` returns `noindex, nofollow`
- [ ] `curl -s https://vigilafrica.org/ | grep robots` returns `index, follow`
- [ ] Disclaimer banner visible above the dashboard on prod and staging
- [ ] Event detail page (`/events/:id`) shows the micro-disclaimer below the event title
- [ ] Pasting `https://vigilafrica.org` into Slack / X renders a social card with the OG image
- [ ] Dashboard freshness indicator shows "Last updated Xm ago" when healthy

## Follow-up specs (NOT in this PR)

- `feat-public-about-page` — single consolidated `/about` page
- `chore-validate-event-state-tagging` — 20-event sample audit (Nigeria + Ghana, floods + wildfires)
- `chore-mobile-and-a11y-audit` — Lighthouse + axe + manual mobile/a11y testing
- `feat-ssr-public-pages` — SEO prerendering decision for the marketing homepage
- `feat-event-provenance-display` — surface source + timestamps on event cards/detail with confidence labels
