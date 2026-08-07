---
id: chore-web-hardening
status: archived
branch: chore/web-hardening
merged_pr: https://github.com/didi-rare/vigilafrica/pull/191
archived_on: 2026-08-03
---

# Proposal: Close the Cheap Findings From the 2026-07-26 Production Web Audit (chore-web-hardening)

## Why

A production Lighthouse audit on 2026-07-26 (Lighthouse 13.3.0, three runs against `https://vigilafrica.org`) returned **desktop 99 / mobile 70**, with Best Practices and SEO at 100 in every run. Most of the gap is one large, traffic-gated problem (mobile Total Blocking Time — deliberately **not** in this proposal, see *Out of Scope*).

What is left over is a set of **five small, unrelated, individually-cheap defects** that share one property: each is a handful of lines, none depends on the others, and none is worth its own proposal. Bundling them keeps five trivial changes from either becoming five PRs or — the likelier outcome on this project — being forgotten entirely.

Two of them are not cosmetic:

- **Accessibility is 97, not 100, and has been silently so.** One audit fails: *"Links rely on color to be distinguishable"* (WCAG 1.4.1 use-of-colour). ⚠️ The mobile run reports Accessibility **100**, which is misleading — the audit is `NOT APPLICABLE` there because MapLibre collapses its attribution control on narrow viewports, so the failing element is absent from the DOM. **Desktop 97 is the true score.** Anyone reading only the mobile report would conclude this is already fixed.
- **Three security headers are missing or incomplete**, all flagged `High`/`Medium` by Lighthouse's Trust-and-Safety audits. These are `Unscored`, so Best Practices reads 100 while the findings stand — they will not surface in any score-based check.

Audit evidence is recorded in memory under `project-web-perf-audit-2026-07-26`.

## What Changes

Five independent edits. Each can be reviewed and reverted alone.

### 1. Make the MapLibre attribution link distinguishable (a11y 97 → 100)

`web/src/` CSS: give the attribution links a non-colour affordance (`text-decoration: underline`). This is the **only** failing accessibility audit on the site; both failing elements (`a`, `div.maplibregl-ctrl-attrib-inner`) are the same control.

**Corrected 2026-07-27 (doc/code drift, found in independent review).** This section originally specified `.maplibregl-ctrl-attrib-inner a`. What shipped is the *outer* selector — [`Map.css`](../../web/src/components/Map.css):

```css
.maplibregl-ctrl-attrib a { text-decoration: underline !important; }
```

The implementation is deliberate and better: the outer selector also reaches the **compact/collapsed control** used at narrow viewports, which `-inner` would miss. `!important` is required because MapLibre's own `.maplibregl-ctrl-attrib a{…text-decoration:none}` has *identical* specificity (one class + one element), so precedence would otherwise depend on stylesheet order. The proposal text simply was not updated to match the code.

> Note for `feat-dark-mode-toggle`: its step 7 promises to re-verify **contrast ratio** under a light theme. That would **not** catch this, which is a use-of-colour failure. Widen that step to cover link-distinguishability so a light theme cannot reintroduce it.

### 2. Composite the `signal-ping` animation

[`web/src/App.css`](../../web/src/App.css) animates `box-shadow` spread in `@keyframes signal-ping` (~L432-436), applied to `.signal-dot` / `.signal-dot--sm`. Lighthouse reports **2 non-composited animated elements — "Unsupported CSS Property: box-shadow"** in all three runs. Box-shadow animation repaints every frame.

Re-implement as a pseudo-element ring animating `transform: scale()` + `opacity` (both compositor-driven). CLS is currently **0** and must stay 0 — `inset: 0` means the ring grows from the dot's own footprint, so scaling has no layout effect.

⚠️ **Regression caught while implementing:** `App.css` already had a `prefers-reduced-motion` rule listing `.signal-dot { animation: none }`. Moving the animation onto `::after` takes it *out of that rule's reach*, so reduced-motion users would have kept the pulse. The selector had to move with the animation (`.signal-dot::after`). The file's catch-all `*, *::before, *::after { animation-duration: 0.001ms !important }` block would also have neutered it, but relying on that alone leaves a dead selector and an implicit dependency on rule order — worth knowing for any future animation moved onto a pseudo-element.

**One intended visual difference.** The old ring reached a fixed 7px spread: 22px outer diameter on the 8px dot (2.75×), but 20px on the 6px `--sm` dot (3.33×). `transform: scale()` is relative, so a single keyframe now scales both *proportionally* — the small dot's ring becomes 16.5px rather than 20px. This reads as more consistent, not less, but it is a real pixel difference and the reason the screenshot check below is scoped to "apart from the intended `.signal-dot` change".

### 3. Add the two missing security headers

[`web/vercel.json`](../../web/vercel.json) — the existing CSP, `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy` and `Permissions-Policy` stay as they are. Add:

- `Cross-Origin-Opener-Policy: same-origin` — currently **no COOP header at all** (Lighthouse: `High`). Low risk, but ⚠️ **corrected 2026-07-27**: this originally read *"the app opens no cross-origin popups"*, which is false. `web/src/` contains **15** `target="_blank"` anchors to cross-origin destinations (GitHub, the API host, the Apache licence, upstream `event.source_url`). The conclusion survives the corrected premise, for a reason the original did not state: COOP `same-origin` only severs the `window.opener` relationship, **all 15 already carry `rel="noopener noreferrer"`** (verified across multi-line JSX, 15/15), and the codebase contains **zero** uses of `window.open`, `postMessage`, or `<iframe>`. So nothing depends on the relationship COOP breaks.
- `Strict-Transport-Security: max-age=63072000; includeSubDomains` — Vercel sends a default HSTS header, but without `includeSubDomains` (Lighthouse: `Medium`). **This is newly correct**: `www.vigilafrica.org` was NXDOMAIN until 2026-07-23 and now resolves and serves `308 → apex` (re-verified live 2026-07-26), and `api.` / `analytics.` already send `includeSubDomains` from Caddy ([`deploy/Caddyfile.example`](../../deploy/Caddyfile.example)). The subdomains are consistent, so the directive is safe.

⚠️ **Do not touch the `script-src` directive in this PR.** Lighthouse flags its host allowlist as `High` and recommends nonces; a static Vite build on Vercel has no per-request nonce mechanism, so that finding is accepted rather than fixed. `script-src` has already silently broken Umami analytics once (shipped broken in v1.3.0, fixed in v1.3.1) — it is the single most regression-prone line in this file.

### 4. Emit source maps

[`web/vite.config.ts`](../../web/vite.config.ts) — set `build.sourcemap: true`. Lighthouse reports *"Large JavaScript file is missing a source map"* for `map-vendor`. Use `true`, not `'hidden'`: hidden maps omit the `sourceMappingURL` comment, so neither Lighthouse nor a browser devtools session can find them, which defeats the point. **This project is open-source under a public GitHub repo — published source maps disclose nothing that is not already readable.**

### 5. Emit `llms.txt`

[`web/vite.config.ts`](../../web/vite.config.ts) — extend the existing `seoFilesPlugin()` that already emits `robots.txt` and `sitemap.xml`. Lighthouse 13.3.0 added an **Agentic Browsing** category; we score 2/2 on both scored audits, and `llms.txt follows recommendations` is `NOT APPLICABLE` because we do not have one.

Mirror the established staging/production split exactly: production gets a real file, staging is suppressed, and a build with `VITE_ENV` unset is treated as production (same defaulting rationale already documented in that plugin). Per the audit's stated requirement the file must be **Markdown containing at least one H1**. Keep it short: what VigilAfrica is, the data source (NASA EONET), coverage (Nigeria, Ghana), the standing "confirm with local authorities" caveat, and a pointer to the repo.

## Out of Scope

- **Mobile TBT (1,140 ms) and everything behind it** — deferring MapLibre init, and cutting first-render DOM work. Those are `perf-defer-map-init` and `perf-mobile-first-render`, both **deliberately held**: the Umami read on 2026-07-26 showed essentially no external traffic, so the mobile score is currently theoretical. Do not fold them in here.
- **CSP nonces / `strict-dynamic` / Trusted Types** — accepted risk, see §3. Trusted Types in particular would fight MapLibre for no benefit on a site with no user-generated HTML.
- **HSTS `preload`** — deliberately excluded pending explicit maintainer sign-off. Adding the directive is cheap, but *submitting to hstspreload.org* is effectively irreversible and would hard-fail every subdomain that is ever served over plain HTTP. Lighthouse will continue to report it as `Medium`; that is a documented choice, not an oversight.
- **Font-weight trimming** — belongs to `chore-type-tokens` §6.
- **Font `preload` hints / subsetting** — `font-display` already passes and fonts are not render-blocking; low value.

## Verification

Local (done on `chore/web-hardening`):

- [x] `npm run type-check` clean · `npm run lint` clean · `npx stylelint "src/**/*.css"` clean · `npm run test` **70/70** · `npm run build` clean
- [x] **Production build emits `llms.txt`, `robots.txt`, `sitemap.xml`**; the file begins with an `# H1` as the audit requires
- [x] **Staging build (`VITE_ENV=staging`) emits robots.txt only** — `Disallow: /`, no sitemap, **no `llms.txt`** — confirming the new file inherited the existing production-only guard rather than needing its own
- [x] Source maps emitted — 9 `.map` files including `map-vendor` (1.95 MB map alongside a 946 kB chunk)
- [x] `.map` and `.txt` are excluded from the SPA rewrite by the existing extension lookahead in `vercel.json`, so they serve as static files
- [x] Reduced-motion coverage followed the animation onto `.signal-dot::after` (see the regression note in *What Changes* §2)

Post-deploy (staging, then production):

- [ ] Lighthouse desktop **in incognito**: Accessibility **97 → 100**
- [ ] "Avoid non-composited animations": **2 elements → 0**; CLS still **0**
- [ ] `curl -I https://vigilafrica.org` shows `cross-origin-opener-policy: same-origin` and `strict-transport-security` containing `includeSubDomains`
- [ ] Umami still capturing after the header change (the v1.3.0 regression class) — confirm a pageview lands in the dashboard
- [ ] "Missing source maps for large first-party JavaScript" audit clears
- [ ] `https://vigilafrica.org/llms.txt` → HTTP 200; `https://staging.vigilafrica.org/llms.txt` → 404
- [ ] Visual check of `.signal-dot` at 375/768/1280 px: ring still pulses, sits *behind* the dot, and the `--sm` variant's now-proportional ring reads correctly

⚠️ **Run Lighthouse in incognito.** The first run of this audit was taken with browser extensions loaded and was materially wrong: TBT read **80 ms vs 10 ms**, unused JS **536 KiB vs 149 KiB**, plus a phantom 64 KiB "Minify JavaScript" item that was **100% extension code**. Extension noise is attributed to the page.

## Origin

Production Lighthouse audit, 2026-07-26 — three runs (desktop with extensions, desktop incognito, mobile Moto G / Slow 4G), all served an identical bundle so the runs are directly comparable. The five items here are the subset that is cheap, independent of traffic volume, and would otherwise never be written down. Full audit record, including the findings deliberately **not** in this proposal, is in memory under `project-web-perf-audit-2026-07-26`.
