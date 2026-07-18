---
id: feature-current-location-marker
status: proposed
branch: tbd
---

# Proposal: "You Are Here" Current-Location Marker on the Map (feature-current-location-marker)

## Why

The map already **flies the camera** to the visitor's IP-resolved location on load ([`web/src/components/EventsDashboard.tsx:247-255`](../../web/src/components/EventsDashboard.tsx) computes `mapCenter` from `contextData.location`, and [`web/src/components/Map.tsx:271-278`](../../web/src/components/Map.tsx) `flyTo`s there), but it draws **no marker** at that point. The user sees the map re-center on "somewhere near me" with no indication of *where* they are relative to the hazard markers around them.

For an awareness tool whose core question is "what's happening **near me**", a visible "you are here" anchor is a small change with outsized orientation value: it turns an unexplained camera move into a legible "this is you, these fires/floods are around you." It also makes the existing `/v1/context` resolution *visible* to the user, which currently only manifests as a silent re-center.

Surfaced by the maintainer on 2026-07-18 while reviewing the `/v1/context` "near me" behaviour during the post-v1.3.0 verification pass.

## Non-goals up front (read this first)

- **This does NOT add browser GPS.** Location stays **IP-derived, server-side** via `/v1/context` (there is no `navigator.geolocation` anywhere in the frontend today, by design). No geolocation permission prompt, no new PII, consistent with the project's stated privacy posture (no cookies/PII/consent banner). Adding real GPS is a separate proposal with its own consent + privacy review — see *Out of Scope*.
- Because the position is IP-derived, it is **approximate (city/state level, sometimes worse)**. The marker and its copy MUST communicate approximation and MUST NOT imply pinpoint accuracy — this is a safety-adjacent product with a standing "confirm with authorities" disclaimer. Honesty about precision is a requirement, not a nicety.

## What Changes

Frontend only (`web/src/`). No API, ingestion, or schema changes — the data (`location.lat`/`location.lng`) is already fetched and cached under the TanStack Query key `['context']`.

### 1. Pass the resolved location into the map

- Extend `MapProps` ([`Map.tsx:24-28`](../../web/src/components/Map.tsx)) with an optional `userLocation?: [number, number] | null` (`[lng, lat]`, matching MapLibre's coordinate order and the existing `center` convention).
- In `EventsDashboard.tsx`, derive it from `contextData.location` (reusing the same `lat`/`lng` guard already used for `mapCenter` at lines 251-255) and pass `userLocation={...}` to `<Map>` ([`EventsDashboard.tsx:417-421`](../../web/src/components/EventsDashboard.tsx)). Null when context has no location (localhost / lookup failure) → no marker.
- **Keep the prop reference-stable (§8.8):** a `userLocation={[lng, lat]}` literal mints a new array every render, which would re-run the marker effect (§2) on each render and churn the DOM marker. Either `useMemo` the tuple (`[location.lng, location.lat]`) or have the marker effect depend on the primitive `lng`/`lat` values, not the array reference.

### 2. Render a distinct "self" marker

- Add a dedicated marker for the user location as its **own** `maplibregl.Marker`, created/updated/removed in a small `useEffect` keyed on `userLocation` — **outside** the clustered `events-map-source` and the `syncMarkers` effect ([`Map.tsx:282-358`](../../web/src/components/Map.tsx)), which only manages event-source features. This keeps it from being clustered or culled with hazard markers.
- Build its element following the existing `createMarkerElement` recipe ([`Map.tsx:69-87`](../../web/src/components/Map.tsx)) — a new factory (e.g. `createUserLocationElement`) or a `.map-marker--user` variant. **But it is NOT a `<button>` (§9.1):** the existing hazard markers are buttons because they open popups; the user marker has no popup or interaction, so a `<button>` would be a non-interactive element with a misleading interactive role. Use a non-interactive element (`<div role="img">` with an `aria-label`), or mark it `aria-hidden` and convey the location in the accessible alternative (§3, §12.11).
- **Differentiate by shape/icon, not colour alone (§9.4):** it must be distinguishable from the hazard variants (`--fire` = lime, `--flood` = sky) by its **glyph/shape**, not merely a new colour token. Recommended: a person/dot glyph with a soft "approximate area" halo rather than a sharp pinpoint, to signal imprecision.
- Add `--marker-user-*` custom properties to [`web/src/styles/tokens.css`](../../web/src/styles/tokens.css) (mirroring the `--marker-fire-*` / `--marker-flood-*` blocks at 162-168) and the corresponding `.map-marker--user` rules in [`Map.css`](../../web/src/components/Map.css).

### 3. Accessibility & copy

- The marker element carries an `aria-label` like **"Your approximate location (based on your network)"**. If it has a popup/tooltip, the copy reiterates that it's approximate and network-derived.
- **Convey the location in the non-map accessible alternative (§12.11):** the map canvas already pairs with a visible event list; a screen-reader user on that list otherwise never learns where "here" is. Surface the resolved `location.state`/`location.country` (already available in `contextData`) in accessible text near the list/map — e.g. a `role="status"` line like "Showing events near {state}, {country} (approximate)".
- Respect `prefers-reduced-motion` for any pulse animation (the existing `marker-pulse` keyframes at [`Map.css:138-153`](../../web/src/components/Map.css) should be gated for reduced-motion in this variant).

### 4. (Optional, decide during design) minor related tidy

- `flyTo` hard-codes `zoom: 7` on re-center ([`Map.tsx:277`](../../web/src/components/Map.tsx)), ignoring the `zoom` prop. Not required for this feature; note it as an optional adjacent cleanup, not bundled unless trivial.

## Out of Scope

- **Browser GPS / `navigator.geolocation`.** Deliberately excluded — it introduces a permission prompt and precise-location PII that the current privacy posture avoids. A separate `feature-precise-geolocation-optin` proposal would own consent UX, the privacy note, and fallback-to-IP behaviour.
- **Wiring `/v1/context` `nearby_events`.** That field is fetched but currently unused in the UI ([`ContextResponse`](../../web/src/api/events.ts) `nearby_events`); surfacing a "near you" list/panel is a separate feature. This proposal only adds the marker; map events keep coming from the existing `fetchEvents` query.
- **A distance/"X km from you" readout** on event cards or popups — a reasonable follow-up once the anchor exists, not part of this.
- **Manual location override** (letting a user drop/adjust their own pin) — future.
- **Changing the centering logic** — the priority order (selected-country > context location > Nigeria default) stays as-is; this only adds a marker at the context location when one exists.

## Capabilities

### New Capabilities
- `map-user-location-marker`: renders a distinct, accessibility-labelled "approximate location" marker at the IP-resolved `/v1/context` location, when present.

### Modified Capabilities
- None. Centering, event markers, clustering, and the `context_resolve` analytics event are all unchanged. (`context_resolve` already fires at [`EventsDashboard.tsx:223-231`](../../web/src/components/EventsDashboard.tsx); no new analytics event is required, though a future `user_marker_shown` could be considered if adoption signal is wanted.)

## Acceptance Criteria

- [ ] When `/v1/context` returns a non-null `location`, a single distinct marker renders at `[location.lng, location.lat]`, visually differentiated from fire/flood markers.
- [ ] When `location` is null (localhost / lookup failure), **no** user marker renders and the map behaves exactly as today (centers on Nigeria default).
- [ ] The user marker is not clustered with, or culled alongside, event markers (it lives outside `events-map-source` / `syncMarkers`).
- [ ] The marker has an `aria-label` conveying it is an **approximate**, network-derived location; any tooltip/popup copy says the same. Nothing implies GPS-level precision.
- [ ] The marker is a **non-interactive** element (`role="img"`, not a `<button>`) since it has no popup/action (§9.1).
- [ ] The marker is differentiated from hazard markers by **shape/glyph**, not colour alone (§9.4), and meets **3:1 contrast** against the dark satellite basemap (§9.5).
- [ ] The resolved location is conveyed in the **non-map accessible alternative** — accessible text (e.g. `role="status"`) names the approximate state/country (§12.11).
- [ ] Pulse/animation respects `prefers-reduced-motion`.
- [ ] Marker colors/tokens are defined in `tokens.css` (no hard-coded colors in `Map.tsx`/`Map.css`), consistent with the existing token system.
- [ ] Updating `userLocation` (e.g. context refetch) moves/removes the single marker without leaking duplicate DOM markers across re-renders.
- [ ] `npm run build`, `npm run test` (vitest), and `vitest-axe` accessibility assertions pass; no new CSP violations (the marker is same-origin DOM/CSS, no external assets).
- [ ] No regression to existing event-marker rendering, clustering, popups, or `map_marker_clicked`.

## Risks

- **R1 — False precision / safety implication.** A crisp pin could read as "you are exactly here," misleading in an emergency-awareness context when the source is coarse IP geolocation. *Mitigation:* approximate-halo styling + explicit "approximate (based on your network)" copy; this is an acceptance criterion, not optional.
- **R2 — Marker lifecycle leaks.** Imperative MapLibre markers must be explicitly removed; a naive effect could accumulate duplicates on refetch. *Mitigation:* single ref-held marker, updated-or-removed in a cleanup-aware effect; covered by a test asserting exactly one user marker after location changes.
- **R3 — Visual collision with a nearby hazard marker.** If the user's location coincides with an event, markers may overlap. *Mitigation:* distinct size + z-order for the user marker; acceptable overlap for the prototype, revisit if it obscures events. The z-order MUST be a **named CSS custom property** in `index.css` (§7.10), never a hardcoded `z-index`.
- **R4 — Scope creep toward GPS / nearby-events panel.** *Mitigation:* Out-of-Scope draws the line; those are separate proposals.

## Verification Plan

1. Unit/component (vitest + Testing Library): with a mocked `['context']` query returning a location, assert one marker is present **queried by its accessible label** (`getByLabelText(/your approximate location/i)`), not by CSS class (§13.3); with null location, assert none.
2. Accessibility (vitest-axe): the marker exposes the approximate-location label; no a11y violations introduced.
3. Lifecycle: change the mocked location and assert the marker count stays at one (no duplicates), and removing the location removes the marker.
4. Manual (staging, `X-Forwarded-For` for a Lagos IP like the session's `105.112.0.1`): confirm the marker appears near Lagos, is visually distinct from event markers, and the tooltip/label reads as approximate. Confirm on a localhost run that no marker appears.
5. Reduced-motion: with `prefers-reduced-motion: reduce`, confirm the pulse is disabled.

## Origin

Maintainer feature idea raised 2026-07-18 during the `/v1/context` review in the post-v1.3.0 verification pass — the observation that the map re-centers on the resolved location but never shows the user where "here" is. Builds directly on the existing IP-context resolution (v0.4 GeoIP), the MapLibre marker system, and the `context_resolve` analytics already in place. Frontend-only; touches `web/src/`, so this OpenSpec record satisfies the Sentinel gate for implementation.
