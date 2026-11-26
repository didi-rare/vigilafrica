// Browser evidence for the "you are here" marker (feature-current-location-marker).
//
// jsdom mocks MapLibre, so the unit tests prove the marker's lifecycle and
// accessible name but nothing about where it is drawn, what is painted on top of
// what, whether the pulse really stops under reduced motion, or its contrast on
// the real basemap. This script answers those in a real browser, against a
// served build, with every API response mocked:
//
//   1. position  — the marker's centre is the map's centre once the camera has
//                  flown to the context location (MapLibre's own projection);
//   2. stacking  — the marker paints above a hazard pin at the same spot, an open
//                  popup paints above the marker, and a click on the spot still
//                  reaches the hazard pin (the marker is pointer-events: none);
//   3. motion    — the computed pulse animation is named under no-preference and
//                  `none` under reduce, while the static halo stays visible;
//   4. contrast  — the disc and its ring against the basemap pixels actually
//                  adjacent to them, plus the background-independent floor;
//   5. absence   — with `location: null` there is no marker and the status
//                  line is empty;
//   6. geometry  — dashboard and map-container boxes, to compare against a
//                  control build (the status line must reserve no layout);
//   7. placement — the status line on the visible map, clear of MapLibre's
//                  attribution, at five viewports;
//   8. two pre-existing defects, informational (see preExisting below).
//
// /v1/context cannot resolve a location on localhost, which is why it is mocked
// with a Lagos fixture here. Basemap tiles are real (ESRI World Imagery), so the
// contrast samples need network access; the script reports how many tiles loaded.
//
// Usage (see README.md). Needs a WebGL2-capable browser: on the Windows dev host
// that is the installed Chrome, PW_CHANNEL=chrome.
//   TARGET_URL=http://localhost:4273/ OUT_DIR=/tmp/marker PW_CHANNEL=chrome \
//     node scripts/bench-dashboard-cls/check-user-marker.mjs
//   # control arm (a build without the feature): EXPECT=none
//
// Exits non-zero when an expectation fails or a measurement could not be made.
import { mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { chromium } from 'playwright'

const TARGET_URL = process.env.TARGET_URL ?? 'http://localhost:4273/'
const OUT_DIR = process.env.OUT_DIR ?? 'user-marker-evidence'
const CHANNEL = process.env.PW_CHANNEL || undefined
// `marker`: the branch, the feature must be present. `none`: a control build;
// only geometry is recorded and the marker must be absent.
const EXPECT = process.env.EXPECT ?? 'marker'
const VIEWPORT = { width: 1350, height: 940 }
const LABEL_PREFIX = 'Your approximate location'

const LAGOS = { country: 'Nigeria', state: 'Lagos', lat: 6.5244, lng: 3.3792 }
// Map.tsx flies to the context location at a hard-coded zoom 7.
const FLY_ZOOM = 7
// Put the single hazard this many pixels south of the user, so its pin's badge
// (the 32px circle 20-52px above its point) covers the user marker's centre and
// its popup, which opens above the pin, covers it too.
const HAZARD_OFFSET_PX = 45

// Web Mercator with MapLibre's 512px tiles.
const worldSize = 512 * 2 ** FLY_ZOOM
const latToY = (lat) => {
  const phi = (lat * Math.PI) / 180
  return ((1 - Math.log(Math.tan(Math.PI / 4 + phi / 2)) / Math.PI) / 2) * worldSize
}
const yToLat = (y) => (Math.atan(Math.sinh(Math.PI * (1 - (2 * y) / worldSize))) * 180) / Math.PI
const HAZARD = { lat: yToLat(latToY(LAGOS.lat) + HAZARD_OFFSET_PX), lng: LAGOS.lng }

const eventsPayload = {
  data: [{
    id: '1f0a0000-0000-4000-8000-000000000001',
    source_id: 'EONET_MARKER_1', source: 'eonet',
    title: 'Flooding south of Lagos',
    category: 'floods', status: 'open', geometry_type: 'Point',
    latitude: HAZARD.lat, longitude: HAZARD.lng,
    country_name: 'Nigeria', state_name: 'Lagos',
    event_date: '2026-11-20T12:00:00Z', source_url: 'https://eonet.gsfc.nasa.gov/x',
    ingested_at: '2026-11-20T12:05:00Z', enriched_at: null,
  }],
  meta: { total: 1, limit: 50, offset: 0 },
}

const problems = []
const fail = (msg) => { problems.push(msg); console.log(`  FAIL ${msg}`) }
const ok = (msg) => console.log(`  ok   ${msg}`)

// WCAG 2.x relative luminance and contrast ratio.
function luminance([r, g, b]) {
  const lin = (c) => { const s = c / 255; return s <= 0.04045 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4 }
  return 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b)
}
const contrast = (a, b) => {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x)
  return (hi + 0.05) / (lo + 0.05)
}
const parseRgb = (css) => {
  const m = /rgba?\(([\d.]+),\s*([\d.]+),\s*([\d.]+)/.exec(css)
  if (!m) throw new Error(`not an rgb() colour: ${css}`)
  return [Number(m[1]), Number(m[2]), Number(m[3])]
}

async function openDashboard(browser, { reducedMotion, location }) {
  const page = await browser.newPage({ viewport: VIEWPORT, reducedMotion })
  let tiles = 0
  page.on('response', (r) => { if (r.url().includes('World_Imagery') && r.ok()) tiles++ })
  await page.route('**/health', (r) => r.fulfill({ json: {
    status: 'ok', version: 'bench',
    last_ingestion: {
      status: 'success', started_at: '2026-11-26T10:00:00Z', completed_at: new Date().toISOString(),
      events_fetched: 1, events_stored: 1, error: null,
    },
  } }))
  await page.route('**/v1/context', (r) => r.fulfill({ json: {
    location, location_source: location ? 'ip' : 'unavailable', nearby_events: [],
  } }))
  await page.route('**/v1/states**', (r) => r.fulfill({ json: { states: ['Lagos'] } }))
  await page.route('**/v1/events**', (r) => r.fulfill({ json: eventsPayload }))
  await page.goto(TARGET_URL, { waitUntil: 'domcontentloaded' })
  await page.waitForSelector('.maplibregl-canvas', { timeout: 30000 })
  await page.locator('.dashboard-map-container').scrollIntoViewIfNeeded()
  return { page, tileCount: () => tiles }
}

// Wait until the camera has stopped: a box stable across three polls.
async function waitForStill(page, selector) {
  let last = null
  let stable = 0
  for (let i = 0; i < 80 && stable < 3; i++) {
    await page.waitForTimeout(300)
    const box = await page.locator(selector).first().boundingBox()
    const key = box ? `${box.x.toFixed(1)},${box.y.toFixed(1)}` : 'none'
    stable = key === last ? stable + 1 : 0
    last = key
  }
  return stable >= 3
}

async function geometry(page) {
  return page.evaluate(() => {
    const r = (sel) => {
      const el = document.querySelector(sel)
      if (!el) return null
      const b = el.getBoundingClientRect()
      return { x: Math.round(b.x), y: Math.round(b.y + window.scrollY), w: Math.round(b.width), h: Math.round(b.height) }
    }
    return {
      dashboard: r('#dashboard'),
      layout: r('.dashboard-layout'),
      mapContainer: r('.dashboard-map-container'),
      mapWrapper: r('.map-wrapper'),
    }
  })
}

async function topmostAt(page, x, y) {
  return page.evaluate(([px, py]) => {
    const el = document.elementFromPoint(px, py)
    if (!el) return 'nothing'
    if (el.closest('.map-user-marker')) return 'user-marker'
    if (el.closest('.maplibregl-popup')) return 'popup'
    if (el.closest('.map-marker')) return 'hazard-marker'
    return `${el.tagName.toLowerCase()}.${String(el.className).split(' ')[0]}`
  }, [x, y])
}

// Pixels of a viewport screenshot, decoded in the page (no image library needed).
async function screenshotPixels(page, clip) {
  const png = await page.screenshot({ clip })
  return page.evaluate(async (b64) => {
    const img = new Image()
    img.src = `data:image/png;base64,${b64}`
    await img.decode()
    const c = document.createElement('canvas')
    c.width = img.width; c.height = img.height
    const ctx = c.getContext('2d')
    ctx.drawImage(img, 0, 0)
    return { w: img.width, h: img.height, data: Array.from(ctx.getImageData(0, 0, img.width, img.height).data) }
  }, png.toString('base64'))
}

async function branchLagos(browser, reducedMotion) {
  console.log(`\n[lagos context, prefers-reduced-motion: ${reducedMotion}]`)
  const { page, tileCount } = await openDashboard(browser, { reducedMotion, location: LAGOS })
  const userSel = `[role="img"][aria-label^="${LABEL_PREFIX}"]`
  try {
    await page.waitForSelector(userSel, { timeout: 15000 })
  } catch {
    fail('user marker never appeared')
    await page.close()
    return
  }
  if (!(await waitForStill(page, userSel))) fail('camera never settled')

  const count = await page.locator(userSel).count()
  count === 1 ? ok('exactly one user marker') : fail(`expected 1 user marker, found ${count}`)

  const info = await page.evaluate((sel) => {
    const el = document.querySelector(sel)
    const dot = el.querySelector('.map-user-marker__dot')
    const halo = el.querySelector('.map-user-marker__halo')
    const pulse = el.querySelector('.map-user-marker__pulse')
    const hazard = document.querySelector('.map-marker')
    const map = document.querySelector('.map-container').getBoundingClientRect()
    const box = el.getBoundingClientRect()
    const d = getComputedStyle(dot)
    return {
      tag: el.tagName, label: el.getAttribute('aria-label'), tabindex: el.getAttribute('tabindex'),
      position: getComputedStyle(el).position,
      zIndex: getComputedStyle(el).zIndex,
      pointerEvents: getComputedStyle(el).pointerEvents,
      hazardPosition: hazard ? getComputedStyle(hazard).position : null,
      popupZ: null,
      centre: { x: box.x + box.width / 2, y: box.y + box.height / 2 },
      size: { w: box.width, h: box.height },
      mapCentre: { x: map.x + map.width / 2, y: map.y + map.height / 2 },
      dotFill: d.backgroundColor, dotRing: d.borderTopColor, dotSize: dot.getBoundingClientRect().width,
      haloBorderStyle: getComputedStyle(halo).borderTopStyle,
      haloOpacity: getComputedStyle(halo).opacity,
      haloDisplay: getComputedStyle(halo).display,
      pulseAnimation: getComputedStyle(pulse).animationName,
      chip: document.querySelector('.dashboard-location-status')?.textContent ?? null,
    }
  }, userSel)

  console.log(`  marker <${info.tag.toLowerCase()}> label="${info.label}" position=${info.position} z-index=${info.zIndex} pointer-events=${info.pointerEvents} size=${info.size.w}x${info.size.h}`)
  console.log(`  hazard .map-marker computed position=${info.hazardPosition}`)
  info.tag !== 'BUTTON' && info.tabindex === null
    ? ok('marker is not a button and not focusable') : fail(`marker is interactive (<${info.tag}> tabindex=${info.tabindex})`)
  info.position === 'absolute' ? ok('marker keeps position: absolute') : fail(`marker position is ${info.position}`)
  info.pointerEvents === 'none' ? ok('marker is pointer-events: none') : fail(`pointer-events ${info.pointerEvents}`)

  const dx = info.centre.x - info.mapCentre.x
  const dy = info.centre.y - info.mapCentre.y
  console.log(`  marker centre (${info.centre.x.toFixed(1)}, ${info.centre.y.toFixed(1)}) vs map centre (${info.mapCentre.x.toFixed(1)}, ${info.mapCentre.y.toFixed(1)}): d=(${dx.toFixed(1)}, ${dy.toFixed(1)})px`)
  Math.abs(dx) <= 1.5 && Math.abs(dy) <= 1.5
    ? ok('marker sits on the context location (map centre after flyTo)') : fail('marker is not at the map centre')

  const chipOk = /^Your approximate location: Lagos, Nigeria \(based on your network\)$/.test(info.chip ?? '')
  chipOk
    ? ok(`status line: "${info.chip}"`) : fail(`status line reads ${JSON.stringify(info.chip)}`)

  console.log(`  pulse animation-name=${info.pulseAnimation}  halo border=${info.haloBorderStyle} opacity=${info.haloOpacity} display=${info.haloDisplay}`)
  if (reducedMotion === 'reduce') {
    info.pulseAnimation === 'none' ? ok('pulse stopped under reduce') : fail(`pulse still animating under reduce (${info.pulseAnimation})`)
  } else {
    info.pulseAnimation === 'map-user-marker-pulse' ? ok('pulse runs under no-preference (check is not vacuous)') : fail(`pulse not running under no-preference (${info.pulseAnimation})`)
  }
  info.haloDisplay !== 'none' && Number(info.haloOpacity) > 0 && info.haloBorderStyle === 'dashed'
    ? ok('static dashed halo visible') : fail('halo not visible')

  const cx = info.centre.x
  const cy = info.centre.y
  const tag = reducedMotion === 'reduce' ? 'reduce' : 'motion'

  // ── Contrast ───────────────────────────────────────────────────────────────
  const fill = parseRgb(info.dotFill)
  const ring = parseRgb(info.dotRing)
  const floor = Math.sqrt((luminance(fill) + 0.05) / (luminance(ring) + 0.05))
  console.log(`  disc ${info.dotFill} / ring ${info.dotRing}: disc-vs-ring ${contrast(fill, ring).toFixed(2)}:1; floor over ANY background = ${floor.toFixed(2)}:1`)
  floor >= 3 ? ok('every background meets 3:1 against the disc or its ring') : fail(`floor ${floor.toFixed(2)} < 3`)

  // Colours actually adjacent to the dot: hide the dot and the pulse, keep the
  // halo (it composites over the basemap right next to the ring), and sample
  // the annulus just outside the dot's edge. Twice: over the basemap alone
  // (hazard pins hidden), and over the hazard pin this fixture puts under the
  // marker, which is the overlap case.
  if (tag === 'reduce') {
    const setVis = (v, alsoHazards) => page.evaluate(([sel, vis, hz]) => {
      const el = document.querySelector(sel)
      el.querySelector('.map-user-marker__dot').style.visibility = vis
      el.querySelector('.map-user-marker__pulse').style.visibility = vis
      if (hz) document.querySelectorAll('.map-marker').forEach((m) => { m.style.visibility = vis })
    }, [userSel, v, alsoHazards])
    const r = info.dotSize / 2
    const half = Math.ceil(r + 5)
    const clip = { x: Math.round(cx - half), y: Math.round(cy - half), width: half * 2, height: half * 2 }
    for (const [what, hideHazards] of [['basemap only', true], ['over the hazard pin', false]]) {
      await setVis('hidden', hideHazards)
      const px = await screenshotPixels(page, clip)
      await setVis('', true)
      let n = 0; let minRing = Infinity; let minFill = Infinity; let minBest = Infinity; let maxL = 0; let minL = 1
      for (let y = 0; y < px.h; y++) {
        for (let x = 0; x < px.w; x++) {
          const dist = Math.hypot(x + 0.5 - (cx - clip.x), y + 0.5 - (cy - clip.y))
          if (dist < r + 1 || dist > r + 4) continue
          const i = (y * px.w + x) * 4
          const bg = [px.data[i], px.data[i + 1], px.data[i + 2]]
          const cr = contrast(ring, bg); const cf = contrast(fill, bg)
          minRing = Math.min(minRing, cr); minFill = Math.min(minFill, cf); minBest = Math.min(minBest, Math.max(cr, cf))
          maxL = Math.max(maxL, luminance(bg)); minL = Math.min(minL, luminance(bg)); n++
        }
      }
      console.log(`  ${what}: annulus ${n} px (tiles loaded=${tileCount()}), bg luminance ${minL.toFixed(3)}-${maxL.toFixed(3)}; min ring-vs-bg ${minRing.toFixed(2)}:1, min disc-vs-bg ${minFill.toFixed(2)}:1, min best-of-two ${minBest.toFixed(2)}:1`)
      if (n === 0 || tileCount() === 0) fail(`${what}: no sample (tiles did not load?)`)
      else minBest >= 3 ? ok(`${what}: every adjacent pixel is >= 3:1 against the disc or the ring`) : fail(`${what}: a pixel at ${minBest.toFixed(2)}:1`)
    }
  }

  // ── Screenshots ────────────────────────────────────────────────────────────
  await page.locator('.map-wrapper').screenshot({ path: join(OUT_DIR, `map-lagos-${tag}.png`) })
  await page.screenshot({ path: join(OUT_DIR, `marker-crop-${tag}.png`), clip: { x: Math.round(cx - 90), y: Math.round(cy - 120), width: 180, height: 180 } })
  await page.locator('.dashboard-location-status').screenshot({ path: join(OUT_DIR, `status-line-${tag}.png`) })

  // ── Stacking ───────────────────────────────────────────────────────────────
  const setPointer = (value) => page.evaluate(([sel, v]) => {
    const el = document.querySelector(sel)
    for (const n of [el, ...el.querySelectorAll('*')]) n.style.pointerEvents = v
  }, [userSel, value])

  const hazardBox = await page.locator('.map-marker').first().boundingBox()
  console.log(`  hazard pin box ${hazardBox ? `${hazardBox.x.toFixed(0)},${hazardBox.y.toFixed(0)} ${hazardBox.width}x${hazardBox.height}` : 'MISSING'}`)
  if (!hazardBox) { fail('hazard marker missing'); await page.close(); return }

  await setPointer('auto')
  const paintedTop = await topmostAt(page, cx, cy)
  await setPointer('')
  const hitTop = await topmostAt(page, cx, cy)
  console.log(`  at the marker centre: painted on top = ${paintedTop}; receives the click = ${hitTop}`)
  paintedTop === 'user-marker' ? ok('user marker paints above the hazard pin') : fail(`painted on top: ${paintedTop}`)
  hitTop === 'hazard-marker' ? ok('the click passes through to the hazard pin') : fail(`click target: ${hitTop}`)

  await page.mouse.click(cx, cy)
  try {
    await page.waitForSelector('.maplibregl-popup', { timeout: 5000 })
    ok('clicking the marker spot opened the hazard popup')
  } catch {
    fail('no popup after clicking the marker spot')
    await page.close()
    return
  }
  await page.waitForTimeout(400)
  const popupZ = await page.evaluate(() => getComputedStyle(document.querySelector('.maplibregl-popup')).zIndex)
  const content = await page.locator('.maplibregl-popup-content').boundingBox()
  const inside = content && cx >= content.x && cx <= content.x + content.width && cy >= content.y && cy <= content.y + content.height
  if (!inside) {
    fail(`popup content ${JSON.stringify(content)} does not cover the marker centre; cannot probe`)
  } else {
    await setPointer('auto')
    const overPopup = await topmostAt(page, cx, cy)
    await setPointer('')
    console.log(`  popup z-index=${popupZ}; with the popup open, painted on top at the marker centre = ${overPopup}`)
    overPopup === 'popup' ? ok('an open popup paints above the user marker') : fail(`painted on top with popup open: ${overPopup}`)
  }
  await page.screenshot({ path: join(OUT_DIR, `popup-over-marker-${tag}.png`), clip: { x: Math.round(cx - 140), y: Math.round(cy - 150), width: 280, height: 220 } })

  console.log(`  geometry ${JSON.stringify(await geometry(page))}`)
  await page.close()
}

async function nullContext(browser) {
  console.log('\n[null context]')
  const { page } = await openDashboard(browser, { reducedMotion: 'no-preference', location: null })
  await page.waitForSelector('.map-marker', { timeout: 15000 }).catch(() => {})
  await page.waitForTimeout(1500)
  const n = await page.locator(`[aria-label^="${LABEL_PREFIX}"]`).count()
  const status = await page.evaluate(() => {
    const el = document.querySelector('.dashboard-location-status')
    return el ? { text: el.textContent, role: el.getAttribute('role') } : null
  })
  n === 0 ? ok('no user marker') : fail(`${n} user marker(s) with a null location`)
  if (EXPECT === 'marker') {
    status && status.role === 'status' && status.text === ''
      ? ok('status region mounted and empty') : fail(`status region ${JSON.stringify(status)}`)
  }
  await page.locator('.map-wrapper').screenshot({ path: join(OUT_DIR, `map-null-${EXPECT}.png`) })
  console.log(`  geometry ${JSON.stringify(await geometry(page))}`)
  await page.close()
}

async function controlLagos(browser) {
  console.log('\n[lagos context, control build]')
  const { page } = await openDashboard(browser, { reducedMotion: 'no-preference', location: LAGOS })
  await page.waitForSelector('.map-marker', { timeout: 15000 }).catch(() => {})
  await waitForStill(page, '.map-marker')
  const n = await page.locator(`[aria-label^="${LABEL_PREFIX}"]`).count()
  n === 0 ? ok('control has no user marker (feature absent, as expected)') : fail(`control shows ${n} user marker(s)`)
  await page.locator('.map-wrapper').screenshot({ path: join(OUT_DIR, 'map-lagos-control.png') })
  console.log(`  geometry ${JSON.stringify(await geometry(page))}`)
  await page.close()
}

// The status line is positioned from outside the map against --map-height, so
// check it lands on the visible map, clear of MapLibre's attribution (which the
// tile licences require to stay readable), at desktop, tablet and phone widths.
const PLACEMENT_VIEWPORTS = [[1350, 940], [1024, 768], [900, 800], [768, 1024], [375, 812]]

async function statusPlacement(browser) {
  console.log('\n[status line placement]')
  for (const [width, height] of PLACEMENT_VIEWPORTS) {
    const page = await browser.newPage({ viewport: { width, height } })
    await page.route('**/health', (r) => r.fulfill({ json: { status: 'ok', version: 'bench', last_ingestion: null } }))
    await page.route('**/v1/context', (r) => r.fulfill({ json: { location: LAGOS, location_source: 'ip', nearby_events: [] } }))
    await page.route('**/v1/states**', (r) => r.fulfill({ json: { states: ['Lagos'] } }))
    await page.route('**/v1/events**', (r) => r.fulfill({ json: { data: [], meta: { total: 0, limit: 50, offset: 0 } } }))
    await page.goto(TARGET_URL, { waitUntil: 'domcontentloaded' })
    await page.waitForSelector('.map-user-marker', { timeout: 30000 }).catch(() => {})
    await page.locator('.dashboard-map-container').scrollIntoViewIfNeeded()
    await page.waitForSelector('.dashboard-location-status__chip', { timeout: 10000 }).catch(() => {})
    await page.waitForTimeout(500)
    const g = await page.evaluate(() => {
      const r = (s) => document.querySelector(s)?.getBoundingClientRect() ?? null
      const pick = (b) => b && { left: b.left, top: b.top, right: b.right, bottom: b.bottom }
      return {
        chip: pick(r('.dashboard-location-status__chip')),
        map: pick(r('.map-wrapper')),
        container: pick(r('.dashboard-map-container')),
        attrib: pick(r('.maplibregl-ctrl-attrib')),
      }
    })
    const label = `${width}x${height}`
    if (!g.chip || !g.map || !g.container) {
      fail(`${label}: chip/map/container missing ${JSON.stringify(g)}`)
      await page.close()
      continue
    }
    const visibleBottom = Math.min(g.map.bottom, g.container.bottom)
    const onMap = g.chip.left >= g.map.left && g.chip.right <= g.map.right &&
      g.chip.top >= g.map.top && g.chip.bottom <= visibleBottom
    const overlapsAttrib = g.attrib !== null &&
      g.chip.left < g.attrib.right && g.chip.right > g.attrib.left &&
      g.chip.top < g.attrib.bottom && g.chip.bottom > g.attrib.top
    console.log(`  ${label.padEnd(9)} chip y ${g.chip.top.toFixed(0)}-${g.chip.bottom.toFixed(0)} x ${g.chip.left.toFixed(0)}-${g.chip.right.toFixed(0)}; visible map bottom ${visibleBottom.toFixed(0)}; attribution ${g.attrib ? `y ${g.attrib.top.toFixed(0)}-${g.attrib.bottom.toFixed(0)} x ${g.attrib.left.toFixed(0)}-${g.attrib.right.toFixed(0)}` : 'absent'}`)
    onMap ? ok(`${label}: status line inside the visible map`) : fail(`${label}: status line outside the visible map`)
    !overlapsAttrib ? ok(`${label}: clear of the attribution`) : fail(`${label}: covers the attribution`)
    await page.locator('.dashboard-map-container').screenshot({ path: join(OUT_DIR, `layout-${width}.png`) })
    await page.close()
  }
}

// Two pre-existing defects this harness turned up, measured on every run so the
// numbers quoted in the change record stay reproducible. Informational only:
// neither belongs to this feature, so neither fails the run.
//   a. Hazard pins after the first are displaced: `.map-marker` sets
//      `position: relative`, overriding MapLibre's `absolute`, so pins are laid
//      out in normal flow. Two unclustered pins on one row; the second's
//      bottom-centre is compared with its Web Mercator projection.
//   b. A dashboard re-render (here, a page change) flies the camera back to the
//      context location, discarding the visitor's pan. Uses a hazard pin at the
//      context point as the camera probe, so it also runs on a control build.
async function preExisting(browser) {
  console.log('\n[pre-existing, informational]')
  const xAt = (lng) => ((lng + 180) / 360) * worldSize
  const pin = (id, lng, title) => ({
    ...eventsPayload.data[0], id, source_id: id, title, latitude: LAGOS.lat, longitude: lng,
  })
  const rows = [pin('a', LAGOS.lng, 'Pin A'), pin('b', LAGOS.lng + 1.5, 'Pin B')]
  const page = await browser.newPage({ viewport: VIEWPORT })
  await page.route('**/health', (r) => r.fulfill({ json: { status: 'ok', version: 'bench', last_ingestion: null } }))
  await page.route('**/v1/context', (r) => r.fulfill({ json: { location: LAGOS, location_source: 'ip', nearby_events: [] } }))
  await page.route('**/v1/states**', (r) => r.fulfill({ json: { states: [] } }))
  await page.route('**/v1/events**', (r) => {
    const offset = Number(new URL(r.request().url()).searchParams.get('offset') ?? 0)
    return r.fulfill({ json: { data: rows, meta: { total: 120, limit: 50, offset } } })
  })
  await page.goto(TARGET_URL, { waitUntil: 'domcontentloaded' })
  await page.waitForSelector('.map-marker', { timeout: 30000 }).catch(() => {})
  await page.locator('.dashboard-map-container').scrollIntoViewIfNeeded()
  await waitForStill(page, '.map-marker')
  const pins = () => page.evaluate(() => {
    const c = document.querySelector('.map-container').getBoundingClientRect()
    return {
      centre: { x: c.x + c.width / 2, y: c.y + c.height / 2 },
      pins: [...document.querySelectorAll('.map-marker')].map((m) => {
        const b = m.getBoundingClientRect()
        return { label: m.getAttribute('aria-label'), position: getComputedStyle(m).position, x: b.x + b.width / 2, y: b.y + b.height }
      }),
    }
  })
  const before = await pins()
  for (const p of before.pins) {
    const lng = p.label.startsWith('Pin A') ? LAGOS.lng : LAGOS.lng + 1.5
    const ex = before.centre.x + (xAt(lng) - xAt(LAGOS.lng))
    console.log(`  a. ${p.label}: position=${p.position}, drawn ${(p.x - ex).toFixed(1)}px right and ${(p.y - before.centre.y).toFixed(1)}px below its point`)
  }
  const pinA = () => pins().then((g) => {
    const a = g.pins.find((p) => p.label.startsWith('Pin A'))
    return a ? [Math.round(a.x - g.centre.x), Math.round(a.y - g.centre.y)] : null
  })
  const box = await page.locator('.map-container').boundingBox()
  await page.mouse.move(box.x + box.width / 2 + 60, box.y + box.height / 2 + 60)
  await page.mouse.down()
  await page.mouse.move(box.x + box.width / 2 - 140, box.y + box.height / 2 - 90, { steps: 10 })
  await page.mouse.up()
  await page.waitForTimeout(1500)
  const dragged = await pinA()
  await page.getByRole('button', { name: 'Next page of events' }).click()
  await page.waitForTimeout(5000)
  const paged = await pinA()
  console.log(`  b. context-point pin offset from the map centre: after a drag ${JSON.stringify(dragged)}, after "Next page" ${JSON.stringify(paged)}`)
  await page.close()
}

mkdirSync(OUT_DIR, { recursive: true })
console.log(`target ${TARGET_URL}  expect=${EXPECT}  channel=${CHANNEL ?? '(bundled)'}  viewport ${VIEWPORT.width}x${VIEWPORT.height}`)
console.log(`hazard fixture at lat ${HAZARD.lat.toFixed(5)} (${HAZARD_OFFSET_PX}px south of the user at zoom ${FLY_ZOOM})`)
const browser = await chromium.launch({ channel: CHANNEL })
try {
  if (EXPECT === 'marker') {
    await branchLagos(browser, 'no-preference')
    await branchLagos(browser, 'reduce')
    await statusPlacement(browser)
  } else {
    await controlLagos(browser)
  }
  await nullContext(browser)
  await preExisting(browser)
} finally {
  await browser.close()
}

writeFileSync(join(OUT_DIR, `result-${EXPECT}.txt`), problems.length ? problems.join('\n') : 'PASS')
if (problems.length) {
  console.log(`\nFAIL (${problems.length})`)
  for (const p of problems) console.log(`  - ${p}`)
  process.exit(1)
}
console.log('\nPASS')
