import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'
import { axe } from 'vitest-axe'

import { EventDetail } from './EventDetail'
import { fetchEventById } from '../api/events'
import type { VigilEvent } from '../api/events'

vi.mock('../components/Map', () => ({
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
    fetchEventById: vi.fn(),
  }
})

const mockFetchEventById = vi.mocked(fetchEventById)

const baseEvent: VigilEvent = {
  id: 'evt-1',
  source_id: 'EONET_TEST',
  source: 'eonet',
  title: 'Flood in Test State',
  category: 'floods',
  status: 'open',
  geometry_type: 'Point',
  latitude: 11.7752786,
  longitude: 14.3762427,
  country_name: 'Nigeria',
  state_name: 'Borno',
  event_date: '2026-06-01T12:00:00Z',
  source_url: 'https://eonet.gsfc.nasa.gov/api/v3/events/EONET_TEST',
  ingested_at: '2026-06-01T12:05:00Z',
  enriched_at: '2026-06-01T12:06:00Z',
}

function renderDetail(event: VigilEvent) {
  mockFetchEventById.mockResolvedValue(event)
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[`/events/${event.id}`]}>
        <Routes>
          <Route path="/events/:id" element={<EventDetail />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('EventDetail', () => {
  it('renders the cartographic DMS coordinate readout for the event position', async () => {
    renderDetail(baseEvent)

    // 11.7752786° → 11°47′N ; 14.3762427° → 14°23′E
    expect(await screen.findByText('11°47′N 14°23′E')).toBeInTheDocument()
  })

  it('carries a rounded 60′ into the next degree (never renders invalid minutes)', async () => {
    renderDetail({
      ...baseEvent,
      // 9.9959° → 59.75′ rounds to 60′ → must carry to 10°00′, not 09°60′.
      // -0.9999° exercises the same rollover in the western hemisphere.
      latitude: 9.9959,
      longitude: -0.9999,
    })

    expect(await screen.findByText('10°00′N 01°00′W')).toBeInTheDocument()
    expect(screen.queryByText(/60′/)).not.toBeInTheDocument()
  })

  it('omits the coordinate readout when the event has no position', async () => {
    renderDetail({ ...baseEvent, latitude: null, longitude: null })

    await screen.findByRole('heading', { level: 1, name: /flood in test state/i })
    expect(screen.queryByText(/[0-9]{2}°[0-9]{2}′/)).not.toBeInTheDocument()
  })

  it('has no axe-detectable accessibility violations', async () => {
    const { container } = renderDetail(baseEvent)
    await screen.findByRole('heading', { level: 1, name: /flood in test state/i })

    const results = await axe(container)
    expect(results.violations).toHaveLength(0)
  })
})
