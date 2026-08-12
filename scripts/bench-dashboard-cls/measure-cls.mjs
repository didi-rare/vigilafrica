// Layout-shift A/B for the events dashboard.
//
// Compares the CLS of two built-and-served versions of the site under identical
// mocked API responses, with the lazy dashboard chunk deliberately delayed so the
// `.dashboard-fallback` reservation is genuinely on screen before the mount.
//
// A CONTROL arm is mandatory here, not optional: the page already carries a small
// pre-existing shift (the freshness banner appearing), so a single-arm number
// cannot tell "my change shifts the page" from "the page already shifted".
//
// Usage — see README.md. Both arms must already be built and served:
//   BASELINE_URL=http://localhost:4174/ BRANCH_URL=http://localhost:4173/ \
//     node scripts/bench-dashboard-cls/measure-cls.mjs
import { chromium } from 'playwright'

const BASELINE_URL = process.env.BASELINE_URL ?? 'http://localhost:4174/'
const BRANCH_URL   = process.env.BRANCH_URL   ?? 'http://localhost:4173/'
const RUNS         = Number(process.env.RUNS ?? 8)
// Viewport taller than the `.dashboard-fallback` cap, so the cap is what governs
// the reservation rather than 100vh.
const VIEWPORT     = { width: 1920, height: 1600 }
// Without this the chunk arrives too fast to observe the shift it exists to prevent.
const CHUNK_DELAY_MS = Number(process.env.CHUNK_DELAY_MS ?? 1200)
const TOTAL = 3268   // continental-scale projection
const PAGE  = 50

const eventsPayload = {
  data: Array.from({ length: PAGE }, (_, i) => ({
    id: `1f0a0000-0000-4000-8000-${String(i).padStart(12, '0')}`,
    source_id: `EONET_${i}`, source: 'eonet',
    title: `Flooding event near settlement ${i}`,
    category: i % 4 === 0 ? 'wildfires' : 'floods',
    status: 'open', geometry_type: 'Point',
    latitude: 6.5, longitude: 3.3,
    country_name: 'Nigeria', state_name: 'Lagos',
    event_date: '2026-08-01T12:00:00Z',
    source_url: 'https://eonet.gsfc.nasa.gov/x',
    ingested_at: '2026-08-01T12:05:00Z', enriched_at: null,
  })),
  meta: { total: TOTAL, limit: PAGE, offset: 0 },
}

async function measureOnce(browser, target, waitSelector) {
  const page = await browser.newPage({ viewport: VIEWPORT })

  await page.addInitScript(() => {
    window.__cls = 0
    window.__sources = []
    new PerformanceObserver(list => {
      for (const entry of list.getEntries()) {
        if (entry.hadRecentInput) continue
        window.__cls += entry.value
        for (const s of entry.sources ?? []) {
          const node = s.node
          const cls = typeof node?.className === 'string' ? node.className.split(' ')[0] : ''
          window.__sources.push(`${node?.tagName ?? '?'}${cls ? '.' + cls : ''}`)
        }
      }
    }).observe({ type: 'layout-shift', buffered: true })
  })

  await page.route('**/health', r => r.fulfill({ json: {
    status: 'ok', version: 'bench',
    last_ingestion: {
      status: 'success', started_at: '2026-08-07T10:00:00Z',
      completed_at: new Date().toISOString(),
      events_fetched: TOTAL, events_stored: TOTAL, error: null,
    },
  }}))
  await page.route('**/v1/context', r => r.fulfill({ json: { location: null, nearby_events: [] } }))
  await page.route('**/v1/states**', r => r.fulfill({ json: { states: ['Lagos', 'Kano', 'Rivers'] } }))
  await page.route('**/v1/events**', r => r.fulfill({ json: eventsPayload }))
  await page.route('**/assets/EventsDashboard-*.js', async r => {
    await new Promise(res => setTimeout(res, CHUNK_DELAY_MS))
    await r.continue()
  })

  await page.goto(target, { waitUntil: 'domcontentloaded' })
  await page.waitForSelector(waitSelector, { timeout: 25000 })
  await page.waitForTimeout(1500)

  const result = await page.evaluate(() => ({
    cls: window.__cls,
    sources: [...new Set(window.__sources)],
  }))
  await page.close()
  return result
}

const browser = await chromium.launch()
try {
  for (const arm of [
    // The baseline predates `.dashboard-results`, so it waits on a selector both
    // versions have.
    { label: 'BASELINE (control)', target: BASELINE_URL, selector: '.dashboard-layout' },
    { label: 'BRANCH', target: BRANCH_URL, selector: '.dashboard-layout' },
  ]) {
    const samples = []
    const sources = new Set()
    for (let i = 0; i < RUNS; i++) {
      const r = await measureOnce(browser, arm.target, arm.selector)
      samples.push(r.cls)
      r.sources.forEach(s => sources.add(s))
    }
    const shifting = samples.filter(v => v > 0.001).length
    console.log(
      `${arm.label.padEnd(20)} CLS [${samples.map(v => v.toFixed(4)).join(', ')}]  ` +
      `shifting ${shifting}/${RUNS}  sources: ${[...sources].join(', ') || '(none)'}`,
    )
  }
} finally {
  await browser.close()
}
