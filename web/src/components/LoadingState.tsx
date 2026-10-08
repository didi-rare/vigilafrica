/**
 * LoadingState is the single loading treatment for the dashboard's three
 * loading regions (chore-web-audit-leftovers items 3 and 5): the lazy-chunk
 * Suspense fallback in App.tsx, the events data-fetch state, and the map's own
 * Suspense fallback. Before this component the outer fallback was plain text
 * while the inner ones showed a spinner — two treatments for what is, to the
 * user, one wait.
 *
 * Accessibility (developers-react.md §9.7/§9.8): the region is `role="status"`
 * with an explicit `aria-live="polite"`, the same pattern FreshnessIndicator
 * and the result count already use. The visible message IS the live text, so
 * the spinner is decorative (`aria-hidden`) rather than carrying its own
 * `aria-label` — labelling both would announce the wait twice. No `aria-busy`
 * is set on an ancestor: it asks assistive technology to defer changes under
 * the busy element, which would suppress exactly this announcement.
 *
 * Styling lives in App.css (`.dashboard-state`, `.spinner`), not in the
 * dashboard's stylesheet: that file is bundled into the lazy chunk's CSS, and
 * the outer fallback renders before the chunk exists.
 */
export function LoadingState({ message }: { message: string }) {
  return (
    <div className="dashboard-state loading" role="status" aria-live="polite">
      <span className="spinner" aria-hidden="true" />
      <p>{message}</p>
    </div>
  )
}
