import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { axe } from 'vitest-axe'

import App from './App'

vi.mock('./components/EventsDashboard', () => ({
  EventsDashboard: () => (
    <section id="dashboard" aria-labelledby="dashboard-heading">
      <h2 id="dashboard-heading">Latest Localized Events</h2>
    </section>
  ),
}))

vi.mock('./pages/EventDetail', () => ({
  EventDetail: () => <div>Event detail</div>,
}))

function getFocusableElements(container: HTMLElement) {
  return Array.from(
    container.querySelectorAll<HTMLElement>(
      'a[href], button, input, select, textarea, [tabindex]:not([tabindex="-1"])',
    ),
  ).filter((element) => !element.hasAttribute('disabled'))
}

describe('App', () => {
  it('renders skip-to-main as the first focusable element', () => {
    const { container } = render(<App />)

    const focusable = getFocusableElements(container)
    expect(focusable[0]).toHaveTextContent('Skip to main content')
    expect(focusable[0]).toHaveAttribute('href', '#main')
    expect(container.querySelector('main')).toHaveAttribute('id', 'main')
  })

  it('renders current status copy and milestone accessibility labels', async () => {
    render(<App />)

    expect(
      screen.getByRole('banner', { name: /project status/i }),
    ).toHaveTextContent('v0.7 complete · v1.0 staging live · production launch in progress')
    expect(screen.getByText(/v1.0 \(Credible Public Launch\) is active/i)).toBeInTheDocument()
    expect(screen.getByText(/v0.7 · Second country stable/i)).toHaveTextContent(
      'v0.7 · Second country stable ✅ Complete',
    )
    for (const icon of screen.getAllByText('✅')) {
      expect(icon).toHaveAttribute('aria-hidden', 'true')
    }
    expect(screen.getByText('🔄')).toHaveAttribute('aria-hidden', 'true')
  })

  it('uses valid list semantics for the process steps', () => {
    const { container } = render(<App />)

    expect(screen.getByText('Poll').closest('li')).toHaveClass('step')
    expect(screen.getByText('Enrich').closest('li')).toHaveClass('step')
    expect(screen.getByText('Serve').closest('li')).toHaveClass('step')
    expect(container.querySelector('article[role="listitem"]')).toBeNull()
  })

  it('has no aria-allowed-role violations on the landing page', async () => {
    const { container } = render(<App />)

    const results = await axe(container, {
      rules: {
        'aria-allowed-role': { enabled: true },
      },
    })
    expect(results.violations).toHaveLength(0)
  })
})
