// Is a loading affordance visible ABOVE THE FOLD while the dashboard chunk loads?
//
// chore-web-audit-leftovers item 4: at 375x812 the `.dashboard-fallback` starts
// below the fold (y≈1098), so a phone user on a slow connection saw the hero and
// nothing that said anything was loading. This script holds the dashboard chunk
// back indefinitely, screenshots the viewport, and reports where the fallback's
// affordances sit relative to the fold — so "visible above the fold" is a
// measured rectangle, not a claim.
//
// Usage (the target must already be built and served, see README.md):
//   TARGET_URL=http://localhost:4173/ VIEWPORT=375x812 \
//     OUT=/tmp/fallback-375.png node scripts/bench-dashboard-cls/capture-fallback.mjs
import { chromium } from 'playwright'

const TARGET_URL = process.env.TARGET_URL ?? 'http://localhost:4173/'
const [VW, VH]   = (process.env.VIEWPORT ?? '375x812').split('x').map(Number)
const OUT        = process.env.OUT ?? `fallback-${VW}x${VH}.png`

const browser = await chromium.launch()
try {
  const page = await browser.newPage({ viewport: { width: VW, height: VH } })

  await page.route('**/health', r => r.fulfill({ json: { status: 'ok', version: 'bench', last_ingestion: null } }))
  await page.route('**/v1/context', r => r.fulfill({ json: { location: null, nearby_events: [] } }))
  await page.route('**/v1/states**', r => r.fulfill({ json: { states: [] } }))
  await page.route('**/v1/events**', r => r.fulfill({ json: { data: [], meta: { total: 0, limit: 50, offset: 0 } } }))
  // Never let the chunk arrive: the fallback is the steady state under test.
  await page.route('**/assets/EventsDashboard-*.js', () => new Promise(() => {}))

  await page.goto(TARGET_URL, { waitUntil: 'domcontentloaded' })
  await page.waitForSelector('.dashboard-fallback', { timeout: 15000 })
  await page.waitForTimeout(500)

  const report = await page.evaluate(() => {
    const rect = (sel) => {
      const el = document.querySelector(sel)
      if (!el) return null
      const r = el.getBoundingClientRect()
      return { top: Math.round(r.top), bottom: Math.round(r.bottom), height: Math.round(r.height) }
    }
    const status = document.querySelector('.dashboard-fallback [role="status"]')
    return {
      viewport: { width: window.innerWidth, height: window.innerHeight },
      bar: rect('.load-progress'),
      fallback: rect('.dashboard-fallback'),
      card: rect('.dashboard-fallback .loading-state'),
      spinner: rect('.dashboard-fallback .loading-state__spinner'),
      statusText: status?.textContent?.trim() ?? null,
      ariaLive: status?.getAttribute('aria-live') ?? null,
    }
  })

  await page.screenshot({ path: OUT, fullPage: false })

  const inView = (r) => r && r.top < report.viewport.height && r.bottom > 0
  console.log(JSON.stringify({
    ...report,
    barAboveFold: inView(report.bar),
    cardAboveFold: inView(report.card),
    screenshot: OUT,
  }, null, 2))
} finally {
  await browser.close()
}
