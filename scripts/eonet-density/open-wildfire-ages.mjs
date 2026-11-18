// Counts OPEN EONET wildfires in the production Nigeria and Ghana boxes by age,
// to set the length of the D6 display window (feature-place-first-direction).
//
// Why: reads are unwindowed, and the ingestor's open query has no days window
// on purpose (api/internal/ingestor/eonet.go, runIngest), so every wildfire the
// feed still marks open is shown, however old. This says how old they are, and
// how many a window of N days would hide.
//
// How it mirrors production:
//   - The request is the ingestor's open query, byte for byte apart from the
//     category: bbox=<min_lon,min_lat,max_lon,max_lat> formatted %.4f,
//     category=wildfires, status=open, no days parameter.
//   - Each event is placed and dated by its MOST RECENT geometry, as
//     normalizer.selectMostRecentGeometry does (ported line for line: the last
//     geometry unless an earlier one has a strictly later RFC 3339 date), and
//     must pass the same inclusive withinBBox guard.
//   - An event in both boxes counts once, as the ingestor's source_id upsert
//     would store it once.
//   - Columns are by the country named in the title ("Wildfire in <country>
//     <id>"), not by box: the Nigeria box also covers parts of Benin, Niger and
//     Cameroon, and production stores and shows those fires under their own
//     country_name.
//
// What it does NOT measure:
//   - Whether an old open fire is still burning or was simply never closed
//     upstream. The feed cannot say; D6's assumption is "mostly never closed".
//   - Stored rows. With --api it also reads the production API's open
//     wildfires for comparison; without it, it reports the feed only.
//
// Ages are whole days from the event date to the run time (UTC), so every
// figure is a dated snapshot.
//
// Run:  node scripts/eonet-density/open-wildfire-ages.mjs [--api https://api.vigilafrica.org]

const usage = 'usage: node open-wildfire-ages.mjs [--api <base url>]'
const fail = (msg) => {
  console.error(`error: ${msg}\n${usage}`)
  process.exit(1)
}
let apiBase = null
{
  const args = process.argv.slice(2)
  for (let i = 0; i < args.length; i++) {
    if (args[i] !== '--api') fail(`unknown argument "${args[i]}"`)
    const v = args[++i]
    if (v === undefined || !/^https?:\/\/[^/]+$/.test(v)) fail('--api needs a base URL such as https://api.vigilafrica.org (no path)')
    apiBase = v
  }
}

// Mirrors DefaultCountries in api/internal/ingestor/eonet.go -- keep in sync by hand.
// BBox is [min_lon, min_lat, max_lon, max_lat].
const COUNTRIES = [
  { code: 'NG', bbox: [2.0, 4.0, 15.0, 14.0] },
  { code: 'GH', bbox: [-3.5, 4.5, 1.2, 11.2] },
]
const EONET = 'https://eonet.gsfc.nasa.gov/api/v3/events'
const UA = { 'User-Agent': 'VigilAfrica-density/1.0' }

// Age bands in days, upper bound inclusive.
const BANDS = [
  ['0-7 d', 7],
  ['8-30 d', 30],
  ['31-90 d', 90],
  ['91-180 d', 180],
  ['181-365 d', 365],
  ['366-540 d', 540],
  ['541-730 d', 730],
  ['over 730 d', Infinity],
]
// Candidate D6 window lengths.
const WINDOWS = [7, 14, 30, 60, 90, 180, 365]

const titleCountry = (t) => String(t ?? '').match(/^Wildfire in (.+?) \d+$/)?.[1] ?? 'unknown'
const withinBBox = (b, lon, lat) => lon >= b[0] && lon <= b[2] && lat >= b[1] && lat <= b[3]

// Go's parseGeometryDate accepts RFC 3339 only; Date.parse alone accepts more.
const RFC3339 = /^d{4}-d{2}-d{2}Td{2}:d{2}:d{2}(.d+)?(Z|[+-]d{2}:d{2})$/
const geomTime = (d) => (typeof d === 'string' && RFC3339.test(d) ? Date.parse(d) : null)

// A line-for-line port of normalizer.selectMostRecentGeometry: start from the
// last geometry; an earlier one replaces it only if its date parses and is
// strictly later (or the current pick has no parseable date).
function mostRecentGeometry(geoms) {
  if (!geoms.length) return null
  let selected = geoms[geoms.length - 1]
  let selectedT = geomTime(selected?.date)
  for (const g of geoms.slice(0, -1)) {
    const t = geomTime(g?.date)
    if (t === null) continue
    if (selectedT === null || t > selectedT) {
      selected = g
      selectedT = t
    }
  }
  return selected
}

async function getJson(url) {
  const res = await fetch(url, { headers: UA })
  if (!res.ok) throw new Error(`HTTP ${res.status} for ${url}`)
  return res.json()
}

const now = Date.now()
const ageDays = (iso) => Math.floor((now - Date.parse(iso)) / 86_400_000)

function report(label, rows) {
  // rows: [{ date: ISO string, country: string }]
  const dated = rows.filter((r) => !Number.isNaN(Date.parse(r.date)))
  const ages = dated.map((r) => ageDays(r.date)).sort((a, b) => a - b)
  const sorted = dated.map((r) => r.date.slice(0, 10)).sort()
  console.log(`\n--- ${label} ---`)
  const byCountry = rows.reduce((m, r) => ((m[r.country] = (m[r.country] || 0) + 1), m), {})
  console.log(`  by country                   : ${Object.entries(byCountry).sort((a, b) => b[1] - a[1]).map(([k, v]) => `${k} ${v}`).join(' | ')}`)
  console.log(`  open wildfires               : ${rows.length}${rows.length - dated.length ? `   (${rows.length - dated.length} with no usable date)` : ''}`)
  if (!dated.length) return
  console.log(`  oldest / newest event date   : ${sorted[0]} / ${sorted[sorted.length - 1]}`)
  const mid = ages.length / 2
  const median = ages.length % 2 ? ages[Math.floor(mid)] : (ages[mid - 1] + ages[mid]) / 2
  console.log(`  youngest / median / oldest age: ${ages[0]} / ${median} / ${ages[ages.length - 1]} days`)
  console.log('\n  age band     Nigeria  Ghana  other    all')
  let lo = -1
  for (const [name, hi] of BANDS) {
    const inBand = dated.filter((r) => {
      const a = ageDays(r.date)
      return a > lo && a <= hi
    })
    const n = (pred, w) => String(inBand.filter(pred).length).padStart(w)
    const other = (r) => r.country !== 'Nigeria' && r.country !== 'Ghana'
    console.log(
      `  ${name.padEnd(12)} ${n((r) => r.country === 'Nigeria', 7)}  ${n((r) => r.country === 'Ghana', 5)}  ${n(other, 5)}  ${String(inBand.length).padStart(5)}`,
    )
    lo = hi
  }
  console.log('\n  a window of N days would hide:')
  for (const w of WINDOWS) {
    const hidden = ages.filter((a) => a > w).length
    console.log(`    N = ${String(w).padStart(3)}   ${String(hidden).padStart(4)} of ${dated.length}`)
  }
}

console.log(`=== Open wildfires by age, production NG + GH boxes, run ${new Date(now).toISOString()} ===`)

const seen = new Set()
const feedRows = []
let excluded = 0
let undated = 0
for (const c of COUNTRIES) {
  const bbox = c.bbox.map((v) => v.toFixed(4)).join(',')
  const url = `${EONET}?bbox=${bbox}&category=wildfires&status=open`
  const body = await getJson(url)
  if (!Array.isArray(body?.events)) throw new Error(`response has no "events" array for ${url}`)
  console.log(`  ${c.code} request: ${url}  -> ${body.events.length} events`)
  for (const ev of body.events) {
    const g = mostRecentGeometry(ev.geometry ?? [])
    if (!g || g.type !== 'Point' || !Array.isArray(g.coordinates)) { excluded++; continue }
    const [lon, lat] = g.coordinates
    if (!withinBBox(c.bbox, lon, lat)) { excluded++; continue }
    if (seen.has(ev.id)) continue
    seen.add(ev.id)
    if (Number.isNaN(Date.parse(g.date ?? ''))) undated++
    feedRows.push({ id: String(ev.id), date: String(g.date ?? ''), country: titleCountry(ev.title) })
  }
  await new Promise((r) => setTimeout(r, 250))
}
console.log(`  dropped by withinBBox or non-point geometry: ${excluded}`)
report('EONET feed (what the ingestor would store)', feedRows)

if (apiBase) {
  // The production API, paged. Each stored row carries country_name and event_date.
  const apiRows = []
  for (let offset = 0; ; offset += 200) {
    const body = await getJson(`${apiBase}/v1/events?category=wildfires&status=open&limit=200&offset=${offset}`)
    const page = Array.isArray(body?.data) ? body.data : null
    if (!page) throw new Error('API response has no "data" array')
    // source_id is the feed's own event id (e.g. "EONET_13123").
    for (const e of page) apiRows.push({ id: String(e.source_id ?? ''), date: String(e.event_date ?? ''), country: String(e.country_name ?? 'unknown') })
    if (page.length < 200) break
  }
  report(`production API ${apiBase} (what visitors see)`, apiRows)

  // Same events, not just the same counts: compare the id sets.
  const feedIds = new Set(feedRows.map((r) => r.id))
  const apiIds = new Set(apiRows.map((r) => r.id))
  const onlyFeed = [...feedIds].filter((id) => !apiIds.has(id))
  const onlyApi = [...apiIds].filter((id) => !feedIds.has(id))
  console.log(`\n--- feed vs API, by event id ---`)
  console.log(`  in both: ${[...feedIds].filter((id) => apiIds.has(id)).length}   feed only: ${onlyFeed.length}${onlyFeed.length ? ` (${onlyFeed.join(', ')})` : ''}   API only: ${onlyApi.length}${onlyApi.length ? ` (${onlyApi.join(', ')})` : ''}`)
}
