// Does `prefers-reduced-motion: reduce` actually stop the animations that moved
// onto pseudo-elements?
//
// #191 moved the `.signal-dot` pulse onto `::after` and nearly shipped with the
// reduced-motion rule still pointing at the element — reduced-motion users would
// have kept the pulse. chore-web-audit-leftovers does the same move for the
// staging banner's glow, so this reads the COMPUTED animation on each pseudo-
// element under both media states rather than trusting the selector.
//
// Exits non-zero when a check cannot be made (an element is missing — e.g. the
// staging banner, because a production build was served by mistake) or when
// an animation that must be `none` under `reduce` is not.
//
// Usage (run against a STAGING build, which is the only one with the banner):
//   TARGET_URL=http://localhost:4175/ node scripts/bench-dashboard-cls/check-reduced-motion.mjs
import { chromium } from 'playwright'

const TARGET_URL = process.env.TARGET_URL ?? 'http://localhost:4175/'

// `mustStop`: under `reduce`, the computed animation-name has to be `none` —
// an explicit rule, not the catch-all's 1µs single iteration.
const CHECKS = [
  { selector: '.staging-banner', pseudo: '::before', label: 'staging stripe', mustStop: false },
  { selector: '.staging-banner', pseudo: '::after',  label: 'staging glow',   mustStop: true },
  { selector: '.signal-dot',     pseudo: '::after',  label: 'signal ring',    mustStop: true },
  { selector: '.dashboard-fallback__progress', pseudo: '::after', label: 'load-progress segment', mustStop: true },
  { selector: '.loading-state__spinner', pseudo: null, label: 'spinner', mustStop: false },
]

const problems = []
const browser = await chromium.launch()
try {
  for (const reducedMotion of ['no-preference', 'reduce']) {
    const page = await browser.newPage({ viewport: { width: 1350, height: 940 }, reducedMotion })
    await page.route('**/v1/**', r => r.fulfill({ json: { data: [], meta: { total: 0, limit: 50, offset: 0 }, states: [], location: null, nearby_events: [] } }))
    await page.route('**/health', r => r.fulfill({ json: { status: 'ok', version: 'bench', last_ingestion: null } }))
    // Hold the chunk so the fallback (progress bar + spinner) is present.
    await page.route('**/assets/EventsDashboard-*.js', () => new Promise(() => {}))
    await page.goto(TARGET_URL, { waitUntil: 'domcontentloaded' })
    await page.waitForSelector('.dashboard-fallback', { timeout: 15000 })

    const rows = await page.evaluate((checks) => checks.map(({ selector, pseudo, label, mustStop }) => {
      const el = document.querySelector(selector)
      if (!el) return { label, present: false, mustStop }
      const cs = getComputedStyle(el, pseudo)
      return {
        label, present: true, mustStop,
        animationName: cs.animationName,
        iterationCount: cs.animationIterationCount,
        duration: cs.animationDuration,
        opacity: cs.opacity,
        transform: cs.transform,
        width: cs.width,
      }
    }), CHECKS)

    console.log(`\nprefers-reduced-motion: ${reducedMotion}`)
    for (const r of rows) {
      if (!r.present) {
        console.log(`  ${r.label.padEnd(24)} MISSING — is this a staging build with the chunk held?`)
        problems.push(`${r.label}: element not on page (${reducedMotion})`)
        continue
      }
      console.log(`  ${r.label.padEnd(24)} animation=${r.animationName} x${r.iterationCount} ${r.duration}  opacity=${r.opacity}  transform=${r.transform}  width=${r.width}`)
      if (reducedMotion === 'reduce' && r.mustStop && r.animationName !== 'none') {
        problems.push(`${r.label}: still animating under reduce (animation-name=${r.animationName})`)
      }
      if (reducedMotion === 'no-preference' && r.mustStop && r.animationName === 'none') {
        problems.push(`${r.label}: not animating under no-preference — the check is vacuous`)
      }
    }
    await page.close()
  }
} finally {
  await browser.close()
}

if (problems.length > 0) {
  console.log('\nFAIL')
  for (const p of problems) console.log(`  - ${p}`)
  process.exitCode = 1
} else {
  console.log('\nOK — every animation that must stop under reduce reads animation-name: none')
}
