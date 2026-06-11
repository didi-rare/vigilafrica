import { render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
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

  describe('staging banner (VITE_ENV=staging)', () => {
    afterEach(() => {
      vi.unstubAllEnvs()
    })

    it('renders the banner with icon, copy, and accessible label', () => {
      vi.stubEnv('VITE_ENV', 'staging')

      render(<App />)

      const banner = screen.getByLabelText(/test environment notice/i)
      expect(banner).toBeInTheDocument()
      expect(banner).toHaveClass('staging-banner')
      expect(banner).toHaveAttribute('role', 'note')
      expect(banner).toHaveTextContent(/Staging environment — pre-release\/test data/i)

      // Icon is decoratively hidden so screen readers don't double-announce
      const icon = banner.querySelector('.staging-banner__icon')
      expect(icon).not.toBeNull()
      expect(icon).toHaveAttribute('aria-hidden', 'true')
    })
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
    const { container } = render(<App />)

    expect(screen.getByText(/v0.7 · Second country stable/i)).toHaveTextContent(
      /v0\.7 · Second country stable\s+Complete/,
    )
    // Milestone status icons (complete / in-progress) are now SVGs and must be
    // decorative (aria-hidden) — the text label carries the meaning. There may be
    // zero in-progress icons when no milestone is active, so assert ≥1 and all hidden.
    const statusIcons = container.querySelectorAll('.milestone-tag svg')
    expect(statusIcons.length).toBeGreaterThan(0)
    statusIcons.forEach((icon) => {
      expect(icon).toHaveAttribute('aria-hidden', 'true')
    })
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
