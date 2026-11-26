# Change: "You Are Here" Marker at the IP-Resolved Location (feature-current-location-marker)

**Status:** Implemented on `feat/current-location-marker` (cut from `origin/development` at `53698c2`), not yet reviewed or merged. Prerequisite of Phase 3 (the near-me panel) of [`feature-place-first-direction`](../../proposals/feature-place-first-direction.md), decision D5: "sequence the marker first, as its own small frontend-only change".
**Scoping record:** [`openspec/proposals/feature-current-location-marker.md`](../../proposals/feature-current-location-marker.md), written 2026-07-18. It holds the why, the non-goals, the risks and the acceptance criteria. This record turns them into the built change and corrects the facts that have gone stale since July.

## Why (summary of the proposal)

When `/v1/context` resolves the visitor's location from their IP, the map flies there but draws nothing at that point, so the camera move is unexplained and the visitor cannot see where "here" is relative to the hazards around it. A marker that says plainly that the point is approximate and network-derived turns that camera move into orientation, and makes the `/v1/context` result visible.

## What changes

Frontend only (`web/src/`); no API, contract, ingestion or schema change.

1. **`<Map>` takes a `userLocation` prop** (`[lng, lat]` or `null`). The dashboard derives it from `contextData.location`, memoised on the two numbers; the map's marker effect also depends on the numbers, not the array, so a fresh array each render cannot churn the marker.
2. **One "you are here" marker**, a single ref-held `maplibregl.Marker` kept apart from the event markers: not in the clustered source, not managed by `syncMarkers`. Created once the map has loaded and a location is known, moved with `setLngLat` when it changes, removed when it goes `null` and on unmount.
3. **The marker element** is a `<div role="img">` labelled "Your approximate location (based on your network)": not a button, no popup, no tab stop, `pointer-events: none`. It differs from the hazard pins by shape: a centre-anchored light disc with a person glyph inside a dashed "approximate area" halo, against the hazards' bottom-anchored teardrop pins with a flame or wave. The pulse ring is declared only under `prefers-reduced-motion: no-preference`; the static halo stays under `reduce`.
4. **A visible status line, `role="status"`**, rendered by the dashboard outside the lazily-loaded map: "Your approximate location: {state}, {country} (based on your network)". It is the marker's legend for sighted users and its text alternative for everyone else (§12.11). It overlays the bottom-left of the map, so it takes no layout space.
5. **Tokens and stacking.** Marker colours are `--marker-user-*` tokens in `tokens.css`. Stacking uses two named custom properties in `index.css`: `--z-map-user-marker` (above the hazard pins) and `--z-map-popup` (popups stay above the marker). `--map-height` moves the map's 600px height into `tokens.css`, so the status line can be positioned against the map's bottom edge from outside the map component.
6. **Evidence harness:** `scripts/bench-dashboard-cls/check-user-marker.mjs` (new), and `measure-cls.mjs` gains `CONTEXT_LOCATION=lagos` and `PW_CHANNEL`.

## Decisions taken during the build

| # | Question | Decision and reason |
|---|---|---|
| B1 | Proposal §4: make `flyTo` use the `zoom` prop instead of the hard-coded 7? | **Not done.** The dashboard does not pass `zoom`, so the prop's default (5) would replace today's 7 and every visitor would see a wider re-centre. That is a centering change, which the proposal puts out of scope, not a trivial tidy. |
| B2 | Marker above or below the hazard pins? | **Above, but transparent to clicks.** Above, so a hazard at the same spot cannot hide where "here" is (R3). `pointer-events: none`, so the click on that spot still reaches the hazard pin underneath (measured: the hazard popup opens). |
| B3 | Popups versus the raised marker | Raising the marker alone would paint it over open popups. Popups get `--z-map-popup`, equal to the marker's value: on a tie the later element wins, and a popup is always later in the DOM than the canvas container holding the marker. Keeping popups at 1, below MapLibre's control corners (2), leaves the existing popup-versus-attribution order unchanged. |
| B4 | Where the accessible text lives, and whether it is visible | **Visible, outside the map component.** The marker has no popup, so without a visible line a sighted user never reads "approximate" (R1). Rendering it in the dashboard keeps it when the map chunk or WebGL fails. Overlaid, so it cannot shift the page (two-arm CLS: identical to the control). |
| B5 | Move the marker, or recreate it, when the location changes? | **Move** (`setLngLat` on the one ref-held marker), as R2 describes; recreate only after it was removed for a `null` location. |
| B6 | New `--z-*` properties against §7.10's "don't add new ad-hoc properties" | The proposal's R3 requires a named property in `index.css`; no existing `--z-*` value sits between the markers (auto) and MapLibre's controls (2). Two are added next to `--z-map-hud`, with the ordering explained in a comment. The scale still awaits `chore-z-index-tokens`. |

## Corrections to the proposal's facts (checked against `53698c2` on 2026-11-26)

- **"Nigeria default" is half gone.** The *server's* hidden default centre was removed (`api/internal/handlers/context.go:240-245`: "No hidden default centre"); a failed lookup now returns `location: null, location_source: "unavailable"`. The *web app* still falls back to the Nigeria centroid (`EventsDashboard.tsx:343-351`, `COUNTRY_CENTERS['Nigeria']`) and `<Map>`'s default `center` is the same point. So "centers on Nigeria default" is still true of the browser today, but `feature-continental-coverage` replaces it with a continental default view. The acceptance criterion is restated in `spec.md` as "the map behaves exactly as without this change".
- **Line references have all moved.** At `53698c2`: `mapCenter` is `EventsDashboard.tsx:343-351` (proposal: 247-255); `<Map>` is rendered at `:626` (417-421); `context_resolve` fires at `:316-327` (223-231); `MapProps` is `Map.tsx:53-57` (24-28); `createMarkerElement` is `:98-116` (69-87); the `flyTo` effect is `:300-307` (271-278); the `syncMarkers` effect is `:309-387` (282-358); the marker token block is `tokens.css:202-209` (162-168); `@keyframes marker-pulse` is `Map.css:152-167` (138-153).
- **MapLibre is v6** (6.8.0): ESM, no default export, no CSP build. Its `Marker` leaves the role of a custom element alone and adds a `tabindex` only to markers with a popup, so a custom `role="img"` element without a popup stays non-interactive.
- **`/v1/context` now returns `location_source`** (`explicit`, `dev_override`, `ip`, `unavailable`); the web app's `ContextResponse` type does not declare it. The marker does not need it, but see the note on `dev_override` in `spec.md`.
- **GeoIP can return a country with an empty state** (`api/internal/geoip/reader.go:62-64` sets `State` only when a subdivision exists), so the status line handles a country-only location.
- **The hazard pulse is not the one to gate.** The proposal points at `marker-pulse` (`Map.css:152-167`); that is the hazard pins' animation. The user marker has its own `map-user-marker-pulse`, gated explicitly. The hazard pulse is still stopped under `reduce` only by the catch-all in `App.css:1249-1258`; unchanged here.

## Found while measuring, not fixed here (out of scope)

- **Hazard pins after the first are drawn displaced.** `.map-marker { position: relative }` (`Map.css`) has the same specificity as MapLibre's `.maplibregl-marker { position: absolute }` and loads later, so the pins are laid out in normal flow inside the canvas container, each shifted by the widths of the pins before it. Measured on both the control and this branch (`check-user-marker.mjs`, informational section): with two unclustered pins on one row, the second is drawn 39.9px right of its point. The user marker sets `position: absolute` and is not affected. Needs its own fix.
- **Any dashboard re-render flies the camera back to the context location.** `mapCenter` is a fresh `[lng, lat]` array on every render when it comes from context, and the `flyTo` effect depends on it. Measured on both builds with the same script: drag the map away (a pin at the context point ends up about 220px left and 165px above the map centre; the exact figure varies with drag inertia), click "Next page", and the pin is back at the centre (0,0). A centering change, out of scope here.
- On a 375px-wide phone the 600px map overflows its 60vh container (487px), so MapLibre's attribution is drawn below the container. Pre-existing; the status line is placed inside the visible part.

## Not in this change

Everything the proposal lists as out of scope: browser GPS, wiring `nearby_events`, a distance readout, a manual location override, and any change to the centering logic.
