import { lazy, Suspense, useEffect, useRef } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { fetchEventById, eventKeys } from '../api/events'
import { track } from '../analytics'
import { FeedbackPrompt } from '../components/FeedbackPrompt'
import { Droplet, Flame, MapPin } from 'lucide-react'

import './EventDetail.css'

const Map = lazy(async () => {
  const module = await import('../components/Map')
  return { default: module.Map }
})

// Format decimal lat/lng into a cartographic DMS-style readout, matching the
// landing hero's coordinate motif (e.g. 09°04′N 07°29′E). Ground Truth.
function formatCoordinates(lat: number, lng: number): string {
  const dms = (value: number, positive: string, negative: string) => {
    const dir = value >= 0 ? positive : negative
    const abs = Math.abs(value)
    const deg = Math.floor(abs)
    const min = Math.round((abs - deg) * 60)
    return `${String(deg).padStart(2, '0')}°${String(min).padStart(2, '0')}′${dir}`
  }
  return `${dms(lat, 'N', 'S')} ${dms(lng, 'E', 'W')}`
}

export function EventDetail() {
  const { id } = useParams<{ id: string }>()
  const { data: event, isPending, error } = useQuery({
    queryKey: eventKeys.detail(id ?? ''),
    queryFn: () => fetchEventById(id!), // enabled only when id is defined (enabled: !!id below)
    enabled: !!id,
  })

  // Fire `event_detail_opened` once per distinct event the detail page shows —
  // a Tier-1 funnel conversion. Deduped by event id so a refetch or StrictMode
  // double-mount doesn't double-count.
  const reportedEventRef = useRef<string | null>(null)
  useEffect(() => {
    if (!event || reportedEventRef.current === event.id) return
    reportedEventRef.current = event.id
    track('event_detail_opened', {
      event_id: event.id,
      category: event.category,
      state: event.state_name ?? '',
    })
  }, [event])

  if (isPending) return <div className="container section">Loading event telemetry...</div>
  if (error || !event) return <div className="container section">Event not found in Command Center.</div>

  const categoryClass = event.category === 'floods' ? 'flood' : 'fire'
  const coordinates = event.latitude !== null && event.longitude !== null
    ? { lat: event.latitude, lng: event.longitude }
    : null

  return (
    <div className="event-detail-page">
      <div className="container">
        <Link to="/" className="back-link">← Back to Sentinel Dashboard</Link>
        
        <header className="event-detail-header">
          <div className="header-main">
            <span className={`badge badge--${categoryClass}`}>
              {event.category === 'floods' ? (
                <><Droplet size={14} aria-hidden="true" /> Floods</>
              ) : (
                <><Flame size={14} aria-hidden="true" /> Wildfires</>
              )}
            </span>
            <h1>{event.title}</h1>
            <p className="event-detail-disclaimer" role="note">
              Location may be approximate — confirm with local authorities before making safety decisions.
            </p>
          </div>
          <div className="header-meta">
             <span className="status-indicator">
                <span className={`status-dot ${event.status}`} /> {event.status}
             </span>
             {coordinates && (
               <span className="detail-coord">
                 <MapPin size={13} aria-hidden="true" />
                 {formatCoordinates(coordinates.lat, coordinates.lng)}
               </span>
             )}
             <span className="event-date">
                Detected: {event.event_date ? new Date(event.event_date).toLocaleString() : 'Active'}
             </span>
          </div>
        </header>

        <div className="detail-layout">
          <div className="detail-info">
            <section className="info-group">
              <label>Location Context</label>
              <div className="event-location glass-effect">
                <strong>{event.state_name}</strong>, {event.country_name}
                <small>
                  {coordinates
                    ? `Coordinates: ${coordinates.lat.toFixed(4)}, ${coordinates.lng.toFixed(4)}`
                    : 'Area geometry available; point coordinates were not provided for this event.'}
                </small>
              </div>
            </section>

            <section className="info-group">
              <label>Data Integrity</label>
              <ul className="telemetry-log">
                <li>Source: <code>{event.source}</code></li>
                <li>Sentinel ID: <code>{event.id}</code></li>
                <li>Ingested At: {new Date(event.ingested_at).toLocaleString()}</li>
              </ul>
            </section>

            {event.source_url && (
              <a href={event.source_url} target="_blank" rel="noopener noreferrer" className="btn btn-outline">
                View Original Satellite Source →
              </a>
            )}
          </div>

          <div className="detail-map">
            {coordinates ? (
              <Suspense fallback={<div className="map-unavailable glass-effect">Loading map telemetry...</div>}>
                <Map
                  events={[{
                  id: event.id,
                  lat: coordinates.lat,
                  lng: coordinates.lng,
                  category: event.category,
                  title: event.title
                }]}
                  center={[coordinates.lng, coordinates.lat]}
                  zoom={10}
                />
              </Suspense>
            ) : (
              <div className="map-unavailable glass-effect">
                Detailed map view unavailable for area-based geometry events.
              </div>
            )}
          </div>
        </div>

        <FeedbackPrompt eventId={event.id} />
      </div>
    </div>
  )
}

