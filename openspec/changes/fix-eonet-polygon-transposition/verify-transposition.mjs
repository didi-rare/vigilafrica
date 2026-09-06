#!/usr/bin/env node
// Re-derives the evidence in proposal.md from live sources. Every number in that
// document comes from this script; re-run it rather than trusting the write-up.
//
//   node verify-transposition.mjs            # the +/-90 census across EONET floods
//   node verify-transposition.mjs 1104078    # one event, EONET vs GDACS side by side
//
// Why a census AND a pairwise check: the census proves transposition without any
// geographic knowledge (a latitude beyond +/-90 is impossible), but it can only
// prove it where |longitude| > 90. The pairwise check covers the rest by going to
// GDACS, which is upstream of EONET and therefore the origin rather than a second
// opinion.

const EONET_EVENTS = 'https://eonet.gsfc.nasa.gov/api/v3/events?category=floods&status=all&limit=40'
const eonetEvent = (id) => `https://eonet.gsfc.nasa.gov/api/v3/events/${id}`
const gdacsData = (id) => `https://www.gdacs.org/gdacsapi/api/events/geteventdata?eventtype=FL&eventid=${id}`
const gdacsGeom = (id, ep) => `https://www.gdacs.org/gdacsapi/api/polygons/getgeometry?eventtype=FL&eventid=${id}&episodeid=${ep}`

async function getJSON(url) {
  const res = await fetch(url)
  if (!res.ok) throw new Error(`${res.status} ${res.statusText} for ${url}`)
  return res.json()
}

// Flatten arbitrarily nested GeoJSON coordinate arrays down to [a, b] pairs.
function points(coords) {
  if (!Array.isArray(coords)) return []
  if (typeof coords[0] === 'number') return [coords]
  return coords.flatMap(points)
}

function range(pts) {
  const a = pts.map((p) => p[0])
  const b = pts.map((p) => p[1])
  return { n: pts.length, a: [Math.min(...a), Math.max(...a)], b: [Math.min(...b), Math.max(...b)] }
}

const fmt = (r) => `n=${String(r.n).padEnd(5)} dim0 ${r.a[0].toFixed(3)}..${r.a[1].toFixed(3)}  dim1 ${r.b[0].toFixed(3)}..${r.b[1].toFixed(3)}`

// ---------------------------------------------------------------- census ----
async function census() {
  const { events } = await getJSON(EONET_EVENTS)
  let polygons = 0
  const impossible = []

  for (const e of events) {
    const g = (e.geometry || [])[0]
    if (!g || g.type !== 'Polygon') continue
    polygons++
    const r = range(points(g.coordinates))
    // GeoJSON is [lon, lat]. If dim1 is really latitude it cannot exceed +/-90.
    if (Math.max(Math.abs(r.b[0]), Math.abs(r.b[1])) > 90) {
      impossible.push({ id: e.id, title: e.title, lat: r.b })
    }
  }

  console.log(`polygon flood events checked   : ${polygons}`)
  console.log(`claimed latitude outside +/-90 : ${impossible.length}  <-- physically impossible`)
  for (const x of impossible) {
    console.log(`   ${x.id.padEnd(12)} ${x.title.slice(0, 34).padEnd(34)} "lat" ${x.lat[0].toFixed(2)}..${x.lat[1].toFixed(2)}`)
  }
  if (!impossible.length) {
    console.log('\nNo impossible latitudes found — upstream may have been FIXED.')
    console.log('Re-check the pairwise comparison before assuming the defect persists.')
  }
}

// -------------------------------------------------------------- pairwise ----
async function pairwise(gdacsId, eonetId) {
  let match = null

  // Prefer a direct lookup. The events we care about are months old and have
  // aged out of the recent-events page, so scanning that page alone silently
  // degrades to a GDACS-only report — which proves nothing about EONET.
  if (eonetId) {
    match = await getJSON(eonetEvent(eonetId))
  } else {
    const all = await getJSON(EONET_EVENTS)
    for (const e of all.events) {
      if ((e.sources || []).some((s) => (s.url || '').includes(`eventid=${gdacsId}`))) match = e
    }
    if (!match) {
      console.log(`(${gdacsId} is not in the recent EONET page — pass its EONET id as a second`)
      console.log(` argument to compare both sides, e.g. "${gdacsId} EONET_22248")\n`)
    }
  }

  if (match) {
    const g = (match.geometry || [])[0]
    console.log(`EONET  ${match.id}  ${match.title}`)
    console.log(`       ${fmt(range(points(g.coordinates)))}   type=${g.type}`)
  }

  const data = await getJSON(gdacsData(gdacsId))
  const props = data.properties || data
  console.log(`GDACS  country=${props.country} iso3=${props.iso3}  centroid=${JSON.stringify((data.geometry || {}).coordinates)}`)

  // Episode matters: EONET does not always mirror episode 1. Match on vertex
  // count, which is what identified episode 2 as the right one for 1104105.
  const want = match ? points(((match.geometry || [])[0] || {}).coordinates).length : null
  for (const ep of [1, 2, 3, 4]) {
    let feats
    try {
      feats = (await getJSON(gdacsGeom(gdacsId, ep))).features || []
    } catch {
      continue
    }
    for (const f of feats) {
      if ((f.properties || {}).Class !== 'Poly_Affected') continue
      const r = range(points(f.geometry.coordinates))
      const flag = want && r.n === want ? '   <-- SAME VERTEX COUNT as EONET' : ''
      console.log(`GDACS  episode ${ep}  ${fmt(r)}${flag}`)
    }
  }
  console.log('\nIf the two dim ranges are swapped, the coordinates are transposed.')
}

const [gdacsArg, eonetArg] = process.argv.slice(2)
await (gdacsArg ? pairwise(gdacsArg, eonetArg) : census())
