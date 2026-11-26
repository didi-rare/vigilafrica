// Automated keyboard walkthrough of the home page (feature-current-location-marker,
// §9.12): press Tab from the top of the page until focus leaves the document or
// wraps to the first element, recording every element that receives focus.
//
// Checks, with /v1/context mocked to Lagos so the "you are here" marker and its
// status line are on the page:
//   - nothing inside the user marker, and not the status line, ever takes focus;
//   - the walk reaches the end of the page (no focus trap) within MAX_TABS;
//   - no element takes focus twice in one pass.
// Run it against a control build too and diff the two orders: the change should
// add no tab stop.
//
// This is not a substitute for a person with a keyboard and a screen reader; it
// checks focus order and reachability, not what is announced or whether focus
// is visible.
//
// Usage:
//   TARGET_URL=http://localhost:4273/ PW_CHANNEL=chrome OUT=/tmp/order-branch.txt \
//     node scripts/bench-dashboard-cls/keyboard-walkthrough.mjs
import { writeFileSync } from 'node:fs'
import { chromium } from 'playwright'

const TARGET_URL = process.env.TARGET_URL ?? 'http://localhost:4273/'
const CHANNEL = process.env.PW_CHANNEL || undefined
const OUT = process.env.OUT ?? null
const MAX_TABS = Number(process.env.MAX_TABS ?? 300)
const LAGOS = { country: 'Nigeria', state: 'Lagos', lat: 6.5244, lng: 3.3792 }

const event = (id, lat, lng, title, category) => ({
  id, source_id: id, source: 'eonet', title, category, status: 'open', geometry_type: 'Point',
  latitude: lat, longitude: lng, country_name: 'Nigeria', state_name: 'Lagos',
  event_date: '2026-11-20T12:00:00Z', source_url: 'https://eonet.gsfc.nasa.gov/x',
  ingested_at: '2026-11-20T12:05:00Z', enriched_at: null,
})
const events = [
  event('1f0a0000-0000-4000-8000-000000000001', 6.28, 3.38, 'Flooding south of Lagos', 'floods'),
  event('1f0a0000-0000-4000-8000-000000000002', 6.9, 3.9, 'Wildfire near Ijebu', 'wildfires'),
]

const browser = await chromium.launch({ channel: CHANNEL })
const problems = []
try {
  const page = await browser.newPage({ viewport: { width: 1350, height: 940 } })
  await page.route('**/health', (r) => r.fulfill({ json: {
    status: 'ok', version: 'bench',
    last_ingestion: { status: 'success', started_at: null, completed_at: new Date().toISOString(), events_fetched: 2, events_stored: 2, error: null },
  } }))
  await page.route('**/v1/context', (r) => r.fulfill({ json: { location: LAGOS, location_source: 'ip', nearby_events: [] } }))
  await page.route('**/v1/states**', (r) => r.fulfill({ json: { states: ['Lagos'] } }))
  await page.route('**/v1/events**', (r) => r.fulfill({ json: { data: events, meta: { total: 2, limit: 50, offset: 0 } } }))
  await page.goto(TARGET_URL, { waitUntil: 'domcontentloaded' })
  await page.waitForSelector('.map-marker', { timeout: 30000 })
  await page.waitForTimeout(4000) // camera fly-to, pins re-synced
  const hasUserMarker = (await page.locator('.map-user-marker').count()) > 0
  console.log(`target ${TARGET_URL}; user marker on page: ${hasUserMarker}`)

  const describe = () => page.evaluate(() => {
    const el = document.activeElement
    if (!el || el === document.body || el === document.documentElement) return null
    const name = el.getAttribute('aria-label') || el.textContent?.replace(/\s+/g, ' ').trim().slice(0, 50) || ''
    const role = el.getAttribute('role') || el.tagName.toLowerCase()
    return {
      key: `${role} "${name}"`,
      inUserMarker: !!el.closest('.map-user-marker'),
      inStatusLine: !!el.closest('.dashboard-location-status'),
      inMap: !!el.closest('.map-wrapper'),
      path: `${el.tagName.toLowerCase()}${el.id ? '#' + el.id : ''}${typeof el.className === 'string' && el.className ? '.' + el.className.split(' ')[0] : ''}`,
    }
  })

  const order = []
  const seen = new Set()
  let ended = false
  for (let i = 0; i < MAX_TABS; i++) {
    await page.keyboard.press('Tab')
    const d = await describe()
    if (d === null) { if (order.length > 0) { ended = true; break } continue }
    if (order.length > 0 && d.key === order[0].key && d.path === order[0].path) { ended = true; break }
    const id = `${d.key} ${d.path}`
    if (seen.has(id)) problems.push(`focused twice in one pass: ${id}`)
    seen.add(id)
    order.push(d)
    if (d.inUserMarker) problems.push(`user marker took focus: ${id}`)
    if (d.inStatusLine) problems.push(`status line took focus: ${id}`)
  }
  if (!ended) problems.push(`focus did not leave or wrap the page within ${MAX_TABS} Tab presses (trap?)`)

  const lines = order.map((d, i) => `${String(i + 1).padStart(3)}  ${d.inMap ? '[map] ' : ''}${d.key}  <${d.path}>`)
  console.log(lines.join('\n'))
  console.log(`\n${order.length} tab stops; walk ${ended ? 'reached the end of the page' : 'DID NOT END'}`)
  if (OUT) writeFileSync(OUT, lines.join('\n') + '\n')
} finally {
  await browser.close()
}

if (problems.length) {
  console.log('\nFAIL')
  for (const p of problems) console.log(`  - ${p}`)
  process.exit(1)
}
console.log('\nPASS')
