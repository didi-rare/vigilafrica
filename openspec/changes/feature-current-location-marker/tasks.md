# feature-current-location-marker

Ticked with evidence. Branch `feat/current-location-marker`, cut from `origin/development` at `53698c2`. Browser evidence comes from `scripts/bench-dashboard-cls/check-user-marker.mjs`, `keyboard-walkthrough.mjs` and `measure-cls.mjs`, run on 2026-11-26 against `vite preview` builds of this branch (port 4273) and of `53698c2` (port 4274, exported with `git archive`), in the installed Chrome (`PW_CHANNEL=chrome`). `/v1/context` was mocked with a Lagos location in every browser run: a local build cannot resolve one. Figures below are from the runs after the review fixes.

## 1. Record

- [x] 1.1 Re-verify the proposal against the current tree. Corrections are listed in `proposal.md` ("Corrections to the proposal's facts"): the server-side Nigeria default is gone but the web fallback remains; every line reference has moved; MapLibre is v6; `location_source` exists; GeoIP `state` can be empty; the pulse the proposal names belongs to the hazard pins
- [x] 1.2 Decide proposal §4 (the `flyTo` zoom tidy): not done, because it changes the re-centre zoom from 7 to 5 for every visitor (`proposal.md` B1)
- [x] 1.3 Point the scoping proposal at this record: frontmatter `status: in-progress`, `branch: feat/current-location-marker`, plus a note at the top
- [x] 1.4 Apply the independent review (2026-11-26, "merge after fixes"): 2 required and 8 smaller findings, listed in `proposal.md` "Review fixes"
- [ ] 1.5 Maintainer sign-off on the second `--z-*` property against §7.10 (`proposal.md` B6) and on the 50 km halo radius assumption (B8)

## 2. Map (`web/src/components/Map.tsx`)

- [x] 2.1 `userLocation?: readonly [number, number] | null` prop; the marker effect depends on the numbers, not the array. Test "does not touch the marker when re-rendered with an equal but new array" (no `setLngLat`, no `remove`, still one marker)
- [x] 2.2 One ref-held `maplibregl.Marker`, outside `markers` and `syncMarkers`, created after load, moved with `setLngLat`, removed on `null` and on unmount. Tests: "renders one non-interactive, labelled marker at the location once the map loads", "keeps exactly one marker, moved in place, when the location changes", "removes the marker when the location goes null, and adds it back when one returns", "removes the marker on unmount", "is not culled when the event markers re-sync after a viewport change" (also checks the clustered source never receives it), "ignores a location with a non-finite coordinate", "renders no user marker when the location is null" / "when the prop is omitted", "draws exactly one user marker across the StrictMode effect cleanup cycle"
- [x] 2.3 `<div role="img">` labelled "Your approximate location (based on your network)", no popup, no `tabindex`. Found by accessible name in tests (`getByRole('img', { name })`, `getAllByLabelText`), not by class (§13.3). The MapLibre mock attaches marker elements to the map container on `addTo` and detaches them on `remove`, so a leaked marker shows up as a second DOM node
- [x] 2.4 The tests guard what they claim: 9 deliberate breakages, each failing at least one test. First round, 7 in `Map.tsx`: removal on `null` dropped; recreate without removing; depend on the array; a `<button>` element; no removal on unmount; label without "approximate"; marker stored in the event-marker map. After the review: halo pinned at 56px (fails the halo-size test); the state guard removed in the dashboard (fails the country-only and neither-name tests). Files restored after each
- [x] 2.5 Halo sized to a 50 km ground radius (`userLocationHalo.ts`, `proposal.md` B8). Unit tests (`userLocationHalo.test.ts`, 4): 512px-tile resolution, same ground size at every zoom (×2 per zoom level), 56px floor when zoomed out, paint cap. Map test "sizes the halo to a ground radius": about 165px at zoom 7, ×4 at zoom 9, 56px at zoom 3; "stops resizing the halo once the location is gone". In the browser: 164.6px at zoom 7 against an independent formula (164.6px); after wheel zooms it kept halo/pin-distance constant at 233.3 / 117.2 / 58.6px (expected 234.1 / 117.0 / 58.5) and held the 56.0px floor when zoomed out further (`halo-zoom7.png`, `halo-zoomed-in.png`, `halo-zoomed-out.png`, `halo-far-out.png`, `halo-farther-out.png`)

## 3. Styles

- [x] 3.1 Shape, not colour alone (§9.4): centre-anchored disc with a person glyph and a dashed halo, against bottom-anchored teardrop pins. Screenshots `marker-crop-motion.png`, `map-lagos-reduce.png`
- [x] 3.2 Colours as `--marker-user-*` tokens in `tokens.css`; `npm run lint:styles` passes with no colour literal outside `tokens.css`
- [x] 3.3 3:1 contrast (§9.5). The evidence is the floor: from the computed colours (disc `#f8fafc`, ring `#050714`), against any background the better of the two is at least 4.38:1. Informational only (cannot fail while the floor holds): sampled pixels next to the disc at Lagos, worst 7.41:1 over the basemap alone and 4.95:1 over the hazard pin under the marker
- [x] 3.4 Pulse only under `prefers-reduced-motion: no-preference`. Computed `animation-name`: `map-user-marker-pulse` under no-preference, `none` under reduce; dashed halo visible (opacity 1) in both
- [x] 3.5 Z-order via named properties in `index.css` (`--z-map-user-marker`, `--z-map-popup`). Measured with `elementFromPoint` at the marker centre: the marker paints over the hazard pin; an open popup paints over the marker; with the marker's real `pointer-events: none` the click reaches the hazard pin, and clicking there opened its popup
- [x] 3.6 Marker drawn at the context location: its centre equals the map centre after the fly-to, to 0.0px, at both motion settings

## 4. Dashboard (`web/src/components/EventsDashboard.tsx`)

- [x] 4.1 `userLocation` from `contextData.location`, only with a non-empty `state`, passed to `<Map>` (not memoised; `Map` depends on the numbers). Tests: Lagos → `data-user-location="3.3792,6.5244"` on the Map mock; country-only → no marker
- [x] 4.2 `role="status"` line outside the lazy map, always mounted. Tests: state and country ("Your approximate location: Lagos, Nigeria (based on your network)"), country only ("… Nigeria …", no marker), state only ("… Lagos …", marker), neither (empty, no marker), null context (empty, waiting on the resolved query data)
- [x] 4.3 No layout cost. Status line inside the visible map and clear of MapLibre's attribution at 1350x940, 1024x768, 900x800, 768x1024 and 375x812 (`layout-*.png`). Dashboard and map boxes identical to the control (`#dashboard` 1523px at 1350x940; map container 680x800; map 680x600)
- [x] 4.4 Two-arm CLS, 8 runs per cell, Lagos context, with the branch arm reading CLS only after the status chip has rendered: 1920x1600 control 0.0054 (8/8) vs branch 0.0054 (8/8); 1350x940 0.0000 vs 0.0000; 375x812 control 0.0001 ×8, branch 0.0001 ×7 / 0.0000 ×1 (0/8 shifting on both). Null context, 1920x1600 (first run): 0.0054 on both. The 0.0054 is the pre-existing freshness banner
- [x] 4.5 No axe violations: Map test "has no axe violations with the user marker and event markers on the map"; dashboard test "has no accessibility violations with a resolved location"

## 5. Verification (from `web/`)

- [x] 5.1 `npm run lint`: pass
- [x] 5.2 `npm run type-check`: pass
- [x] 5.3 `npm run lint:styles`: pass
- [x] 5.4 `npm run test`: 13 files, 131 tests pass (108 before this change; 13 new in `Map.test.tsx`, 6 new in `EventsDashboard.test.tsx`, 4 in the new `userLocationHalo.test.ts`)
- [x] 5.5 `npm run build`: pass. `web/src/data/milestones.json` unchanged
- [ ] 5.6 Manual check on staging with a Lagos `X-Forwarded-For` (proposal verification step 4): not possible locally; do it after the merge to `development` deploys
- [x] 5.7 Keyboard walkthrough (§9.12), **automated**, not by a person: `keyboard-walkthrough.mjs` pressed Tab from the top of the page with the Lagos context. Focus order (21 stops): skip link, home logo, For partners, GitHub (nav), Explore latest events, Contribute on GitHub, country / category / state comboboxes, the two event cards, the map canvas, the two hazard pins, the attribution toggle, the MapLibre link, then the footer's five links; then focus left the page (no trap). The user marker and the status line never took focus, and the order is identical to the control build's. A human pass with a screen reader is still needed for what is announced and whether focus is visible, which this does not check
- [ ] 5.8 Independent re-review of the fixes

## 6. Harness

- [x] 6.1 `scripts/bench-dashboard-cls/check-user-marker.mjs`: position, stacking, motion, halo size against zoom, contrast (floor checked, samples informational), absence, geometry and status-line placement, plus two pre-existing defects reported for information. `EXPECT=none` runs it against a control build. Branch run: 43 checks pass, 0 fail
- [x] 6.2 `measure-cls.mjs`: `CONTEXT_LOCATION=lagos` (the branch arm waits for `.dashboard-location-status__chip`, the control arm for the served `/v1/context` response) and `PW_CHANNEL`
- [x] 6.3 `scripts/bench-dashboard-cls/keyboard-walkthrough.mjs`: Tab order, no focus on the marker or the status line, no trap; diff against a control build
- [x] 6.4 README section with the commands and the 2026-11-26 results
