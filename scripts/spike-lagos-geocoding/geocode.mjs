// Days 4-5 geocoding harness for spike-lagos-report-geocoding.
// Two INDEPENDENT gazetteers so accuracy is not self-certified by one source:
//   A. Nominatim  -> OpenStreetMap data
//   B. Open-Meteo -> GeoNames data (no API key)
// Each retrieved locality is queried twice: with Lagos context (the realistic
// pipeline) and bare (the ambiguity / harm diagnostic).

const UA = 'VigilAfrica-spike/1.0 (didi.pepple@gmail.com)'
const sleep = (ms) => new Promise((r) => setTimeout(r, ms))

// The 17 RETRIEVED locality mentions from the Days 2-3 findings.
// `name` is the query string; `event` ties it back to the frozen ground truth.
const MENTIONS = [
  { name: 'Ijede', event: '2025-08-03' },
  { name: 'Oko Ope', event: '2025-08-03' },
  { name: 'Anjorin', event: '2025-08-03' },
  { name: 'Abule Eko', event: '2025-08-03' },
  { name: 'Odetedo', event: '2025-08-03' },
  { name: 'Ikota', event: '2025-09-25' },
  { name: 'Kusenla Road', event: '2025-09-25' },
  { name: 'Murtala Muhammed International Airport', event: '2026-06-28' },
  { name: 'Lekki', event: '2026-06-30' },
  { name: 'Okota', event: '2026-06-30' },
  { name: 'Oshodi', event: '2026-06-30' },
  { name: 'Okokomaiko', event: '2026-06-30' },
  { name: 'Ipaja', event: '2026-06-30' },
  { name: 'Ikoyi', event: '2026-07-14' },
  { name: 'Lekki', event: '2026-07-14' },
  { name: 'Victoria Island', event: '2026-07-14' },
  { name: 'Oworonshoki', event: '2026-07-14' },
]

// Lagos bounding box, used only to classify results, never to constrain queries.
const LAGOS = { minLat: 6.35, maxLat: 6.75, minLon: 2.65, maxLon: 4.4 }
const inLagos = (lat, lon) =>
  lat >= LAGOS.minLat && lat <= LAGOS.maxLat && lon >= LAGOS.minLon && lon <= LAGOS.maxLon

function haversineKm(a, b) {
  const R = 6371
  const toRad = (d) => (d * Math.PI) / 180
  const dLat = toRad(b.lat - a.lat)
  const dLon = toRad(b.lon - a.lon)
  const x =
    Math.sin(dLat / 2) ** 2 +
    Math.cos(toRad(a.lat)) * Math.cos(toRad(b.lat)) * Math.sin(dLon / 2) ** 2
  return 2 * R * Math.asin(Math.sqrt(x))
}

async function getJson(url) {
  const res = await fetch(url, { headers: { 'User-Agent': UA, Accept: 'application/json' } })
  if (!res.ok) return { error: `HTTP ${res.status}` }
  return res.json()
}

async function nominatim(query) {
  const url =
    'https://nominatim.openstreetmap.org/search?format=jsonv2&addressdetails=1&limit=1&q=' +
    encodeURIComponent(query)
  const j = await getJson(url)
  if (j.error) return { error: j.error }
  if (!Array.isArray(j) || j.length === 0) return { found: false }
  const h = j[0]
  const a = h.address || {}
  return {
    found: true,
    lat: +h.lat,
    lon: +h.lon,
    display: h.display_name,
    country: a.country,
    state: a.state,
    // OSM tags Nigerian LGAs inconsistently; capture every plausible carrier.
    lga: a.county || a.city_district || a.municipality || a.suburb || a.town || null,
    type: `${h.category}/${h.type}`,
  }
}

async function openMeteo(name) {
  const url =
    'https://geocoding-api.open-meteo.com/v1/search?count=10&language=en&format=json&name=' +
    encodeURIComponent(name)
  const j = await getJson(url)
  if (j.error) return { error: j.error }
  const all = j.results || []
  if (all.length === 0) return { found: false, nCandidates: 0 }
  // Realistic pipeline: prefer a Nigerian hit, as the article states the country.
  const ng = all.filter((r) => r.country_code === 'NG')
  const pick = ng[0] || all[0]
  return {
    found: true,
    lat: pick.latitude,
    lon: pick.longitude,
    country: pick.country,
    state: pick.admin1,
    lga: pick.admin2 || null,
    nCandidates: all.length,
    nNigerian: ng.length,
    // What a naive pipeline with no country filter would have picked:
    naiveTop: `${all[0].name}, ${all[0].admin1 || '-'}, ${all[0].country}`,
    naiveIsNigeria: all[0].country_code === 'NG',
  }
}

const rows = []
for (const m of MENTIONS) {
  const contextQuery = `${m.name}, Lagos, Nigeria`
  const nom = await nominatim(contextQuery)
  await sleep(1100) // Nominatim usage policy: <= 1 request/second
  const om = await openMeteo(m.name)
  await sleep(300)

  let agreeKm = null
  if (nom.found && om.found) agreeKm = haversineKm(nom, om)

  rows.push({
    name: m.name,
    event: m.event,
    nom_found: nom.found === true,
    nom_lat: nom.lat ?? null,
    nom_lon: nom.lon ?? null,
    nom_inLagos: nom.found ? inLagos(nom.lat, nom.lon) : null,
    nom_lga: nom.lga,
    nom_state: nom.state,
    nom_type: nom.type,
    nom_display: nom.display,
    nom_error: nom.error || null,
    om_found: om.found === true,
    om_inLagos: om.found ? inLagos(om.lat, om.lon) : null,
    om_state: om.state,
    om_lga: om.lga,
    om_nNigerian: om.nNigerian ?? 0,
    om_naiveTop: om.naiveTop ?? null,
    om_naiveIsNigeria: om.naiveIsNigeria ?? null,
    agreeKm: agreeKm === null ? null : +agreeKm.toFixed(2),
  })
  console.log(
    `${m.name.padEnd(38)} OSM:${(rows.at(-1).nom_found ? (rows.at(-1).nom_inLagos ? 'lagos' : 'OUTSIDE') : 'none').padEnd(8)}` +
      ` GN:${(rows.at(-1).om_found ? (rows.at(-1).om_inLagos ? 'lagos' : 'OUTSIDE') : 'none').padEnd(8)}` +
      ` sep:${agreeKm === null ? '-' : agreeKm.toFixed(1) + 'km'}`,
  )
}

const fs = await import('node:fs')
fs.writeFileSync(new URL('./geocode-results.json', import.meta.url), JSON.stringify(rows, null, 2))
console.log('\nwrote geocode-results.json')
