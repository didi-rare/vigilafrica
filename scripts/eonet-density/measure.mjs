// Measures EONET event density for the CURRENTLY INGESTED geography versus
// Africa-wide, to size the case for feature-continental-coverage.
//
// Committed because this number has been wrong twice, in opposite directions,
// and it drives an investment decision:
//   - the proposal originally claimed 43 events for NG+GH  (far too low)
//   - a later re-measurement claimed 159                   (Ghana was outside
//     the bbox entirely -- Ghana spans -3.5..1.2 degE, and the box started at 2.6)
//
// The fix is to stop hand-rolling bounding boxes and use the PRODUCTION values
// from api/internal/ingestor/eonet.go, applying the same withinBBox guard the
// ingestor applies, so this measures what the pipeline would actually store.
//
// Run:  node scripts/eonet-density/measure.mjs

const START = '2024-10-01'
const END = '2026-08-04'

// Mirrors DefaultCountries in api/internal/ingestor/eonet.go.
// BBox is [min_lon, min_lat, max_lon, max_lat] -- keep in sync with that file.
const COUNTRIES = [
  { code: 'NG', bbox: [2.0, 4.0, 15.0, 14.0] },
  { code: 'GH', bbox: [-3.5, 4.5, 1.2, 11.2] },
]

// Africa-wide, generous. Deliberately wider than the continent: an over-wide box
// slightly OVERSTATES the Africa side, so the resulting ratio is an upper bound
// on the case for widening -- the conservative direction for a build decision.
const AFRICA = [-26.0, -35.0, 64.0, 38.0]

// The two categories the ingestor actually requests today.
const CATEGORIES = ['floods', 'wildfires']

const withinBBox = (b, lon, lat) => lon >= b[0] && lon <= b[2] && lat >= b[1] && lat <= b[3]

async function getJson(url) {
  const res = await fetch(url, { headers: { 'User-Agent': 'VigilAfrica-density/1.0' } })
  if (!res.ok) throw new Error(`HTTP ${res.status} for ${url}`)
  return res.json()
}

function firstCoord(ev) {
  const g = ev.geometry?.[0]
  if (!g) return null
  if (g.type === 'Point') return g.coordinates
  return g.coordinates?.[0]?.[0] ?? null
}

// EONET's bbox query parameter order is minLon,maxLat,maxLon,minLat.
async function countBox(bbox, seen) {
  const [minLon, minLat, maxLon, maxLat] = bbox
  const byCat = Object.fromEntries(CATEGORIES.map((c) => [c, 0]))
  let excluded = 0
  let total = 0

  for (const cat of CATEGORIES) {
    const url =
      `https://eonet.gsfc.nasa.gov/api/v3/events?status=all&start=${START}&end=${END}` +
      `&category=${cat}&bbox=${minLon},${maxLat},${maxLon},${minLat}&limit=5000`
    const json = await getJson(url)
    for (const ev of json.events ?? []) {
      const c = firstCoord(ev)
      if (!c) continue
      // The ingestor re-validates every event against the requested box; a real
      // leak (a Florida wildfire) is on record, so the guard is not theoretical.
      if (!withinBBox(bbox, c[0], c[1])) { excluded++; continue }
      if (seen.has(ev.id)) continue
      seen.add(ev.id)
      byCat[cat]++
      total++
    }
    await new Promise((r) => setTimeout(r, 250))
  }
  return { total, byCat, excluded }
}

const ingestedSeen = new Set()
const ingested = { total: 0, byCat: Object.fromEntries(CATEGORIES.map((c) => [c, 0])), excluded: 0 }
for (const c of COUNTRIES) {
  const r = await countBox(c.bbox, ingestedSeen)
  console.log(`  ${c.code}  ${String(r.total).padStart(4)} events  (${CATEGORIES.map((k) => `${k}:${r.byCat[k]}`).join(' ')})`)
  ingested.total += r.total
  ingested.excluded += r.excluded
  for (const k of CATEGORIES) ingested.byCat[k] += r.byCat[k]
}

const africa = await countBox(AFRICA, new Set())

const line = (label, o) =>
  console.log(
    `  ${label.padEnd(22)} ${String(o.total).padStart(5)} events   ` +
      CATEGORIES.map((k) => `${k}: ${String(o.byCat[k]).padStart(5)}`).join('   '),
  )

console.log(`\n=== EONET density, ${START} -> ${END} ===`)
line('ingested today (NG+GH)', ingested)
line('Africa-wide', africa)
console.log(`\n  events multiplier : ${(africa.total / ingested.total).toFixed(1)}x`)
console.log(`  floods multiplier : ${(africa.byCat.floods / ingested.byCat.floods).toFixed(1)}x`)
console.log(`  wildfire share of Africa-wide : ${((100 * africa.byCat.wildfires) / africa.total).toFixed(1)}%`)
console.log(`  excluded by withinBBox guard  : ${ingested.excluded} (NG+GH), ${africa.excluded} (Africa)`)
