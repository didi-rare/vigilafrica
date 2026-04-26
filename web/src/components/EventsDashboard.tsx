import { lazy, Suspense } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { fetchEvents, fetchContext, fetchHealth, fetchStates, getApiBaseUrl, eventKeys, stateKeys, healthKeys, contextKeys } from '../api/events'
import type { HealthResponse, EventCategory } from '../api/events'

import './EventsDashboard.css'

const STALENESS_THRESHOLD_HOURS = 2

const Map = lazy(async () => {
  const module = await import('./Map')
  return { default: module.Map }
})

// Supported countries with their map centroids [lng, lat].
const SUPPORTED_COUNTRIES = ['Nigeria', 'Ghana'] as const
type SupportedCountry = typeof SUPPORTED_COUNTRIES[number]

const COUNTRY_CENTERS: Record<SupportedCountry, [number, number]> = {
  Nigeria: [8.6753, 9.082],
  Ghana:   [-1.0232, 7.9465],
}

// Module-level selector — Date.now() is called outside React render
function selectFreshness(health: HealthResponse) {
  const completedAt = health?.last_ingestion?.completed_at
  const lastSuccess = health?.last_ingestion?.status === 'success' && completedAt
    ? new Date(completedAt)
    : null
  const hoursStale = lastSuccess
    ? (Date.now() - lastSuccess.getTime()) / (1000 * 60 * 60)
    : null
  const isStale = hoursStale !== null && hoursStale > STALENESS_THRESHOLD_HOURS
  const isDegraded = health.status === 'degraded'

  let message: string | null = null
  if (isDegraded) {
    let apiError = health.last_ingestion?.error
    if (!apiError && health.last_ingestion_by_country) {
      for (const key of Object.keys(health.last_ingestion_by_country)) {
        if (health.last_ingestion_by_country[key]?.error) {
          apiError = health.last_ingestion_by_country[key]!.error
          break
        }
      }
    }

    if (apiError) {
      message = `NASA EONET Ingestion Error: ${apiError}`
    } else {
      message = health.last_ingestion?.status === 'failure'
        ? 'Last ingestion run failed — data may be outdated. Check system logs.'
        : 'One or more country ingestion runs failed — some regional data may be outdated.'
    }
  } else if (isStale) {
    message = `Data last updated ${Math.floor(hoursStale!)} hours ago — ingestion may be stalled.`
  }

  return {
    hoursStale,
    isStale,
    isDegraded,
    message,
  }
}

function FreshnessIndicator() {
  const { data } = useQuery({
    queryKey: healthKeys.all,
    queryFn: fetchHealth,
    refetchInterval: 5 * 60 * 1000,
    staleTime: 60 * 1000,
    select: selectFreshness,
  })

  if (!data?.message) return null
  const { isDegraded, message } = data

  return (
    <div className={`freshness-banner ${isDegraded ? 'freshness-banner--error' : 'freshness-banner--warn'}`}
      role="alert"
      aria-live="polite"
    >
      <span className="freshness-icon" aria-hidden="true">
        {isDegraded ? '⚠️' : '🕐'}
      </span>
      {message}
    </div>
  )
}

export function EventsDashboard() {
  // §4.3: filter state lives in the URL — survives refresh, navigation, and link-sharing
  const [searchParams, setSearchParams] = useSearchParams()
  const selectedCountry  = searchParams.get('country') ?? ''
  const selectedCategory = (searchParams.get('category') ?? '') as EventCategory | ''
  const selectedState    = searchParams.get('state') ?? ''

  function handleCountryChange(country: string) {
    setSearchParams(prev => {
      const next = new URLSearchParams(prev)
      if (country) next.set('country', country); else next.delete('country')
      next.delete('state')
      return next
    })
  }

  function handleCategoryChange(category: string) {
    setSearchParams(prev => {
      const next = new URLSearchParams(prev)
      if (category) next.set('category', category); else next.delete('category')
      return next
    })
  }

  function handleStateChange(state: string) {
    setSearchParams(prev => {
      const next = new URLSearchParams(prev)
      if (state) next.set('state', state); else next.delete('state')
      return next
    })
  }

  // §5.1: all data fetching via TanStack Query
  // §5.2: query keys from factory functions
  const {
    data: eventsData,
    isPending: eventsLoading,
    error: eventsError,
    refetch: refetchEvents,
    isFetching: eventsFetching,
  } = useQuery({
    queryKey: eventKeys.list(selectedCountry, selectedCategory, selectedState),
    queryFn: () => fetchEvents(
      selectedCategory || undefined,
      selectedState || undefined,
      selectedCountry || undefined,
    ),
  })

  const { data: statesData } = useQuery({
    queryKey: stateKeys.list(selectedCountry),
    queryFn: () => fetchStates(selectedCountry || undefined),
    staleTime: 5 * 60 * 1000,
  })

  const { data: contextData } = useQuery({
    queryKey: contextKeys.all,
    queryFn: () => fetchContext(),
  })

  // Filter out events without coordinates to prevent MapLibre from crashing
  const mapEvents = eventsData?.data
    ?.filter(e => e.latitude !== null && e.longitude !== null)
    ?.map(e => ({
      id: e.id,
      lat: e.latitude as number,   // safe: filtered null above
      lng: e.longitude as number,  // safe: filtered null above
      category: e.category,
      title: e.title
    })) || []

  // Map center: selected country centroid > IP geolocation > Nigeria default
  const mapCenter: [number, number] =
    (selectedCountry && selectedCountry in COUNTRY_CENTERS)
      ? COUNTRY_CENTERS[selectedCountry as SupportedCountry]
      : contextData?.location &&
        typeof contextData.location.lng === 'number' &&
        typeof contextData.location.lat === 'number'
        ? [contextData.location.lng, contextData.location.lat]
        : COUNTRY_CENTERS['Nigeria']

  const availableStates = statesData ?? []

  return (
    <section id="dashboard" className="dashboard section" aria-labelledby="dashboard-heading">
      <div className="container">
        <span className="section-label">Real-time Data</span>
        <h2 id="dashboard-heading" className="section-title">Latest Localized Events</h2>
        <p className="section-subtitle">
          Events ingested from NASA EONET and tagged with African administrative boundaries.
        </p>

        <FreshnessIndicator />

        {/* ── Filters ── §9.3: visible labels via sr-only + htmlFor */}
        <div className="dashboard-filters" role="group" aria-label="Event filters">
          <div className="dashboard-filter-group">
            <label htmlFor="filter-country" className="sr-only">Country</label>
            <select
              id="filter-country"
              className="dashboard-filter-select"
              value={selectedCountry}
              onChange={e => handleCountryChange(e.target.value)}
              aria-label="Filter by country"
            >
              <option value="">All Countries</option>
              {SUPPORTED_COUNTRIES.map(c => (
                <option key={c} value={c}>{c}</option>
              ))}
            </select>
          </div>

          <div className="dashboard-filter-group">
            <label htmlFor="filter-category" className="sr-only">Category</label>
            <select
              id="filter-category"
              className="dashboard-filter-select"
              value={selectedCategory}
              onChange={e => handleCategoryChange(e.target.value)}
              aria-label="Filter by category"
            >
              <option value="">All Categories</option>
              <option value="floods">🌊 Floods</option>
              <option value="wildfires">🔥 Wildfires</option>
            </select>
          </div>

          <div className="dashboard-filter-group">
            <label htmlFor="filter-state" className="sr-only">State / Region</label>
            <select
              id="filter-state"
              className="dashboard-filter-select"
              value={selectedState}
              onChange={e => handleStateChange(e.target.value)}
              disabled={availableStates.length === 0}
              aria-label="Filter by state"
            >
              <option value="">All States</option>
              {availableStates.map(s => (
                <option key={s} value={s}>{s}</option>
              ))}
            </select>
          </div>
        </div>

        <div className="dashboard-layout">
          <div className="dashboard-sidebar">
            {eventsLoading && (
              <div className="dashboard-state loading">
                <div className="spinner"></div>
                <p>Fetching satellite telemetry...</p>
              </div>
            )}

            {eventsError && (
              <div className="dashboard-state error" role="alert">
                <span role="img" aria-label="alert">⚠️</span>
                <p>Failed to connect to VigilAfrica Command Center</p>
                {import.meta.env.VITE_SHOW_ERROR_DETAIL === 'true' && (
                  <>
                    <p className="dashboard-state-detail">
                      <span className="dashboard-state-label">API:</span>{' '}
                      <code>{getApiBaseUrl()}</code>
                    </p>
                    <p className="dashboard-state-detail dashboard-state-detail--muted">
                      {eventsError instanceof Error ? eventsError.message : String(eventsError)}
                    </p>
                  </>
                )}
                <button
                  type="button"
                  className="dashboard-retry-button"
                  onClick={() => { void refetchEvents() }}
                  disabled={eventsFetching}
                  aria-label="Retry connection"
                >
                  {eventsFetching ? 'Retrying…' : 'Retry'}
                </button>
              </div>
            )}

            {eventsData && eventsData.data && (
              <div className="events-list">
                {eventsData.data.map((event) => {
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
            <Suspense fallback={<div className="dashboard-state loading"><div className="spinner"></div><p>Loading map telemetry...</p></div>}>
              <Map events={mapEvents} center={mapCenter} />
            </Suspense>
          </div>
        </div>
      </div>
    </section>
  )
}


