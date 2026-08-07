// Task 4.6: does the pagination bar change the dashboard's mounted height enough
// to invalidate the `.dashboard-fallback` reservation (min(100vh, 1530px))?
//
// The prod API blocks cross-origin fetches from localhost, so responses are
// route-mocked. That is also more useful: it lets us measure BOTH today's
// 43-event reality and the 50-event continental page, and A/B the bar in the
// same page by hiding it and re-measuring.
//
// Usage -- see README.md. Requires the branch build to be served at TARGET_URL.
import { chromium } from 'playwright'

const TARGET = process.env.TARGET_URL ?? 'http://localhost:4173/'
const VIEWPORTS = [
  { name: '1350x940  (the CLS trial viewport)', width: 1350, height: 940 },
  { name: '1920x1600 (taller than the 1530px cap)', width: 1920, height: 1600 },
  { name: '768x1024  (reservation is load-bearing here)', width: 768, height: 1024 },
  { name: '375x812   (no reservation below 480px)', width: 375, height: 812 },
]

function mkEvents(offset, total, pageSize = 50) {
  const count = Math.max(0, Math.min(pageSize, total - offset))
  return {
    data: Array.from({ length: count }, (_, i) => ({
      id: `1f0a0000-0000-4000-8000-${String(offset + i).padStart(12, '0')}`,
      source_id: `EONET_${offset + i}`, source: 'eonet',
      title: `Flooding event near settlement ${offset + i}`,
      category: i % 4 === 0 ? 'wildfires' : 'floods',
      status: 'open', geometry_type: 'Point',
      latitude: 6.5 + (i % 20) * 0.1, longitude: 3.3 + (i % 20) * 0.1,
      country_name: 'Nigeria', state_name: 'Lagos',
      event_date: '2026-08-01T12:00:00Z',
      source_url: 'https://eonet.gsfc.nasa.gov/x',
      ingested_at: '2026-08-01T12:05:00Z', enriched_at: '2026-08-01T12:06:00Z',
    })),
    meta: { total, limit: pageSize, offset },
  }
}

async function measure(browser, vp, total) {
  const page = await browser.newPage({ viewport: { width: vp.width, height: vp.height } })
  await page.route('**/health', r => r.fulfill({ json: {
    status: 'ok', version: 'measure',
    last_ingestion: { status: 'success', started_at: '2026-08-07T10:00:00Z',
      completed_at: new Date().toISOString(), events_fetched: total, events_stored: total, error: null },
  }}))
  await page.route('**/v1/context', r => r.fulfill({ json: { location: null, nearby_events: [] } }))
  await page.route('**/v1/states**', r => r.fulfill({ json: { states: ['Lagos', 'Kano', 'Rivers'] } }))
  await page.route('**/v1/events**', r => {
    const offset = Number(new URL(r.request().url()).searchParams.get('offset') ?? 0)
    r.fulfill({ json: mkEvents(offset, total) })
  })

  await page.goto(TARGET, { waitUntil: 'networkidle' })
  await page.evaluate(() => window.scrollTo(0, document.body.scrollHeight))
  await page.waitForSelector('.dashboard-results', { timeout: 20000 })
  await page.waitForTimeout(1200)

  const withBar = await page.evaluate(() => {
    const dash = document.querySelector('#dashboard')
    const bar = document.querySelector('.dashboard-results')
    return {
      dashboardHeight: Math.round(dash.getBoundingClientRect().height),
      barHeight: Math.round(bar.getBoundingClientRect().height),
      barText: bar.textContent.trim().replace(/\s+/g, ' '),
      cards: document.querySelectorAll('.event-card').length,
    }
  })

  // A/B in the same page: remove the bar entirely and re-measure. `display:none`
  // also collapses its margin, so the delta is the bar's full contribution.
  const withoutBarHeight = await page.evaluate(() => {
    const bar = document.querySelector('.dashboard-results')
    bar.style.display = 'none'
    const h = Math.round(document.querySelector('#dashboard').getBoundingClientRect().height)
    bar.style.display = ''
    return h
  })

  await page.close()
  return { ...withBar, withoutBarHeight, delta: withBar.dashboardHeight - withoutBarHeight }
}

const browser = await chromium.launch()
for (const total of [43, 3268]) {
  console.log(`\n\n########  total = ${total} events  ########`)
  for (const vp of VIEWPORTS) {
    const m = await measure(browser, vp, total)
    // Mirrors `.dashboard-fallback` in web/src/App.css: min(100vh, 1530px), and
    // 0 below 480px. Keep in step if that rule changes.
    const reserved = vp.width <= 480 ? 0 : Math.min(vp.height, 1530)
    console.log(`\n${vp.name}`)
    console.log(`  cards rendered      : ${m.cards}`)
    console.log(`  bar text            : ${m.barText}`)
    console.log(`  dashboard WITH bar  : ${m.dashboardHeight}px`)
    console.log(`  dashboard WITHOUT   : ${m.withoutBarHeight}px   (delta +${m.delta}px)`)
    console.log(`  fallback reserves   : ${reserved}px  -> ${reserved >= m.dashboardHeight ? 'covers it' : `under-reserves by ${m.dashboardHeight - reserved}px`}`)
  }
}
await browser.close()
