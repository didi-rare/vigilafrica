---
id: feat-for-partners-page
status: proposed
branch: feat/for-partners-page
---

# Proposal: Public /for-partners Page (feat-for-partners-page)

> **Retroactive / as-built proposal.** This document records a change that has
> already shipped (PR #112, commits `f18b9ac` + `5e4c0ac`). It is written after
> the fact so the `/for-partners` surface carries the same OpenSpec governance
> footprint as its sprint siblings (`chore-analytics-and-feedback`,
> `feature-daily-flood-digest`). The "What Changes" and "Acceptance Criteria"
> sections describe what was actually built, not a forward plan. Nothing here
> proposes new work.

## Why

VigilAfrica has live anchor-partnership outreach in flight — NRCS (operational
anchor), Code for Africa (distribution + grant relay), and the Bezos Earth
Fund / climate-grant track — but until now there was no single public surface to
point those conversations at. Every partnership email had to re-explain, in
prose, what the project is, what it offers, how to integrate, and the critical
framing that it is a *supplementary* awareness layer rather than an authoritative
alert system. That framing matters: a humanitarian or DRR partner cannot be left
with the impression that VigilAfrica is a replacement for NiMet, NEMA, NADMO, or
Ghana Met.

A partner-facing page closes that gap with one honest, link-shareable URL
(`/for-partners`) that:

1. States plainly what VigilAfrica provides (localised event API, daily flood
   digest, hosted map/dashboard, open data layer).
2. Gives a compact, copy-pasteable REST API reference so a technical partner can
   self-serve without a call.
3. Leads with the **supplementary / non-warranty** posture — "awareness tool,
   never the sole source" — so the relationship is correctly framed from day one.
4. Routes all contact through GitHub Issues / Discussions only, honouring
   [ADR-006 — Contact / Community: GitHub Issues Only](../specs/vigilafrica/decisions.md)
   (no published personal email).

It was built as the Day 5 deliverable of the 2026-05-29 partnership-readiness
sprint, bridging the NRCS outreach follow-up window and the CfA / grant tracks.

## What Changes

### Frontend (React / TypeScript)

1. **New route `/for-partners`** in `web/src/App.tsx`, code-split via
   `lazy()` + `<Suspense>` (same pattern as the existing `EventDetail` route),
   with a `<div className="container section">Loading...</div>` fallback.
2. **New page component** `web/src/pages/ForPartners.tsx` — a static, render-only
   component (no API calls, no data fetching) composed of six sections: hero,
   "What VigilAfrica provides" (capability cards), "Integrate" (REST API
   reference), the "Supplementary, never the sole source" responsible-use
   callout, "Who we partner with" (audience cards), and an open-source + contact
   card.
3. **New stylesheet** `web/src/pages/ForPartners.css` — all colours sourced from
   design tokens (`tokens.css`; stylelint rejects literals on
   `color`/`background`/`border-color`/`fill`/`stroke`), reusing the shared
   global classes from `App.css` (`.container`, `.btn`, `.glass-effect`,
   `.audience-grid`, `.audience-card`, etc.), with a `<=640px` responsive block.
4. **Nav wiring** in `web/src/App.tsx`: a new `.nav-actions` flex group wrapping
   a "For partners" `<Link>` and the existing "View on GitHub" button. On
   `<=480px` the GitHub button's text label is hidden (icon + `aria-label`
   remain) so the nav bar does not overflow on mobile.
5. **Per-page document title**: set to `For partners — VigilAfrica` in a
   `useEffect`, restoring the previous title on unmount (the site has no
   react-helmet; `index.html` title is static).
6. **API origin via config, not hardcoded** (commit `5e4c0ac`, from
   /openspec-review): the displayed base URL and the "Try a live request" link
   read from `getApiBaseUrl()` (sourced from `VITE_API_BASE_URL`) rather than a
   literal `https://api.vigilafrica.org`, satisfying React standards §15.4
   (single API-origin config point) and §5.4 (components reach the network only
   through `src/api/`).

### Contact model

All partnership / pilot / integration contact routes exclusively through GitHub
**Issues** (`/issues/new`) and **Discussions** (`/discussions`), plus the
Apache-2.0 license link. No `mailto:` link appears anywhere on the page, per
ADR-006.

### Tests

`web/src/pages/ForPartners.test.tsx` (vitest + Testing Library + `vitest-axe`):
asserts the page `h1`, the absence of any `mailto:` link with a GitHub-issue
primary CTA (the ADR-006 guard), the presence of the supplementary/non-warranty
heading, the per-page document title, and zero axe-detectable accessibility
violations.

### Explicitly unchanged

- **No new dependencies.** Reuses existing `react-router-dom`, `lucide-react`,
  and the design-token/CSS system already in the web app.
- **No new analytics events.** The six v1 custom events
  (`chore-analytics-and-feedback`) are an intentionally closed set; the Umami
  pageview for `/for-partners` is captured automatically. The page fires nothing
  custom.
- **No backend / API change.** The page is read-only frontend; no Go code, no
  new endpoints, no schema change. It only *documents* the existing API surface.

## Security & Secrets

- No secrets, keys, or credentials are introduced; the page references only
  public URLs (GitHub, Apache, and the env-sourced API origin).
- Every `target="_blank"` link carries `rel="noopener noreferrer"` (no reverse
  tabnabbing).
- No user input is rendered and there is no `dangerouslySetInnerHTML`, so there
  is no XSS or injection surface.
- The API origin is read from `VITE_API_BASE_URL` (via `getApiBaseUrl()`), never
  hardcoded — so no environment-specific URL is baked into source.

## Out of Scope

- **A contact form or email capture.** Contact is GitHub-only by ADR-006;
  introducing an email surface would contradict that decision.
- **Per-partner / gated content.** The page is a single public surface; no
  authenticated or partner-specific views.
- **New analytics events** for partner-CTA clicks. Deliberately excluded to keep
  the v1 event set closed; revisit only if partner-funnel measurement becomes a
  real need.
- **SEO / meta-tag infrastructure** (Open Graph, structured data, sitemap entry).
  Only the per-page `document.title` is set; richer metadata is a separate concern.
- **i18n / localisation** of the partner copy.
- **A `/for-partners` entry in the API's OpenAPI spec** — this is a web page, not
  an API resource.

## Capabilities

### New Capabilities

- `partner-landing`: A public, link-shareable `/for-partners` page presenting the
  project's offering, API reference, responsible-use framing, target audiences,
  and GitHub-only contact path for partnership conversations.

### Modified Capabilities

- `frontend-navigation`: Adds a "For partners" link to the top nav and a route
  for `/for-partners`, with mobile-overflow handling for the GitHub button.

## Acceptance Criteria

The shipped page satisfies all of the following:

- [x] Navigating to `/for-partners` renders the page; the route is code-split via
      `lazy()` + `<Suspense>` in `web/src/App.tsx`.
- [x] A "For partners" `<Link>` appears in the top nav (`.nav-actions` group);
      on `<=480px` the GitHub button's text label is hidden while its icon and
      `aria-label` remain, keeping the nav on one line.
- [x] The page renders six sections — hero, capabilities, API integrate,
      responsible-use callout, audiences, and open-source/contact — as a static
      component with no API calls and no custom analytics events.
- [x] The responsible-use section carries the heading "Supplementary, never the
      sole source" and states the awareness-tool / no-warranty / cross-check
      framing (NiMet, NEMA, NADMO, Ghana Met).
- [x] Contact routes only through GitHub Issues and Discussions (plus the
      Apache-2.0 license link); no `mailto:` link exists anywhere on the page,
      satisfying ADR-006. A test asserts zero `a[href^="mailto:"]` elements and a
      GitHub-issue primary CTA.
- [x] The displayed API base URL and the "Try a live request" link are sourced
      from `getApiBaseUrl()` (`VITE_API_BASE_URL`), not a hardcoded
      `https://api.vigilafrica.org` literal (React §15.4 / §5.4).
- [x] The page sets `document.title` to `For partners — VigilAfrica` on mount and
      restores the prior title on unmount.
- [x] No new dependencies are added; all styling uses design tokens from
      `tokens.css` and shared `App.css` classes (stylelint clean — no colour
      literals).
- [x] `ForPartners.test.tsx` passes, including a `vitest-axe` check reporting zero
      accessibility violations.
- [x] `tsc` build, eslint (0 errors), stylelint (clean), and the full vitest
      suite (42/42) pass.

## Risks

- **R1 — Partner copy over-promises capability.** Mitigation: the supplementary /
  non-warranty callout is a first-class section, and a test guards its presence,
  so the framing cannot be silently removed.
- **R2 — Contact channel drifts off GitHub** (e.g. an email link added later).
  Mitigation: the `mailto:`-absence assertion in `ForPartners.test.tsx` fails the
  build if a published email is ever introduced, keeping ADR-006 enforced in CI.
- **R3 — Hardcoded API origin regresses.** Already caught once in
  /openspec-review (`5e4c0ac`); now sourced from `VITE_API_BASE_URL`, so the
  page shows the correct origin per environment (prod vs staging) and no literal
  URL remains in `web/src/pages/ForPartners.tsx`.
- **R4 — Nav overflow on small screens.** Mitigation: the `<=480px` rule that
  drops the GitHub button label addresses this; verified in a mobile preview
  smoke test.

## Verification Plan

(Performed at ship time; recorded here for completeness.)

1. `tsc` typecheck and Vite build: pass.
2. `eslint`: 0 errors. `stylelint`: clean (no colour literals; tokens only).
3. `vitest`: 42/42 passing, including the `vitest-axe` accessibility check on
   `/for-partners`.
4. Desktop + mobile preview smoke test: nav link reaches the page, the GitHub
   button collapses to icon-only at `<=480px`, and all CTAs open the correct
   GitHub Issues / Discussions / OpenAPI / license targets in a new tab.
5. ADR-006 guard: confirmed no `mailto:` link on the page (asserted in test).
6. Config-origin check: confirmed no literal `https://api.vigilafrica.org` in
   `web/src/pages/ForPartners.tsx` after `5e4c0ac`.

## Origin

Day 5 deliverable of the 2026-05-29 partnership-readiness sprint, scoped to give
the NRCS / Code for Africa / Bezos Earth Fund outreach a single, honest,
link-shareable surface. Shipped in PR #112 (`f18b9ac`), with a follow-up fix
(`5e4c0ac`) from /openspec-review routing the API origin through
`VITE_API_BASE_URL`. This proposal is filed retroactively so the page's
governance matches the analytics and daily-digest features delivered in the same
sprint.
