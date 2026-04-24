import { useEffect, useRef, useState } from 'react'
import maplibregl from 'maplibre-gl/dist/maplibre-gl-csp'
import { setWorkerUrl } from 'maplibre-gl/dist/maplibre-gl-csp'
import maplibreWorkerUrl from 'maplibre-gl/dist/maplibre-gl-csp-worker.js?url'
import 'maplibre-gl/dist/maplibre-gl.css'
import './Map.css'

setWorkerUrl(maplibreWorkerUrl)

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

export function Map({ events, center = [8.6753, 9.082], zoom = 5 }: MapProps) {
  const mapContainer = useRef<HTMLDivElement>(null)
  const map = useRef<maplibregl.Map | null>(null)
  const markers = useRef<maplibregl.Marker[]>([])
  const [isLoaded, setIsLoaded] = useState(false)
  const initialCenter = useRef(center)
  const initialZoom = useRef(zoom)

  // Initialization - Run once per mount. Guard prevents StrictMode double-invoke (§12.3).
  useEffect(() => {
    if (map.current || !mapContainer.current) return

    map.current = new maplibregl.Map({
      container: mapContainer.current,
      style: {
        version: 8,
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

    map.current.on('load', () => {
      setIsLoaded(true)
    })

    return () => {
      map.current?.remove()
      map.current = null
      markers.current = []
    }
  }, [])

  // Fly-to when center changes (e.g., context loaded)
  useEffect(() => {
    if (!map.current || !isLoaded) return

    // Guard against invalid coordinates that would crash flyTo
    if (typeof center[0] !== 'number' || typeof center[1] !== 'number') return

    map.current.flyTo({ center, zoom: 7, speed: 0.8 })
  }, [center, isLoaded])

  // Marker Management
  useEffect(() => {
    if (!map.current || !isLoaded) return

    // Clear existing markers properly
    markers.current.forEach((marker) => marker.remove())
    markers.current = []

    events.forEach((event) => {
      const el = createMarkerElement(event)

      const popupContent = document.createElement('div')
      const title = document.createElement('h3')
      title.textContent = event.title
      popupContent.appendChild(title)

      const marker = new maplibregl.Marker({ element: el, anchor: 'bottom' })
        .setLngLat([event.lng, event.lat])
        .setPopup(new maplibregl.Popup({ offset: 20 }).setDOMContent(popupContent))
        .addTo(map.current!)

      markers.current.push(marker)
    })
  }, [events, isLoaded])

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
