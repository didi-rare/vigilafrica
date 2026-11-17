// Counts EONET flood events for Africa BY THE COUNTRY NAMED IN THE EVENT TITLE,
// per month and per country.
//
// Why this exists next to measure.mjs:
//   measure.mjs places each event by its first coordinate and tests it against a
//   bounding box. For floods that is unreliable on two counts.
//   1. Flood polygons arrive with latitude and longitude transposed for SOME
//      events and not others (see openspec/changes/archive/*fix-eonet-polygon-
//      transposition*). On 2026-11-17 the September 2026 polygons were transposed
//      and the late-October and November ones were not, so neither "read as
//      delivered" nor "always swap" places them all correctly. Production
//      resolves each polygon against GDACS; a counting script cannot.
//   2. The Africa box is a rectangle that also covers southern Europe and the
//      Middle East, and floods there were being counted as African.
//   Every EONET flood in the window is GDACS-sourced and titled
//   "Flood in <country>[, <country>] <id>", so the title is the one location
//   field that does not depend on coordinate order.
//
// What it does NOT measure:
//   - Stored rows. Production polls status=open plus status=closed&days=30 per
//     country box; this counts feed availability over the whole window.
//   - Wildfires. They are points, so measure.mjs's coordinates are sound for them.
//   - Sub-national location. A title says which country, not which state.
//
// Run:  node scripts/eonet-density/floods-by-title.mjs [--start YYYY-MM-DD] [--end YYYY-MM-DD]

const arg = (name, fallback) => {
  const i = process.argv.indexOf(`--${name}`)
  return i > -1 && process.argv[i + 1] ? process.argv[i + 1] : fallback
}
const START = arg('start', '2024-10-01')
const END = arg('end', new Date().toISOString().slice(0, 10))

// The countries production ingests today (DefaultCountries in
// api/internal/ingestor/eonet.go) -- keep in sync by hand.
const INGESTED = ['Nigeria', 'Ghana']

// Names as GDACS writes them in titles, plus common variants. Matching is by
// substring, so "Guinea" also covers Guinea-Bissau and Equatorial Guinea, and
// "Congo" covers both Congos -- all African, so the total is unaffected.
const AFRICAN = [
  'Algeria', 'Angola', 'Benin', 'Botswana', 'Burkina Faso', 'Burundi', 'Cabo Verde', 'Cape Verde',
  'Cameroon', 'Central African Republic', 'Chad', 'Comoros', 'Congo', 'Cote d', 'Côte d', 'Ivory Coast',
  'Djibouti', 'Egypt', 'Equatorial Guinea', 'Eritrea', 'Eswatini', 'Swaziland', 'Ethiopia', 'Gabon',
  'Gambia', 'Ghana', 'Guinea', 'Kenya', 'Lesotho', 'Liberia', 'Libya', 'Madagascar', 'Malawi', 'Mali',
  'Mauritania', 'Mauritius', 'Morocco', 'Mozambique', 'Namibia', 'Niger', 'Rwanda', 'Sao Tome',
  'Senegal', 'Seychelles', 'Sierra Leone', 'Somalia', 'South Africa', 'Sudan', 'Tanzania', 'Togo',
  'Tunisia', 'Uganda', 'Zambia', 'Zimbabwe', 'Western Sahara',
]
// The only non-African name in GDACS's vocabulary that contains one of the above.
const NOT_AFRICAN = /Papua New Guinea/
const isAfrican = (text) => !NOT_AFRICAN.test(text) && AFRICAN.some((name) => text.includes(name))

// No bbox on purpose: a server-side box filter would be applied to the same
// unreliable coordinates this script exists to avoid.
const url =
  `https://eonet.gsfc.nasa.gov/api/v3/events?status=all&start=${START}&end=${END}` +
  `&category=floods&limit=20000`
const res = await fetch(url, { headers: { 'User-Agent': 'VigilAfrica-density/1.0' } })
if (!res.ok) throw new Error(`HTTP ${res.status} for ${url}`)
const events = (await res.json()).events ?? []

const rows = events.map((ev) => ({
  title: String(ev.title).replaceAll('&#039;', "'"),
  date: String(ev.geometry?.[0]?.date ?? '').slice(0, 10),
  sources: (ev.sources ?? []).map((s) => s.id).join('+'),
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
console.log(`  earliest flood in the feed  : ${rows.map((r) => r.date).filter(Boolean).sort()[0] ?? 'none'}`)
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
  for (const part of r.title.replace(/^Flood in /, '').replace(/ \d+$/, '').split(', ')) {
    if (isAfrican(part)) byCountry[part] = (byCountry[part] || 0) + 1
  }
}
const ranked = Object.entries(byCountry).sort((a, b) => b[1] - a[1])
console.log(`\n  by country (${ranked.length} countries with at least one flood):`)
console.log('  ' + ranked.map(([name, n]) => `${name} ${n}`).join(' | '))

// November-March is the West African dry season: the months in which a
// Nigeria+Ghana flood view is expected to be empty.
const dry = african.filter((r) => ['11', '12', '01', '02', '03'].includes(r.date.slice(5, 7)))
console.log(`\n  dated November-March        : ${dry.length} of ${african.length}`)
console.log(`  ingested today (${INGESTED.join('+')}) : ${ingested.length}`)
console.log(`  floods multiplier           : ${ingested.length ? (african.length / ingested.length).toFixed(1) + 'x' : 'n/a (no ingested-country floods in window)'}`)
