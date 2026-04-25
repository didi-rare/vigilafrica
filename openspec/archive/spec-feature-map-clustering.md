# Spec: Map Performance & Clustering (feature-map-clustering)

## Context
The React frontend uses MapLibre GL JS with individual `maplibregl.Marker` DOM elements per event. At scale (1,000+ points) this degrades into browser lockups. We need native WebGL clustering for low zoom levels while preserving the existing flood/fire SVG button markers at high zoom.

## Architecture: Hybrid Source-Driven Marker Sync

We use a **hybrid approach**: a hidden GeoJSON source (with `cluster: true`) drives two things simultaneously:

1. **Cluster circles** — rendered as native MapLibre WebGL layers (circle + symbol). Fast, GPU-accelerated, handles thousands of points.
2. **Unclustered DOM markers** — the existing `maplibregl.Marker` flood/fire button elements, but only mounted for points that are currently unclustered in the viewport.

A `syncMarkers()` function (called on `zoomend` and `moveend`) calls `map.querySourceFeatures('events-source')`, filters for features without `point_count` (i.e., unclustered), diffs against currently mounted markers, and adds/removes only what changed. This avoids both bulk teardown and invisible-but-focusable markers.

### Performance Characteristics
- `querySourceFeatures` is synchronous and sub-millisecond — reads MapLibre's internal tile index.
- DOM work only runs on `moveend`/`zoomend` (user stops interacting), not per-frame.
- At any viewport-realistic zoom, unclustered visible points are < 50 — far fewer DOM nodes than the current approach which mounts all events unconditionally.

## Components to Touch
1. `web/src/components/Map.tsx` — main implementation
2. `web/src/components/Map.test.tsx` — existing marker tests will break; full rewrite required

## Implementation Plan

### 1. Add GeoJSON Source
In the `map.on('load')` callback, call `map.addSource('events-source', { ... })` with:
```
type: 'geojson',
cluster: true,
clusterMaxZoom: 14,
clusterRadius: 50,
data: { type: 'FeatureCollection', features: [] }
```
The source starts empty; data is set via `map.getSource('events-source').setData(...)` in `syncMarkers()`.

### 2. Add Cluster Layers
After the source, add two layers:

**`clusters` layer** (circle, filtered `has point_count`):
- Color via `step` on `point_count`: `< 10` → `#F5A623` (amber), `< 50` → `#E8622A` (orange), `≥ 50` → `#C0392B` (red)
- Radius via `step`: `< 10` → 20px, `< 50` → 28px, `≥ 50` → 36px
- Stroke: 2px white, 0.5 opacity

**`cluster-count` layer** (symbol, same filter):
- `text-field`: `['get', 'point_count_abbreviated']`
- `text-font`: `['Open Sans Bold']`
- `text-size`: 13
- `text-color`: white

### 3. Implement `syncMarkers()`
```ts
function syncMarkers(
  map: maplibregl.Map,
  events: EventMarker[],
  markersRef: React.MutableRefObject<Map<string, maplibregl.Marker>>
) {
  // 1. Build lookup: id → EventMarker
  const eventById = new Map(events.map(e => [e.id, e]))

  // 2. Update GeoJSON source
  const source = map.getSource('events-source') as maplibregl.GeoJSONSource
  source.setData({
    type: 'FeatureCollection',
    features: events.map(e => ({
      type: 'Feature',
      geometry: { type: 'Point', coordinates: [e.lng, e.lat] },
      properties: { id: e.id, title: e.title, category: e.category },
    })),
  })

  // 3. Get unclustered point IDs currently in viewport
  const unclusteredIds = new Set(
    map.querySourceFeatures('events-source', { sourceLayer: '' })
      .filter(f => !f.properties?.point_count)
      .map(f => String(f.properties?.id))
      .filter(id => eventById.has(id))
  )

  // 4. Remove markers no longer unclustered
  for (const [id, marker] of markersRef.current) {
    if (!unclusteredIds.has(id)) {
      marker.remove()
      markersRef.current.delete(id)
    }
  }

  // 5. Add markers for newly unclustered points
  for (const id of unclusteredIds) {
    if (!markersRef.current.has(id)) {
      const event = eventById.get(id)!
      const el = createMarkerElement(event)
      const popupContent = document.createElement('div')
      const title = document.createElement('h3')
      title.textContent = event.title
      popupContent.appendChild(title)
      const marker = new maplibregl.Marker({ element: el, anchor: 'bottom' })
        .setLngLat([event.lng, event.lat])
        .setPopup(new maplibregl.Popup({ offset: 20 }).setDOMContent(popupContent))
        .addTo(map)
      markersRef.current.set(id, marker)
    }
  }
}
```

Change `markers` ref type from `maplibregl.Marker[]` to `Map<string, maplibregl.Marker>`.

### 4. Wire `syncMarkers()` into the Map
- Call `syncMarkers()` inside the existing `events` + `isLoaded` `useEffect` (replaces the current bulk marker loop).
- Register `map.on('zoomend', () => syncMarkers(...))` and `map.on('moveend', () => syncMarkers(...))` in the load handler.

### 5. Cluster Click Handler
```ts
map.on('click', 'clusters', (e) => {
  const features = map.queryRenderedFeatures(e.point, { layers: ['clusters'] })
  const clusterId = features[0].properties?.cluster_id
  const source = map.getSource('events-source') as maplibregl.GeoJSONSource
  source.getClusterExpansionZoom(clusterId, (err, zoom) => {
    if (err || zoom == null) return
    map.easeTo({ center: (features[0].geometry as GeoJSON.Point).coordinates as [number, number], zoom })
  })
})
map.on('mouseenter', 'clusters', () => { map.getCanvas().style.cursor = 'pointer' })
map.on('mouseleave', 'clusters', () => { map.getCanvas().style.cursor = '' })
```

### 6. Cleanup
In the `useEffect` cleanup function, also call `map.removeLayer('clusters')`, `map.removeLayer('cluster-count')`, and `map.removeSource('events-source')` before `map.remove()`.

## Acceptance Criteria
- [ ] Map displays clustered circles (amber/orange/red) with numerical counts when zoomed out.
- [ ] Clicking a cluster zooms in to expand it.
- [ ] Flood/fire SVG button markers with popups render for unclustered individual events at high zoom.
- [ ] `querySourceFeatures` diff is used — markers not in the current viewport are removed from the DOM.
- [ ] The browser does not freeze when 5,000+ points are loaded.
- [ ] `Map.test.tsx` updated — all existing tests pass under the new architecture.

## Verification Plan
1. Insert 2,000+ mock events via `api:seed` or direct DB mutation.
2. Load the dashboard at zoom 5 — confirm a single large red cluster renders.
3. Click the cluster and verify smooth zoom-in expansion.
4. Zoom to level 15+ over a dense area — confirm flood/fire button markers appear with working popups.
5. Pan away from markers — confirm they are removed from the DOM (`document.querySelectorAll('.map-marker')` returns 0).
