import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
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

    expect(await screen.findByRole("heading", { level: 3, name: /Lagos Flood 42/i })).toBeInTheDocument()
    expect(screen.getByText('Accra Wildfire')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /Lagos Flood.*Lagos, Nigeria/i })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /Accra Wildfire.*Greater Accra, Ghana/i })).toBeInTheDocument()
    expect(screen.getByRole('img', { name: /event locations map/i })).toHaveTextContent('Lagos Flood 42')
  })

  it('shows a generic degraded banner without leaking ingestion error details', async () => {
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
      'Latest ingestion did not complete successfully. Data may be delayed while operators investigate.',
    )
    expect(screen.queryByText(/EONET quota exhausted/i)).not.toBeInTheDocument()
  })

  it('updates URL-backed filters and refetches events for the selected country', async () => {
    const user = userEvent.setup()
    renderWithProviders(<EventsDashboard />)

    await screen.findByRole("heading", { level: 3, name: /Lagos Flood 42/i })
    // The filters are now the custom <Select> listbox: open it, then pick the option.
    await user.click(screen.getByRole('combobox', { name: /filter by country/i }))
    await user.click(screen.getByRole('option', { name: 'Ghana' }))

    await waitFor(() => {
      // Trailing 0 is the pagination offset — every fetch now names its page.
      expect(mockFetchEvents).toHaveBeenLastCalledWith(undefined, undefined, 'Ghana', 0)
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

  it('renders the attempted API base URL and underlying error message when VITE_SHOW_ERROR_DETAIL is enabled', async () => {
    vi.stubEnv('VITE_API_BASE_URL', 'https://api.staging.vigilafrica.org')
    vi.stubEnv('VITE_SHOW_ERROR_DETAIL', 'true')
    mockFetchEvents.mockRejectedValue(new Error('Failed to fetch events from VigilAfrica API (HTTP 503)'))

    const { container } = renderWithProviders(<EventsDashboard />)

    expect(await screen.findByText('https://api.staging.vigilafrica.org')).toBeInTheDocument()
    expect(
      screen.getByText(/Failed to fetch events from VigilAfrica API \(HTTP 503\)/i),
    ).toBeInTheDocument()

    const results = await axe(container)
    expect(results.violations).toHaveLength(0)

    vi.unstubAllEnvs()
  })

  it('hides diagnostic detail in production builds without VITE_SHOW_ERROR_DETAIL', async () => {
    vi.stubEnv('VITE_API_BASE_URL', 'https://api.vigilafrica.org')
    vi.stubEnv('VITE_SHOW_ERROR_DETAIL', '')
    mockFetchEvents.mockRejectedValue(new Error('Failed to fetch events from VigilAfrica API (HTTP 503)'))

    renderWithProviders(<EventsDashboard />)

    expect(await screen.findByRole('button', { name: /retry connection/i })).toBeInTheDocument()
    expect(screen.queryByText('https://api.vigilafrica.org')).not.toBeInTheDocument()
    expect(
      screen.queryByText(/Failed to fetch events from VigilAfrica API \(HTTP 503\)/i),
    ).not.toBeInTheDocument()

    vi.unstubAllEnvs()
  })

  it('has no obvious accessibility violations in the loaded dashboard state', async () => {
    const { container } = renderWithProviders(<EventsDashboard />)

    await screen.findByRole("heading", { level: 3, name: /Lagos Flood 42/i })
    const results = await axe(container)
    expect(results.violations).toHaveLength(0)
  })

  it('renders the public disclaimer above the dashboard data', async () => {
    renderWithProviders(<EventsDashboard />)

    const disclaimer = await screen.findByRole('note', {
      name: /important data limitation notice/i,
    })
    expect(disclaimer).toHaveTextContent(/not an official emergency alert system/i)
    expect(disclaimer).toHaveTextContent(/confirm with local authorities/i)
  })

  it('renders a "last updated" indicator when ingestion is healthy', async () => {
    renderWithProviders(<EventsDashboard />)

    await screen.findByRole("heading", { level: 3, name: /Lagos Flood 42/i })
    // Named explicitly: the result count is a second polite live region now.
    expect(screen.getByRole('status', { name: /data freshness/i })).toHaveTextContent(/last updated/i)
  })

  it('renders a placeholder freshness state when last_ingestion is absent', async () => {
    mockFetchHealth.mockResolvedValueOnce({
      status: 'ok',
      version: 'test',
      last_ingestion: null,
    })

    renderWithProviders(<EventsDashboard />)

    await screen.findByRole("heading", { level: 3, name: /Lagos Flood 42/i })
    expect(screen.getByRole('status', { name: /data freshness/i })).toHaveTextContent(/data freshness unknown/i)
  })
})

// ── Pagination (feature-events-pagination) ───────────────────────────────────

const PAGE_SIZE = 50
// Deliberately not a round multiple of PAGE_SIZE: 3,268 is the continental-scale
// projection this change exists to serve, and a ragged last page (18 rows) is
// where off-by-one range arithmetic shows up.
const CONTINENTAL_TOTAL = 3268

// buildPage returns what the API would return for a given offset: a full page
// until the tail, then the remainder, and `meta` echoing the applied values.
function buildPage(offset: number, total = CONTINENTAL_TOTAL): EventsResponse {
  const count = Math.max(0, Math.min(PAGE_SIZE, total - offset))
  return {
    data: Array.from({ length: count }, (_, i) => ({
      ...eventsResponse.data[0],
      id: `event-${offset + i}`,
      source_id: `EONET_${offset + i}`,
      title: `Flood ${offset + i}`,
    })),
    meta: { total, limit: PAGE_SIZE, offset },
  }
}

describe('EventsDashboard pagination', () => {
  beforeEach(() => {
    // Serve whichever page the component asks for, so the offset the UI computes
    // is the thing under test rather than something the mock hard-codes.
    mockFetchEvents.mockImplementation(
      (_category, _state, _country, offset = 0) => Promise.resolve(buildPage(offset)),
    )
    mockFetchStates.mockResolvedValue(['Lagos', 'Greater Accra'])
    mockFetchContext.mockResolvedValue(contextResponse)
    mockFetchHealth.mockResolvedValue(okHealth)
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('discloses the true total rather than presenting the page as complete', async () => {
    renderWithProviders(<EventsDashboard />)

    const count = await screen.findByRole('status', { name: /result count/i })
    expect(count).toHaveTextContent('Showing 1–50 of 3,268 matching events')
  })

  it('requests the first page with offset 0 and an explicit limit', async () => {
    renderWithProviders(<EventsDashboard />)

    await screen.findByRole('status', { name: /result count/i })
    expect(mockFetchEvents).toHaveBeenLastCalledWith(undefined, undefined, undefined, 0)
  })

  it('advances to the next page and updates the visible range', async () => {
    const user = userEvent.setup()
    renderWithProviders(<EventsDashboard />)

    await screen.findByRole('status', { name: /result count/i })
    await user.click(screen.getByRole('button', { name: /next page/i }))

    await waitFor(() => {
      expect(mockFetchEvents).toHaveBeenLastCalledWith(undefined, undefined, undefined, 50)
    })
    await waitFor(() => {
      expect(screen.getByRole('status', { name: /result count/i }))
        .toHaveTextContent('Showing 51–100 of 3,268 matching events')
    })
    expect(screen.getByText('Page 2 of 66')).toBeInTheDocument()
  })

  it('disables Previous on the first page and Next on the last', async () => {
    const user = userEvent.setup()
    renderWithProviders(<EventsDashboard />)

    await screen.findByRole('status', { name: /result count/i })
    expect(screen.getByRole('button', { name: /previous page/i })).toBeDisabled()
    expect(screen.getByRole('button', { name: /next page/i })).toBeEnabled()

    await user.click(screen.getByRole('button', { name: /next page/i }))
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /previous page/i })).toBeEnabled()
    })
  })

  it('disables Next on the ragged final page', async () => {
    // 3,268 / 50 = 65 full pages + 18. Page 66 starts at offset 3,250.
    renderWithProviders(<EventsDashboard />, ['/?page=66'])

    await waitFor(() => {
      expect(screen.getByRole('status', { name: /result count/i }))
        .toHaveTextContent('Showing 3,251–3,268 of 3,268 matching events')
    })
    expect(screen.getByRole('button', { name: /next page/i })).toBeDisabled()
    expect(screen.getByRole('button', { name: /previous page/i })).toBeEnabled()
  })

  it('returns to the first page when a filter changes', async () => {
    const user = userEvent.setup()
    renderWithProviders(<EventsDashboard />, ['/?page=4'])

    await waitFor(() => {
      expect(mockFetchEvents).toHaveBeenLastCalledWith(undefined, undefined, undefined, 150)
    })

    await user.click(screen.getByRole('combobox', { name: /filter by country/i }))
    await user.click(screen.getByRole('option', { name: 'Ghana' }))

    // Offset 0, not 150 — carrying the offset forward would land the user on an
    // empty page of a smaller result set and read as "no events in Ghana".
    await waitFor(() => {
      expect(mockFetchEvents).toHaveBeenLastCalledWith(undefined, undefined, 'Ghana', 0)
    })
  })

  it('keeps the map markers in step with the current page', async () => {
    renderWithProviders(<EventsDashboard />, ['/?page=2'])

    const map = await screen.findByRole('img', { name: /event locations map/i })
    await waitFor(() => {
      expect(map).toHaveTextContent('Flood 50')
    })
    // Page 1's markers must be gone, not merged in.
    expect(map).not.toHaveTextContent('Flood 0,')
  })

  it('treats a page past the end as empty, not as an error, and offers a way back', async () => {
    const user = userEvent.setup()
    renderWithProviders(<EventsDashboard />, ['/?page=999'])

    const count = await screen.findByRole('status', { name: /result count/i })
    await waitFor(() => {
      expect(count).toHaveTextContent('That page is past the end of 3,268 matching events')
    })
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /first page/i }))
    await waitFor(() => {
      expect(mockFetchEvents).toHaveBeenLastCalledWith(undefined, undefined, undefined, 0)
    })
  })

  it('falls back to page 1 for a non-numeric page parameter', async () => {
    renderWithProviders(<EventsDashboard />, ['/?page=banana'])

    await screen.findByRole('status', { name: /result count/i })
    // NaN would otherwise reach the API as `offset=NaN` and come back a 400.
    expect(mockFetchEvents).toHaveBeenLastCalledWith(undefined, undefined, undefined, 0)
  })

  it('hides the pager when everything fits on one page but still states the total', async () => {
    mockFetchEvents.mockImplementation(
      (_category, _state, _country, offset = 0) => Promise.resolve(buildPage(offset, 43)),
    )
    renderWithProviders(<EventsDashboard />)

    const count = await screen.findByRole('status', { name: /result count/i })
    expect(count).toHaveTextContent('Showing 1–43 of 43 matching events')
    expect(screen.queryByRole('button', { name: /next page/i })).not.toBeInTheDocument()
  })

  it('exposes the page controls to the keyboard', async () => {
    const user = userEvent.setup()
    renderWithProviders(<EventsDashboard />)

    await screen.findByRole('status', { name: /result count/i })
    const next = screen.getByRole('button', { name: /next page/i })

    // Native <button>, so it is in the tab order by default; assert nothing has
    // removed it, then drive it with the keyboard rather than a click.
    expect(next.tagName).toBe('BUTTON')
    expect(next).not.toHaveAttribute('tabindex', '-1')

    next.focus()
    expect(next).toHaveFocus()
    await user.keyboard('{Enter}')

    await waitFor(() => {
      expect(mockFetchEvents).toHaveBeenLastCalledWith(undefined, undefined, undefined, 50)
    })
  })

  it('has no accessibility violations with the pager rendered', async () => {
    const { container } = renderWithProviders(<EventsDashboard />)

    await screen.findByRole('button', { name: /next page/i })
    const results = await axe(container)
    expect(results.violations).toHaveLength(0)
  })
})
