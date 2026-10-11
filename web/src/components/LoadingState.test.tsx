import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { axe } from 'vitest-axe'

import { LoadingState } from './LoadingState'

describe('LoadingState', () => {
  it('is a polite, individually addressable status live region whose text is the visible message', () => {
    render(<LoadingState message="Loading dashboard telemetry..." />)

    const region = screen.getByRole('status', { name: /loading status/i })
    expect(region).toHaveAttribute('aria-live', 'polite')
    expect(region).toHaveTextContent('Loading dashboard telemetry...')
  })

  it('keeps the spinner decorative so the wait is announced once', () => {
    render(<LoadingState message="Fetching satellite telemetry..." />)

    const region = screen.getByRole('status', { name: /loading status/i })
    // The live content is exactly the message: nothing else in the region
    // contributes text, and the spinner is outside the accessibility tree.
    expect(region).toHaveTextContent(/^Fetching satellite telemetry\.\.\.$/)
    expect(region.querySelector('[aria-hidden="true"]')).not.toBeNull()
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
