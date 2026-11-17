// Counts EONET flood events for Africa BY THE COUNTRY NAMED IN THE EVENT TITLE,
// per month, per country and per UN-geoscheme region.
//
// Why this exists next to measure.mjs:
//   measure.mjs places each event by its first coordinate and tests it against a
//   bounding box. For floods that is unreliable on two counts.
//   1. The Africa box is a rectangle that also covers southern Europe and the
//      Middle East, and floods there were counted as African. (In an uncommitted
//      reviewer check on 2026-11-17, within the proposal's own window this was
//      the larger cause of the overcount, about 44 events versus 27 for
//      transposition; that check is not reproducible from a committed script.)
//   2. Flood geometry changed shape in the feed. In the windows this script is
//      normally run for, floods are Point geometry until late June 2026 and
//      Polygon after (the geometry summary this script prints gives the counts
//      and the first Polygon date). measure.mjs reads a polygon's first position
//      as [lon, lat], but polygons were delivered with latitude and longitude
//      transposed for a period (an uncommitted reviewer check on 2026-11-17 put
//      the switch to the correct order at about 2026-09-25; points were always
//      correctly ordered). Production resolves each polygon against GDACS (see
//      openspec/changes/archive/*fix-eonet-polygon-transposition*); a counting
//      script cannot.
//   Every EONET flood in the window is GDACS-sourced and titled
//   "Flood in <country>[, <country>] <id>", so the title is the one location
//   field that does not depend on coordinate order.
//
// What it does NOT measure:
//   - Stored rows. Production polls status=open plus status=closed&days=30 per
//     country box; this counts feed availability over the whole window.
//   - Wildfires. They are points, so measure.mjs's coordinates are sound for them
//     (but its box still admits non-African events, and counts for a closed window
//     drift upward as the feed back-fills; see measure.mjs).
//   - Sub-national location. A title says which country, not which state.
//
// Output, in order: window header and sources, count of titles that are not
// "Flood in ...", earliest flood in the window, earliest African flood, a
// per-month table (Africa, Nigeria, Ghana), by-country counts, by-region counts
// (a multi-country title counts once per region named), the November-March
// count, the ingested-country count, the floods multiplier, and a geometry
// summary (Point versus Polygon floods worldwide, date of the first Polygon).
// Counts for a closed historical window can drift upward as the feed
// back-fills, so every figure is a dated snapshot.
//
// Run:  node scripts/eonet-density/floods-by-title.mjs [--start YYYY-MM-DD] [--end YYYY-MM-DD]

const usage = 'usage: node floods-by-title.mjs [--start YYYY-MM-DD] [--end YYYY-MM-DD]'
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
    // Round-trip so an impossible date such as 2026-02-30 is rejected too.
    const parsed = DATE.test(v) ? new Date(`${v}T00:00:00Z`) : null
    if (!parsed || Number.isNaN(parsed.getTime()) || parsed.toISOString().slice(0, 10) !== v) fail(`${a} must be YYYY-MM-DD, got "${v}"`)
    opts[a.slice(2)] = v
  }
  if (opts.end < opts.start) fail(`--end ${opts.end} is earlier than --start ${opts.start}`)
}
const START = opts.start
const END = opts.end
const LIMIT = 20000

// The countries production ingests today (DefaultCountries in
// api/internal/ingestor/eonet.go) -- keep in sync by hand.
const INGESTED = ['Nigeria', 'Ghana']

// Names as GDACS writes them in titles, plus common variants. Matching is by
// substring, so "Guinea" also covers Guinea-Bissau and Equatorial Guinea, and
// "Congo" covers both Congos -- all African, so the total is unaffected.
// Nigeria and South Sudan also match through the substrings "Niger" and "Sudan",
// which is harmless for the Africa total; the Nigeria/Ghana columns do not use
// this list, they use their own full-name match (INGESTED).
const AFRICAN = [
  'Algeria', 'Angola', 'Benin', 'Botswana', 'Burkina Faso', 'Burundi', 'Cabo Verde', 'Cape Verde',
  'Cameroon', 'Central African Republic', 'Chad', 'Comoros', 'Congo', 'Cote d', 'Côte d', 'Ivory Coast',
  'Djibouti', 'Egypt', 'Equatorial Guinea', 'Eritrea', 'Eswatini', 'Swaziland', 'Ethiopia', 'Gabon',
  'Gambia', 'Ghana', 'Guinea', 'Kenya', 'Lesotho', 'Liberia', 'Libya', 'Madagascar', 'Malawi', 'Mali',
  'Mauritania', 'Mauritius', 'Morocco', 'Mozambique', 'Namibia', 'Niger', 'Rwanda', 'Sao Tome', 'São Tomé',
  'Réunion', 'Reunion', 'Mayotte', 'Saint Helena',
  'Senegal', 'Seychelles', 'Sierra Leone', 'Somalia', 'South Africa', 'Sudan', 'Tanzania', 'Togo',
  'Tunisia', 'Uganda', 'Zambia', 'Zimbabwe', 'Western Sahara',
]
// The only non-African name in GDACS's vocabulary that contains one of the above.
// Applied per title part (parts are separated by ", "), so a multi-country title
// that names Papua New Guinea AND an African country still counts as African.
const NOT_AFRICAN = /Papua New Guinea/
const isAfricanPart = (part) => !NOT_AFRICAN.test(part) && AFRICAN.some((name) => part.includes(name))
const titleParts = (title) => title.replace(/^Flood in /, '').replace(/ \d+$/, '').split(', ')
const isAfrican = (title) => titleParts(title).some(isAfricanPart)

// UN geoscheme sub-regions (Northern, Western, Middle, Eastern, Southern Africa),
// labelled North / West / Central / East-Horn / Southern. Order matters: the first
// matching pattern wins, so specific names precede the substrings they contain.
// The UN scheme puts Sudan in North and South Sudan in East; Mauritius, Comoros,
// Madagascar, Mayotte, Reunion and Seychelles are in East.
const REGIONS = [
  ['East/Horn', /South Sudan|Burundi|Comoros|Djibouti|Eritrea|Ethiopia|Kenya|Madagascar|Malawi|Mauritius|Mayotte|Mozambique|R[ée]union|Rwanda|Seychelles|Somalia|Tanzania|Uganda|Zambia|Zimbabwe/],
  ['North', /Algeria|Egypt|Libya|Morocco|Sudan|Tunisia|Western Sahara/],
  ['Central', /Equatorial Guinea|Angola|Cameroon|Central African Republic|Chad|Congo|Gabon|Sao Tom[eé]|São Tomé/],
  ['West', /Guinea|Benin|Burkina Faso|Cabo Verde|Cape Verde|Cote d|Côte d|Ivory Coast|Gambia|Ghana|Liberia|Mali|Mauritania|Nigeria|Niger|Saint Helena|Senegal|Sierra Leone|Togo/],
  ['Southern', /Botswana|Eswatini|Swaziland|Lesotho|Namibia|South Africa/],
]
const regionOf = (part) => REGIONS.find(([, re]) => re.test(part))?.[0] ?? 'Unmapped'

// No bbox on purpose: a server-side box filter would be applied to the same
// unreliable coordinates this script exists to avoid.
const url =
  `https://eonet.gsfc.nasa.gov/api/v3/events?status=all&start=${START}&end=${END}` +
  `&category=floods&limit=${LIMIT}`
const res = await fetch(url, { headers: { 'User-Agent': 'VigilAfrica-density/1.0' } })
if (!res.ok) throw new Error(`HTTP ${res.status} for ${url}`)
const body = await res.json()
if (!Array.isArray(body?.events)) throw new Error(`response has no "events" array for ${url}`)
const events = body.events
if (events.length >= LIMIT) {
  console.warn(`WARNING: ${events.length} events returned, at or above the limit of ${LIMIT}; the result may be truncated`)
}

const rows = events.map((ev) => ({
  title: String(ev.title).replaceAll('&#039;', "'"),
  date: String(ev.geometry?.[0]?.date ?? '').slice(0, 10),
  sources: (ev.sources ?? []).map((s) => s.id).join('+'),
  geomType: ev.geometry?.[0]?.type ?? 'none',
}))

// The method rests on every flood carrying a "Flood in <country>" title. If the
// feed ever stops doing that, say so loudly instead of silently undercounting.
const offPattern = rows.filter((r) => !/^Flood in /.test(r.title))
const bySource = rows.reduce((m, r) => ((m[r.sources] = (m[r.sources] || 0) + 1), m), {})

const african = rows.filter((r) => isAfrican(r.title))
const mentions = (list, name) => list.filter((r) => r.title.includes(name)).length
const ingested = african.filter((r) => INGESTED.some((name) => r.title.includes(name)))

console.log(`=== EONET floods by title, ${START} -> ${END} ===`)
console.log(`  floods worldwide            : ${rows.length}   sources: ${JSON.stringify(bySource)}`)
console.log(`  titles not "Flood in ..."   : ${offPattern.length}${offPattern.length ? '   <-- METHOD ASSUMPTION BROKEN, do not trust the counts' : ''}`)
console.log(`  earliest flood in the window: ${rows.map((r) => r.date).filter(Boolean).sort()[0] ?? 'none'}`)
console.log(`  earliest African flood      : ${african.map((r) => r.date).filter(Boolean).sort()[0] ?? 'none'}`)

console.log(`\n  month     Africa   ${INGESTED.map((n) => n.padStart(7)).join('  ')}`)
// Every month from the first African flood to END, so an empty month prints as
// 0 instead of vanishing from the table.
const firstMonth = african.map((r) => r.date.slice(0, 7)).filter(Boolean).sort()[0]
const months = []
for (let [y, m] = (firstMonth ?? END.slice(0, 7)).split('-').map(Number); `${y}-${String(m).padStart(2, '0')}` <= END.slice(0, 7); m === 12 ? (y++, (m = 1)) : m++) {
  months.push(`${y}-${String(m).padStart(2, '0')}`)
}
for (const m of months) {
  const inMonth = african.filter((r) => r.date.startsWith(m))
  console.log(`  ${m}   ${String(inMonth.length).padStart(5)}   ${INGESTED.map((n) => String(mentions(inMonth, n)).padStart(7)).join('  ')}`)
}
console.log(`  total     ${String(african.length).padStart(5)}   ${INGESTED.map((n) => String(mentions(african, n)).padStart(7)).join('  ')}`)

// A flood titled with two countries counts once per country here, so this
// column can sum to more than the Africa total above.
const byCountry = {}
for (const r of african) {
  for (const part of titleParts(r.title)) {
    if (isAfricanPart(part)) byCountry[part] = (byCountry[part] || 0) + 1
  }
}
const ranked = Object.entries(byCountry).sort((a, b) => b[1] - a[1])
console.log(`\n  by country (${ranked.length} countries with at least one flood):`)
console.log('  ' + ranked.map(([name, n]) => `${name} ${n}`).join(' | '))

// A title naming countries in two regions counts once in each, so this line can
// sum to more than the Africa total.
const byRegion = {}
for (const r of african) {
  const regions = new Set(titleParts(r.title).filter(isAfricanPart).map(regionOf))
  for (const reg of regions) byRegion[reg] = (byRegion[reg] || 0) + 1
}
console.log('\n  by region (UN geoscheme; a multi-country title counts once per region named):')
console.log('  ' + Object.entries(byRegion).sort((a, b) => b[1] - a[1]).map(([name, n]) => `${name} ${n}`).join(' | '))

// November-March is the West African dry season: the months in which a
// Nigeria+Ghana flood view is expected to be empty.
const dry = african.filter((r) => ['11', '12', '01', '02', '03'].includes(r.date.slice(5, 7)))
console.log(`\n  dated November-March        : ${dry.length} of ${african.length}`)
console.log(`  ingested today (${INGESTED.join('+')}) : ${ingested.length}`)
console.log(`  floods multiplier           : ${ingested.length ? (african.length / ingested.length).toFixed(1) + 'x' : 'n/a (no ingested-country floods in window)'}`)

const points = rows.filter((r) => r.geomType === 'Point').length
const polygons = rows.filter((r) => r.geomType === 'Polygon')
const otherGeom = rows.length - points - polygons.length
console.log(`\n  geometry (worldwide floods in window): Point ${points} | Polygon ${polygons.length}${otherGeom ? ` | other ${otherGeom}` : ''}`)
console.log(`  first Polygon flood         : ${polygons.map((r) => r.date).filter(Boolean).sort()[0] ?? 'none'}`)
