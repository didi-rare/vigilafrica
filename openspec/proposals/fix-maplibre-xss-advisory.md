---
id: fix-maplibre-xss-advisory
status: in-progress
branch: fix/maplibre-xss-advisory
---

# Proposal: Migrate to maplibre-gl 6 to Clear a CVSS 10.0 XSS (fix-maplibre-xss-advisory)

## Why

**GHSA-jrc7-96c5-q579** — *MapLibre GL JS: XSS Sanitizer Bypass in `DOM.sanitize()` via Live
NamedNodeMap Removal Skip* — published **2026-09-08**, **CVSS 10.0**.

⚠️ **There is no patched 5.x.** Vulnerable range is `<= 6.4.0`, first patched release is **6.4.1**,
and the 5 line ended at 5.24.0. Verified against the GitHub advisory API directly rather than
trusting npm's summary, because "the fix requires a major bump" is exactly the kind of claim
[[feedback_verify_before_claiming]] records being wrong before.

This is `maplibre-gl` rendering the map — **production code shipped to users**, not build tooling. So
unlike the `lib/pq` case, where exposure was startup-only against our own database, the practical
risk here is real: a sanitizer bypass in the component that renders user-facing content.

Three advisories landed alongside it and are cleared in the same change: `vitest` (4.1.5 → 4.1.11),
`js-yaml` and `colord` (both transitive, non-major).

## What Changes

### 1. maplibre-gl 5.23.0 → 6.8.0

⚠️ **Not a version bump.** v6 **removed the CSP build this app deliberately used** —
`dist/maplibre-gl-csp.js` and its separate worker no longer exist, and `Map.tsx` imported both plus
called `setWorkerUrl`. v6 is also ESM-only with **no default export**, so `import maplibregl from
'maplibre-gl'` silently degrades to `any` and every callback loses its types. Imports become a
namespace import and `web/src/maplibre-gl-csp.d.ts` is deleted.

Dropping the CSP build is safe **for our policy specifically**: `vercel.json` already grants
`worker-src 'self' blob:` and `child-src 'self' blob:`. The CSP build was belt and braces, not a
requirement of the header we serve. Anyone tightening that header must re-run the check below.

### 2. The worker wiring, which broke twice

⚠️ **Both failures left every other gate green.** Recorded in detail because the shape recurs.

**First:** v6 derives its worker path at *runtime* from its own bundle URL —
`new URL('./maplibre-gl-worker.mjs', <chunk url>)` — which the bundler cannot see, so Vite never
emitted the file and the request 404'd. **The map container and canvas still render without a
worker**, so `tsc`, ESLint, 91 unit tests and `npm run build` all passed against a map that could not
process a single tile. `setWorkerUrl` is therefore **still required on v6**.

**Second:** pointing at the worker with plain `?url` *still* 404'd. v6 builds main and worker in one
context and extracts a shared chunk, so the worker imports `./maplibre-gl-shared.mjs` relatively and
`?url` copies one file without its dependency. `?worker&url` makes Vite bundle the worker with its
imports.

### 3. `@types/geojson` becomes a declared dependency

It was an **undeclared transitive** of maplibre 5 that made the global `GeoJSON` namespace
resolvable. v6 does not pull it in, and `tsconfig.app.json` restricts `types` to `["vite/client"]`,
so `export as namespace GeoJSON` never applied here regardless. Types are now imported explicitly.

## Verification

`web/scripts/csp-map-check.py` is committed as the harness, per
[[feedback_commit_the_measurement_harness]]. It serves the built bundle with the **real** CSP header
read from `vercel.json`, loads it in Chromium, and fails on any CSP violation **or any failed
maplibre asset**:

```
workers started            : 1
maplibre container present : 1
maplibre canvas present    : 1
failed maplibre assets     : 0
CSP violations             : 0
PASS
```

⚠️ **The check had to be tightened mid-way.** Its first version treated "container present + no CSP
violations" as PASS — which is precisely what a dead worker looks like, and it passed while the
worker was 404ing. Proving a gate by re-breaking what it guards is the standing lesson in
[[reference_green_gates_prove_only_what_they_check]].

Not in CI, because it needs a Chromium download. Run it whenever maplibre, the worker wiring, or the
CSP header changes.

## Out of Scope / Not Verified

- ⚠️ **Actual tile rendering.** Headless SwiftShader cannot reliably compile the fragment shaders, so
  the check proves the worker and assets load under CSP — **not that pixels are correct**. This needs
  a look at the staging map after deploy, and that is a deliberate gap rather than an oversight.
- The `map-vendor` chunk remains over the 1000 kB warning threshold; code-splitting it is
  pre-existing work tracked with the held perf proposals.
- The `%VITE_ANALYTICS_URL%` 404 in a local build is an unsubstituted env var, pre-existing and
  unrelated.

## Impact

- **Affected:** `web/package.json`, `web/package-lock.json`, `web/src/components/Map.tsx` and its
  test, `web/src/maplibre-gl-csp.d.ts` (deleted), plus the new harness.
- **Risk:** the map is the product's primary surface, and the failure mode found here is invisible to
  every automated gate. Staging must be looked at, not just measured.
- **Blocks:** everything. The web audit gate fails on every PR until this lands, including #265.
