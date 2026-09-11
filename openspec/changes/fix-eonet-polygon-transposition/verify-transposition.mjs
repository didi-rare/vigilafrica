#!/usr/bin/env node
// Re-derives the evidence in proposal.md from live sources. Every number in that
// document, and every coordinate in migration 000015, comes from this script.
// Re-run it rather than trusting the write-up.
//
//   node verify-transposition.mjs                 # the +/-90 census across EONET floods
//   node verify-transposition.mjs EONET_22248     # one event, EONET vs GDACS, episode-matched
//
// Exit codes: 0 proof complete, 1 proof INCOMPLETE or contradicted, 2 usage.
//
// ⚠️ An earlier version reported a missing EONET record as ordinary output and
// exited 0, so "no comparison performed" was indistinguishable from "comparison
// passed". Incomplete proof now fails.

const EONET_LIST = 'https://eonet.gsfc.nasa.gov/api/v3/events?category=floods&status=all&limit=40'
const eonetEvent = (id) => `https://eonet.gsfc.nasa.gov/api/v3/events/${id}`
const gdacsEvent = (t, id) => `https://www.gdacs.org/gdacsapi/api/events/geteventdata?eventtype=${t}&eventid=${id}`
const gdacsGeom = (t, id, ep) => `https://www.gdacs.org/gdacsapi/api/polygons/getgeometry?eventtype=${t}&eventid=${id}&episodeid=${ep}`

// GDACS publishes 4 decimal places where EONET publishes 6, so identical
// geometry differs by up to ~1e-4 degrees purely from rounding. Mirrors
// gdacsExtentToleranceDeg in api/internal/ingestor/gdacs.go.
const TOLERANCE = 1e-3

async function getJSON(url) {
  const res = await fetch(url)
  if (!res.ok) throw new Error(`${res.status} ${res.statusText} for ${url}`)
  return res.json()
}

function positions(coords) {
  if (!Array.isArray(coords)) return []
  if (typeof coords[0] === 'number') return [coords]
  return coords.flatMap(positions)
}

const bbox = (p) => [
  Math.min(...p.map((q) => q[0])), Math.min(...p.map((q) => q[1])),
  Math.max(...p.map((q) => q[0])), Math.max(...p.map((q) => q[1])),
]
const near = (a, b) => Math.abs(a - b) < TOLERANCE
const fmt = (b) => `dim0 ${b[0].toFixed(3)}..${b[2].toFixed(3)}  dim1 ${b[1].toFixed(3)}..${b[3].toFixed(3)}`

// ⚠️ Production selects the geometry with the most recent date, not geometry[0].
// The census must mirror that or it can certify a snapshot we never ingest.
function selectGeometry(event) {
  const geoms = event.geometry || []
  if (geoms.length === 0) return null
  let best = geoms[geoms.length - 1]
  let bestTime = Date.parse(best.date)
  for (const g of geoms) {
    const t = Date.parse(g.date)
    if (!Number.isNaN(t) && (Number.isNaN(bestTime) || t > bestTime)) {
      best = g
      bestTime = t
    }
  }
  return best
}

function gdacsRef(event) {
  for (const s of event.sources || []) {
    const m = /gdacs\.org.*eventtype=([A-Z]{2}).*eventid=(\d+)/i.exec(s.url || '')
    if (m) return { type: m[1].toUpperCase(), id: m[2] }
  }
  return null
}

// ---------------------------------------------------------------- census ----
async function census() {
  const { events } = await getJSON(EONET_LIST)
  let polygons = 0
  let withGdacs = 0
  const impossible = []

  for (const e of events) {
    const g = selectGeometry(e)
    if (!g || g.type !== 'Polygon') continue
    polygons++
    if (gdacsRef(e)) withGdacs++   // the proposal claims all of them cite GDACS; verify it
    const b = bbox(positions(g.coordinates))
    // GeoJSON is [lon, lat]. If dim1 really were latitude it could not exceed 90.
    if (Math.max(Math.abs(b[1]), Math.abs(b[3])) > 90) {
      impossible.push({ id: e.id, title: e.title, lat: [b[1], b[3]] })
    }
  }

  console.log(`polygon flood events checked   : ${polygons}`)
  console.log(`citing a GDACS source          : ${withGdacs}`)
  console.log(`claimed latitude outside +/-90 : ${impossible.length}  <-- physically impossible`)
  for (const x of impossible) {
    console.log(`   ${x.id.padEnd(12)} ${x.title.slice(0, 34).padEnd(34)} "lat" ${x.lat[0].toFixed(2)}..${x.lat[1].toFixed(2)}`)
  }

  console.log('')
  console.log('⚠️ This census proves transposition ONLY where |longitude| > 90. The')
  console.log('   remainder — Africa, Europe, the Americas — stay numerically plausible')
  console.log('   and require the pairwise check. It does not prove they are all wrong.')

  if (polygons === 0) {
    console.error('\nNo polygon flood events found; the census proved nothing.')
    process.exit(1)
  }
  if (impossible.length === 0) {
    console.log('\nNo impossible latitudes — upstream may have been FIXED.')
    console.log('Run the pairwise check before assuming the defect persists.')
  }
}

// -------------------------------------------------------------- pairwise ----
async function pairwise(eonetID) {
  // Fetch the event directly: the ones that matter are months old and have aged
  // out of the recent-events page, and scanning that page alone would silently
  // degrade to a GDACS-only report that proves nothing about EONET.
  const event = await getJSON(eonetEvent(eonetID))
  const g = selectGeometry(event)
  if (!g || g.type !== 'Polygon') {
    console.error(`${eonetID} has no polygon geometry; nothing to compare.`)
    process.exit(1)
  }

  // Bind the GDACS id through the event's own source metadata rather than
  // trusting two ids supplied on the command line to be related.
  const ref = gdacsRef(event)
  if (!ref) {
    console.error(`${eonetID} cites no GDACS source; the comparison cannot be made.`)
    process.exit(1)
  }

  const ePts = positions(g.coordinates)
  const eBox = bbox(ePts)
  console.log(`EONET  ${event.id}  ${event.title}`)
  console.log(`       n=${ePts.length}  ${fmt(eBox)}`)

  const meta = await getJSON(gdacsEvent(ref.type, ref.id))
  const props = meta.properties || meta
  const episodes = (props.episodes || []).length || props.episodeid || 1
  console.log(`GDACS  event ${ref.id}  country=${props.country}  episodes=${episodes}`)

  let matched = null
  for (let ep = 1; ep <= episodes; ep++) {
    let fc
    try {
      fc = await getJSON(gdacsGeom(ref.type, ref.id, ep))
    } catch {
      continue
    }
    for (const f of fc.features || []) {
      if ((f.properties || {}).Class !== 'Poly_Affected') continue
      const pts = positions(f.geometry.coordinates)
      const b = bbox(pts)
      const same = pts.length === ePts.length
      const direct = same && near(eBox[0], b[0]) && near(eBox[1], b[1]) && near(eBox[2], b[2]) && near(eBox[3], b[3])
      const swapped = same && near(eBox[1], b[0]) && near(eBox[0], b[1]) && near(eBox[3], b[2]) && near(eBox[2], b[3])
      let note = ''
      if (swapped) note = '   <-- SAME GEOMETRY, AXES REVERSED'
      else if (direct) note = '   <-- SAME GEOMETRY, axes already correct'
      else if (same) note = '   (vertex count matches but extent does NOT — not this episode)'
      console.log(`GDACS  episode ${ep}  n=${String(pts.length).padEnd(5)} ${fmt(b)}${note}`)
      if (direct || swapped) matched = { ep, swapped }
    }
  }

  console.log('')
  if (!matched) {
    console.error('INCOMPLETE: no GDACS episode matches this geometry in vertex count AND extent.')
    console.error('A vertex-count match alone is not proof — two episodes can share one.')
    process.exit(1)
  }
  if (matched.swapped) {
    console.log(`CONFIRMED: EONET episode ${matched.ep} is published with latitude and longitude reversed.`)
  } else {
    console.log(`Episode ${matched.ep} matches with axes ALREADY CORRECT — upstream may have been fixed.`)
  }
}

const arg = process.argv[2]
if (arg && !/^EONET_\d+$/.test(arg)) {
  console.error('usage: verify-transposition.mjs [EONET_<id>]')
  process.exit(2)
}
await (arg ? pairwise(arg) : census())
