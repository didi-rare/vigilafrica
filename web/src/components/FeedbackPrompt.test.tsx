import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { axe } from 'vitest-axe'

import { FeedbackPrompt } from './FeedbackPrompt'

const trackMock = vi.fn()

beforeEach(() => {
  trackMock.mockReset()
  // The Umami tracker attaches window.umami at runtime; stub it so we can
  // assert the analytics.ts track() wrapper forwards the right payload.
  window.umami = { track: trackMock }
})

afterEach(() => {
  delete window.umami
})

describe('FeedbackPrompt', () => {
  it('fires feedback_submitted with value "yes" and the event id on the Yes click', async () => {
    const user = userEvent.setup()
    render(<FeedbackPrompt eventId="evt-123" />)

    await user.click(screen.getByRole('button', { name: /yes, this event detail was useful/i }))

    expect(trackMock).toHaveBeenCalledTimes(1)
    expect(trackMock).toHaveBeenCalledWith('feedback_submitted', {
      value: 'yes',
      event_id: 'evt-123',
    })
  })

  it('fires value "no" and shows a confirmation that replaces the buttons', async () => {
    const user = userEvent.setup()
    render(<FeedbackPrompt eventId="evt-999" />)

    await user.click(screen.getByRole('button', { name: /no, this event detail was not useful/i }))

    expect(trackMock).toHaveBeenCalledWith('feedback_submitted', {
      value: 'no',
      event_id: 'evt-999',
    })
    // Buttons are gone, a status confirmation is announced.
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
    expect(screen.getByRole('status')).toHaveTextContent(/your feedback was recorded/i)
  })

  it('never throws when the tracker is absent', async () => {
    delete window.umami
    const user = userEvent.setup()
    render(<FeedbackPrompt eventId="evt-1" />)

    await user.click(screen.getByRole('button', { name: /yes/i }))
    expect(screen.getByRole('status')).toHaveTextContent(/recorded/i)
  })

  it('has no axe-detectable accessibility violations', async () => {
    const { container } = render(<FeedbackPrompt eventId="evt-1" />)
    const results = await axe(container)
    expect(results.violations).toHaveLength(0)
  })
})
