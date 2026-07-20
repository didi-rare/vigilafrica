import { useState } from 'react'
import { ThumbsUp, ThumbsDown } from 'lucide-react'
import { track } from '../analytics'
import './FeedbackPrompt.css'

type FeedbackPromptProps = {
  /** The event whose detail page this prompt sits on — sent as `event_id`. */
  eventId: string
}

/**
 * One-click "Was this useful?" feedback row for the event detail page
 * (chore-analytics-and-feedback). A Yes / No click fires a single
 * `feedback_submitted` analytics event and swaps the buttons for a confirmation
 * announced to assistive tech via the polite live region.
 *
 * Accessibility (developers-react.md §9): semantic <button> elements (§9.1),
 * the group is labelled by the visible question (§9.3), the confirmation is an
 * `aria-live="polite"` region present in the DOM before it fills (§9.7), and
 * the buttons carry explicit `aria-label`s so the decorative emoji is not read
 * (§9.11). Focus-visible styling lives in FeedbackPrompt.css (§9.6).
 */
export function FeedbackPrompt({ eventId }: FeedbackPromptProps) {
  const [submitted, setSubmitted] = useState<'yes' | 'no' | null>(null)

  function handleVote(value: 'yes' | 'no') {
    // Fire once per click; analytics is fire-and-forget (track() never throws).
    track('feedback_submitted', { value, event_id: eventId })
    setSubmitted(value)
  }

  return (
    <section className="feedback-prompt" aria-labelledby="feedback-prompt-heading">
      <p id="feedback-prompt-heading" className="feedback-prompt__question">
        Was this useful?
      </p>

      {submitted === null && (
        <div
          className="feedback-prompt__actions"
          role="group"
          aria-labelledby="feedback-prompt-heading"
        >
          <button
            type="button"
            className="feedback-prompt__button"
            onClick={() => handleVote('yes')}
            aria-label="Yes, this event detail was useful"
          >
            <ThumbsUp size={15} aria-hidden="true" /> Yes
          </button>
          <button
            type="button"
            className="feedback-prompt__button"
            onClick={() => handleVote('no')}
            aria-label="No, this event detail was not useful"
          >
            <ThumbsDown size={15} aria-hidden="true" /> No
          </button>
        </div>
      )}

      {/* Live region is always rendered so the announcement is heard on update. */}
      <p className="feedback-prompt__confirmation" role="status" aria-live="polite">
        {submitted !== null && 'Thanks — your feedback was recorded.'}
      </p>
    </section>
  )
}
