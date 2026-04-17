import { Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { fetchEvents, fetchContext, fetchHealth } from '../api/events'
import { Map } from './Map'
import './EventsDashboard.css'

const STALENESS_THRESHOLD_HOURS = 2

function FreshnessIndicator() {
  const { data: health } = useQuery({
    queryKey: ['health'],
    queryFn: fetchHealth,
    refetchInterval: 5 * 60 * 1000, // re-check every 5 minutes
    staleTime: 60 * 1000,
  })

  if (!health?.last_ingestion?.completed_at) return null

  const lastSuccess = health.last_ingestion.status === 'success'
    ? new Date(health.last_ingestion.completed_at)
    : null

  const hoursStale = lastSuccess
    ? (Date.now() - lastSuccess.getTime()) / (1000 * 60 * 60)
    : null

  const isStale = hoursStale !== null && hoursStale > STALENESS_THRESHOLD_HOURS
  const isDegraded = health.status === 'degraded'

  if (!isStale && !isDegraded) return null

  return (
    <div className={`freshness-banner ${isDegraded ? 'freshness-banner--error' : 'freshness-banner--warn'}`}
      role="alert"
      aria-live="polite"
    >
      <span className="freshness-icon" aria-hidden="true">
        {isDegraded ? '⚠️' : '🕐'}
      </span>
      {isDegraded
        ? 'Last ingestion run failed — data may be outdated. Check system logs.'
        : `Data last updated ${Math.floor(hoursStale!)} hours ago — ingestion may be stalled.`
      }
    </div>
  )
}

export function EventsDashboard() {
  const { data: eventsData, isLoading: eventsLoading, error: eventsError } = useQuery({
    queryKey: ['events'],
    queryFn: () => fetchEvents()
  })

  const { data: contextData } = useQuery({
    queryKey: ['context'],
    queryFn: () => fetchContext()
  })

  // Filter out events without coordinates to prevent MapLibre from crashing
  const mapEvents = eventsData?.data
    ?.filter(e => e.latitude !== null && e.longitude !== null)
    ?.map(e => ({
      id: e.id,
      lat: e.latitude as number,
      lng: e.longitude as number,
      category: e.category,
      title: e.title
    })) || []

  const isLoading = eventsLoading
  const error = eventsError
  const data = eventsData

  const mapCenter: [number, number] = contextData?.location && 
    typeof contextData.location.lng === 'number' && typeof contextData.location.lat === 'number'
    ? [contextData.location.lng, contextData.location.lat]
    : [8.6753, 9.082] // default to Nigeria center

  return (
    <section id="dashboard" className="dashboard section" aria-labelledby="dashboard-heading">
      <div className="container">
        <span className="section-label">Real-time Data</span>
        <h2 id="dashboard-heading" className="section-title">Latest Localized Events</h2>
        <p className="section-subtitle">
          These events are continuously ingested from NASA EONET and automatically tagged with Nigerian administrative boundaries.
        </p>

        <FreshnessIndicator />

        <div className="dashboard-layout">
          <div className="dashboard-sidebar">
            {isLoading && (
              <div className="dashboard-state loading">
                <div className="spinner"></div>
                <p>Fetching satellite telemetry...</p>
              </div>
            )}

            {error && (
              <div className="dashboard-state error">
                <span role="img" aria-label="alert">⚠️</span>
                <p>Failed to connect to VigilAfrica Command Center</p>
              </div>
            )}

            {data && data.data && (
              <div className="events-list">
                {data.data.map((event) => {
                  const categoryClass = event.category === 'floods' ? 'flood' : 'fire'
                  const titleMatch = event.title.match(/^(.*)\s(\d+)$/)
                  const displayTitle = titleMatch ? titleMatch[1] : event.title
                  const eventId = titleMatch ? titleMatch[2] : ''

                  return (
                    <Link 
                      key={event.id} 
                      to={`/events/${event.id}`}
                      className="event-card-link"
                    >
                      <article className={`event-card event-card--${categoryClass}`}>
                        <div className="event-header">
                          <span className={`badge badge--${categoryClass}`}>
                            {event.category === 'floods' ? '🌊 Floods' : '🔥 Wildfires'}
                          </span>
                          <span className="event-date">
                            {event.event_date ? new Date(event.event_date).toLocaleDateString() : 'Active'}
                          </span>
                        </div>
                        <h3 className="event-title">
                          {displayTitle}
                          {eventId && <span className="event-id"> {eventId}</span>}
                        </h3>
                        
                        <div className="event-location glass-effect">
                          <span className="location-pin" aria-hidden="true">📍</span>
                          {event.state_name ? (
                            <span className="location-text">
                              <strong>{event.state_name}</strong>, {event.country_name}
                            </span>
                          ) : (
                            <span className="location-text coords">
                              {event.latitude?.toFixed(4)}, {event.longitude?.toFixed(4)}
                            </span>
                          )}
                        </div>

                        <div className="event-meta">
                          <span className="status-indicator">
                            <span className={`status-dot ${event.status}`} /> {event.status}
                          </span>
                          {event.source_url && (
                            <span className="event-link">
                              Details →
                            </span>
                          )}
                        </div>
                      </article>
                    </Link>
                  )
                })}
              </div>
            )}
          </div>

          <div className="dashboard-map-container">
            <Map events={mapEvents} center={mapCenter} />
          </div>
        </div>
      </div>
    </section>
  )
}
