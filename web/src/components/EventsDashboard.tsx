import { useQuery } from '@tanstack/react-query'
import { fetchEvents } from '../api/events'
import './EventsDashboard.css'

export function EventsDashboard() {
  const { data, isLoading, error } = useQuery({
    queryKey: ['events'],
    queryFn: () => fetchEvents()
  })

  return (
    <section id="dashboard" className="dashboard section" aria-labelledby="dashboard-heading">
      <div className="container">
        <span className="section-label">Real-time Data</span>
        <h2 id="dashboard-heading" className="section-title">Latest Localized Events</h2>
        <p className="section-subtitle">
          These events are continuously ingested from NASA EONET and automatically tagged with Nigerian administrative boundaries.
        </p>

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
          <div className="events-grid">
            {data.data.map((event) => (
              <article key={event.id} className="event-card">
                <div className="event-header">
                  <span className={`badge badge--${event.category === 'floods' ? 'flood' : 'fire'}`}>
                    {event.category === 'floods' ? '🌊 Floods' : '🔥 Wildfires'}
                  </span>
                  <span className="event-date">
                    {event.event_date ? new Date(event.event_date).toLocaleDateString() : 'Active'}
                  </span>
                </div>
                <h3 className="event-title">{event.title}</h3>
                
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
                    <a href={event.source_url} target="_blank" rel="noopener noreferrer" className="event-link">
                      Source →
                    </a>
                  )}
                </div>
              </article>
            ))}
          </div>
        )}
      </div>
    </section>
  )
}
