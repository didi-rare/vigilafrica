import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { axe } from 'vitest-axe'

import App from './App'

// Separate file from App.test.tsx on purpose: a module mock is file-wide, and
// this one never resolves, which is exactly the state under test — the lazy
// dashboard chunk is still in flight, so the Suspense fallback is the steady
// state (chore-web-audit-leftovers items 3–5).
vi.mock('./components/EventsDashboard', () => new Promise(() => {}))

vi.mock('./pages/EventDetail', () => ({
  EventDetail: () => <div>Event detail</div>,
}))

describe('App — dashboard chunk pending', () => {
  it('announces the wait through a polite status region inside the reservation', () => {
    render(<App />)

    const status = screen.getByRole('status', { name: /loading status/i })
    expect(status).toHaveAttribute('aria-live', 'polite')
    expect(status).toHaveTextContent('Loading dashboard telemetry...')
    // The region lives inside the height reservation (the CLS fix from #193),
    // which has no accessible handle of its own.
    expect(status.closest('.dashboard-fallback')).not.toBeNull()
  })

  it('shows the fixed progress bar as a decorative affordance', () => {
    const { container } = render(<App />)

    // Decorative by design (the status region carries the announcement), so it
    // has no accessible handle to query by; the class is the contract the CLS
    // and fallback-capture harnesses measure against.
    const bar = container.querySelector('.dashboard-fallback__progress')
    expect(bar).not.toBeNull()
    expect(bar).toHaveAttribute('aria-hidden', 'true')
  })

  it('has no accessibility violations while the fallback is showing', async () => {
    const { container } = render(<App />)

    const results = await axe(container)
    expect(results.violations).toHaveLength(0)
  })
})
