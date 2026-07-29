import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { track } from './analytics'

const trackMock = vi.fn()

// ⚠️ `window.localStorage` is UNDEFINED in this test environment, not merely
// empty: jsdom 29 delegates storage to Node, and Node disables localStorage
// unless started with `--localstorage-file`. Touching it directly in a hook
// throws a TypeError and fails every test in the file. Every case below that
// needs storage installs this in-memory stub first; the cases that need storage
// to be *absent* simply omit it, which mirrors the real environment exactly.
function installStorage(seed: Record<string, string> = {}): void {
  const store = new Map(Object.entries(seed))
  Object.defineProperty(window, 'localStorage', {
    configurable: true,
    writable: true,
    value: {
      getItem: (key: string) => (store.has(key) ? store.get(key)! : null),
      setItem: (key: string, value: string) => void store.set(key, String(value)),
      removeItem: (key: string) => void store.delete(key),
      clear: () => store.clear(),
      key: (index: number) => [...store.keys()][index] ?? null,
      get length() {
        return store.size
      },
    },
  })
}

function removeStorage(): void {
  Reflect.deleteProperty(window, 'localStorage')
}

/** Point navigator.userAgent at `ua` for the duration of one test. */
function stubUserAgent(ua: string): void {
  vi.spyOn(window.navigator, 'userAgent', 'get').mockReturnValue(ua)
}

beforeEach(() => {
  trackMock.mockReset()
  window.umami = { track: trackMock }
})

afterEach(() => {
  delete window.umami
  removeStorage()
  vi.restoreAllMocks()
})

describe('track', () => {
  it('forwards the event name and payload to the Umami tracker', () => {
    track('state_filter_selected', { state: 'Lagos' })

    expect(trackMock).toHaveBeenCalledTimes(1)
    expect(trackMock).toHaveBeenCalledWith('state_filter_selected', { state: 'Lagos' })
  })

  it('never throws when the tracker is absent', () => {
    delete window.umami

    expect(() => track('context_resolve', { country: 'NG', state: 'Lagos' })).not.toThrow()
  })

  it('never throws when the tracker itself throws', () => {
    window.umami = {
      track: () => {
        throw new Error('tracker exploded')
      },
    }

    expect(() => track('map_marker_clicked', { event_id: 'e1', category: 'floods' })).not.toThrow()
  })
})

describe('track — self-exclusion', () => {
  // The deployed Umami tracker already honours `umami.disabled` for explicit
  // track() calls as well as automatic pageviews, so this wrapper's flag check
  // is redundant defence in depth rather than a gap-filler (an earlier comment
  // here claimed otherwise and was wrong — see analytics.ts). These cases still
  // pin the behaviour: if the wrapper ever becomes the only gate, or the
  // tracker changes, the contract is asserted here rather than assumed.
  it('suppresses events when the umami.disabled flag is set', () => {
    installStorage({ 'umami.disabled': '1' })

    track('state_filter_selected', { state: 'Lagos' })

    expect(trackMock).not.toHaveBeenCalled()
  })

  it('suppresses events for any set flag value, including "0"', () => {
    // Umami's documented usage sets 1, but the key's *presence* is the signal —
    // a stray "0" or "false" must not read as "tracking enabled".
    installStorage({ 'umami.disabled': '0' })

    track('state_filter_selected', { state: 'Lagos' })

    expect(trackMock).not.toHaveBeenCalled()
  })

  it('records events when storage is present but the flag is unset', () => {
    installStorage({ 'unrelated.key': 'x' })

    track('category_filter_selected', { category: 'wildfires' })

    expect(trackMock).toHaveBeenCalledTimes(1)
  })

  it.each([
    ['Chrome-Lighthouse', 'Mozilla/5.0 (Linux; Android 11; moto g power) Chrome-Lighthouse'],
    ['HeadlessChrome', 'Mozilla/5.0 (X11; Linux x86_64) HeadlessChrome/150.0.0.0'],
    ['PageSpeed', 'Mozilla/5.0 (compatible) PageSpeed Insights'],
  ])('suppresses events for synthetic agent %s', (_label, ua) => {
    stubUserAgent(ua)

    track('event_detail_opened', { event_id: 'e1', category: 'floods', state: 'Lagos' })

    expect(trackMock).not.toHaveBeenCalled()
  })

  it('still records events for an ordinary mobile browser user agent', () => {
    stubUserAgent(
      'Mozilla/5.0 (Linux; Android 11; moto g power) AppleWebKit/537.36 Chrome/150.0.0.0 Mobile Safari/537.36',
    )

    track('category_filter_selected', { category: 'wildfires' })

    expect(trackMock).toHaveBeenCalledTimes(1)
  })

  it('records events when localStorage is absent entirely and the agent is real', () => {
    // The real state of this test environment, and of locked-down browsers. An
    // unavailable store must not read as "excluded".
    removeStorage()

    track('feedback_submitted', { value: 'yes', event_id: 'e1' })

    expect(trackMock).toHaveBeenCalledTimes(1)
  })

  it('records events when localStorage is present but throws on access', () => {
    installStorage()
    vi.spyOn(window.localStorage, 'getItem').mockImplementation(() => {
      throw new Error('SecurityError: localStorage is not available')
    })

    track('feedback_submitted', { value: 'no', event_id: 'e1', reason: 'not useful' })

    expect(trackMock).toHaveBeenCalledTimes(1)
  })

  it('re-reads the flag on every call so it takes effect without a reload', () => {
    installStorage()

    track('state_filter_selected', { state: 'Lagos' })
    expect(trackMock).toHaveBeenCalledTimes(1)

    window.localStorage.setItem('umami.disabled', '1')
    track('state_filter_selected', { state: 'Kano' })

    expect(trackMock).toHaveBeenCalledTimes(1)
  })
})
