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
export interface AnalyticsEventMap {
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
interface UmamiTracker {
  track: (eventName: string, eventData?: Record<string, unknown>) => void
}

declare global {
  interface Window {
    umami?: UmamiTracker
  }
}

/**
 * Fire a custom analytics event. No-ops silently when the tracker is absent.
 *
 * @example track('state_filter_selected', { state: 'Lagos' })
 */
export function track<E extends AnalyticsEventName>(
  eventName: E,
  data: AnalyticsEventMap[E],
): void {
  // Guard the whole call: window.umami may be undefined, and even when present
  // we never want a tracker bug to bubble into React render or a click handler.
  try {
    window.umami?.track(eventName, data)
  } catch {
    // Intentionally swallowed — analytics is fire-and-forget (proposal R5).
  }
}
