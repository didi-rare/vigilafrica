// chore-design-tokens — is the token migration a zero-visual-diff refactor?
//
// Screenshots the three page types (home with the dashboard mounted,
// /for-partners, /events/:id) at 375 / 768 / 1280 px against TWO served
// builds — the branch and a control built from the base branch — and counts
// differing pixels with pixelmatch. Every API response is route-mocked so the
// two arms render identical data, and every non-localhost request (map tiles,
// analytics) is aborted so neither arm depends on the network.
//
// Usage — see README.md. Requires both builds to be served:
//   BASELINE_URL (default http://localhost:4174/)  the control build
//   BRANCH_URL   (default http://localhost:4173/)  the branch build
//   OUT_DIR      (default scripts/bench-design-tokens/out) where PNGs + diffs go
//
// Exit code is 1 when any page/viewport differs at pixelmatch's default
// perceptual threshold (0.1), so this can gate a PR; the exact (threshold 0)
// count is printed as well because a pure-CSS refactor should hit 0 there too.
import { chromium } from 'playwright'
import { PNG } from 'pngjs'
import pixelmatch from 'pixelmatch'
import { mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'

const BASELINE = process.env.BASELINE_URL ?? 'http://localhost:4174/'
const BRANCH = process.env.BRANCH_URL ?? 'http://localhost:4173/'
const OUT = process.env.OUT_DIR ?? 'scripts/bench-design-tokens/out'

const EVENT_ID = '1f0a0000-0000-4000-8000-000000000007'
const PAGES = [
  { name: 'home', path: '/', ready: '.event-card' },
  { name: 'partners', path: '/for-partners', ready: '.partners-card' },
  { name: 'event', path: `/events/${EVENT_ID}`, ready: '.event-detail-header' },
]
const VIEWPORTS = [
  { name: '375', width: 375, height: 812 },
  { name: '768', width: 768, height: 1024 },
  { name: '1280', width: 1280, height: 900 },
]

// Fixed clock for the fixture so both arms compute the same relative dates
// and the same freshness band.
const NOW = '2026-08-07T12:00:00Z'

function mkEvent(i) {
  return {
    id: `1f0a0000-0000-4000-8000-${String(i).padStart(12, '0')}`,
    source_id: `EONET_${20000 + i}`, source: 'eonet',
    title: i % 4 === 0 ? `Wildfire near settlement ${i}` : `Flooding event near settlement ${i}`,
    category: i % 4 === 0 ? 'wildfires' : 'floods',
    status: i % 3 === 0 ? 'closed' : 'open', geometry_type: 'Point',
    latitude: 6.5 + (i % 20) * 0.1, longitude: 3.3 + (i % 20) * 0.1,
    country_name: 'Nigeria', state_name: i % 2 ? 'Lagos' : 'Kano',
    event_date: '2026-08-01T12:00:00Z',
    source_url: 'https://eonet.gsfc.nasa.gov/api/v3/events/EONET_1',
    ingested_at: '2026-08-01T12:05:00Z', enriched_at: '2026-08-01T12:06:00Z',
  }
}

async function mock(page) {
  // Registered first so it has the lowest priority: Playwright matches the
  // most recently registered route first.
  await page.route(url => !/^https?:\/\/localhost(:\d+)?\//.test(url.href), r => r.abort())
  await page.route('**/health', r => r.fulfill({ json: {
    status: 'ok', version: 'screenshot-diff',
    last_ingestion: { status: 'success', started_at: NOW, completed_at: NOW,
      events_fetched: 43, events_stored: 43, error: null },
  } }))
  await page.route('**/v1/context', r => r.fulfill({ json: { location: null, nearby_events: [] } }))
  await page.route('**/v1/states**', r => r.fulfill({ json: { states: ['Kano', 'Lagos', 'Rivers'] } }))
  await page.route('**/v1/events**', r => {
    const offset = Number(new URL(r.request().url()).searchParams.get('offset') ?? 0)
    const data = Array.from({ length: Math.max(0, Math.min(50, 43 - offset)) }, (_, i) => mkEvent(offset + i))
    r.fulfill({ json: { data, meta: { total: 43, limit: 50, offset } } })
  })
  // Registered after the list route so it takes priority for the detail URL.
  await page.route(`**/v1/events/${EVENT_ID}`, r => r.fulfill({ json: mkEvent(7) }))
}

async function shoot(browser, base, pg, vp) {
  const context = await browser.newContext({
    viewport: { width: vp.width, height: vp.height },
    deviceScaleFactor: 1,
    reducedMotion: 'reduce',
  })
  await context.addInitScript(`{
    const fixed = Date.parse(${JSON.stringify(NOW)});
    const RealDate = Date;
    class FixedDate extends RealDate {
      constructor(...a) { super(...(a.length ? a : [fixed])); }
      static now() { return fixed; }
    }
    window.Date = FixedDate;
  }`)
  const page = await context.newPage()
  await mock(page)
  await page.goto(new URL(pg.path, base).href, { waitUntil: 'networkidle' })
  // The dashboard chunk is loaded when it scrolls into view, so walk the whole
  // page once (identically in both arms), then return to the top.
  await page.evaluate(async () => {
    for (let y = 0; y < document.body.scrollHeight; y += 400) {
      window.scrollTo(0, y)
      await new Promise(r => setTimeout(r, 50))
    }
    window.scrollTo(0, document.body.scrollHeight)
  })
  await page.waitForSelector(pg.ready, { timeout: 30_000 })
  await page.evaluate(() => document.fonts.ready)
  // Let the lazy chunk, the map and any reveal transitions settle.
  await page.waitForTimeout(1_500)
  await page.evaluate(() => window.scrollTo(0, 0))
  await page.waitForTimeout(300)
  const png = await page.screenshot({ fullPage: true, animations: 'disabled', caret: 'hide' })
  await context.close()
  return PNG.sync.read(png)
}

function compare(a, b) {
  const width = Math.max(a.width, b.width)
  const height = Math.max(a.height, b.height)
  const pad = img => {
    if (img.width === width && img.height === height) return img
    const out = new PNG({ width, height })
    PNG.bitblt(img, out, 0, 0, img.width, img.height, 0, 0)
    return out
  }
  const A = pad(a), B = pad(b)
  const diff = new PNG({ width, height })
  const perceptual = pixelmatch(A.data, B.data, diff.data, width, height, { threshold: 0.1 })
  const exact = pixelmatch(A.data, B.data, null, width, height, { threshold: 0 })
  return { exact, perceptual, diff, sizeMatch: a.width === b.width && a.height === b.height, size: `${a.width}x${a.height} vs ${b.width}x${b.height}` }
}

// MapLibre needs WebGL2. Without it the map throws, the app's error boundary
// replaces the whole dashboard with an error state, and there are no event
// cards to compare. Playwright's default headless shell has no GPU path, so
// software GL is forced; set PW_CHANNEL=chrome (or chromium) to use a full
// browser build where the headless shell still cannot create a WebGL2 context.
const browser = await chromium.launch({
  channel: process.env.PW_CHANNEL || undefined,
  args: ['--use-gl=angle', '--use-angle=swiftshader', '--enable-unsafe-swiftshader', '--ignore-gpu-blocklist'],
})
mkdirSync(OUT, { recursive: true })
const rows = []
let failed = false
for (const pg of PAGES) {
  for (const vp of VIEWPORTS) {
    const [base, branch] = await Promise.all([shoot(browser, BASELINE, pg, vp), shoot(browser, BRANCH, pg, vp)])
    const r = compare(base, branch)
    const tag = `${pg.name}-${vp.name}`
    writeFileSync(join(OUT, `${tag}-baseline.png`), PNG.sync.write(base))
    writeFileSync(join(OUT, `${tag}-branch.png`), PNG.sync.write(branch))
    if (r.perceptual > 0 || !r.sizeMatch) {
      writeFileSync(join(OUT, `${tag}-diff.png`), PNG.sync.write(r.diff))
      failed = true
    }
    rows.push({ page: pg.name, viewport: vp.name, size: r.size, exact: r.exact, perceptual: r.perceptual })
  }
}
await browser.close()
console.table(rows)
console.log(failed ? `DIFFERENCES FOUND — see ${OUT}/*-diff.png` : 'ZERO perceptual difference on every page and viewport')
process.exit(failed ? 1 : 0)
