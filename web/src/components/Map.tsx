import { useEffect, useRef, useState } from 'react'
import maplibregl from 'maplibre-gl'
import 'maplibre-gl/dist/maplibre-gl.css'
import './Map.css'

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

export function Map({ events, center = [8.6753, 9.082], zoom = 5 }: MapProps) {
  const mapContainer = useRef<HTMLDivElement>(null)
  const map = useRef<maplibregl.Map | null>(null)
  const markers = useRef<maplibregl.Marker[]>([])
  const [isLoaded, setIsLoaded] = useState(false)
  const initialCenter = useRef(center)
  const initialZoom = useRef(zoom)

  // Initialization - Run only once; uses refs for initial center/zoom to satisfy exhaustive-deps
  useEffect(() => {
    if (!mapContainer.current) return

    map.current = new maplibregl.Map({
      container: mapContainer.current,
      style: {
        version: 8,
        sources: {
          'osm': {
            type: 'raster',
            tiles: ['https://tile.openstreetmap.org/{z}/{x}/{y}.png'],
            tileSize: 256,
            attribution: '© OpenStreetMap contributors'
          },
          'satellite': {
            type: 'raster',
            tiles: ['https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}'],
            tileSize: 256,
            attribution: '© ESRI World Imagery'
          }
        },
        layers: [
          {
            id: 'satellite-layer',
            type: 'raster',
            source: 'satellite',
            paint: {
              'raster-brightness-max': 0.6,
              'raster-saturation': -0.4,
              'raster-contrast': 0.2
            }
          }
        ]
      },
      center: initialCenter.current,
      zoom: initialZoom.current
    })

    map.current.on('load', () => {
      setIsLoaded(true)
    })

    return () => {
      map.current?.remove()
      map.current = null
    }
  }, []) // Empty dependency array for init

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
    markers.current.forEach(m => m.remove())
    markers.current = []

    events.forEach(event => {
      const el = document.createElement('div')
      el.className = `map-marker pulse-${event.category === 'floods' ? 'flood' : 'fire'}`
      
      const marker = new maplibregl.Marker({ element: el })
        .setLngLat([event.lng, event.lat])
        .setPopup(new maplibregl.Popup({ offset: 25 }).setHTML(`<h3>${event.title}</h3>`))
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
