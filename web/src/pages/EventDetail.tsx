import { lazy, Suspense } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { fetchEventById } from '../api/events'

import './EventDetail.css'

const Map = lazy(async () => {
  const module = await import('../components/Map')
  return { default: module.Map }
})

export function EventDetail() {
  const { id } = useParams<{ id: string }>()
  const { data: event, isLoading, error } = useQuery({
    queryKey: ['event', id],
    queryFn: () => fetchEventById(id!),
    enabled: !!id
  })

  if (isLoading) return <div className="container section">Loading event telemetry...</div>
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
              {event.category === 'floods' ? '🌊 Floods' : '🔥 Wildfires'}
            </span>
            <h1>{event.title}</h1>
          </div>
          <div className="header-meta">
             <span className="status-indicator">
                <span className={`status-dot ${event.status}`} /> {event.status}
             </span>
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
      </div>
    </div>
  )
}

