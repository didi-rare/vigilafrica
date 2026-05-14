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

  it('does NOT render the staging banner on production (VITE_ENV unset)', () => {
    render(<App />)

    expect(screen.queryByLabelText(/test environment notice/i)).not.toBeInTheDocument()
  })

  it('renders the two-CTA hero with explore as primary and GitHub as secondary', () => {
    render(<App />)

    const exploreCta = screen.getByRole('link', { name: /explore latest events/i })
    expect(exploreCta).toHaveAttribute('href', '#dashboard')
    expect(exploreCta).toHaveClass('btn-primary')

    const contributeCta = screen.getByRole('link', { name: /contribute on github/i })
    expect(contributeCta).toHaveAttribute('href', 'https://github.com/didi-rare/vigilafrica')
    expect(contributeCta).toHaveAttribute('target', '_blank')
    expect(contributeCta).toHaveAttribute('rel', expect.stringContaining('noopener'))
  })

  it('renders the locked footer disclaimer text', () => {
    render(<App />)

    expect(
      screen.getByText(/Awareness tool — not an official emergency alert system\./i),
    ).toBeInTheDocument()
  })

  it('renders milestone accessibility labels', () => {
    render(<App />)

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
