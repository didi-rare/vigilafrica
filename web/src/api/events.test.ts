import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { fetchEvents, fetchStates } from './events'

const fetchMock = vi.fn()

describe('events API client', () => {
  beforeEach(() => {
    fetchMock.mockResolvedValue({
      ok: true,
      json: async () => ({ data: [], meta: { total: 0, limit: 50, offset: 0 } }),
    })
    vi.stubGlobal('fetch', fetchMock)
    window.history.pushState({}, '', '/dashboard')
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    fetchMock.mockReset()
  })

  it('builds event list URLs with category, state, and country filters', async () => {
    await fetchEvents('floods', 'Lagos', 'Nigeria')

    expect(fetchMock).toHaveBeenCalledTimes(1)
    const requestedUrl = new URL(String(fetchMock.mock.calls[0][0]))

    expect(requestedUrl.origin).toBe('https://vigil.test')
    expect(requestedUrl.pathname).toBe('/v1/events')
    expect(requestedUrl.searchParams.get('category')).toBe('floods')
    expect(requestedUrl.searchParams.get('state')).toBe('Lagos')
    expect(requestedUrl.searchParams.get('country')).toBe('Nigeria')
  })

  it('omits empty event filters from the request URL', async () => {
    await fetchEvents(undefined, undefined, 'Ghana')

    const requestedUrl = new URL(String(fetchMock.mock.calls[0][0]))

    expect(requestedUrl.searchParams.has('category')).toBe(false)
    expect(requestedUrl.searchParams.has('state')).toBe(false)
    expect(requestedUrl.searchParams.get('country')).toBe('Ghana')
  })

  it('builds state URLs with an optional country filter', async () => {
    fetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ states: ['Greater Accra'] }),
    })

    await expect(fetchStates('Ghana')).resolves.toEqual(['Greater Accra'])

    const requestedUrl = new URL(String(fetchMock.mock.calls[0][0]))
    expect(requestedUrl.pathname).toBe('/v1/states')
    expect(requestedUrl.searchParams.get('country')).toBe('Ghana')
  })

  it('normalizes failed event requests into a user-safe error', async () => {
    fetchMock.mockResolvedValueOnce({ ok: false })

    await expect(fetchEvents()).rejects.toThrow('Failed to fetch events from VigilAfrica API')
  })
})
