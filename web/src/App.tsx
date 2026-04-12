// F-009: VigilAfrica Landing Page
// Spec: product.md §4 F-009
// Acceptance criteria:
//   - VigilAfrica name and tagline rendered
//   - No Vite/React template content
//   - "early prototype" status notice visible
//   - Links to GitHub repository
//   - Responsive at 375px and 1280px
//   - No API calls (static component only)
import './App.css'

const GITHUB_URL = 'https://github.com/didi-rare/vigilafrica'

const MILESTONES = [
  { label: 'v0.1 · Foundation', active: true },
  { label: 'v0.2 · First real data flow', active: false },
  { label: 'v0.3 · Localization engine', active: false },
  { label: 'v0.4 · Map + near-me experience', active: false },
]

const STEPS = [
  {
    icon: '📡',
    title: 'Poll',
    desc: 'Fetches natural event data from NASA\u2019s EONET feed \u2014 floods and wildfires, filtered to Nigeria.',
  },
  {
    icon: '🗺️',
    title: 'Enrich',
    desc: 'Maps raw coordinates to familiar names — "Benue State, Nigeria" instead of [8.13, 7.33].',
  },
  {
    icon: '📍',
    title: 'Serve',
    desc: 'Delivers localised event data through a REST API and an interactive map experience.',
  },
]

const AUDIENCE = [
  { icon: '🏥', label: 'NGO Field Teams',     desc: 'Situational awareness without geospatial expertise' },
  { icon: '📰', label: 'Local Journalists',   desc: 'Verify event locations by state name, not coordinates' },
  { icon: '🚛', label: 'Logistics Planners',  desc: 'Assess route risk from active flood and wildfire events' },
  { icon: '🏛️', label: 'Civic Responders',   desc: 'Community preparedness and local response planning' },
]

function App() {
  return (
    <div id="app">
      {/* ── Prototype Banner ── */}
      <div id="prototype-banner" className="prototype-banner" role="banner" aria-label="Project status">
        🚧 Early Prototype — v0.1 in active development · Not yet usable for real event monitoring
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
          View on GitHub
        </a>
      </nav>

      <main>
        {/* ── Hero ── */}
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
              Open-source. Nigeria first.
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

        {/* ── How It Works ── */}
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
              <article key={step.title} className="step" role="listitem">
                <span className="step-icon" aria-hidden="true">{step.icon}</span>
                <h3>{step.title}</h3>
                <p>{step.desc}</p>
                {i < STEPS.length - 1 && (
                  <span className="step-arrow" aria-hidden="true">→</span>
                )}
              </article>
            ))}
            </div>
          </div>
        </section>

        {/* ── Built For ── */}
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
                  <span className="audience-icon" aria-hidden="true">{item.icon}</span>
                  <h3>{item.label}</h3>
                  <p>{item.desc}</p>
                </article>
              ))}
            </div>
          </div>
        </section>

        {/* ── Status / Roadmap ── */}
        <section id="roadmap" className="status" aria-labelledby="status-heading">
          <div className="container">
            <div className="status-card">
              <div className="status-header">
                <span className="status-dot" aria-hidden="true" />
                <span>Project Status</span>
              </div>

              <h2 id="status-heading" className="section-title" style={{ marginBottom: '12px' }}>
                Building in the open — Nigeria first
              </h2>

              <p>
                VigilAfrica is being built milestone by milestone. The current focus (v0.1) is
                establishing the foundation — a working API and this landing page. Event data,
                maps, and localisation are coming in v0.2–v0.4.
              </p>

              <nav aria-label="Milestone progress">
                <ul className="milestone-list">
                  {MILESTONES.map((m) => (
                    <li
                      key={m.label}
                      className={`milestone${m.active ? ' milestone--active' : ''}`}
                      aria-current={m.active ? 'step' : undefined}
                    >
                      {m.label}
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
  )
}

export default App
