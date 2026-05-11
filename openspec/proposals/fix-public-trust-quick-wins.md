# Proposal: Public Trust Quick Wins (fix-public-trust-quick-wins)

**Status:** Proposed — derived from the v3 external website review report (§16.1 "Quick Wins — Same Day").

## Why

The v3 review identified concrete, verified gaps in the production frontend that cost trust at first impression. Independently verified against the codebase:

- No "this is not an emergency alert system" disclaimer anywhere in the React app — only string match for "awareness" is an NGO audience descriptor (see [web/src/App.tsx](web/src/App.tsx))
- Staging and production serve identical metadata, including `<meta name="robots" content="index, follow">` — staging is indexable just like production (see [web/index.html:10](web/index.html#L10))
- No `og:image`, no `twitter:card`, no canonical URL — link previews are degraded
- `FreshnessIndicator` returns `null` when healthy ([web/src/components/EventsDashboard.tsx:54-77](web/src/components/EventsDashboard.tsx#L54-L77)) — users cannot distinguish "data is fresh" from "this widget is broken"
- The hero CTA is GitHub-focused, which under-serves civic users who came to see events

Each of these is a sub-15-minute fix individually, but they share a coherent theme — **public trust at first impression** — so they're batched into one spec and one PR. The disclaimer copy is a positioning commitment worth recording in a spec so future copy edits can't silently dilute it.

## What Changes

Six tactical changes to the frontend:

1. **Dashboard disclaimer banner** — non-dismissable banner near the top of the dashboard route stating that VigilAfrica is an awareness tool, not an official emergency alert system, with the exact copy locked in this spec
2. **Event-detail micro-disclaimer** — single line on `/events/:id` reminding readers the location may be approximate
3. **Staging environment banner** — visible banner only on `staging.vigilafrica.org` clarifying that it's a test environment, gated by a build-time `VITE_ENV=staging` flag
4. **Staging `noindex, nofollow`** — same flag flips the `robots` meta tag on staging only, so staging stops competing with production in search results
5. **Two-CTA hero** — primary "Explore latest events" routing to `/events`, secondary "Contribute on GitHub" linking to the repo
6. **SEO/social metadata** — add `og:image`, `twitter:card`, `<link rel="canonical">`, and a discoverable GitHub Releases atom feed; make `FreshnessIndicator` always render (showing "Last updated Xm ago" when healthy)

A small footer is added with disclaimer-anchored microcopy, links to `CHANGELOG.md` (or GitHub releases), the openspec roadmap, GitHub Issues, and license.

## Out of Scope (queued as follow-up specs to avoid losing them)

The v3 report identified several items beyond §16.1 that we explicitly defer. Each is named below so the work doesn't get lost:

| Deferred item | Queued spec ID | Why deferred |
|---|---|---|
| Single consolidated `/about` page (mission + methodology + data sources + limitations + contact + roadmap link) | `feat-public-about-page` | Real content work, ~half a day; deserves its own focused PR |
| 20-event live data accuracy sample audit (Nigeria + Ghana, floods + wildfires) | `chore-validate-event-state-tagging` | Evidence-gathering with a documented methodology + result write-up; spec defines acceptance bar (95% country / 90% state tagging) |
| Manual mobile screenshot QA + keyboard navigation + colour contrast + screen-reader smoke test | `chore-mobile-and-a11y-audit` | Requires real-device testing + Lighthouse + axe runs; not bundleable with a code-only PR |
| Static or prerendered marketing homepage for crawler-readable content | `feat-ssr-public-pages` | Bigger architectural decision (Vite SSR vs prerender plugin vs static export); deserves a design discussion |
| Event-level confidence/approximation labels on event cards and detail pages | `feat-event-provenance-display` | Touches data model surface area (`source`, `source_url`, `ingested_at`, `enriched_at` already exist in API contract); benefits from real-event sampling first |

Also explicitly NOT planned (per v3 §6.1):

- Separate `/methodology`, `/data-sources`, `/limitations`, `/status`, `/contact`, press kit, partner section pages — consolidated into the single `/about` page when that spec is built
- GDACS-style alert levels / confidence scoring (mission creep; the disclaimer covers the same risk)

## User Impact

Civic users land on the dashboard and immediately understand:
- This is an awareness tool, not a safety-decision system
- The data is fresh (or stale, with a visible last-updated time)

Contributors still see the GitHub path, just as a secondary CTA. Search engines stop indexing staging. Link previews on Slack/Twitter/X show a real social card instead of a blank rectangle. None of these change application behavior; all of them change first-impression credibility.
