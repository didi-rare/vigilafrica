import { lazy, Suspense, useEffect, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { fetchEvents, fetchContext, fetchHealth, fetchStates, getApiBaseUrl, eventKeys, stateKeys, healthKeys, contextKeys } from '../api/events'
import type { HealthResponse, EventCategory, VigilEvent } from '../api/events'
import { track } from '../analytics'

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

function formatLastUpdated(minutesAgo: number): string {
  if (minutesAgo < 1) return 'Last updated just now'
  if (minutesAgo < 60) return `Last updated ${minutesAgo}m ago`
  const hoursAgo = Math.floor(minutesAgo / 60)
  if (hoursAgo < 24) return `Last updated ${hoursAgo}h ago`
  const daysAgo = Math.floor(hoursAgo / 24)
  return `Last updated ${daysAgo}d ago`
}

type FreshnessSnapshot =
  | { kind: 'error'; message: string }
  | { kind: 'unknown' }
  | { kind: 'healthy'; lastSuccess: Date }

// selectFreshness returns the raw snapshot the UI needs WITHOUT computing any
// time-relative strings. Date.now() lives in the FreshnessIndicator component
// instead, so the "X minutes ago" label ticks on every render and stays
// accurate between refetches (chore-post-v11-quality-sweep F6).
function selectFreshness(health: HealthResponse): FreshnessSnapshot {
  if (health.status === 'degraded') {
    const message = health.last_ingestion?.status === 'failure'
      ? 'Latest ingestion did not complete successfully. Data may be delayed while operators investigate.'
      : 'One or more country ingestion runs did not complete successfully. Some regional data may be delayed.'
    return { kind: 'error', message }
  }
  const completedAt = health?.last_ingestion?.completed_at
  if (health?.last_ingestion?.status === 'success' && completedAt) {
    return { kind: 'healthy', lastSuccess: new Date(completedAt) }
  }
  return { kind: 'unknown' }
}

// useNowTick re-renders the caller every `intervalMs`. Used by
// FreshnessIndicator to keep the "X minutes ago" label ticking accurately.
function useNowTick(intervalMs: number): number {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), intervalMs)
    return () => window.clearInterval(id)
  }, [intervalMs])
  return now
}

function FreshnessIndicator() {
  const { data } = useQuery({
    queryKey: healthKeys.all,
    queryFn: fetchHealth,
    refetchInterval: 5 * 60 * 1000,
    staleTime: 60 * 1000,
    select: selectFreshness,
  })
  // Force re-render every minute so the relative-time label stays fresh even
  // when no new query data has arrived (refetchInterval is 5 minutes).
  const now = useNowTick(60 * 1000)

  // Pre-load: useQuery hasn't resolved yet. Returning null here is acceptable
  // because there is no freshness state to render — the loading state for the
  // dashboard data itself covers UX continuity.
  if (!data) return null

  // Compute the time-relative description here, in render, against the ticking
  // `now`. The selector only returns the raw lastSuccess timestamp (F6).
  let resolved: { kind: 'ok' | 'warn' | 'error'; message: string } | { kind: 'unknown' }
  if (data.kind === 'error' || data.kind === 'unknown') {
    resolved = data
  } else {
    const minutesAgo = Math.floor((now - data.lastSuccess.getTime()) / (1000 * 60))
    const hoursAgo = minutesAgo / 60
    if (hoursAgo > STALENESS_THRESHOLD_HOURS) {
      resolved = {
        kind: 'warn',
        message: `Data last updated ${Math.floor(hoursAgo)} hours ago — ingestion may be stalled.`,
      }
    } else {
      resolved = { kind: 'ok', message: formatLastUpdated(minutesAgo) }
    }
  }

  if (resolved.kind === 'unknown') {
    return (
      <div
        className="freshness-banner freshness-banner--ok"
        role="status"
        aria-live="polite"
      >
        <span className="freshness-icon" aria-hidden="true">🟢</span>
        Data freshness unknown — no ingestion history available.
      </div>
    )
  }

  const variantClass = resolved.kind === 'error'
    ? 'freshness-banner--error'
    : resolved.kind === 'warn'
      ? 'freshness-banner--warn'
      : 'freshness-banner--ok'

  const icon = resolved.kind === 'error' ? '⚠️' : resolved.kind === 'warn' ? '🕐' : '🟢'
  const ariaRole = resolved.kind === 'ok' ? 'status' : 'alert'

  return (
    <div
      className={`freshness-banner ${variantClass}`}
      role={ariaRole}
      aria-live="polite"
    >
      <span className="freshness-icon" aria-hidden="true">{icon}</span>
      {resolved.message}
    </div>
  )
}

function DashboardDisclaimer() {
  return (
    <div
      className="dashboard-disclaimer"
      role="note"
      aria-label="Important data limitation notice"
    >
      <span className="dashboard-disclaimer__icon" aria-hidden="true">ⓘ</span>
      <p>
        VigilAfrica is an awareness and visualization tool, not an official emergency alert system.
        Event locations and timing may be approximate. Always confirm with local authorities and
        official emergency agencies before making safety decisions.
      </p>
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
    // Track only an actual category selection, not a reset to "All Categories"
    // (empty value) — the KPI is value-moment selections, not deselects.
    if (category) track('category_filter_selected', { category })
    setSearchParams(prev => {
      const next = new URLSearchParams(prev)
      if (category) next.set('category', category); else next.delete('category')
      return next
    })
  }

  function handleStateChange(state: string) {
    // Track only an actual state selection, not a reset to "All States".
    if (state) track('state_filter_selected', { state })
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

  // Fire `context_resolve` once when /v1/context returns a non-null location —
  // the "what's near me?" answer landed. Deduped by the resolved country+state
  // so a TanStack refetch or StrictMode double-mount doesn't double-count.
  const reportedContextRef = useRef<string | null>(null)
  useEffect(() => {
    const location = contextData?.location
    if (!location) return
    const key = `${location.country}|${location.state}`
    if (reportedContextRef.current === key) return
    reportedContextRef.current = key
    track('context_resolve', { country: location.country, state: location.state })
  }, [contextData])

  // Filter out events without coordinates to prevent MapLibre from crashing.
  // The type predicate narrows lat/lng to number, replacing the previous
  // `as number` cast (chore-post-v11-quality-sweep F7).
  const mapEvents = (eventsData?.data ?? [])
    .filter((e): e is VigilEvent & { latitude: number; longitude: number } =>
      e.latitude !== null && e.longitude !== null)
    .map(e => ({
      id: e.id,
      lat: e.latitude,
      lng: e.longitude,
      category: e.category,
      title: e.title,
    }))

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

        <DashboardDisclaimer />
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
                  // F5: render the raw event title rather than splitting off a
                  // trailing number as an "ID". The regex was fragile — titles
                  // like "Flood in Lagos 2024" treated 2024 as an event ID.

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
                            {/* F8: explicit en-GB locale so the same event renders the same date everywhere. */}
                            {event.event_date ? new Date(event.event_date).toLocaleDateString('en-GB') : 'Active'}
                          </span>
                        </div>
                        <h3 className="event-title">
                          {event.title}
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


