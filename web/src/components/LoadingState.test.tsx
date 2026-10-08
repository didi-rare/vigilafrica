import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { axe } from 'vitest-axe'

import { LoadingState } from './LoadingState'

describe('LoadingState', () => {
  it('is a polite status live region whose text is the visible message', () => {
    render(<LoadingState message="Loading dashboard telemetry..." />)

    const region = screen.getByRole('status')
    expect(region).toHaveAttribute('aria-live', 'polite')
    expect(region).toHaveTextContent('Loading dashboard telemetry...')
  })

  it('keeps the spinner decorative so the wait is announced once', () => {
    render(<LoadingState message="Fetching satellite telemetry..." />)

    const region = screen.getByRole('status')
    // The only thing in the region besides the message is the spinner; it must
    // be hidden from the accessibility tree and carry no label of its own, so
    // the accessible content of the region is exactly the message.
    const hidden = region.querySelectorAll('[aria-hidden="true"]')
    expect(hidden).toHaveLength(1)
    expect(hidden[0]).not.toHaveAttribute('aria-label')
    expect(region).not.toHaveAttribute('aria-label')
    expect(region).toHaveTextContent(/^Fetching satellite telemetry\.\.\.$/)
  })

  it('renders the same treatment without a live region when announce is false', () => {
    render(<LoadingState message="Loading map telemetry..." announce={false} />)

    expect(screen.queryByRole('status')).toBeNull()
    expect(screen.getByText('Loading map telemetry...')).toBeInTheDocument()
  })

  it('has no accessibility violations', async () => {
    const { container } = render(<LoadingState message="Loading map telemetry..." />)

    const results = await axe(container)
    expect(results.violations).toHaveLength(0)
  })
})
