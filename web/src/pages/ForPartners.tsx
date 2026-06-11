// Public /for-partners page (sprint Day 5).
//
// A single, honest surface to point partner conversations at — humanitarian /
// DRR orgs (e.g. anticipatory-action focal points), newsrooms and data
// journalists, logistics and civic responders. Covers what VigilAfrica
// provides (API + daily flood digest + map), how to integrate, the
// supplementary / non-warranty framing (ADR: awareness tool, never sole
// source), and the Apache-2.0 open-source posture.
//
// Static component: no API calls, no new analytics events (the Umami pageview
// is captured automatically; the six v1 custom events are intentionally a
// closed set — see analytics.ts).
import { useEffect, type ReactNode } from 'react'
import { Link } from 'react-router-dom'
import {
  Satellite,
  Newspaper,
  Map,
  MapPin,
  Building2,
  Truck,
  ShieldAlert,
} from 'lucide-react'

import { getApiBaseUrl } from '../api/events'
import './ForPartners.css'

const GITHUB_URL = 'https://github.com/didi-rare/vigilafrica'
const OPENAPI_URL = `${GITHUB_URL}/blob/main/openspec/specs/vigilafrica/openapi.yaml`
const NEW_ISSUE_URL = `${GITHUB_URL}/issues/new`
const DISCUSSIONS_URL = `${GITHUB_URL}/discussions`
const LICENSE_URL = 'https://www.apache.org/licenses/LICENSE-2.0'

type Capability = {
  icon: ReactNode
  title: string
  desc: string
}

const CAPABILITIES: Capability[] = [
  {
    icon: <Satellite size={20} />,
    title: 'Localised event API',
    desc: 'A REST API that serves active floods and wildfires resolved to country and state names — not bare coordinates. Filter by country, state, or category.',
  },
  {
    icon: <Newspaper size={20} />,
    title: 'Daily flood digest',
    desc: 'A machine-readable digest of the day’s flood events by administrative name, served as JSON and deliverable as a daily email to your focal points.',
  },
  {
    icon: <Map size={20} />,
    title: 'Map & dashboard',
    desc: 'A hosted, link-shareable map and dashboard your team can use directly — no GIS setup, no account, no install.',
  },
  {
    icon: <MapPin size={20} />,
    title: 'Open data layer',
    desc: 'Apache-2.0 licensed and self-hostable. Run your own instance, credit VigilAfrica as the data layer, or build on top — no lock-in.',
  },
]

type Endpoint = {
  method: string
  path: string
  desc: string
}

const ENDPOINTS: Endpoint[] = [
  { method: 'GET', path: '/v1/events', desc: 'List flood & wildfire events — filter by country, state, category.' },
  { method: 'GET', path: '/v1/events/{id}', desc: 'Full detail for a single event.' },
  { method: 'GET', path: '/v1/context', desc: 'Situational “near me” resolution and nearby events.' },
  { method: 'GET', path: '/v1/states', desc: 'Distinct state / region names with active data.' },
  { method: 'GET', path: '/v1/digest/today.json', desc: 'Today’s flood digest, by administrative name.' },
  { method: 'GET', path: '/health', desc: 'Service status, version, and ingestion freshness.' },
]

type Audience = {
  icon: ReactNode
  label: string
  desc: string
}

const AUDIENCES: Audience[] = [
  {
    icon: <Building2 size={18} />,
    label: 'Humanitarian & DRR teams',
    desc: 'Supplementary situational awareness for anticipatory-action and early-warning workflows, by admin name.',
  },
  {
    icon: <Newspaper size={18} />,
    label: 'Newsrooms & data journalists',
    desc: 'A credited, verifiable event-data layer for flood and wildfire reporting across Nigeria and Ghana.',
  },
  {
    icon: <Truck size={18} />,
    label: 'Logistics & operations',
    desc: 'Route- and area-level risk signals drawn from active flood and wildfire events.',
  },
  {
    icon: <ShieldAlert size={18} />,
    label: 'Civic & community responders',
    desc: 'Plain-language, location-first awareness for local preparedness and response planning.',
  },
]

export function ForPartners() {
  // The public API origin for whatever environment this page is served from
  // (prod → api.vigilafrica.org, staging → api.staging…). Sourced from
  // VITE_API_BASE_URL via the api layer — never hard-coded (React §15.4, §5.4).
  const apiBaseUrl = getApiBaseUrl()

  // Per-page title (the site has no react-helmet; index.html title is static).
  // Restore the previous title on unmount so SPA navigation stays correct.
  useEffect(() => {
    const previousTitle = document.title
    document.title = 'For partners — VigilAfrica'
    return () => {
      document.title = previousTitle
    }
  }, [])

  return (
    <div className="for-partners-page">
      {/* ── Hero ── */}
      <section id="partners-hero" className="partners-hero" aria-labelledby="partners-hero-heading">
        <div className="container">
          <span className="section-label">For partners</span>
          <h1 id="partners-hero-heading" className="hero-title">
            An open event-data layer for African flood &amp; wildfire response
          </h1>
          <p className="hero-desc">
            VigilAfrica turns NASA satellite event data into local administrative context — floods and
            wildfires by country and state across Nigeria and Ghana. It is built to <em>support</em> the
            people and organisations already doing response work, as an open, supplementary data layer.
          </p>
          <div className="hero-cta">
            <a
              id="partners-contact-cta"
              href={NEW_ISSUE_URL}
              target="_blank"
              rel="noopener noreferrer"
              className="btn btn-primary"
            >
              Start a conversation →
            </a>
            <a
              id="partners-api-cta"
              href={OPENAPI_URL}
              target="_blank"
              rel="noopener noreferrer"
              className="btn btn-outline"
            >
              Read the API reference
            </a>
          </div>
          <p className="hero-cta-note">Open source · Nigeria and Ghana live · Apache 2.0</p>
        </div>
      </section>

      {/* ── What we provide ── */}
      <section id="partners-capabilities" className="partners-section" aria-labelledby="partners-capabilities-heading">
        <div className="container">
          <h2 id="partners-capabilities-heading" className="section-title">What VigilAfrica provides</h2>
          <p className="section-subtitle">
            Four ways a partner can use VigilAfrica — from a raw API to a ready-to-share dashboard.
          </p>
          <ul className="partners-card-grid" role="list">
            {CAPABILITIES.map((cap) => (
              <li key={cap.title} className="partners-card">
                <div className="partners-card__icon" aria-hidden="true">{cap.icon}</div>
                <h3 className="partners-card__title">{cap.title}</h3>
                <p className="partners-card__desc">{cap.desc}</p>
              </li>
            ))}
          </ul>
        </div>
      </section>

      {/* ── Integrate ── */}
      <section id="partners-integrate" className="partners-section" aria-labelledby="partners-integrate-heading">
        <div className="container">
          <span className="section-label">Integrate</span>
          <h2 id="partners-integrate-heading" className="section-title">A small, stable REST API</h2>
          <p className="section-subtitle">
            JSON over HTTPS, no key required to read. Base URL{' '}
            <code className="partners-inline-code">{apiBaseUrl}</code>.
          </p>
          <div className="partners-endpoints-panel">
            <div className="partners-endpoints-head">
              <span className="mono-label">API&nbsp;REFERENCE</span>
              <span className="signal-dot signal-dot--sm" aria-hidden="true" />
            </div>
            <dl className="partners-endpoints">
              {ENDPOINTS.map((ep) => (
                <div key={ep.path} className="partners-endpoint">
                  <dt className="partners-endpoint__sig">
                    <span className="partners-endpoint__method">{ep.method}</span>
                    <code className="partners-endpoint__path">{ep.path}</code>
                  </dt>
                  <dd className="partners-endpoint__desc">{ep.desc}</dd>
                </div>
              ))}
            </dl>
          </div>
          <div className="partners-links">
            <a href={OPENAPI_URL} target="_blank" rel="noopener noreferrer">
              Full OpenAPI specification
            </a>
            <span aria-hidden="true"> · </span>
            <a href={`${apiBaseUrl}/v1/events`} target="_blank" rel="noopener noreferrer">
              Try a live request
            </a>
          </div>
        </div>
      </section>

      {/* ── Responsible use / supplementary framing ── */}
      <section id="partners-responsible" className="partners-section" aria-labelledby="partners-responsible-heading">
        <div className="container">
          <div className="partners-callout glass-effect">
            <div className="partners-callout__icon" aria-hidden="true">
              <ShieldAlert size={22} />
            </div>
            <div>
              <h2 id="partners-responsible-heading" className="partners-callout__title">
                Supplementary, never the sole source
              </h2>
              <p className="partners-callout__body">
                VigilAfrica is an awareness tool, not an official emergency-alert system. It is designed to
                sit <strong>alongside</strong> authoritative sources — NiMet, NEMA, NADMO, Ghana Met and your
                own field verification — never to replace them. We make no warranty of completeness or
                accuracy, and any operational decision should be cross-checked against official channels.
                Partnerships are framed around this from day one.
              </p>
            </div>
          </div>
        </div>
      </section>

      {/* ── Who we partner with ── */}
      <section id="partners-audiences" className="partners-section" aria-labelledby="partners-audiences-heading">
        <div className="container">
          <h2 id="partners-audiences-heading" className="section-title">Who we partner with</h2>
          <p className="section-subtitle">
            If your work touches floods or wildfires in Nigeria or Ghana, there is probably a fit.
          </p>
          <div className="audience-grid">
            {AUDIENCES.map((item) => (
              <article key={item.label} className="audience-card">
                <div className="audience-icon" aria-hidden="true">{item.icon}</div>
                <h3>{item.label}</h3>
                <p>{item.desc}</p>
              </article>
            ))}
          </div>
        </div>
      </section>

      {/* ── Open source + contact ── */}
      <section id="partners-contact" className="partners-section" aria-labelledby="partners-contact-heading">
        <div className="container">
          <div className="partners-contact-card glass-effect">
            <h2 id="partners-contact-heading" className="section-title">Let&rsquo;s talk</h2>
            <p>
              VigilAfrica is open source under the{' '}
              <a href={LICENSE_URL} target="_blank" rel="noopener noreferrer">Apache&nbsp;2.0</a>{' '}
              license and built in the open. The best way to start a partnership, pilot, or integration
              conversation is to open a GitHub issue or discussion — we keep all collaboration in the
              open repository.
            </p>
            <div className="hero-cta">
              <a
                href={NEW_ISSUE_URL}
                target="_blank"
                rel="noopener noreferrer"
                className="btn btn-primary"
              >
                Open a GitHub issue →
              </a>
              <a
                href={DISCUSSIONS_URL}
                target="_blank"
                rel="noopener noreferrer"
                className="btn btn-outline"
              >
                Start a discussion
              </a>
            </div>
            <p className="partners-back">
              <Link to="/">← Back to the dashboard</Link>
            </p>
          </div>
        </div>
      </section>
    </div>
  )
}
