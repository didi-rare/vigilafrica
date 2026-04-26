import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { axe } from 'vitest-axe'
import type { ReactElement } from 'react'

import { EventsDashboard } from './EventsDashboard'
import { fetchContext, fetchEvents, fetchHealth, fetchStates } from '../api/events'
import type { ContextResponse, EventsResponse, HealthResponse } from '../api/events'

vi.mock('./Map', () => ({
  Map: ({ events }: { events: readonly { title: string }[] }) => (
    <div aria-label="Event locations map" role="img">
      {events.map(event => event.title).join(', ')}
    </div>
  ),
}))

vi.mock('../api/events', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api/events')>()
  return {
    ...actual,
    fetchEvents: vi.fn(),
    fetchStates: vi.fn(),
    fetchContext: vi.fn(),
    fetchHealth: vi.fn(),
  }
})

const mockFetchEvents = vi.mocked(fetchEvents)
const mockFetchStates = vi.mocked(fetchStates)
const mockFetchContext = vi.mocked(fetchContext)
const mockFetchHealth = vi.mocked(fetchHealth)

const successfulIngestion: NonNullable<HealthResponse['last_ingestion']> = {
  status: 'success',
  started_at: '2026-04-24T00:00:00Z',
  completed_at: new Date().toISOString(),
  events_fetched: 2,
  events_stored: 2,
  error: null,
}

const okHealth: HealthResponse = {
  status: 'ok',
  version: 'test',
  last_ingestion: successfulIngestion,
}

const contextResponse: ContextResponse = {
  location: null,
  nearby_events: [],
}

const eventsResponse: EventsResponse = {
  data: [
    {
      id: 'event-lagos',
      source_id: 'EONET_LAGOS',
      source: 'eonet',
      title: 'Lagos Flood 42',
      category: 'floods',
      status: 'open',
      geometry_type: 'Point',
      latitude: 6.5244,
      longitude: 3.3792,
      country_name: 'Nigeria',
      state_name: 'Lagos',
      event_date: '2026-04-23T12:00:00Z',
      source_url: 'https://example.test/eonet/lagos',
      ingested_at: '2026-04-23T12:05:00Z',
      enriched_at: '2026-04-23T12:06:00Z',
    },
    {
      id: 'event-accra',
      source_id: 'EONET_ACCRA',
      source: 'eonet',
      title: 'Accra Wildfire',
      category: 'wildfires',
      status: 'closed',
      geometry_type: 'Point',
      latitude: 5.6037,
      longitude: -0.187,
      country_name: 'Ghana',
      state_name: 'Greater Accra',
      event_date: null,
      source_url: null,
      ingested_at: '2026-04-23T12:05:00Z',
      enriched_at: null,
    },
  ],
  meta: {
    total: 2,
    limit: 50,
    offset: 0,
  },
}

function renderWithProviders(ui: ReactElement, initialEntries = ['/']) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  })

  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={initialEntries}>{ui}</MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('EventsDashboard', () => {
  beforeEach(() => {
    mockFetchEvents.mockResolvedValue(eventsResponse)
    mockFetchStates.mockResolvedValue(['Lagos', 'Greater Accra'])
    mockFetchContext.mockResolvedValue(contextResponse)
    mockFetchHealth.mockResolvedValue(okHealth)
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('renders localized event cards and the map alternative from API data', async () => {
    renderWithProviders(<EventsDashboard />)

    expect(await screen.findByText('Lagos Flood')).toBeInTheDocument()
    expect(screen.getByText('Accra Wildfire')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /Lagos Flood.*Lagos, Nigeria/i })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /Accra Wildfire.*Greater Accra, Ghana/i })).toBeInTheDocument()
    expect(screen.getByRole('img', { name: /event locations map/i })).toHaveTextContent('Lagos Flood 42')
  })

  it('shows the EONET ingestion error banner when health is degraded', async () => {
    mockFetchHealth.mockResolvedValueOnce({
      ...okHealth,
      status: 'degraded',
      last_ingestion: {
        ...successfulIngestion,
        status: 'failure',
        error: 'EONET quota exhausted',
      },
    })

    renderWithProviders(<EventsDashboard />)

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'NASA EONET Ingestion Error: EONET quota exhausted',
    )
  })

  it('updates URL-backed filters and refetches events for the selected country', async () => {
    const user = userEvent.setup()
    renderWithProviders(<EventsDashboard />)

    await screen.findByText('Lagos Flood')
    await user.selectOptions(screen.getByLabelText(/filter by country/i), 'Ghana')

    await waitFor(() => {
      expect(mockFetchEvents).toHaveBeenLastCalledWith(undefined, undefined, 'Ghana')
    })
    expect(mockFetchStates).toHaveBeenLastCalledWith('Ghana')
  })

  it('shows a retry button when the events fetch fails', async () => {
    mockFetchEvents.mockRejectedValue(new Error('Network down'))

    renderWithProviders(<EventsDashboard />)

    expect(
      await screen.findByRole('button', { name: /retry connection/i }),
    ).toBeInTheDocument()
  })

  it('refetches events when the retry button is clicked', async () => {
    const user = userEvent.setup()
    mockFetchEvents
      .mockRejectedValueOnce(new Error('Network down'))
      .mockResolvedValueOnce(eventsResponse)

    renderWithProviders(<EventsDashboard />)

    const retryButton = await screen.findByRole('button', { name: /retry connection/i })
    const initialCalls = mockFetchEvents.mock.calls.length

    await user.click(retryButton)

    await waitFor(() => {
      expect(mockFetchEvents.mock.calls.length).toBeGreaterThan(initialCalls)
    })
  })

  it('renders the attempted API base URL and underlying error message in the error state', async () => {
    vi.stubEnv('VITE_API_BASE_URL', 'https://api.staging.vigilafrica.org')
    mockFetchEvents.mockRejectedValue(new Error('Failed to fetch events from VigilAfrica API (HTTP 503)'))

    renderWithProviders(<EventsDashboard />)

    expect(await screen.findByText('https://api.staging.vigilafrica.org')).toBeInTheDocument()
    expect(
      screen.getByText(/Failed to fetch events from VigilAfrica API \(HTTP 503\)/i),
    ).toBeInTheDocument()

    vi.unstubAllEnvs()
  })

  it('has no obvious accessibility violations in the loaded dashboard state', async () => {
    const { container } = renderWithProviders(<EventsDashboard />)

    await screen.findByText('Lagos Flood')
    const results = await axe(container)
    expect(results.violations).toHaveLength(0)
  })
})
