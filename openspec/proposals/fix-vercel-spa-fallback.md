---
id: fix-vercel-spa-fallback
status: proposed
branch: tbd
---

# Proposal: Add Vercel SPA Fallback Rewrite for Deep-Link Routes (fix-vercel-spa-fallback)

## Why

Direct navigation to any client-side route deeper than `/` returns **Vercel's generic `404: NOT_FOUND` page** instead of the SPA. Confirmed live on 2026-05-14:

```
$ curl -I https://staging.vigilafrica.org/events/<any-id>
HTTP/2 404
$ curl -I https://vigilafrica.org/events/<any-id>
HTTP/2 404
```

Playwright screenshots confirm the rendered output is Vercel's branded 404 page, not React's error boundary.

Root cause: [web/vercel.json](web/vercel.json) sets CSP/security headers but has no `rewrites` block. Vercel looks for a literal file or function at `/events/<id>`, doesn't find one, and returns 404 before the SPA bundle ever loads. Users navigating client-side via React Router from `/` work fine because the bundle is already in memory and the URL is updated via `history.pushState`.

User impact:
- Any shared `/events/<id>` link 404s
- Any bookmark to a specific event 404s
- Any browser refresh on `/events/<id>` 404s
- Search engines cannot crawl event detail pages
- `/api-status`, `/about`, or any future top-level route will hit the same wall

This affects both staging and production — both v1.0.x and v1.1.x. Likely latent since the SPA was deployed to Vercel.

## What Changes

Add a single `rewrites` block to [web/vercel.json](web/vercel.json):

```json
{
  "rewrites": [
    { "source": "/((?!assets/|api/|_vercel/).*)", "destination": "/index.html" }
  ]
}
```

The negative-lookahead avoids rewriting:
- `/assets/*` — hashed JS/CSS bundles served as static files
- `/api/*` — reserved in case Vercel functions are ever added (CSP `connect-src` allows external API hosts but this guards against accidental collision)
- `/_vercel/*` — Vercel's internal endpoints

For everything else (`/`, `/events/<id>`, future top-level routes), Vercel serves `index.html`, the SPA bundle loads, React Router reads the URL, and the correct page renders.

## Out of Scope

- Server-side rendering / static prerendering for SEO (separate `feat-ssr-public-pages` — already named as a deferred follow-up in [fix-public-trust-quick-wins](openspec/archive/spec-fix-public-trust-quick-wins.md))
- A custom 404 page for genuinely unknown routes (React Router already handles "no route matched" inside the SPA; the fallback only triggers if `index.html` itself fails to load)
- Caching headers for the rewrite — Vercel applies the existing static-asset caching by default

## Verification

After the change:
- [ ] `curl -sI https://vigilafrica.org/events/<any-id>` returns `HTTP/2 200` with `content-type: text/html`
- [ ] Same for `https://staging.vigilafrica.org/events/<any-id>`
- [ ] Loading `/events/<id>` directly in a browser renders the event detail page (same as click-through from dashboard)
- [ ] `curl https://vigilafrica.org/assets/index-<hash>.js` still serves the JS bundle (not rewritten to index.html — would be catastrophic)
- [ ] Browser refresh on `/events/<id>` keeps the page (no 404 flash)

## Origin

Surfaced during the 2026-05-14 staging+production validation pass. The bug was masked because internal navigation works perfectly via React Router; only direct/shared/refreshed deep-links surface it.
