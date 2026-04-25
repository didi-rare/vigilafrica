import { useEffect, useMemo, useRef, useState } from 'react'
import maplibregl from 'maplibre-gl/dist/maplibre-gl-csp'
import { setWorkerUrl } from 'maplibre-gl/dist/maplibre-gl-csp'
import maplibreWorkerUrl from 'maplibre-gl/dist/maplibre-gl-csp-worker.js?url'
import 'maplibre-gl/dist/maplibre-gl.css'
import './Map.css'

setWorkerUrl(maplibreWorkerUrl)

const SOURCE_ID = 'events-map-source'
const CLUSTERS_LAYER_ID = 'events-map-clusters'
const CLUSTER_COUNT_LAYER_ID = 'events-map-cluster-count'

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
}

type EventsGeoJSON = GeoJSON.FeatureCollection<GeoJSON.Point, { id: string; title: string; category: string }>
type MarkerRecord = {
  marker: maplibregl.Marker
  event: EventMarker
}

function getMarkerVariant(category: string): 'flood' | 'fire' {
  return category === 'floods' ? 'flood' : 'fire'
}

function getMarkerGlyph(category: string): string {
  if (category === 'floods') {
    return `
      <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
        <path d="M4 10.75c1.17 0 1.76.49 2.27.91.46.38.8.66 1.55.66.74 0 1.08-.28 1.54-.66.51-.42 1.1-.91 2.27-.91s1.76.49 2.27.91c.46.38.8.66 1.54.66.75 0 1.09-.28 1.55-.66.51-.42 1.1-.91 2.27-.91v2.3c-.74 0-1.08.28-1.54.66-.51.42-1.1.91-2.28.91-1.17 0-1.76-.49-2.27-.91-.46-.38-.8-.66-1.54-.66-.75 0-1.09.28-1.55.66-.51.42-1.1.91-2.27.91s-1.76-.49-2.27-.91c-.46-.38-.8-.66-1.54-.66-.75 0-1.09.28-1.55.66-.51.42-1.1.91-2.27.91v-2.3c.74 0 1.08-.28 1.54-.66.51-.42 1.1-.91 2.28-.91Zm0 5.1c1.17 0 1.76.49 2.27.91.46.38.8.66 1.55.66.74 0 1.08-.28 1.54-.66.51-.42 1.1-.91 2.27-.91s1.76.49 2.27.91c.46.38.8.66 1.54.66.75 0 1.09-.28 1.55-.66.51-.42 1.1-.91 2.27-.91v2.3c-.74 0-1.08.28-1.54.66-.51.42-1.1.91-2.28.91-1.17 0-1.76-.49-2.27-.91-.46-.38-.8-.66-1.54-.66-.75 0-1.09.28-1.55.66-.51.42-1.1.91-2.27.91s-1.76-.49-2.27-.91c-.46-.38-.8-.66-1.54-.66-.75 0-1.09.28-1.55.66-.51.42-1.1.91-2.27.91v-2.3c.74 0 1.08-.28 1.54-.66.51-.42 1.1-.91 2.28-.91Z" fill="currentColor"/>
      </svg>
    `
  }

  return `
    <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <path d="M13.8 2.5c.36 1.73-.11 3.24-1.42 4.53-1.12 1.1-1.55 2.17-1.29 3.22.24.95.92 1.77 2.04 2.45-.09-1.45.31-2.62 1.22-3.51.71-.69 1.19-1.62 1.44-2.79 2.15 1.72 3.23 3.95 3.23 6.68 0 1.98-.66 3.67-1.98 5.07-1.32 1.4-3 2.1-5.04 2.1-1.98 0-3.64-.67-4.97-2.02C5.7 16.89 5.03 15.23 5.03 13.25c0-1.7.46-3.2 1.39-4.5.75-1.06 1.95-2.23 3.59-3.51.42 1.12.44 2.12.04 2.99-.21.48-.55.97-1 1.49-.67.76-.95 1.59-.84 2.5.09.72.43 1.37 1.02 1.95-.03-1.35.36-2.47 1.18-3.38.76-.84 1.21-1.58 1.35-2.22.11-.46.12-1.14.04-2.07Z" fill="currentColor"/>
    </svg>
  `
}

function createMarkerElement(event: EventMarker): HTMLButtonElement {
  const variant = getMarkerVariant(event.category)
  const button = document.createElement('button')
  button.type = 'button'
  button.className = `map-marker map-marker--${variant}`
  button.setAttribute('aria-label', `${event.title} (${event.category})`)
  button.innerHTML = `
    <span class="map-marker__pulse" aria-hidden="true"></span>
    <span class="map-marker__badge" aria-hidden="true">
      <span class="map-marker__glyph">${getMarkerGlyph(event.category)}</span>
    </span>
    <span class="map-marker__pointer" aria-hidden="true"></span>
  `

  return button
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

export function Map({ events, center = [8.6753, 9.082], zoom = 5 }: MapProps) {
  const mapContainer = useRef<HTMLDivElement>(null)
  const mapInstance = useRef<maplibregl.Map | null>(null)
  const markers = useRef<globalThis.Map<string, MarkerRecord>>(new globalThis.Map())
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
        // Required so the cluster-count symbol layer can render text glyphs.
        glyphs: 'https://demotiles.maplibre.org/font/{fontstack}/{range}.pbf',
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
          'text-font': ['Open Sans Bold'],
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

        const marker = new maplibregl.Marker({ element: el, anchor: 'bottom' })
          .setLngLat([event.lng, event.lat])
          .setPopup(new maplibregl.Popup({ offset: 20 }).setDOMContent(popupContent))
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
