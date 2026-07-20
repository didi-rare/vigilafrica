import { render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { axe } from 'vitest-axe'
import { MemoryRouter } from 'react-router-dom'

import { ForPartners } from './ForPartners'

function renderPage() {
  return render(
    <MemoryRouter>
      <ForPartners />
    </MemoryRouter>,
  )
}

afterEach(() => {
  // The page sets document.title in an effect; reset between tests.
  document.title = ''
})

describe('ForPartners', () => {
  it('renders the page h1', () => {
    renderPage()
    expect(
      screen.getByRole('heading', { level: 1, name: /open event-data layer/i }),
    ).toBeInTheDocument()
  })

  it('routes partnership contact through GitHub (ADR-006: no published email)', () => {
    renderPage()
    // No mailto: link anywhere on the page.
    const mailto = document.querySelectorAll('a[href^="mailto:"]')
    expect(mailto).toHaveLength(0)
    // Primary CTA points at the GitHub issue tracker.
    const issueLinks = screen.getAllByRole('link', { name: /github issue|start a conversation/i })
    expect(issueLinks.length).toBeGreaterThan(0)
    expect(issueLinks[0]).toHaveAttribute('href', expect.stringContaining('github.com/didi-rare/vigilafrica/issues'))
  })

  it('states the supplementary / non-warranty framing', () => {
    renderPage()
    expect(screen.getByRole('heading', { name: /supplementary, never the sole source/i })).toBeInTheDocument()
  })

  it('sets a per-page document title', () => {
    renderPage()
    expect(document.title).toBe('For partners — VigilAfrica')
  })

  it('has no axe-detectable accessibility violations', async () => {
    const { container } = renderPage()
    const results = await axe(container)
    expect(results.violations).toHaveLength(0)
  })
})
