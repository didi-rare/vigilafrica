import { lazy, Suspense, useEffect, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router'
import { useQuery } from '@tanstack/react-query'
import { fetchEvents, fetchContext, fetchHealth, fetchStates, getApiBaseUrl, eventKeys, stateKeys, healthKeys, contextKeys, EVENTS_PAGE_SIZE } from '../api/events'
import type { HealthResponse, EventCategory, VigilEvent } from '../api/events'
import { track } from '../analytics'
import { Droplet, Flame, MapPin, CircleCheck, Clock, AlertTriangle, ChevronLeft, ChevronRight } from 'lucide-react'

import { Select, type SelectOption } from './Select'
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

// Static option set for the category filter (empty value = "all").
const CATEGORY_OPTIONS: SelectOption[] = [
  { value: '', label: 'All Categories' },
  { value: 'floods', label: 'Floods' },
  { value: 'wildfires', label: 'Wildfires' },
]

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
        aria-label="Data freshness"
      >
        <span className="freshness-icon" aria-hidden="true"><CircleCheck size={14} /></span>
        Data freshness unknown — no ingestion history available.
      </div>
    )
  }

  const variantClass = resolved.kind === 'error'
    ? 'freshness-banner--error'
    : resolved.kind === 'warn'
      ? 'freshness-banner--warn'
      : 'freshness-banner--ok'

  const FreshnessIcon = resolved.kind === 'error' ? AlertTriangle : resolved.kind === 'warn' ? Clock : CircleCheck
  const ariaRole = resolved.kind === 'ok' ? 'status' : 'alert'

  return (
    <div
      className={`freshness-banner ${variantClass}`}
      role={ariaRole}
      aria-live="polite"
      aria-label="Data freshness"
    >
      <span className="freshness-icon" aria-hidden="true"><FreshnessIcon size={14} /></span>
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

// MAX_PAGE bounds what the URL may request. `offset` must stay a plain integer
// the API will accept: beyond ~1e21 `String(offset)` produces exponential
// notation ("5e+22"), which the Go handler rejects with a 400.
const MAX_PAGE = 1_000_000

// parsePage reads the 1-based `page` URL parameter defensively. The value is
// user-editable, so anything that is not a plain positive integer within range
// falls back to page 1 rather than reaching the API as an offset it will reject.
//
// `Number.parseInt` alone is NOT enough, and both gaps were found by review:
// it accepts trailing junk ("2junk" → 2, silently treated as page 2) and it
// happily returns 1e21 for a long digit string, which is `Number.isFinite` and
// positive yet unusable as an offset. Match the whole string, then bound it.
function parsePage(raw: string | null): number {
  if (raw === null || !/^\d+$/.test(raw)) return 1
  const parsed = Number(raw)
  if (!Number.isSafeInteger(parsed) || parsed < 1) return 1
  return Math.min(parsed, MAX_PAGE)
}

export function EventsDashboard() {
  // §4.3: filter state lives in the URL — survives refresh, navigation, and link-sharing.
  // The page number lives there too, for the same reasons: a shared link to page 3
  // of a filtered set should land on page 3.
  const [searchParams, setSearchParams] = useSearchParams()
  const selectedCountry  = searchParams.get('country') ?? ''
  const selectedCategory = (searchParams.get('category') ?? '') as EventCategory | ''
  const selectedState    = searchParams.get('state') ?? ''
  const currentPage      = parsePage(searchParams.get('page'))
  const offset           = (currentPage - 1) * EVENTS_PAGE_SIZE

  // Every filter change drops `page`, returning to page 1 (task 3.3). Without
  // this, changing country while on page 5 requests an offset the new, smaller
  // result set has no rows for — an empty page that looks like "no events here".
  function handleCountryChange(country: string) {
    setSearchParams(prev => {
      const next = new URLSearchParams(prev)
      if (country) next.set('country', country); else next.delete('country')
      next.delete('state')
      next.delete('page')
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
      next.delete('page')
      return next
    })
  }

  function handleStateChange(state: string) {
    // Track only an actual state selection, not a reset to "All States".
    if (state) track('state_filter_selected', { state })
    setSearchParams(prev => {
      const next = new URLSearchParams(prev)
      if (state) next.set('state', state); else next.delete('state')
      next.delete('page')
      return next
    })
  }

  function goToPage(page: number) {
    setSearchParams(prev => {
      const next = new URLSearchParams(prev)
      // Page 1 is the default, so it stays out of the URL — a clean canonical
      // link, and it keeps the SEO'd landing URL free of a redundant `?page=1`.
      if (page <= 1) next.delete('page'); else next.set('page', String(page))
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
    isPlaceholderData,
  } = useQuery({
    queryKey: eventKeys.list(selectedCountry, selectedCategory, selectedState, offset),
    queryFn: () => fetchEvents(
      selectedCategory || undefined,
      selectedState || undefined,
      selectedCountry || undefined,
      offset,
    ),
    // Task 2.3, evaluated rather than assumed. Carrying the previous response is
    // a clear win for a PAGE change: without it the list empties for a round
    // trip, and on mobile — where `.dashboard-layout` is `height: auto` — the
    // section collapses and re-expands, exactly the shift class #193/#198
    // removed. Keeping the previous page rendered holds the height constant.
    //
    // ⚠️ But it must NOT be carried across a FILTER change, and plain
    // `keepPreviousData` did exactly that. Measured: selecting Ghana left 50
    // Nigeria-era cards on screen, under a country control already reading
    // "Ghana", with the count reporting the previous filter's 3,268 total. The
    // range label was truthful about the rows, but the screen as a whole was
    // not — the same "shows you something other than what it claims" failure
    // this change exists to remove, just relocated from the count to the filter.
    //
    // So: keep the previous data only when the filters are unchanged, i.e. when
    // `offset` is the only part of the key that moved. A filter change falls
    // back to the ordinary loading state, which is honest — the result set
    // genuinely is different and not yet known.
    placeholderData: (previousData, previousQuery) => {
      const previousFilters = previousQuery?.queryKey?.[2] as
        { country: string; category: string; state: string } | undefined
      if (!previousFilters) return undefined
      const sameFilters =
        previousFilters.country === selectedCountry &&
        previousFilters.category === selectedCategory &&
        previousFilters.state === selectedState
      return sameFilters ? previousData : undefined
    },
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

  // ── Pagination arithmetic ──────────────────────────────────────────────────
  // EVERY value here derives from `meta` as the server applied it to the rows
  // currently rendered — never from the offset the URL is asking for.
  //
  // ⚠️ That distinction is load-bearing, and getting it wrong was caught by
  // review. `keepPreviousData` means that during a page change `eventsData` is
  // still the PREVIOUS page while `offset` has already advanced. Computing the
  // range from `offset` made the label read "Showing 51–100 / Page 2" over
  // page-1 rows — the interface asserting something false about what is on
  // screen, which is precisely the failure mode this whole change exists to
  // remove. `meta.offset` describes the rows in hand, so the label cannot lie.
  const total       = eventsData?.meta?.total ?? 0
  const pageSize    = eventsData?.meta?.limit || EVENTS_PAGE_SIZE
  const shownOffset = eventsData?.meta?.offset ?? 0
  const shownCount  = eventsData?.data?.length ?? 0
  const rangeStart  = shownCount > 0 ? shownOffset + 1 : 0
  const rangeEnd    = shownOffset + shownCount
  const totalPages  = total > 0 ? Math.ceil(total / pageSize) : 1
  // The page the user is actually LOOKING AT, which during a placeholder render
  // is the previous one, not `currentPage`.
  const shownPage   = Math.floor(shownOffset / pageSize) + 1
  // `canNext` is computed from rows actually returned rather than from
  // shownPage < totalPages, so an out-of-range page (a hand-edited URL, or a
  // filter that shrank the set) cannot offer a Next that leads nowhere.
  const canPrev = shownPage > 1
  const canNext = rangeEnd < total
  // While placeholder data is on screen the visible list belongs to the previous
  // page, so acting on the pager would skip or repeat a page. Freeze it until the
  // real page lands. ⚠️ This freezes the PAGER only — the event links in the list
  // stay live, and they correctly point at the events actually displayed, which
  // the range label now also describes.
  const controlsBusy = isPlaceholderData

  const availableStates = statesData ?? []

  const countryOptions: SelectOption[] = [
    { value: '', label: 'All Countries' },
    ...SUPPORTED_COUNTRIES.map((c) => ({ value: c, label: c })),
  ]
  const stateOptions: SelectOption[] = [
    { value: '', label: 'All States' },
    ...availableStates.map((s) => ({ value: s, label: s })),
  ]

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

        {/* ── Filters ── each <Select> carries its own sr-only label (a11y tree) */}
        <div className="dashboard-filters" role="group" aria-label="Event filters">
          <Select
            id="filter-country"
            label="Filter by country"
            value={selectedCountry}
            onChange={handleCountryChange}
            options={countryOptions}
          />
          <Select
            id="filter-category"
            label="Filter by category"
            value={selectedCategory}
            onChange={handleCategoryChange}
            options={CATEGORY_OPTIONS}
          />
          <Select
            id="filter-state"
            label="Filter by state or region"
            value={selectedState}
            onChange={handleStateChange}
            options={stateOptions}
            disabled={availableStates.length === 0}
          />
        </div>

        {/*
          The results bar sits ABOVE the layout on purpose. On desktop the
          sidebar is a fixed 800px scroll container, so controls placed under the
          list would scroll out of sight inside it — and the total is the one
          number this whole change exists to make visible. It must not require
          scrolling past an 800px map to find.
        */}
        {/*
          The CONTAINER renders unconditionally, even before the first response.
          Gating it on `eventsData` was measured to double the page's CLS
          (0.0059 → 0.0122 at 1920x1600, 5/5 runs): the bar appeared when the
          fetch resolved and shoved the 800px `.dashboard-layout` down 64px. Its
          `min-height` reserves that space from mount, so the content can fill in
          without moving anything. Same lesson as #193/#198, one element down.
        */}
        <div className="dashboard-results">
          {eventsData && !eventsError && (
            <>
            {/*
              A second polite live region alongside the freshness banner. Both
              carry an aria-label so each is individually addressable — screen
              readers announce the changed CONTENT of a live region, and the
              label distinguishes them when navigating by role.
            */}
            <p
              className="dashboard-results__count"
              role="status"
              aria-live="polite"
              aria-label="Result count"
            >
              {total === 0 ? (
                'No events match these filters'
              ) : shownCount === 0 ? (
                <>That page is past the end of <strong>{total.toLocaleString('en-GB')}</strong> matching events</>
              ) : (
                <>
                  Showing <strong>{rangeStart.toLocaleString('en-GB')}–{rangeEnd.toLocaleString('en-GB')}</strong>
                  {' of '}
                  <strong>{total.toLocaleString('en-GB')}</strong> matching events
                </>
              )}
            </p>

            {/*
              Past-the-end recovery. Stepping back one page from a hand-edited
              `?page=99` would land on another empty page, so offer the only jump
              that is guaranteed to have rows.
            */}
            {total > 0 && shownCount === 0 && (
              <div className="dashboard-results__controls">
                <button
                  type="button"
                  className="dashboard-page-button"
                  onClick={() => goToPage(1)}
                  aria-label="Return to the first page of events"
                >
                  <ChevronLeft size={16} aria-hidden="true" />
                  First page
                </button>
              </div>
            )}

            {totalPages > 1 && shownCount > 0 && (
              <div className="dashboard-results__controls">
                <button
                  type="button"
                  className="dashboard-page-button"
                  onClick={() => goToPage(shownPage - 1)}
                  disabled={!canPrev || controlsBusy}
                  aria-label="Previous page of events"
                >
                  <ChevronLeft size={16} aria-hidden="true" />
                  Previous
                </button>
                <span className="dashboard-results__page">
                  Page {shownPage.toLocaleString('en-GB')} of {totalPages.toLocaleString('en-GB')}
                </span>
                <button
                  type="button"
                  className="dashboard-page-button"
                  onClick={() => goToPage(shownPage + 1)}
                  disabled={!canNext || controlsBusy}
                  aria-label="Next page of events"
                >
                  Next
                  <ChevronRight size={16} aria-hidden="true" />
                </button>
              </div>
            )}
            </>
          )}
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
                <AlertTriangle size={18} aria-hidden="true" />
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
                            {event.category === 'floods' ? (
                              <><Droplet size={13} aria-hidden="true" /> Floods</>
                            ) : (
                              <><Flame size={13} aria-hidden="true" /> Wildfires</>
                            )}
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
                          <span className="location-pin" aria-hidden="true"><MapPin size={13} /></span>
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


