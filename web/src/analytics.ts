// Thin, typed wrapper around the self-hosted Umami tracker
// (chore-analytics-and-feedback).
//
// The tracker script is loaded from `%VITE_ANALYTICS_URL%/script.js` in
// index.html and attaches a global `window.umami`. This module is the ONLY
// place the rest of the app talks to it, so:
//   - every custom event has a typed name + payload (no stringly-typed calls
//     scattered through components), and
//   - tracker absence never throws. `window.umami` is undefined in local dev
//     (no VITE_ANALYTICS_URL), when an ad-blocker removes the script, or before
//     the deferred script has loaded. The `?.` guard (proposal R5) keeps
//     analytics strictly fire-and-forget — it must never become a render
//     dependency or surface an error to the user.

// EventMap is the single source of truth for the six v1 custom events. Adding a
// seventh event means adding a line here — and the compiler then forces every
// call site to supply the right payload. See the proposal's "Events to track"
// table; anything beyond these six is over-instrumentation for v1.
export type AnalyticsEventMap = {
  state_filter_selected: { state: string }
  category_filter_selected: { category: string }
  context_resolve: { country: string; state: string }
  event_detail_opened: { event_id: string; category: string; state: string }
  map_marker_clicked: { event_id: string; category: string }
  feedback_submitted: { value: 'yes' | 'no'; event_id: string; reason?: string }
}

export type AnalyticsEventName = keyof AnalyticsEventMap

// Minimal shape of the Umami browser global we actually use. Umami's real
// surface is larger, but the app only ever calls `track`.
type UmamiTracker = {
  track: (eventName: string, eventData?: Record<string, unknown>) => void
}

declare global {
  interface Window {
    umami?: UmamiTracker
  }
}

// Synthetic/headless agents whose traffic is our own measurement, not a user.
// Lighthouse sets `Chrome-Lighthouse`; PageSpeed Insights and plain headless
// Chrome runs are matched for the same reason.
const SYNTHETIC_USER_AGENT = /Chrome-Lighthouse|HeadlessChrome|PageSpeed/i

// Umami's own documented opt-out key. Set it per browser, per device:
//   localStorage.setItem('umami.disabled', 1)
const UMAMI_DISABLED_KEY = 'umami.disabled'

/**
 * isExcluded reports whether this client's activity is our own and should not
 * be recorded. Two distinct sources of self-inflicted noise, both confirmed
 * dominant in the live data (2026-07-26: ~5 sessions in 7 days, nearly all
 * ours):
 *
 *  1. **Maintainer browsing.** Umami's documented opt-out is the
 *     `umami.disabled` localStorage key, which suppresses the tracker's
 *     *automatic pageviews* — but per umami-software/umami#3031 it does not
 *     stop explicit `umami.track()` calls, which is every event this module
 *     sends. Honouring the same key here closes that half, so one console
 *     command now excludes both halves consistently.
 *  2. **Synthetic audit runs.** Lighthouse uses a fresh browser profile per
 *     run, so a localStorage flag can never persist for it. The user agent is
 *     the only usable signal.
 *
 * Deliberately NOT implemented via Umami's `data-before-send` hook, which
 * would also cover auto-pageviews: that requires a global function resolvable
 * by the deferred tracker script, and the `script-src` directive in
 * `web/vercel.json` has already silently broken analytics once (shipped broken
 * in v1.3.0, fixed in v1.3.1). Auto-pageviews from audit runs therefore still
 * land — they are infrequent and identifiable. See
 * `openspec/proposals/chore-analytics-self-exclusion.md` for the tradeoff.
 *
 * Evaluated per call, not memoised, so setting the flag takes effect without a
 * reload.
 */
function isExcluded(): boolean {
  try {
    // `?.` because localStorage is not merely restricted but *absent* in some
    // environments — including this project's own jsdom test env, where jsdom 29
    // delegates storage to Node and Node disables it without
    // `--localstorage-file`. The try/catch then covers the separate case of a
    // present-but-throwing store (private modes, sandboxed iframes).
    // `!= null` treats a stored "0" as set: presence is the signal, not truthiness.
    if (window.localStorage?.getItem(UMAMI_DISABLED_KEY) != null) return true
  } catch {
    // An unreadable store is NOT grounds to suppress — that would silently drop
    // real traffic, inverting the intent. Fall through to the user-agent check.
  }

  return SYNTHETIC_USER_AGENT.test(window.navigator.userAgent)
}

/**
 * Fire a custom analytics event. No-ops silently when the tracker is absent,
 * or when this client is excluded as our own traffic (see `isExcluded`).
 *
 * @example track('state_filter_selected', { state: 'Lagos' })
 */
export function track<E extends AnalyticsEventName>(
  eventName: E,
  data: AnalyticsEventMap[E],
): void {
  if (isExcluded()) return

  // Guard the whole call: window.umami may be undefined, and even when present
  // we never want a tracker bug to bubble into React render or a click handler.
  try {
    window.umami?.track(eventName, data)
  } catch {
    // Intentionally swallowed — analytics is fire-and-forget (proposal R5).
  }
}
