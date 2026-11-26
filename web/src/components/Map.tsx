import { useEffect, useMemo, useRef, useState } from 'react'
// ⚠️ Imported explicitly rather than relying on the ambient `GeoJSON` namespace.
// maplibre-gl 5 pulled in @types/geojson transitively and made the global
// resolvable; v6 does not, and tsconfig.app.json restricts `types` to
// ["vite/client"], so `export as namespace GeoJSON` never applies here. An
// explicit import does not depend on either.
import type { FeatureCollection, Point } from 'geojson'
// ⚠️ maplibre-gl v6 NO LONGER SHIPS THE CSP BUILD. v5 provided
// dist/maplibre-gl-csp.js plus a separate csp-worker, which this file imported
// and wired up with setWorkerUrl. Neither file exists in v6 — the package is now
// ESM-only and bundles its worker, which Vite resolves.
//
// That is safe under our Content-Security-Policy because vercel.json already
// grants `worker-src 'self' blob:` and `child-src 'self' blob:`, which is what
// the standard build needs to start its worker. The CSP build was belt and
// braces, not a requirement of the policy we actually serve.
//
// ⚠️ v6 also has NO DEFAULT EXPORT — everything is named — so this is a namespace
// import. `import maplibregl from 'maplibre-gl'` silently yields `any` and every
// callback below loses its types.
import * as maplibregl from 'maplibre-gl'
// ⚠️ The worker URL MUST still be set explicitly, and dropping this silently
// breaks the map. v6 derives its worker path at RUNTIME from the main bundle's
// own URL — `new URL('./maplibre-gl-worker.mjs', <chunk url>)` — which the
// bundler cannot see, so Vite never emits the file and the request 404s. The
// map container and canvas still appear, so the failure is invisible to
// type-check, lint, unit tests and `npm run build`; only a browser shows it.
// ⚠️ `?worker&url`, NOT plain `?url`. v6 builds main and worker in one context
// and extracts a shared chunk, so the worker is not self-contained — it imports
// ./maplibre-gl-shared.mjs relatively. Plain `?url` copies the single file and
// that import 404s. `?worker&url` makes Vite BUNDLE the worker with its
// dependencies and return the URL of the result.
import maplibreWorkerUrl from 'maplibre-gl/dist/maplibre-gl-worker.mjs?worker&url'
import 'maplibre-gl/dist/maplibre-gl.css'
import { track } from '../analytics'
import { haloDiameterPx } from './userLocationHalo'
import './Map.css'

maplibregl.setWorkerUrl(maplibreWorkerUrl)

const SOURCE_ID = 'events-map-source'
const CLUSTERS_LAYER_ID = 'events-map-clusters'
const CLUSTER_COUNT_LAYER_ID = 'events-map-cluster-count'
const SVG_NS = 'http://www.w3.org/2000/svg'

interface EventMarker {
  id: string
  lat: number
  lng: number
  category: string
  title: string
}

interface MapProps {
  events: EventMarker[]
  center?: [number, number]
  zoom?: number
  /**
   * The visitor's IP-resolved location from `/v1/context`, as `[lng, lat]`
   * (MapLibre order, same as `center`). `null`/omitted means no location was
   * resolved, and no "you are here" marker is drawn.
   */
  userLocation?: readonly [number, number] | null
}

type EventsGeoJSON = FeatureCollection<Point, { id: string; title: string; category: string }>
type MarkerRecord = {
  marker: maplibregl.Marker
  event: EventMarker
}

function getMarkerVariant(category: string): 'flood' | 'fire' {
  return category === 'floods' ? 'flood' : 'fire'
}

function getMarkerGlyphPath(category: string): string {
  if (category === 'floods') {
    return 'M4 10.75c1.17 0 1.76.49 2.27.91.46.38.8.66 1.55.66.74 0 1.08-.28 1.54-.66.51-.42 1.1-.91 2.27-.91s1.76.49 2.27.91c.46.38.8.66 1.54.66.75 0 1.09-.28 1.55-.66.51-.42 1.1-.91 2.27-.91v2.3c-.74 0-1.08.28-1.54.66-.51.42-1.1.91-2.28.91-1.17 0-1.76-.49-2.27-.91-.46-.38-.8-.66-1.54-.66-.75 0-1.09.28-1.55.66-.51.42-1.1.91-2.27.91s-1.76-.49-2.27-.91c-.46-.38-.8-.66-1.54-.66-.75 0-1.09.28-1.55.66-.51.42-1.1.91-2.27.91v-2.3c.74 0 1.08-.28 1.54-.66.51-.42 1.1-.91 2.28-.91Zm0 5.1c1.17 0 1.76.49 2.27.91.46.38.8.66 1.55.66.74 0 1.08-.28 1.54-.66.51-.42 1.1-.91 2.27-.91s1.76.49 2.27.91c.46.38.8.66 1.54.66.75 0 1.09-.28 1.55-.66.51-.42 1.1-.91 2.27-.91v2.3c-.74 0-1.08.28-1.54.66-.51.42-1.1.91-2.28.91-1.17 0-1.76-.49-2.27-.91-.46-.38-.8-.66-1.54-.66-.75 0-1.09.28-1.55.66-.51.42-1.1.91-2.27.91s-1.76-.49-2.27-.91c-.46-.38-.8-.66-1.54-.66-.75 0-1.09.28-1.55.66-.51.42-1.1.91-2.27.91v-2.3c.74 0 1.08-.28 1.54-.66.51-.42 1.1-.91 2.28-.91Z'
  }

  return 'M13.8 2.5c.36 1.73-.11 3.24-1.42 4.53-1.12 1.1-1.55 2.17-1.29 3.22.24.95.92 1.77 2.04 2.45-.09-1.45.31-2.62 1.22-3.51.71-.69 1.19-1.62 1.44-2.79 2.15 1.72 3.23 3.95 3.23 6.68 0 1.98-.66 3.67-1.98 5.07-1.32 1.4-3 2.1-5.04 2.1-1.98 0-3.64-.67-4.97-2.02C5.7 16.89 5.03 15.23 5.03 13.25c0-1.7.46-3.2 1.39-4.5.75-1.06 1.95-2.23 3.59-3.51.42 1.12.44 2.12.04 2.99-.21.48-.55.97-1 1.49-.67.76-.95 1.59-.84 2.5.09.72.43 1.37 1.02 1.95-.03-1.35.36-2.47 1.18-3.38.76-.84 1.21-1.58 1.35-2.22.11-.46.12-1.14.04-2.07Z'
}

function createHiddenSpan(className: string): HTMLSpanElement {
  const span = document.createElement('span')
  span.className = className
  span.setAttribute('aria-hidden', 'true')
  return span
}

function createGlyph(category: string): SVGSVGElement {
  const svg = document.createElementNS(SVG_NS, 'svg')
  svg.setAttribute('viewBox', '0 0 24 24')
  svg.setAttribute('aria-hidden', 'true')
  svg.setAttribute('focusable', 'false')

  const path = document.createElementNS(SVG_NS, 'path')
  path.setAttribute('d', getMarkerGlyphPath(category))
  path.setAttribute('fill', 'currentColor')
  svg.appendChild(path)

  return svg
}

function createMarkerElement(event: EventMarker): HTMLButtonElement {
  const variant = getMarkerVariant(event.category)
  const button = document.createElement('button')
  button.type = 'button'
  button.className = `map-marker map-marker--${variant}`
  button.setAttribute('aria-label', `${event.title} (${event.category})`)

  const pulse = createHiddenSpan('map-marker__pulse')
  const badge = createHiddenSpan('map-marker__badge')
  const glyph = document.createElement('span')
  glyph.className = 'map-marker__glyph'
  glyph.appendChild(createGlyph(event.category))
  badge.appendChild(glyph)
  const pointer = createHiddenSpan('map-marker__pointer')

  button.append(pulse, badge, pointer)

  return button
}

// The accessible name of the "you are here" marker. It says "approximate" and
// names the source on purpose: the position comes from IP geolocation, which is
// city- or region-level at best, and nothing here may imply GPS precision
// (feature-current-location-marker R1).
const USER_LOCATION_LABEL = 'Your approximate location (based on your network)'

// A person silhouette (head + shoulders): the shape, not the colour, is what
// separates this marker from the flame and wave hazard glyphs (§9.4).
const USER_GLYPH_PATH = 'M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8Zm0 2c-4.42 0-8 2.24-8 5v1h16v-1c0-2.76-3.58-5-8-5Z'

// The user marker is NOT a <button> (§9.1): it opens no popup and does nothing
// on click, so an interactive role would be a lie. It is a labelled image. Its
// children are decorative and hidden; the label on the root is the whole
// accessible surface.
function createUserLocationElement(): HTMLDivElement {
  const root = document.createElement('div')
  root.className = 'map-user-marker'
  root.setAttribute('role', 'img')
  root.setAttribute('aria-label', USER_LOCATION_LABEL)

  // Static "approximate area" halo, sized to a ground radius by the halo
  // effect in Map. It stays visible under reduced motion; only the pulse ring
  // below, a pixel-sized attention cue around the dot, is animated.
  const halo = createHiddenSpan('map-user-marker__halo')
  const pulse = createHiddenSpan('map-user-marker__pulse')
  const dot = createHiddenSpan('map-user-marker__dot')

  const svg = document.createElementNS(SVG_NS, 'svg')
  svg.setAttribute('viewBox', '0 0 24 24')
  svg.setAttribute('aria-hidden', 'true')
  svg.setAttribute('focusable', 'false')
  const path = document.createElementNS(SVG_NS, 'path')
  path.setAttribute('d', USER_GLYPH_PATH)
  path.setAttribute('fill', 'currentColor')
  svg.appendChild(path)
  dot.appendChild(svg)

  root.append(halo, pulse, dot)
  return root
}

function buildEventsGeoJSON(events: readonly EventMarker[]): EventsGeoJSON {
  return {
    type: 'FeatureCollection',
    features: events.map((event) => ({
      type: 'Feature',
      geometry: { type: 'Point', coordinates: [event.lng, event.lat] },
      properties: { id: event.id, title: event.title, category: event.category },
    })),
  }
}

function markerMatchesEvent(record: MarkerRecord, event: EventMarker): boolean {
  return record.event.lat === event.lat &&
    record.event.lng === event.lng &&
    record.event.category === event.category &&
    record.event.title === event.title
}

export function Map({ events, center = [8.6753, 9.082], zoom = 5, userLocation = null }: MapProps) {
  const mapContainer = useRef<HTMLDivElement>(null)
  const mapInstance = useRef<maplibregl.Map | null>(null)
  const markers = useRef<globalThis.Map<string, MarkerRecord>>(new globalThis.Map())
  // The single "you are here" marker. Held apart from `markers` on purpose: it
  // is not an event, is not in the clustered source, and must never be culled
  // by syncMarkers (feature-current-location-marker §2).
  const userMarker = useRef<maplibregl.Marker | null>(null)
  // The marker effect depends on these primitives, not on the `userLocation`
  // array, so a caller passing a fresh `[lng, lat]` literal each render does not
  // churn the DOM marker (§8.8).
  const userLng = userLocation && Number.isFinite(userLocation[0]) ? userLocation[0] : null
  const userLat = userLocation && Number.isFinite(userLocation[1]) ? userLocation[1] : null
  const [isLoaded, setIsLoaded] = useState(false)
  const initialCenter = useRef(center)
  const initialZoom = useRef(zoom)

  // §12.8 — memoize GeoJSON construction so setData only runs when events change
  const geojson = useMemo(() => buildEventsGeoJSON(events), [events])
  const eventsById = useMemo(() => {
    const map = new globalThis.Map<string, EventMarker>()
    for (const event of events) {
      map.set(event.id, event)
    }
    return map
  }, [events])

  // Initialization — run once per mount. Guard against StrictMode double-invoke (§12.3).
  useEffect(() => {
    if (mapInstance.current || !mapContainer.current) return

    const instance = new maplibregl.Map({
      container: mapContainer.current,
      style: {
        version: 8,
        // MapLibre GL JS 5.11+ renders text from local fonts when glyphs is omitted.
        // This keeps cluster labels visible without remote glyph 404s.
        sources: {
          'map-osm': {
            type: 'raster',
            tiles: ['https://tile.openstreetmap.org/{z}/{x}/{y}.png'],
            tileSize: 256,
            attribution: '© OpenStreetMap contributors',
          },
          'map-satellite': {
            type: 'raster',
            tiles: ['https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}'],
            tileSize: 256,
            attribution: '© ESRI World Imagery',
          },
        },
        layers: [
          {
            id: 'map-satellite-layer',
            type: 'raster',
            source: 'map-satellite',
            paint: {
              'raster-brightness-max': 0.6,
              'raster-saturation': -0.4,
              'raster-contrast': 0.2,
            },
          },
        ],
      },
      center: initialCenter.current,
      zoom: initialZoom.current,
    })
    mapInstance.current = instance
    // Capture the current markers collection so cleanup operates on the same
    // Map instance that was populated during this effect's lifetime.
    const markersMap = markers.current

    instance.on('load', () => {
      instance.addSource(SOURCE_ID, {
        type: 'geojson',
        data: { type: 'FeatureCollection', features: [] },
        cluster: true,
        clusterMaxZoom: 14,
        clusterRadius: 50,
      })

      instance.addLayer({
        id: CLUSTERS_LAYER_ID,
        type: 'circle',
        source: SOURCE_ID,
        filter: ['has', 'point_count'],
        paint: {
          'circle-color': [
            'step',
            ['get', 'point_count'],
            '#F5A623',
            10,
            '#E8622A',
            50,
            '#C0392B',
          ],
          'circle-radius': [
            'step',
            ['get', 'point_count'],
            20,
            10,
            28,
            50,
            36,
          ],
          'circle-stroke-width': 2,
          'circle-stroke-color': 'rgba(255, 255, 255, 0.6)',
          'circle-opacity': 0.85,
        },
      })

      instance.addLayer({
        id: CLUSTER_COUNT_LAYER_ID,
        type: 'symbol',
        source: SOURCE_ID,
        filter: ['has', 'point_count'],
        layout: {
          'text-field': ['get', 'point_count_abbreviated'],
          'text-font': [
            'Arial Unicode MS Bold', 'Arial Bold',
            'Noto Sans Bold', 'Roboto Bold', 'DejaVu Sans Bold',
            'sans-serif',
          ],
          'text-size': 13,
        },
        paint: {
          'text-color': '#ffffff',
        },
      })

      instance.on('click', CLUSTERS_LAYER_ID, (e) => {
        const features = instance.queryRenderedFeatures(e.point, { layers: [CLUSTERS_LAYER_ID] })
        if (features.length === 0) return
        const feature = features[0]
        const clusterId = feature.properties?.cluster_id
        if (typeof clusterId !== 'number') return
        const source = instance.getSource(SOURCE_ID) as maplibregl.GeoJSONSource | undefined
        if (!source) return
        source.getClusterExpansionZoom(clusterId).then((nextZoom) => {
          const geometry = feature.geometry
          if (geometry.type !== 'Point') return
          instance.easeTo({
            center: geometry.coordinates as [number, number],
            zoom: nextZoom,
          })
        }).catch(() => {
          // Cluster no longer exists (e.g. data changed between click and response) — ignore.
        })
      })

      instance.on('mouseenter', CLUSTERS_LAYER_ID, () => {
        instance.getCanvas().style.cursor = 'pointer'
      })
      instance.on('mouseleave', CLUSTERS_LAYER_ID, () => {
        instance.getCanvas().style.cursor = ''
      })

      setIsLoaded(true)
    })

    return () => {
      markersMap.forEach((record) => record.marker.remove())
      markersMap.clear()
      // Read at cleanup time on purpose: the marker is created by a later
      // effect, after this one has run, so there is nothing to capture here.
      userMarker.current?.remove()
      userMarker.current = null

      if (instance.getLayer(CLUSTER_COUNT_LAYER_ID)) instance.removeLayer(CLUSTER_COUNT_LAYER_ID)
      if (instance.getLayer(CLUSTERS_LAYER_ID)) instance.removeLayer(CLUSTERS_LAYER_ID)
      if (instance.getSource(SOURCE_ID)) instance.removeSource(SOURCE_ID)

      instance.remove()
      mapInstance.current = null
    }
  }, [])

  // Fly-to when center prop changes (e.g., context loaded).
  useEffect(() => {
    const instance = mapInstance.current
    if (!instance || !isLoaded) return

    if (typeof center[0] !== 'number' || typeof center[1] !== 'number') return
    instance.flyTo({ center, zoom: 7, speed: 0.8 })
  }, [center, isLoaded])

  // "You are here" marker: create it once the map has loaded and a location is
  // known, move it when the location changes, remove it when the location goes
  // away. There is never more than one (feature-current-location-marker R2).
  // Unmount is handled by the initialization effect's cleanup.
  useEffect(() => {
    const instance = mapInstance.current
    if (!instance || !isLoaded) return

    if (userLng === null || userLat === null) {
      userMarker.current?.remove()
      userMarker.current = null
      return
    }

    const lngLat: [number, number] = [userLng, userLat]
    if (userMarker.current) {
      userMarker.current.setLngLat(lngLat)
      return
    }

    userMarker.current = new maplibregl.Marker({
      element: createUserLocationElement(),
      // Centre-anchored: the dot sits ON the point, unlike the bottom-anchored
      // hazard pins, which is one more non-colour difference between them.
      anchor: 'center',
    })
      .setLngLat(lngLat)
      .addTo(instance)
  }, [userLng, userLat, isLoaded])

  // Size the approximate-area halo to a fixed ground radius (userLocationHalo.ts)
  // rather than a fixed pixel size, so zooming in widens it instead of shrinking
  // it into something that reads like a GPS accuracy circle. Ground resolution
  // depends only on the zoom and the MARKER's latitude, so it is recomputed on
  // `zoom` (which fires every frame of a zoom or fly-to) and on `resize` (the
  // paint cap follows the map's size); a pan does not change it. A new latitude
  // re-runs this effect. Written as a CSS custom property on the marker element:
  // a truly dynamic value (§7.4).
  useEffect(() => {
    const instance = mapInstance.current
    if (!instance || !isLoaded || userLat === null) return

    const lat = userLat
    const update = () => {
      const element = userMarker.current?.getElement()
      if (!element) return
      const container = instance.getContainer()
      const longestSide = Math.max(container.clientWidth, container.clientHeight)
      const diameter = haloDiameterPx({ zoom: instance.getZoom(), lat, maxPx: 2 * longestSide })
      element.style.setProperty('--map-user-halo-size', `${diameter.toFixed(1)}px`)
    }

    update()
    instance.on('zoom', update)
    instance.on('resize', update)
    return () => {
      instance.off('zoom', update)
      instance.off('resize', update)
    }
  }, [userLat, isLoaded])

  // Feed data into the clustering source + keep DOM markers in sync with
  // unclustered, in-viewport points (§12.7).
  useEffect(() => {
    const instance = mapInstance.current
    if (!instance || !isLoaded) return

    const source = instance.getSource(SOURCE_ID) as maplibregl.GeoJSONSource | undefined
    if (!source) return
    source.setData(geojson)

    const syncMarkers = () => {
      const map = mapInstance.current
      if (!map) return
      const features = map.querySourceFeatures(SOURCE_ID)
      const unclusteredIds = new Set<string>()
      for (const feature of features) {
        if (feature.properties?.cluster) continue
        const id = feature.properties?.id
        if (typeof id === 'string' && eventsById.has(id)) {
          unclusteredIds.add(id)
        }
      }

      for (const [id, record] of markers.current) {
        if (!unclusteredIds.has(id)) {
          record.marker.remove()
          markers.current.delete(id)
        }
      }

      for (const id of unclusteredIds) {
        const event = eventsById.get(id)
        if (!event) continue

        const existing = markers.current.get(id)
        if (existing && markerMatchesEvent(existing, event)) continue
        if (existing) {
          existing.marker.remove()
          markers.current.delete(id)
        }

        const el = createMarkerElement(event)
        const popupContent = document.createElement('div')
        const title = document.createElement('h3')
        title.textContent = event.title
        popupContent.appendChild(title)

        const popup = new maplibregl.Popup({ offset: 20 }).setDOMContent(popupContent)
        // Engagement-depth signal: fire when the marker popup actually opens
        // (proposal event `map_marker_clicked`). Using the popup 'open' event
        // rather than a raw marker click means we count opens, not toggles.
        popup.on('open', () => {
          track('map_marker_clicked', { event_id: event.id, category: event.category })
        })

        const marker = new maplibregl.Marker({ element: el, anchor: 'bottom' })
          .setLngLat([event.lng, event.lat])
          .setPopup(popup)
          .addTo(map)
        markers.current.set(id, { marker, event })
      }
    }

    const handleSourceData = (e: maplibregl.MapSourceDataEvent) => {
      if (e.sourceId !== SOURCE_ID || !e.isSourceLoaded) return
      syncMarkers()
    }

    syncMarkers()
    instance.on('moveend', syncMarkers)
    instance.on('zoomend', syncMarkers)
    instance.on('sourcedata', handleSourceData)

    return () => {
      instance.off('moveend', syncMarkers)
      instance.off('zoomend', syncMarkers)
      instance.off('sourcedata', handleSourceData)
    }
  }, [geojson, eventsById, isLoaded])

  return (
    <div className="map-wrapper glass-effect">
      <div ref={mapContainer} className="map-container" />
      <div className="map-hud-overlay">
        <span className="hud-label">SENTINEL RADAR v0.4</span>
        <div className="hud-status">
          <span className="status-dot online"></span> SYSTEM LIVE
        </div>
      </div>
    </div>
  )
}
