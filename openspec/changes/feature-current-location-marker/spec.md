# Spec: "You Are Here" Marker (feature-current-location-marker)

**Companion:** [`proposal.md`](proposal.md). Scoping record: [`openspec/proposals/feature-current-location-marker.md`](../../proposals/feature-current-location-marker.md). Line references are to `origin/development` at `53698c2`, checked on 2026-11-26.

## Facts this spec is built on

- **Context.** `GET /v1/context` returns `location: { country, state, lat, lng } | null` and `location_source` (`api/internal/handlers/context.go:35-39`). A failed or impossible lookup is HTTP 200 with `location: null` (`:221-238`); there is no server-side default centre (`:240-245`). On localhost no location resolves unless a `DEV_*` override is set (`:61-79`). `state` is empty when GeoIP has no subdivision (`api/internal/geoip/reader.go:62-64`).
- **Web.** The dashboard fetches context once under `['context']` (`EventsDashboard.tsx:311-314`) and uses it only for `context_resolve` (`:316-327`) and the map centre (`:343-351`: selected country > context location > Nigeria centroid). `<Map>` is lazy-loaded (`:16-19`) and rendered at `:626`.
- **Map.** `Map.tsx` creates the MapLibre map once (`:154-298`), flies to `center` at a hard-coded zoom 7 whenever `center` changes (`:300-307`), and keeps one DOM marker per unclustered event in `markers` via `syncMarkers` (`:309-387`). MapLibre v6 appends marker elements to the canvas container and popups to the map container (`node_modules/maplibre-gl/src/ui/marker.ts:401`, `popup.ts:644`), and MapLibre's CSS makes both `position: absolute` with no z-index. (`.map-marker` overrides that to `relative` for the hazard pins; see the proposal's findings.)
- **Styling gates.** Stylelint rejects colour literals outside `tokens.css` (`web/.stylelintrc.json`); z-index and spacing are review-enforced (§7.5, §7.10). `--z-map-hud: 10` is the only map z property (`index.css:13`).

## 1. Map

- `MapProps.userLocation?: readonly [number, number] | null`, `[lng, lat]`, default `null`.
- `userLng`/`userLat` are derived from the prop and are `null` unless the value is a finite number. The marker effect depends on `[userLng, userLat, isLoaded]`, never on the array.
- `userMarker: useRef<maplibregl.Marker | null>`, separate from `markers`. The effect, once `isLoaded`:
  - no location: remove the marker if there is one, set the ref to `null`;
  - a location and a marker: `setLngLat` (move, no new DOM node);
  - a location and no marker: `new Marker({ element: createUserLocationElement(), anchor: 'center' }).setLngLat(…).addTo(map)`.
- The initialization effect's cleanup removes the user marker and clears the ref before it removes the map.
- `createUserLocationElement()`: `<div class="map-user-marker" role="img" aria-label="Your approximate location (based on your network)">` holding three `aria-hidden` spans: `__halo` (static dashed circle), `__pulse` (animated ring) and `__dot` (light disc, dark ring, person glyph as an `aria-hidden`, non-focusable SVG). No popup, no `tabindex`, no listeners.

## 2. Styles

- `.map-user-marker`: `position: absolute` (Map.css loads after maplibre-gl.css with equal specificity, so any other value would override MapLibre's), 56×56px, `pointer-events: none`, `z-index: var(--z-map-user-marker)`.
- Halo: `--marker-user-halo-fill` and a 1.5px dashed `--marker-user-halo-border`. Dot: 22px, `--marker-user-fill`, 2px `--marker-user-ring` border, glyph in `--marker-user-glyph`.
- Pulse: `animation: map-user-marker-pulse` declared only inside `@media (prefers-reduced-motion: no-preference)`; under `reduce` the computed `animation-name` is `none` and the halo stays.
- `.maplibregl-popup { z-index: var(--z-map-popup) }`.
- `index.css`: `--z-map-user-marker: 1; --z-map-popup: var(--z-map-user-marker);` with the ordering reason in a comment.
- `tokens.css`: `--marker-user-fill: var(--slate-50)`, `--marker-user-ring` and `--marker-user-glyph: var(--navy-950)`, `--marker-user-halo-fill`, `--marker-user-halo-border`, `--marker-user-pulse` (slate-50 at 0.1, 0.75, 0.45), and `--map-height: 600px` (used by `.map-wrapper` and the status line).
- **Contrast (§9.5).** A light disc in a near-black ring: for any background luminance L, either the disc or the ring is at least √((L_fill+0.05)/(L_ring+0.05)) = 4.38:1 against it. The weakest background sits where the two ratios are equal, so no basemap colour can push both below that.

## 3. Dashboard

- `userLocation = useMemo(() => numbers ? [lng, lat] : null, [lng, lat])` from `contextData.location`, passed to `<Map>`.
- A `role="status"`, `aria-live="polite"`, `aria-label="Approximate location"` element inside `.dashboard-map-container`, after the `Suspense`, **always mounted**; its content, only when there is a location:
  - state and country: `Your approximate location: {state}, {country} (based on your network)`;
  - one of them: that one alone;
  - neither: `Your approximate location is marked on the map (based on your network)`.
- `.dashboard-map-container` becomes `position: relative` (no layout change). `.dashboard-location-status` is absolute at `left: 0.75rem`, `bottom: max(2.75rem, calc(100% - var(--map-height) + 2.75rem))` (the higher of the map's and the container's bottom edges, clear of MapLibre's attribution row), `z-index: var(--z-map-hud)`, `pointer-events: none`. The chip inside uses existing HUD tokens plus a miniature of the marker as an `aria-hidden` swatch.

## 4. Unchanged

Centering (`mapCenter`, the `flyTo` effect, its zoom 7), event markers, clustering, popups and `map_marker_clicked`, `context_resolve`, the API and its contract.

## 5. Note: `dev_override`

With `DEV_FORCE_LAGOS=true` or `DEV_OVERRIDE_IP` set, every visitor gets the override's location, and the marker would call it "based on your network". Those settings are for local development, and the API logs a warning at startup when either is set (`context.go:70-77`). Not handled in the web app; if it ever matters, gate the marker on `location_source === 'ip'`, which needs the field added to the web `ContextResponse` type.

## Acceptance criteria

From the proposal, restated where its facts went stale.

1. When `/v1/context` returns a non-null `location`, exactly one marker renders at `[location.lng, location.lat]`, distinct from the fire and flood markers.
2. When `location` is null, no user marker renders, the status region is empty, and the map behaves exactly as it did without this change. (The proposal said "centers on Nigeria default"; that fallback is still the web app's today but is due to change under `feature-continental-coverage`, so the criterion is stated relative to the current behaviour.)
3. The user marker is not clustered with, or culled alongside, event markers: it is not in `events-map-source` and `syncMarkers` never touches it.
4. The marker's accessible name says the location is approximate and network-derived; nothing implies GPS precision. There is no popup or tooltip; the visible status line says the same.
5. The marker is non-interactive: `role="img"`, not a `<button>`, no `tabindex`, `pointer-events: none`.
6. It is told apart from the hazard markers by shape and glyph, not colour alone, and meets 3:1 against the basemap.
7. The resolved state/country is conveyed in accessible text outside the map (`role="status"`).
8. The pulse stops under `prefers-reduced-motion: reduce`; the static halo remains.
9. Marker colours are tokens in `tokens.css`; z-order uses named custom properties.
10. A changed location moves the single marker; a null location removes it; no duplicate DOM markers across re-renders, including re-renders with an equal but new array.
11. `npm run lint`, `type-check`, `lint:styles`, `test` (with `vitest-axe`) and `build` pass; the marker is same-origin DOM and CSS, so no new CSP surface.
12. No regression to event-marker rendering, clustering, popups or `map_marker_clicked`; no layout shift beyond the control build's.
