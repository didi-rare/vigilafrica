import './LoadingState.css'

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
 */
type Props = {
  message: string
}

export function LoadingState({ message }: Props) {
  return (
    <div className="loading-state" role="status" aria-live="polite">
      <span className="loading-state__spinner" aria-hidden="true" />
      <p>{message}</p>
    </div>
  )
}
