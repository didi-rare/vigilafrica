// F-009: VigilAfrica Landing Page
// Spec: product.md §4 F-009
// Acceptance criteria:
//   - VigilAfrica name and tagline rendered
//   - No Vite/React template content
//   - "early prototype" status notice visible
//   - Links to GitHub repository
//   - Responsive at 375px and 1280px
//   - No API calls (static component only)
import { Fragment, Suspense, lazy } from 'react'
import {
  Satellite,
  Map,
  MapPin,
  Building2,
  Newspaper,
  Truck,
  ShieldAlert,
  ArrowRight
} from 'lucide-react'

import { BrowserRouter as Router, Routes, Route } from 'react-router-dom'
import './App.css'
import MILESTONES from './data/milestones.json'

const GithubIcon = () => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    width="20"
    height="20"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    strokeWidth="2"
    strokeLinecap="round"
    strokeLinejoin="round"
  >
    <path d="M15 22v-4a4.8 4.8 0 0 0-1-3.5c3 0 6-2 6-5.5.08-1.25-.27-2.48-1-3.5.28-1.15.28-2.35 0-3.5 0 0-1 0-3 1.5-2.64-.5-5.36-.5-8 0C6 2 5 2 5 2c-.3 1.15-.3 2.35 0 3.5A5.403 5.403 0 0 0 4 9c0 3.5 3 5.5 6 5.5-.39.49-.68 1.05-.85 1.65-.17.6-.22 1.23-.15 1.85v4" />
    <path d="M9 18c-4.51 2-5-2-7-2" />
  </svg>
)

const GITHUB_URL = 'https://github.com/didi-rare/vigilafrica'

const EventsDashboard = lazy(async () => {
  const module = await import('./components/EventsDashboard')
  return { default: module.EventsDashboard }
})

const EventDetail = lazy(async () => {
  const module = await import('./pages/EventDetail')
  return { default: module.EventDetail }
})
const STEPS = [
  {
    icon: <Satellite size={20} />,
    title: 'Poll',
    desc: 'Fetches natural event data from NASA\u2019s EONET feed \u2014 floods and wildfires, filtered to Nigeria and Ghana.',
  },
  {
    icon: <Map size={20} />,
    title: 'Enrich',
    desc: 'Maps raw coordinates to familiar names — "Benue State, Nigeria" instead of [8.13, 7.33].',
  },
  {
    icon: <MapPin size={20} />,
    title: 'Serve',
    desc: 'Delivers localised event data through a REST API and an interactive map experience.',
  },
]

const AUDIENCE = [
  { icon: <Building2 size={18} />, label: 'NGO Field Teams',     desc: 'Situational awareness without geospatial expertise' },
  { icon: <Newspaper size={18} />, label: 'Local Journalists',   desc: 'Verify event locations by state name, not coordinates' },
  { icon: <Truck size={18} />,     label: 'Logistics Planners',  desc: 'Assess route risk from active flood and wildfire events' },
  { icon: <ShieldAlert size={18} />, label: 'Civic Responders',   desc: 'Community preparedness and local response planning' },
]

function App() {
  return (
    <Router>
      <div id="app">
      {/* ── Prototype Banner ── */}
      <div id="prototype-banner" className="prototype-banner" role="banner" aria-label="Project status">
        🛰️ Active Development — v0.6 complete · v0.7 Second Country Stable active
      </div>

      {/* ── Navigation ── */}
      <nav className="nav" aria-label="Main navigation">
        <div className="nav-logo">
          <span className="logo-icon" aria-hidden="true">◉</span>
          <span className="logo-text">VigilAfrica</span>
        </div>
        <a
          id="nav-github-link"
          href={GITHUB_URL}
          target="_blank"
          rel="noopener noreferrer"
          className="btn btn-outline"
          aria-label="View VigilAfrica on GitHub"
        >
          <GithubIcon />
          <span>View on GitHub</span>
        </a>
      </nav>

      <main>
        <Routes>
          <Route path="/" element={
            <>
              <section id="hero" className="hero" aria-labelledby="hero-heading">
                <div className="hero-glow hero-glow--blue" aria-hidden="true" />
                <div className="hero-glow hero-glow--orange" aria-hidden="true" />

                <div className="container">
                  <div className="event-badges" aria-label="Supported event types">
                    <span className="badge badge--flood">🌊 Floods</span>
                    <span className="badge badge--fire">🔥 Wildfires</span>
                  </div>

                  <h1 id="hero-heading" className="hero-title">
                    What is happening<span className="hero-title--accent"> near you?</span>
                  </h1>

                  <p className="hero-desc">
                    VigilAfrica translates raw NASA satellite event data into local African context &mdash;
                    showing floods and wildfires by country and state, not just coordinates.
                    Open-source. Nigeria and Ghana live.
                  </p>

                  <div className="hero-cta">
                    <a
                      id="hero-github-cta"
                      href={GITHUB_URL}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="btn btn-primary"
                    >
                      Follow on GitHub →
                    </a>
                    <span className="hero-cta-note">Open source · Apache 2.0</span>
                  </div>
                </div>
              </section>

              <Suspense fallback={<div className="container section">Loading dashboard telemetry...</div>}>
                <EventsDashboard />
              </Suspense>

              <section id="how-it-works" className="how-it-works" aria-labelledby="how-heading">
                <div className="container">
                  <span className="section-label">Architecture</span>
                  <h2 id="how-heading" className="section-title">How it works</h2>
                  <p className="section-subtitle">
                    A simple three-stage pipeline turns satellite metadata into something any field
                    coordinator can understand.
                  </p>

                  <div className="steps" role="list">
                    {STEPS.map((step, i) => (
                      <Fragment key={step.title}>
                        <article className="step" role="listitem">
                          <div className="step-icon" aria-hidden="true">{step.icon}</div>
                          <h3>{step.title}</h3>
                          <p>{step.desc}</p>
                        </article>
                        {i < STEPS.length - 1 && (
                          <div className="step-arrow" aria-hidden="true">
                            <ArrowRight size={20} />
                          </div>
                        )}
                      </Fragment>
                    ))}
                  </div>
                </div>
              </section>

              <section id="built-for" className="built-for" aria-labelledby="built-heading">
                <div className="container">
                  <span className="section-label">Use cases</span>
                  <h2 id="built-heading" className="section-title">Built for people on the ground</h2>
                  <p className="section-subtitle">
                    Not for data scientists — for the people who need to act on events quickly.
                  </p>

                  <div className="audience-grid">
                    {AUDIENCE.map((item) => (
                      <article key={item.label} className="audience-card">
                        <div className="audience-icon" aria-hidden="true">{item.icon}</div>
                        <h3>{item.label}</h3>
                        <p>{item.desc}</p>
                      </article>
                    ))}
                  </div>
                </div>
              </section>

              <section id="roadmap" className="status" aria-labelledby="status-heading">
                <div className="container">
                  <div className="status-card glass-effect">
                    <div className="status-header">
                      <span className="status-dot" aria-hidden="true" />
                      <span>Project Status</span>
                    </div>

                    <h2 id="status-heading" className="section-title" style={{ marginBottom: '12px' }}>
                      Building in the open — Nigeria &amp; Ghana live
                    </h2>

                    <p>
                      VigilAfrica is being built milestone by milestone. v0.6 (Country Expansion Model)
                      is complete — Ghana is live alongside Nigeria. **v0.7** (Second Country Stable)
                      is active: enrichment quality validation and full Ghana experience in progress.
                    </p>

                    <nav aria-label="Milestone progress">
                      <ul className="milestone-list">
                        {MILESTONES.map((m) => (
                          <li
                            key={m.label}
                            className={`milestone${m.active ? ' milestone--active' : ''}${m.complete ? ' milestone--complete' : ''}`}
                            aria-current={m.active ? 'step' : undefined}
                          >
                            {m.label}
                            {m.complete && (
                               <span className="milestone-tag milestone-tag--complete">
                                ✅ Complete
                              </span>
                            )}
                            {m.active && (
                              <span className="milestone-tag">
                                🔄 In progress
                              </span>
                            )}
                          </li>
                        ))}
                      </ul>
                    </nav>

                    <a
                      id="status-github-cta"
                      href={GITHUB_URL}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="btn btn-primary"
                    >
                      Follow progress on GitHub →
                    </a>
                  </div>
                </div>
              </section>
            </>
          } />
          <Route
            path="/events/:id"
            element={
              <Suspense fallback={<div className="container section">Loading event telemetry...</div>}>
                <EventDetail />
              </Suspense>
            }
          />
        </Routes>
      </main>

      {/* ── Footer ── */}
      <footer className="footer" role="contentinfo">
        <div className="container">
          <p>
            <span className="logo-text">VigilAfrica</span> is open source under the{' '}
            <a
              href="https://www.apache.org/licenses/LICENSE-2.0"
              target="_blank"
              rel="noopener noreferrer"
            >
              Apache 2.0 License
            </a>
            . Maintained by{' '}
            <a
              href="https://github.com/didi-rare"
              target="_blank"
              rel="noopener noreferrer"
            >
              @didi-rare
            </a>
            .
          </p>
          <p className="footer-sub">
            For collaboration or feedback, open a{' '}
            <a href={`${GITHUB_URL}/issues`} target="_blank" rel="noopener noreferrer">
              GitHub Issue
            </a>
            .
          </p>
        </div>
      </footer>
      </div>
    </Router>
  )
}

export default App



