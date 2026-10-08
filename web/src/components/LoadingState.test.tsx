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
    // Same card as the inner dashboard states, so the outer Suspense fallback
    // and the data-fetch state are one treatment (chore-web-audit-leftovers 5).
    expect(region).toHaveClass('dashboard-state')
  })

  it('keeps the spinner decorative so the wait is announced once', () => {
    const { container } = render(<LoadingState message="Fetching satellite telemetry..." />)

    const spinner = container.querySelector('.spinner')
    expect(spinner).not.toBeNull()
    expect(spinner).toHaveAttribute('aria-hidden', 'true')
    expect(screen.getByRole('status')).not.toHaveAttribute('aria-label')
  })

  it('has no accessibility violations', async () => {
    const { container } = render(<LoadingState message="Loading map telemetry..." />)

    const results = await axe(container)
    expect(results.violations).toHaveLength(0)
  })
})
