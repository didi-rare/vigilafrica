// Counts EONET wildfires per month for the production Nigeria and Ghana boxes
// and an Africa-wide rectangle, and lists the categories the product does not
// ingest (severe storms, volcanoes) for the same rectangle. Backs the seasonal
// claim and the D3 storm/volcano counts in feature-place-first-direction.
//
// How it mirrors production:
//   - Boxes are DefaultCountries in api/internal/ingestor/eonet.go, sent in the
//     ingestor's own order (min_lon,min_lat,max_lon,max_lat). EONET returned the
//     same events for that order and its documented minLon,maxLat,maxLon,minLat
//     when checked on 2026-11-18.
//   - Each event is placed and dated by its MOST RECENT geometry, as
//     normalizer.selectMostRecentGeometry does, and must pass the inclusive
//     withinBBox guard. Wildfires are Point geometry, so coordinates are sound
//     (unlike floods; see floods-by-title.mjs).
//   - Months are by that event date (UTC). An event in both country boxes counts
//     in each column; the NG+GH column counts it once.
//
// Known limits:
//   - The Africa rectangle (same as measure.mjs) also covers southern Europe and
//     the Middle East, so the Africa column OVER-COUNTS. The storm and volcano
//     titles are printed in full so the reader can see which are African.
//   - The country boxes are rectangles too: the Nigeria box covers parts of
//     Benin, Niger and Cameroon. That is what production ingests.
//   - This counts feed availability with status=all, not stored rows: production
//     keeps open events plus events closed in the last 30 days.
//   - Counts for a closed window drift upward as the feed back-fills, so every
//     figure is a dated snapshot.
//
// Run:  node scripts/eonet-density/wildfires-by-month.mjs [--start YYYY-MM-DD] [--end YYYY-MM-DD]

const usage = 'usage: node wildfires-by-month.mjs [--start YYYY-MM-DD] [--end YYYY-MM-DD]'
const fail = (msg) => {
  console.error(`error: ${msg}\n${usage}`)
  process.exit(1)
}
const DATE = /^\d{4}-\d{2}-\d{2}$/
const opts = { start: '2024-10-01', end: new Date().toISOString().slice(0, 10) }
{
  const args = process.argv.slice(2)
  for (let i = 0; i < args.length; i++) {
    const a = args[i]
    if (a !== '--start' && a !== '--end') fail(`unknown argument "${a}"`)
    const v = args[++i]
    if (v === undefined || v.startsWith('--')) fail(`${a} needs a value`)
    const parsed = DATE.test(v) ? new Date(`${v}T00:00:00Z`) : null
    if (!parsed || Number.isNaN(parsed.getTime()) || parsed.toISOString().slice(0, 10) !== v) fail(`${a} must be YYYY-MM-DD, got "${v}"`)
    opts[a.slice(2)] = v
  }
  if (opts.end < opts.start) fail(`--end ${opts.end} is earlier than --start ${opts.start}`)
}
const START = opts.start
const END = opts.end
const LIMIT = 20000

// Mirrors DefaultCountries in api/internal/ingestor/eonet.go -- keep in sync by hand.
// BBox is [min_lon, min_lat, max_lon, max_lat].
const COUNTRIES = [
  { code: 'NG', bbox: [2.0, 4.0, 15.0, 14.0] },
  { code: 'GH', bbox: [-3.5, 4.5, 1.2, 11.2] },
]
// Same rectangle as measure.mjs: wider than the continent on purpose.
const AFRICA = [-26.0, -35.0, 64.0, 38.0]
const EONET = 'https://eonet.gsfc.nasa.gov/api/v3/events'

const withinBBox = (b, lon, lat) => lon >= b[0] && lon <= b[2] && lat >= b[1] && lat <= b[3]

function mostRecentGeometry(geoms) {
  let best = null
  let bestT = -Infinity
  for (const g of geoms) {
    const t = Date.parse(g?.date ?? '')
    if (!Number.isNaN(t) && t >= bestT) {
      best = g
      bestT = t
    }
  }
  return best ?? geoms[geoms.length - 1] ?? null
}

// A point for the box test: a Point's coordinates, or a Polygon's first vertex.
function pointOf(g) {
  if (!g) return null
  if (g.type === 'Point') return g.coordinates
  if (g.type === 'Polygon') return g.coordinates?.[0]?.[0] ?? null
  return null
}

async function fetchBox(category, bbox) {
  const url = `${EONET}?status=all&start=${START}&end=${END}&category=${category}&bbox=${bbox.map((v) => v.toFixed(4)).join(',')}&limit=${LIMIT}`
  const res = await fetch(url, { headers: { 'User-Agent': 'VigilAfrica-density/1.0' } })
  if (!res.ok) throw new Error(`HTTP ${res.status} for ${url}`)
  const body = await res.json()
  if (!Array.isArray(body?.events)) throw new Error(`response has no "events" array for ${url}`)
  if (body.events.length >= LIMIT) console.warn(`WARNING: ${body.events.length} events for ${url}; may be truncated`)
  await new Promise((r) => setTimeout(r, 250))
  const kept = []
  let excluded = 0
  for (const ev of body.events) {
    const g = mostRecentGeometry(ev.geometry ?? [])
    const p = pointOf(g)
    if (!p || !withinBBox(bbox, p[0], p[1])) { excluded++; continue }
    kept.push({ id: ev.id, title: String(ev.title).replaceAll('&#039;', "'"), date: String(g.date ?? '').slice(0, 10), closed: ev.closed ? String(ev.closed).slice(0, 10) : null })
  }
  return { kept, excluded }
}

console.log(`=== EONET wildfires by month, ${START} -> ${END} (run ${new Date().toISOString()}) ===`)

const ng = await fetchBox('wildfires', COUNTRIES[0].bbox)
const gh = await fetchBox('wildfires', COUNTRIES[1].bbox)
const af = await fetchBox('wildfires', AFRICA)
const both = new Map([...ng.kept, ...gh.kept].map((e) => [e.id, e]))
console.log(`  excluded by withinBBox: NG ${ng.excluded}, GH ${gh.excluded}, Africa ${af.excluded}`)

const months = []
for (let [y, m] = START.slice(0, 7).split('-').map(Number); `${y}-${String(m).padStart(2, '0')}` <= END.slice(0, 7); m === 12 ? (y++, (m = 1)) : m++) {
  months.push(`${y}-${String(m).padStart(2, '0')}`)
}
const inMonth = (list, m) => list.filter((e) => e.date.startsWith(m)).length
const col = (n, w) => String(n).padStart(w)

console.log('\n  month     NG box  GH box  NG+GH   Africa rect')
for (const m of months) {
  console.log(`  ${m}  ${col(inMonth(ng.kept, m), 6)}  ${col(inMonth(gh.kept, m), 6)}  ${col(inMonth([...both.values()], m), 5)}   ${col(inMonth(af.kept, m), 7)}`)
}
console.log(`  total    ${col(ng.kept.length, 6)}  ${col(gh.kept.length, 6)}  ${col(both.size, 5)}   ${col(af.kept.length, 7)}`)

// Season split for the two boxes: November-February (West African dry season
// peak) versus June-September (the rains).
const season = (months) => [...both.values()].filter((e) => months.includes(e.date.slice(5, 7))).length
console.log(`\n  NG+GH dated Nov-Feb : ${season(['11', '12', '01', '02'])} of ${both.size}`)
console.log(`  NG+GH dated Jun-Sep : ${season(['06', '07', '08', '09'])} of ${both.size}`)

// Categories the product does not ingest, for D3.
for (const [category, label] of [['severeStorms', 'severe storms'], ['volcanoes', 'volcanoes']]) {
  const r = await fetchBox(category, AFRICA)
  console.log(`\n  ${label} in the Africa rectangle: ${r.kept.length}  (rectangle over-counts; titles below)`)
  for (const e of r.kept.sort((a, b) => a.date.localeCompare(b.date))) console.log(`    ${e.date}  ${e.title}`)
}
